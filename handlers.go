package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Основной обработчик обновлений
func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.Message != nil {
		b.handleMessage(update.Message)
	} else if update.EditedMessage != nil {
		b.handleMessage(update.EditedMessage)
	}
}

// Обработчик сообщений
func (b *Bot) handleMessage(message *tgbotapi.Message) {
	userID := message.From.ID

	// Проверяем, является ли пользователь администратором
	if userID == b.AdminID {
		b.handleAdminMessage(message)
		return
	}

	// Обработка пользовательских сообщений
	b.handleUserMessage(message)
}

// Обработчик сообщений администратора
func (b *Bot) handleAdminMessage(message *tgbotapi.Message) {
	userID := message.From.ID

	if message.IsCommand() {
		switch message.Command() {
		case "create":
			b.handleCreateCommand(userID)
		case "start":
			b.sendMessage(userID, "Добро пожаловать, администратор! 👑\nИспользуйте /create для создания нового кэша.")
		default:
			b.sendMessage(userID, "Неизвестная команда. Доступные команды:\n/create - создать новый кэш")
		}
		return
	}

	// Проверяем, есть ли активная админская сессия
	session, err := b.DB.GetAdminSession(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			b.sendMessage(userID, "Введите команду /create для создания нового кэша.")
			return
		}
		log.Printf("Ошибка получения админской сессии: %v", err)
		return
	}

	switch session.Step {
	case "waiting_code":
		b.handleCodeWordInput(userID, message.Text)
	case "waiting_location":
		b.handleLocationInput(userID, message)
	case "waiting_photo":
		b.handlePhotoInput(userID, message)
	}
}

// Обработчик команды /create
func (b *Bot) handleCreateCommand(userID int64) {
	session := &AdminSession{
		UserID: userID,
		Step:   "waiting_code",
	}

	err := b.DB.CreateOrUpdateAdminSession(session)
	if err != nil {
		log.Printf("Ошибка создания админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте еще раз.")
		return
	}

	b.sendMessage(userID, "🔑 Придумайте кодовое слово для нового кэша:")
}

// Обработчик ввода кодового слова
func (b *Bot) handleCodeWordInput(userID int64, codeWord string) {
	codeWord = strings.TrimSpace(codeWord)
	if len(codeWord) < 3 {
		b.sendMessage(userID, "Кодовое слово должно содержать минимум 3 символа. Попробуйте еще раз:")
		return
	}

	// Проверяем, не существует ли уже такое кодовое слово
	_, err := b.DB.GetCacheByCodeWord(codeWord)
	if err == nil {
		b.sendMessage(userID, "Кодовое слово уже существует! Придумайте другое:")
		return
	}

	// Обновляем сессию
	session := &AdminSession{
		UserID:   userID,
		Step:     "waiting_location",
		CodeWord: codeWord,
	}

	err = b.DB.CreateOrUpdateAdminSession(session)
	if err != nil {
		log.Printf("Ошибка обновления админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте еще раз.")
		return
	}

	msg := tgbotapi.NewMessage(userID, "📍 Отлично! Теперь отправьте геолокацию места, где будет спрятан кэш.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("📍 Отправить геолокацию"),
		),
	)
	msg.ReplyMarkup.OneTimeKeyboard = true

	b.API.Send(msg)
}

// Обработчик ввода геолокации
func (b *Bot) handleLocationInput(userID int64, message *tgbotapi.Message) {
	if message.Location == nil {
		b.sendMessage(userID, "Пожалуйста, отправьте геолокацию, используя кнопку ниже.")
		return
	}

	latitude := float64(message.Location.Latitude)
	longitude := float64(message.Location.Longitude)

	// Обновляем сессию
	session, err := b.DB.GetAdminSession(userID)
	if err != nil {
		log.Printf("Ошибка получения админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте команду /create заново.")
		return
	}

	session.Step = "waiting_photo"
	session.Latitude = latitude
	session.Longitude = longitude

	err = b.DB.CreateOrUpdateAdminSession(session)
	if err != nil {
		log.Printf("Ошибка обновления админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте еще раз.")
		return
	}

	// Убираем клавиатуру
	msg := tgbotapi.NewMessage(userID, "📷 Теперь отправьте фотографию этого места:")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.API.Send(msg)
}

