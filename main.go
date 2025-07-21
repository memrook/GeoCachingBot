package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type Bot struct {
	API      *tgbotapi.BotAPI
	DB       *Database
	AdminIDs []int64
	Config   *Config
}

type Config struct {
	PhotoStoragePath          string
	TargetDistanceMeters      float64
	UpdateIntervalSeconds     int
	LiveLocationDurationHours int
}

func main() {
	// Загружаем переменные окружения (опционально для локальной разработки)
	err := godotenv.Load()
	if err != nil {
		log.Printf("Предупреждение: .env файл не найден (%v), используем переменные окружения", err)
	}

	// Получаем токен бота
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN не установлен в .env файле")
	}

	// Получаем ID администраторов (поддерживаем как старый ADMIN_ID, так и новый ADMIN_IDS)
	adminIDs, err := parseAdminIDs()
	if err != nil {
		log.Fatal("Ошибка парсинга ID администраторов: ", err)
	}
	if len(adminIDs) == 0 {
		log.Fatal("Не указан ни один ID администратора. Установите ADMIN_ID или ADMIN_IDS в .env файле")
	}

	// Создаем конфигурацию
	config := &Config{
		PhotoStoragePath:          getEnvString("PHOTO_STORAGE_PATH", "./photos/"),
		TargetDistanceMeters:      getEnvFloat("TARGET_DISTANCE_METERS", 200),
		UpdateIntervalSeconds:     getEnvInt("UPDATE_INTERVAL_SECONDS", 5),
		LiveLocationDurationHours: getEnvInt("LIVE_LOCATION_DURATION_HOURS", 1),
	}

	// Создаем папку для фотографий
	err = os.MkdirAll(config.PhotoStoragePath, 0755)
	if err != nil {
		log.Fatal("Ошибка создания папки для фотографий: ", err)
	}

	// Инициализируем бота
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Ошибка создания бота: ", err)
	}

	bot.Debug = true
	log.Printf("Авторизован как %s", bot.Self.UserName)

	// Инициализируем базу данных
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "geocaching.db"
	}

	db, err := NewDatabase(databasePath)
	if err != nil {
		log.Fatal("Ошибка инициализации базы данных: ", err)
	}
	defer db.Close()

	// Создаем экземпляр бота
	geocachingBot := &Bot{
		API:      bot,
		DB:       db,
		AdminIDs: adminIDs,
		Config:   config,
	}

	// Запускаем обработку обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	log.Printf("Бот запущен с %d администратором(ами)...", len(adminIDs))

	for update := range updates {
		go geocachingBot.handleUpdate(update)
	}
}

// parseAdminIDs парсит ID администраторов из переменных окружения
// Поддерживает как ADMIN_ID (один ID), так и ADMIN_IDS (несколько ID через запятую)
func parseAdminIDs() ([]int64, error) {
	var adminIDs []int64

	// Сначала проверяем новый формат ADMIN_IDS (несколько ID через запятую)
	adminIDsStr := os.Getenv("ADMIN_IDS")
	if adminIDsStr != "" {
		parts := strings.Split(adminIDsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				id, err := strconv.ParseInt(part, 10, 64)
				if err != nil {
					return nil, err
				}
				adminIDs = append(adminIDs, id)
			}
		}
		return adminIDs, nil
	}

	// Если ADMIN_IDS не установлен, проверяем старый формат ADMIN_ID
	adminIDStr := os.Getenv("ADMIN_ID")
	if adminIDStr != "" {
		id, err := strconv.ParseInt(adminIDStr, 10, 64)
		if err != nil {
			return nil, err
		}
		adminIDs = append(adminIDs, id)
		return adminIDs, nil
	}

	// Если ни один не установлен, возвращаем пустой слайс
	return adminIDs, nil
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return value
		}
	}
	return defaultValue
}
