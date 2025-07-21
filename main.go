package main

import (
	"log"
	"os"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type Bot struct {
	API     *tgbotapi.BotAPI
	DB      *Database
	AdminID int64
	Config  *Config
}

type Config struct {
	PhotoStoragePath          string
	TargetDistanceMeters      float64
	UpdateIntervalSeconds     int
	LiveLocationDurationHours int
}

func main() {
	// Загружаем переменные окружения
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла: ", err)
	}

	// Получаем токен бота
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN не установлен в .env файле")
	}

	// Получаем ID администратора
	adminIDStr := os.Getenv("ADMIN_ID")
	if adminIDStr == "" {
		log.Fatal("ADMIN_ID не установлен в .env файле")
	}
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatal("Неверный формат ADMIN_ID: ", err)
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
		API:     bot,
		DB:      db,
		AdminID: adminID,
		Config:  config,
	}

	// Запускаем обработку обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	log.Println("Бот запущен...")

	for update := range updates {
		go geocachingBot.handleUpdate(update)
	}
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