// Обработчик ввода фотографии
func (b *Bot) handlePhotoInput(userID int64, message *tgbotapi.Message) {
	if message.Photo == nil || len(message.Photo) == 0 {
		b.sendMessage(userID, "Пожалуйста, отправьте фотографию.")
		return
	}

	// Получаем сессию
	session, err := b.DB.GetAdminSession(userID)
	if err != nil {
		log.Printf("Ошибка получения админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте команду /create заново.")
		return
	}

	// Получаем файл с наибольшим разрешением
	photo := message.Photo[len(message.Photo)-1]
	fileConfig := tgbotapi.FileConfig{FileID: photo.FileID}
	file, err := b.API.GetFile(fileConfig)
	if err != nil {
		log.Printf("Ошибка получения файла: %v", err)
		b.sendMessage(userID, "Ошибка при загрузке фотографии. Попробуйте еще раз.")
		return
	}

	// Скачиваем и сохраняем фотографию
	photoPath, err := b.downloadAndSavePhoto(file.FilePath, session.CodeWord)
	if err != nil {
		log.Printf("Ошибка сохранения фотографии: %v", err)
		b.sendMessage(userID, "Ошибка при сохранении фотографии. Попробуйте еще раз.")
		return
	}

	// Создаем запись в базе данных
	cache := &Cache{
		CodeWord:  session.CodeWord,
		Latitude:  session.Latitude,
		Longitude: session.Longitude,
		PhotoPath: photoPath,
		CreatedBy: userID,
	}

	err = b.DB.CreateCache(cache)
	if err != nil {
		log.Printf("Ошибка создания кэша: %v", err)
		b.sendMessage(userID, "Ошибка при создании кэша. Попробуйте еще раз.")
		return
	}

	// Удаляем сессию
	b.DB.DeleteAdminSession(userID)

	successMsg := fmt.Sprintf("✅ Кэш успешно создан!\n\n🔑 Кодовое слово: %s\n📍 Координаты: %.6f, %.6f\n\nТеперь пользователи могут найти этот кэш, введя кодовое слово.",
		cache.CodeWord, cache.Latitude, cache.Longitude)

	b.sendMessage(userID, successMsg)
}

// Обработчик сообщений пользователей
func (b *Bot) handleUserMessage(message *tgbotapi.Message) {
	userID := message.From.ID

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			b.sendMessage(userID, "🗺️ Добро пожаловать в GeoCaching Bot!\n\nВведите кодовое слово для поиска кэша:")
		case "stop":
			b.handleStopCommand(userID)
		default:
			b.sendMessage(userID, "🤔 Неизвестная команда.\n\nВведите кодовое слово для поиска кэша или /stop для остановки поиска.")
		}
		return
	}

	// Проверяем, есть ли активная пользовательская сессия
	session, err := b.DB.GetUserSession(userID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Ошибка получения пользовательской сессии: %v", err)
		return
	}

	// Если есть активная сессия, обрабатываем обновления геолокации
	if session != nil {
		b.handleLocationUpdate(userID, message)
		return
	}

	// Если нет активной сессии, ищем кэш по кодовому слову
	b.handleCacheSearch(userID, message.Text)
}

// Обработчик поиска кэша по кодовому слову
func (b *Bot) handleCacheSearch(userID int64, codeWord string) {
	codeWord = strings.TrimSpace(codeWord)
	if len(codeWord) < 3 {
		b.sendMessage(userID, "Кодовое слово должно содержать минимум 3 символа.")
		return
	}

	// Ищем кэш в базе данных
	cache, err := b.DB.GetCacheByCodeWord(codeWord)
	if err != nil {
		if err == sql.ErrNoRows {
			b.sendMessage(userID, "🔍 Кэш с таким кодовым словом не найден.\n\nПроверьте правильность написания и попробуйте еще раз.")
		} else {
			log.Printf("Ошибка поиска кэша: %v", err)
			b.sendMessage(userID, "Произошла ошибка при поиске. Попробуйте еще раз.")
		}
		return
	}

	// Создаем пользовательскую сессию
	userSession := &UserSession{
		UserID:   userID,
		CacheID:  cache.ID,
		IsActive: true,
	}

	err = b.DB.CreateOrUpdateUserSession(userSession)
	if err != nil {
		log.Printf("Ошибка создания пользовательской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте еще раз.")
		return
	}

	// Запрашиваем доступ к геолокации
	msg := tgbotapi.NewMessage(userID, fmt.Sprintf("🎯 Кэш найден: %s\n\n📍 Для начала поиска поделитесь своей геолокацией в реальном времени на %d час(а).",
		cache.CodeWord, b.Config.LiveLocationDurationHours))

	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("📍 Поделиться геолокацией"),
		),
	)
	msg.ReplyMarkup.OneTimeKeyboard = true

	b.API.Send(msg)
}

