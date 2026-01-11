package repository

import (
	"database/sql"
	"fmt"

	"github.com/GenJi77JYXC/tinyurl/internal/model"
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
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )
`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_url TEXT NOT NULL,
    short_code TEXT UNIQUE NOT NULL,
    user_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
)
`)
	if err != nil {
		return nil, err
	}

	return &SQLiteRepo{Db: db}, nil
}

func (r *SQLiteRepo) CreateLink(originalURL string, shortCode string, userID int64) (int64, error) {
	result, err := r.Db.Exec("INSERT INTO links (original_url, short_code, user_id) VALUES (?, ?, ?)", originalURL, shortCode, userID)
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

func (r *SQLiteRepo) GetUserLinks(userID int64, page, limit int) ([]model.Link, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	rows, err := r.Db.Query(`
        SELECT id, original_url, short_code, created_at
        FROM links
        WHERE user_id = ?
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var links []model.Link

	for rows.Next() {
		var link model.Link
		err := rows.Scan(&link.ID, &link.OriginalURL, &link.ShortCode, &link.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		links = append(links, link)
	}

	// 检查遍历过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return links, nil
}
