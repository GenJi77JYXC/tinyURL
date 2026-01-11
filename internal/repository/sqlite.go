package repository

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type SQLiteRepo struct {
	Db *sql.DB
}

func NewSQLiteRepo(dbPath string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 创建表
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS links (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            original_url TEXT NOT NULL,
            short_code TEXT UNIQUE NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		return nil, err
	}

	return &SQLiteRepo{Db: db}, nil
}

func (r *SQLiteRepo) CreateLink(originalURL string, shortCode string) (int64, error) {
	result, err := r.Db.Exec("INSERT INTO links (original_url, short_code) VALUES (?, ?)", originalURL, shortCode)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func (r *SQLiteRepo) GetOriginalURL(shortCode string) (string, error) {
	var originalURL string
	err := r.Db.QueryRow("SELECT original_url FROM links WHERE short_code = ?", shortCode).Scan(&originalURL)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("short code not found")
	}
	if err != nil {
		return "", err
	}
	return originalURL, nil
}

func (r *SQLiteRepo) Close() error {
	return r.Db.Close()
}
