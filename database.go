package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

type Cache struct {
	ID        int64     `json:"id"`
	CodeWord  string    `json:"code_word"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	FileID    string    `json:"file_id"`   // Telegram file_id
	FileType  string    `json:"file_type"` // "photo", "video", "video_note"
	CreatedAt time.Time `json:"created_at"`
	CreatedBy int64     `json:"created_by"`
}

type UserSession struct {
	UserID          int64     `json:"user_id"`
	CacheID         int64     `json:"cache_id"`
	LastLatitude    float64   `json:"last_latitude"`
	LastLongitude   float64   `json:"last_longitude"`
	LastMessageID   int       `json:"last_message_id"`
	LastMessageText string    `json:"last_message_text"`
	IsActive        bool      `json:"is_active"`
	LastUpdate      time.Time `json:"last_update"`
}

type AdminSession struct {
	UserID    int64   `json:"user_id"`
	Step      string  `json:"step"` // "waiting_code", "waiting_location", "waiting_media"
	CodeWord  string  `json:"code_word"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func NewDatabase(dataSourceName string) (*Database, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	database := &Database{db: db}

	if err := database.createTables(); err != nil {
		return nil, err
	}

	// Выполняем миграцию для обновления схемы
	if err := database.migrate(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) createTables() error {
	// Таблица тайников
	cacheTable := `
	CREATE TABLE IF NOT EXISTS caches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code_word TEXT UNIQUE NOT NULL,
		latitude REAL NOT NULL,
		longitude REAL NOT NULL,
		file_id TEXT NOT NULL,
		file_type TEXT NOT NULL DEFAULT 'photo',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_by INTEGER NOT NULL
	);`

	// Таблица пользовательских сессий
	userSessionTable := `
	CREATE TABLE IF NOT EXISTS user_sessions (
		user_id INTEGER PRIMARY KEY,
		cache_id INTEGER NOT NULL,
		last_latitude REAL,
		last_longitude REAL,
		last_message_id INTEGER,
		last_message_text TEXT,
		is_active BOOLEAN DEFAULT TRUE,
		last_update DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (cache_id) REFERENCES caches (id)
	);`

	// Таблица админских сессий
	adminSessionTable := `
	CREATE TABLE IF NOT EXISTS admin_sessions (
		user_id INTEGER PRIMARY KEY,
		step TEXT NOT NULL,
		code_word TEXT,
		latitude REAL,
		longitude REAL
	);`

	queries := []string{cacheTable, userSessionTable, adminSessionTable}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// migrate выполняет миграцию базы данных для обновления схемы
func (d *Database) migrate() error {
	// Проверяем, существует ли старая колонка photo_path
	rows, err := d.db.Query("PRAGMA table_info(caches)")
	if err != nil {
		return err
	}
	defer rows.Close()

	var hasPhotoPath, hasFileID, hasFileType bool

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue interface{}

		err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		if err != nil {
			continue
		}

		switch name {
		case "photo_path":
			hasPhotoPath = true
		case "file_id":
			hasFileID = true
		case "file_type":
			hasFileType = true
		}
	}

	// Если есть старая схема, но нет новых полей - выполняем полную миграцию
	if hasPhotoPath && (!hasFileID || !hasFileType) {
		return d.migrateToNewSchema()
	}

	return nil
}

// migrateToNewSchema выполняет полную миграцию таблицы caches
func (d *Database) migrateToNewSchema() error {
	log.Println("Начинаем миграцию базы данных...")

	// Начинаем транзакцию
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Создаем новую таблицу с правильной структурой
	newCacheTable := `
	CREATE TABLE caches_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code_word TEXT UNIQUE NOT NULL,
		latitude REAL NOT NULL,
		longitude REAL NOT NULL,
		file_id TEXT NOT NULL DEFAULT '',
		file_type TEXT NOT NULL DEFAULT 'photo',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_by INTEGER NOT NULL
	);`

	_, err = tx.Exec(newCacheTable)
	if err != nil {
		return err
	}

	// Копируем существующие данные из старой таблицы (если они есть)
	// Устанавливаем file_id как пустую строку и file_type как 'photo' для старых записей
	copyData := `
	INSERT INTO caches_new (id, code_word, latitude, longitude, file_id, file_type, created_at, created_by)
	SELECT id, code_word, latitude, longitude, '', 'photo', created_at, created_by 
	FROM caches;`

	_, err = tx.Exec(copyData)
	if err != nil {
		// Если ошибка копирования, возможно таблица пустая - это нормально
		// Продолжаем без ошибки
	}

	// Удаляем старую таблицу
	_, err = tx.Exec("DROP TABLE caches;")
	if err != nil {
		return err
	}

	// Переименовываем новую таблицу
	_, err = tx.Exec("ALTER TABLE caches_new RENAME TO caches;")
	if err != nil {
		return err
	}

	// Подтверждаем транзакцию
	err = tx.Commit()
	if err != nil {
		return err
	}

	log.Println("Миграция базы данных успешно завершена!")
	return nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// Методы для работы с тайниками
func (d *Database) CreateCache(cache *Cache) error {
	query := `INSERT INTO caches (code_word, latitude, longitude, file_id, file_type, created_by) 
			  VALUES (?, ?, ?, ?, ?, ?)`

	result, err := d.db.Exec(query, cache.CodeWord, cache.Latitude, cache.Longitude, cache.FileID, cache.FileType, cache.CreatedBy)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	cache.ID = id
	return nil
}

func (d *Database) GetCacheByCodeWord(codeWord string) (*Cache, error) {
	query := `SELECT id, code_word, latitude, longitude, file_id, file_type, created_at, created_by 
			  FROM caches WHERE code_word = ?`

	cache := &Cache{}
	err := d.db.QueryRow(query, codeWord).Scan(
		&cache.ID, &cache.CodeWord, &cache.Latitude, &cache.Longitude,
		&cache.FileID, &cache.FileType, &cache.CreatedAt, &cache.CreatedBy,
	)

	if err != nil {
		return nil, err
	}

	return cache, nil
}

// Методы для работы с пользовательскими сессиями
func (d *Database) CreateOrUpdateUserSession(session *UserSession) error {
	query := `INSERT OR REPLACE INTO user_sessions 
			  (user_id, cache_id, last_latitude, last_longitude, last_message_id, last_message_text, is_active, last_update) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query, session.UserID, session.CacheID, session.LastLatitude,
		session.LastLongitude, session.LastMessageID, session.LastMessageText, session.IsActive, time.Now())
	return err
}

func (d *Database) GetUserSession(userID int64) (*UserSession, error) {
	query := `SELECT user_id, cache_id, last_latitude, last_longitude, last_message_id, last_message_text, is_active, last_update 
			  FROM user_sessions WHERE user_id = ? AND is_active = TRUE`

	session := &UserSession{}
	err := d.db.QueryRow(query, userID).Scan(
		&session.UserID, &session.CacheID, &session.LastLatitude, &session.LastLongitude,
		&session.LastMessageID, &session.LastMessageText, &session.IsActive, &session.LastUpdate,
	)

	if err != nil {
		return nil, err
	}

	return session, nil
}

func (d *Database) DeactivateUserSession(userID int64) error {
	query := `UPDATE user_sessions SET is_active = FALSE WHERE user_id = ?`
	_, err := d.db.Exec(query, userID)
	return err
}

// Методы для работы с админскими сессиями
func (d *Database) CreateOrUpdateAdminSession(session *AdminSession) error {
	query := `INSERT OR REPLACE INTO admin_sessions 
			  (user_id, step, code_word, latitude, longitude) 
			  VALUES (?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query, session.UserID, session.Step, session.CodeWord, session.Latitude, session.Longitude)
	return err
}

func (d *Database) GetAdminSession(userID int64) (*AdminSession, error) {
	query := `SELECT user_id, step, code_word, latitude, longitude 
			  FROM admin_sessions WHERE user_id = ?`

	session := &AdminSession{}
	err := d.db.QueryRow(query, userID).Scan(
		&session.UserID, &session.Step, &session.CodeWord, &session.Latitude, &session.Longitude,
	)

	if err != nil {
		return nil, err
	}

	return session, nil
}

func (d *Database) DeleteAdminSession(userID int64) error {
	query := `DELETE FROM admin_sessions WHERE user_id = ?`
	_, err := d.db.Exec(query, userID)
	return err
}
