package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

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
	if b.isAdmin(userID) {
		// Проверяем, есть ли активная админская сессия
		_, err := b.DB.GetAdminSession(userID)
		if err == nil {
			// Есть активная админская сессия - обрабатываем как админа
			b.handleAdminMessage(message)
			return
		}

		// Если это команда - всегда обрабатываем как админские команды
		if message.IsCommand() {
			b.handleAdminMessage(message)
			return
		}

		// Если не команда, обрабатываем как обычного пользователя (режим тестирования)
		b.handleUserMessage(message)
		return
	}

	// Обработка сообщений обычных пользователей
	b.handleUserMessage(message)
}

// Обработчик сообщений администратора
func (b *Bot) handleAdminMessage(message *tgbotapi.Message) {
	userID := message.From.ID

	if message.IsCommand() {
		switch message.Command() {
		case "create":
			b.handleCreateCommand(userID)
		case "start", "help":
			b.sendAdminWelcome(userID)
		case "stop":
			b.handleAdminStopCommand(userID)
		default:
			b.sendMessage(userID, "Неизвестная команда администратора. Доступные команды:\n/start - главное меню\n/create - создать новый тайник\n/stop - отменить создание тайника\n/help - справка")
		}
		return
	}

	// Проверяем, есть ли активная админская сессия
	session, err := b.DB.GetAdminSession(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			b.sendMessage(userID, "Введите команду /create для создания нового тайника.")
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
	case "waiting_media":
		b.handleMediaInput(userID, message)
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

	b.sendMessage(userID, "🔑 Придумайте кодовое слово для нового тайника:")
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

	msg := tgbotapi.NewMessage(userID, "📍 Отлично! Теперь отправьте геолокацию места, где будет спрятан тайник.\n\n💡 Для создания тайника подходит обычная геопозиция (не трансляция).")

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("📍 Отправить геолокацию"),
		),
	)
	keyboard.OneTimeKeyboard = true
	msg.ReplyMarkup = keyboard

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

	session.Step = "waiting_media"
	session.Latitude = latitude
	session.Longitude = longitude

	err = b.DB.CreateOrUpdateAdminSession(session)
	if err != nil {
		log.Printf("Ошибка обновления админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте еще раз.")
		return
	}

	// Убираем клавиатуру
	msg := tgbotapi.NewMessage(userID, "📷🎥 Теперь отправьте фотографию, видео или видео-заметку этого места:")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.API.Send(msg)
}