// Обработчик обновлений геолокации
func (b *Bot) handleLocationUpdate(userID int64, message *tgbotapi.Message) {
	if message.Location == nil {
		b.sendMessage(userID, "Пожалуйста, отправьте геолокацию для продолжения поиска.\n\nИспользуйте /stop для остановки поиска.")
		return
	}

	// Получаем сессию пользователя
	session, err := b.DB.GetUserSession(userID)
	if err != nil {
		log.Printf("Ошибка получения пользовательской сессии: %v", err)
		return
	}

	// Получаем данные кэша
	cache, err := b.DB.GetCacheByCodeWord("")
	if err != nil {
		// Получаем кэш по ID из сессии
		query := `SELECT id, code_word, latitude, longitude, photo_path, created_at, created_by 
				  FROM caches WHERE id = ?`

		cache = &Cache{}
		err = b.DB.db.QueryRow(query, session.CacheID).Scan(
			&cache.ID, &cache.CodeWord, &cache.Latitude, &cache.Longitude,
			&cache.PhotoPath, &cache.CreatedAt, &cache.CreatedBy,
		)

		if err != nil {
			log.Printf("Ошибка получения кэша: %v", err)
			return
		}
	}

	userLat := float64(message.Location.Latitude)
	userLon := float64(message.Location.Longitude)

	// Проверяем, достиг ли пользователь цели
	if isTargetReached(userLat, userLon, cache.Latitude, cache.Longitude, b.Config.TargetDistanceMeters) {
		b.handleTargetReached(userID, cache)
		return
	}

	// Формируем сообщение с направлением
	directionMsg := fmt.Sprintf("🧭 Направление к кэшу:\n\n%s",
		formatDirectionMessage(userLat, userLon, cache.Latitude, cache.Longitude))

	// Если это первое сообщение, отправляем новое
	if session.LastMessageID == 0 {
		// Убираем клавиатуру
		msg := tgbotapi.NewMessage(userID, directionMsg)
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

		sentMsg, err := b.API.Send(msg)
		if err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
			return
		}

		// Обновляем сессию
		session.LastLatitude = userLat
		session.LastLongitude = userLon
		session.LastMessageID = sentMsg.MessageID
		b.DB.CreateOrUpdateUserSession(session)
	} else {
		// Редактируем существующее сообщение
		edit := tgbotapi.NewEditMessageText(userID, session.LastMessageID, directionMsg)
		_, err := b.API.Send(edit)
		if err != nil {
			log.Printf("Ошибка редактирования сообщения: %v", err)
		}

		// Обновляем сессию
		session.LastLatitude = userLat
		session.LastLongitude = userLon
		b.DB.CreateOrUpdateUserSession(session)
	}
}

// Обработчик достижения цели
func (b *Bot) handleTargetReached(userID int64, cache *Cache) {
	// Деактивируем сессию
	b.DB.DeactivateUserSession(userID)

	// Отправляем поздравительное сообщение
	congratsMsg := fmt.Sprintf("🎉 Поздравляем! Вы нашли кэш: %s\n\n📷 Вот фотография места:", cache.CodeWord)

	msg := tgbotapi.NewMessage(userID, congratsMsg)
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.API.Send(msg)

	// Отправляем фотографию
	photoMsg := tgbotapi.NewPhoto(userID, tgbotapi.FilePath(cache.PhotoPath))
	photoMsg.Caption = "🏆 Вы успешно завершили поиск кэша!"

	_, err := b.API.Send(photoMsg)
	if err != nil {
		log.Printf("Ошибка отправки фотографии: %v", err)
		b.sendMessage(userID, "К сожалению, не удалось загрузить фотографию места.")
	}
}

// Обработчик команды /stop
func (b *Bot) handleStopCommand(userID int64) {
	// Деактивируем пользовательскую сессию
	err := b.DB.DeactivateUserSession(userID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Ошибка деактивации сессии: %v", err)
	}

	msg := tgbotapi.NewMessage(userID, "🛑 Поиск кэша остановлен.\n\nВведите новое кодовое слово для начала поиска.")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.API.Send(msg)
}

// Вспомогательная функция для отправки текстовых сообщений
func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.API.Send(msg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

// Функция для скачивания и сохранения фотографии
func (b *Bot) downloadAndSavePhoto(filePath, codeWord string) (string, error) {
	// Получаем URL файла
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.API.Token, filePath)

	// Скачиваем файл
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Создаем уникальное имя файла
	fileName := fmt.Sprintf("%s_%d%s", codeWord, time.Now().Unix(), filepath.Ext(filePath))
	fullPath := filepath.Join(b.Config.PhotoStoragePath, fileName)

	// Создаем файл
	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Копируем данные
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}