// Обработчик ввода медиафайла (фото или видео)
func (b *Bot) handleMediaInput(userID int64, message *tgbotapi.Message) {
	var fileID string
	var fileType string
	var mediaType string

	// Проверяем тип медиафайла
	if message.Photo != nil && len(message.Photo) > 0 {
		// Получаем файл с наибольшим разрешением
		photo := message.Photo[len(message.Photo)-1]
		fileID = photo.FileID
		fileType = "photo"
		mediaType = "фотографию"
	} else if message.Video != nil {
		fileID = message.Video.FileID
		fileType = "video"
		mediaType = "видео"
	} else if message.VideoNote != nil {
		fileID = message.VideoNote.FileID
		fileType = "video_note"
		mediaType = "видео-заметку"
	} else {
		b.sendMessage(userID, "Пожалуйста, отправьте фотографию, видео или видео-заметку.")
		return
	}

	// Получаем сессию
	session, err := b.DB.GetAdminSession(userID)
	if err != nil {
		log.Printf("Ошибка получения админской сессии: %v", err)
		b.sendMessage(userID, "Произошла ошибка. Попробуйте команду /create заново.")
		return
	}

	// Создаем запись в базе данных (без скачивания файла)
	cache := &Cache{
		CodeWord:  session.CodeWord,
		Latitude:  session.Latitude,
		Longitude: session.Longitude,
		FileID:    fileID,   // Сохраняем file_id вместо пути к файлу
		FileType:  fileType, // Сохраняем тип файла
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

	successMsg := fmt.Sprintf("✅ Тайник успешно создан!\n\n🔑 Кодовое слово: %s\n📍 Координаты: %.6f, %.6f\n📱 Медиафайл: %s\n\nТеперь пользователи могут найти этот тайник, введя кодовое слово.",
		cache.CodeWord, cache.Latitude, cache.Longitude, mediaType)

	b.sendMessage(userID, successMsg)
}

// Обработчик сообщений пользователей
func (b *Bot) handleUserMessage(message *tgbotapi.Message) {
	userID := message.From.ID

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			b.sendMessage(userID, "🗺️ Добро пожаловать в GeoCaching Bot!\n\n🔍 Введите кодовое слово для поиска тайника:\n\n💡 Совет: кодовое слово должно содержать минимум 3 символа")
		case "stop":
			b.handleStopCommand(userID)
		default:
			b.sendMessage(userID, "🤔 Неизвестная команда.\n\nВведите кодовое слово для поиска тайника или /stop для остановки поиска.")
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

	// Если нет активной сессии, но получили геолокацию - просто игнорируем
	if message.Location != nil {
		return
	}

	// Если нет активной сессии и это текстовое сообщение, ищем тайник по кодовому слову
	b.handleCacheSearch(userID, message.Text)
}

// Обработчик поиска тайника по кодовому слову
func (b *Bot) handleCacheSearch(userID int64, codeWord string) {
	// Проверяем, что получили текстовое сообщение
	if codeWord == "" {
		b.sendMessage(userID, "🗺️ Добро пожаловать в GeoCaching Bot!\n\n🔍 Введите кодовое слово для поиска тайника:\n\n💡 Совет: кодовое слово должно содержать минимум 3 символа")
		return
	}

	codeWord = strings.TrimSpace(codeWord)
	if len(codeWord) < 3 {
		b.sendMessage(userID, "Кодовое слово должно содержать минимум 3 символа.")
		return
	}

	// Ищем тайник в базе данных
	cache, err := b.DB.GetCacheByCodeWord(codeWord)
	if err != nil {
		if err == sql.ErrNoRows {
			b.sendMessage(userID, "🔍 Тайник с таким кодовым словом не найден.\n\nПроверьте правильность написания и попробуйте еще раз.")
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

	// Запрашиваем доступ к live-геолокации
	instruction := fmt.Sprintf(`🎯 Тайник найден: %s

📍 Для начала поиска включите трансляцию геопозиции`,
		cache.CodeWord)

	b.sendMessage(userID, instruction)
}

// Обработчик обновлений геолокации
func (b *Bot) handleLocationUpdate(userID int64, message *tgbotapi.Message) {
	if message.Location == nil {
		b.sendMessage(userID, "Пожалуйста, отправьте геолокацию для продолжения поиска.\n\nИспользуйте /stop для остановки поиска.")
		return
	}

	// Проверяем, что это live-геолокация (трансляция), а не статичная точка
	if message.Location.LivePeriod == 0 {
		instruction := `❌ Получена статичная геопозиция!

📍 Для навигации нужна ТРАНСЛЯЦИЯ геопозиции:

1️⃣ Нажмите на скрепку 📎 в поле ввода
2️⃣ Выберите "Геопозиция" 🗺️  
3️⃣ Выберите "Транслировать геопозицию" ⏱️
4️⃣ Установите время и нажмите "Поделиться"

⚠️ Используйте именно ТРАНСЛЯЦИЮ, а не обычную геопозицию!`

		b.sendMessage(userID, instruction)
		return
	}

	// Получаем сессию пользователя
	session, err := b.DB.GetUserSession(userID)
	if err != nil {
		log.Printf("Ошибка получения пользовательской сессии: %v", err)
		return
	}

	// Получаем данные кэша по ID из сессии
	query := `SELECT id, code_word, latitude, longitude, file_id, file_type, created_at, created_by 
			  FROM caches WHERE id = ?`

	cache := &Cache{}
	err = b.DB.db.QueryRow(query, session.CacheID).Scan(
		&cache.ID, &cache.CodeWord, &cache.Latitude, &cache.Longitude,
		&cache.FileID, &cache.FileType, &cache.CreatedAt, &cache.CreatedBy,
	)

	if err != nil {
		log.Printf("Ошибка получения кэша: %v", err)
		return
	}

	userLat := float64(message.Location.Latitude)
	userLon := float64(message.Location.Longitude)

	// Проверяем, достиг ли пользователь цели
	if isTargetReached(userLat, userLon, cache.Latitude, cache.Longitude, b.Config.TargetDistanceMeters) {
		b.handleTargetReached(userID, cache)
		return
	}

	// Формируем сообщение с направлением
	directionMsg := fmt.Sprintf("🧭 Направление к тайнику:\n\n%s",
		formatDirectionMessage(userLat, userLon, cache.Latitude, cache.Longitude))

	// Если это первое сообщение, отправляем новое
	if session.LastMessageID == 0 {
		// Отправляем сообщение с направлением (без ReplyMarkup для совместимости с редактированием)
		msg := tgbotapi.NewMessage(userID, directionMsg)
		msg.ParseMode = "Markdown"
		sentMsg, err := b.API.Send(msg)
		if err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
			return
		}

		// Обновляем сессию
		session.LastLatitude = userLat
		session.LastLongitude = userLon
		session.LastMessageID = sentMsg.MessageID
		session.LastMessageText = directionMsg
		b.DB.CreateOrUpdateUserSession(session)
	} else {
		// Проверяем, изменился ли текст сообщения
		if session.LastMessageText == directionMsg {
			// Текст не изменился, пропускаем обновление
			return
		}

		// Пытаемся отредактировать существующее сообщение
		edit := tgbotapi.NewEditMessageText(userID, session.LastMessageID, directionMsg)
		edit.ParseMode = "Markdown"
		_, err := b.API.Send(edit)

		if err != nil {
			// Если редактирование не удалось, отправляем новое сообщение
			log.Printf("Не удалось отредактировать сообщение: %v. Отправляем новое.", err)

			newMsg := tgbotapi.NewMessage(userID, directionMsg)
			newMsg.ParseMode = "Markdown"
			sentMsg, sendErr := b.API.Send(newMsg)

			if sendErr != nil {
				log.Printf("Ошибка отправки нового сообщения: %v", sendErr)
				return
			}

			// Обновляем ID последнего сообщения
			session.LastMessageID = sentMsg.MessageID
		}

		// Обновляем сессию
		session.LastLatitude = userLat
		session.LastLongitude = userLon
		session.LastMessageText = directionMsg
		b.DB.CreateOrUpdateUserSession(session)
	}
}

// Обработчик достижения цели
func (b *Bot) handleTargetReached(userID int64, cache *Cache) {
	// Деактивируем сессию
	b.DB.DeactivateUserSession(userID)

	// Определяем тип медиафайла по сохраненному типу
	var mediaTypeText string
	switch cache.FileType {
	case "video_note":
		mediaTypeText = "🎥 Вот видео-заметка места:"
	case "video":
		mediaTypeText = "🎥 Вот видео места:"
	case "photo":
		fallthrough
	default:
		mediaTypeText = "📷 Вот фотография места:"
	}

	// Отправляем поздравительное сообщение
	congratsMsg := fmt.Sprintf("🎉 Поздравляем! Вы нашли тайник: %s\n\n%s\n\n💡 Вы можете остановить передачу геолокации и начать поиск нового тайника!", cache.CodeWord, mediaTypeText)

	msg := tgbotapi.NewMessage(userID, congratsMsg)
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.API.Send(msg)

	// Отправляем медиафайл используя file_id
	caption := "🏆 Поиск завершен! Введите новое кодовое слово для следующего тайника."

	switch cache.FileType {
	case "video_note":
		videoNoteMsg := tgbotapi.NewVideoNote(userID, 0, tgbotapi.FileID(cache.FileID))
		_, err := b.API.Send(videoNoteMsg)
		if err != nil {
			log.Printf("Ошибка отправки видео-заметки: %v", err)
			b.sendMessage(userID, "К сожалению, не удалось загрузить видео-заметку места.")
		}
		// Отправляем текст отдельно, так как видео-заметки не поддерживают подписи
		b.sendMessage(userID, caption)
	case "video":
		videoMsg := tgbotapi.NewVideo(userID, tgbotapi.FileID(cache.FileID))
		videoMsg.Caption = caption
		_, err := b.API.Send(videoMsg)
		if err != nil {
			log.Printf("Ошибка отправки видео: %v", err)
			b.sendMessage(userID, "К сожалению, не удалось загрузить видео места.")
		}
	case "photo":
		fallthrough
	default:
		photoMsg := tgbotapi.NewPhoto(userID, tgbotapi.FileID(cache.FileID))
		photoMsg.Caption = caption
		_, err := b.API.Send(photoMsg)
		if err != nil {
			log.Printf("Ошибка отправки фотографии: %v", err)
			b.sendMessage(userID, "К сожалению, не удалось загрузить фотографию места.")
		}
	}
}

// Обработчик команды /stop
func (b *Bot) handleStopCommand(userID int64) {
	// Деактивируем пользовательскую сессию
	err := b.DB.DeactivateUserSession(userID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Ошибка деактивации сессии: %v", err)
	}

	msg := tgbotapi.NewMessage(userID, "🛑 Поиск тайника остановлен.\n\nВведите новое кодовое слово для начала поиска.")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.API.Send(msg)
}

// sendAdminWelcome отправляет приветствие администратору
func (b *Bot) sendAdminWelcome(userID int64) {
	welcomeMsg := `👑 Добро пожаловать, администратор!

📋 **Доступные команды:**
• /start или /help - показать это меню
• /create - создать новый тайник
• /stop - отменить создание/поиск тайника

🎯 **Доступные режимы:**

🔧 **Режим администратора:**
Используйте команды выше для управления тайниками

🔍 **Режим тестирования:**
• Введите кодовое слово для поиска тайника
• Полная навигация как у обычных пользователей
• Поддержка фото, видео и видео-заметок

💡 Переключение между режимами происходит автоматически!`

	msg := tgbotapi.NewMessage(userID, welcomeMsg)
	msg.ParseMode = "Markdown"
	b.API.Send(msg)
}

// handleAdminStopCommand обрабатывает команду /stop для администратора
func (b *Bot) handleAdminStopCommand(userID int64) {
	// Проверяем, есть ли активная админская сессия
	_, err := b.DB.GetAdminSession(userID)
	if err == nil {
		// Есть активная админская сессия - удаляем её
		b.DB.DeleteAdminSession(userID)
		b.sendMessage(userID, "🛑 Создание тайника отменено.\n\nВы можете:\n• /create - создать новый тайник\n• Ввести кодовое слово для поиска тайника")
		return
	}

	// Проверяем, есть ли активная пользовательская сессия
	_, err = b.DB.GetUserSession(userID)
	if err == nil {
		// Есть активная пользовательская сессия - деактивируем её
		b.DB.DeactivateUserSession(userID)
		msg := tgbotapi.NewMessage(userID, "🛑 Поиск тайника остановлен.\n\nВы можете:\n• /create - создать новый тайник\n• Ввести кодовое слово для поиска тайника")
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		b.API.Send(msg)
		return
	}

	// Никаких активных сессий нет
	b.sendMessage(userID, "ℹ️ Нет активных процессов для остановки.\n\nВы можете:\n• /create - создать новый тайник\n• Ввести кодовое слово для поиска тайника")
}

// isAdmin проверяет, является ли пользователь администратором
func (b *Bot) isAdmin(userID int64) bool {
	for _, adminID := range b.AdminIDs {
		if userID == adminID {
			return true
		}
	}
	return false
}

// Вспомогательная функция для отправки текстовых сообщений
func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.API.Send(msg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}
