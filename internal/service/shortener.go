package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/model"
	"github.com/GenJi77JYXC/tinyurl/internal/repository"
	"github.com/GenJi77JYXC/tinyurl/pkg/util"
)

type ShortenerService struct {
	sqlRepo    *repository.SQLiteRepo
	redisRepo  *repository.RedisRepo
	baseURL    string
	codeLength int
}

func NewShortenerService(sqlRepo *repository.SQLiteRepo, redisRepo *repository.RedisRepo, baseURL string, codeLength int) *ShortenerService {
	return &ShortenerService{
		sqlRepo:    sqlRepo,
		redisRepo:  redisRepo,
		baseURL:    baseURL,
		codeLength: codeLength,
	}
}

type ShortenRequest struct {
	URL        string `json:"url" binding:"required"`
	ExpireDays int    `json:"expire_days"`           // 可选，默认 30
	CustomCode string `json:"custom_code,omitempty"` // 可选，自定义短码
}

func (s *ShortenerService) Shorten(req ShortenRequest, userID int64) (string, error) {
	ctx := context.Background()

	// 处理过期时间
	expireDays := req.ExpireDays
	if expireDays <= 0 {
		expireDays = 30 // 默认 30 天
	}
	expireAt := time.Now().AddDate(0, 0, expireDays)

	var shortCode string

	// 自定义短码优先
	if req.CustomCode != "" {
		if len(req.CustomCode) > 20 || !isValidShortCode(req.CustomCode) {
			return "", errors.New("invalid custom code")
		}
		// 检查是否已存在
		_, err := s.sqlRepo.GetOriginalURL(req.CustomCode)
		if err == nil {
			return "", errors.New("custom code already exists")
		}
		shortCode = req.CustomCode
	} else {
		// 自动生成
		res, err := s.sqlRepo.Db.Exec(
			"INSERT INTO links (original_url, short_code, user_id, expire_at) VALUES (?, '', ?, ?)",
			req.URL, userID, expireAt,
		)
		if err != nil {
			return "", err
		}
		id, _ := res.LastInsertId()
		shortCode = util.Base62Encode(id)

		// 更新 short_code
		_, err = s.sqlRepo.Db.Exec("UPDATE links SET short_code = ? WHERE id = ?", shortCode, id)
		if err != nil {
			return "", err
		}
	}

	// 写入 Redis 缓存（带过期）
	err := s.redisRepo.SetShortLink(ctx, shortCode, req.URL, time.Until(expireAt))
	if err != nil {
		log.Printf("Redis set failed: %v", err)
	}

	return fmt.Sprintf("%s/%s", s.baseURL, shortCode), nil

}

// 辅助函数：校验自定义短码（只允许字母数字下划线）
func isValidShortCode(code string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, code)
	return matched
}

func (s *ShortenerService) GetRedirectURL(shortCode string) (string, error) {
	ctx := context.Background()

	// 先查 Redis
	url, err := s.redisRepo.GetOriginalURL(ctx, shortCode)
	if err == nil && url != "" {
		s.redisRepo.IncrementClick(ctx, shortCode)
		return url, nil
	}

	// 查 SQLite 并检查过期
	var originalURL string
	var expireAt time.Time
	err = s.sqlRepo.Db.QueryRow(`
        SELECT original_url, expire_at 
        FROM links 
        WHERE short_code = ?`,
		shortCode,
	).Scan(&originalURL, &expireAt)

	if err == sql.ErrNoRows {
		return "", errors.New("not found")
	}
	if err != nil {
		return "", err
	}

	if time.Now().After(expireAt) {
		return "", errors.New("link expired")
	}

	// 回写 Redis
	s.redisRepo.SetShortLink(ctx, shortCode, originalURL, time.Until(expireAt))
	s.redisRepo.IncrementClick(ctx, shortCode)

	return originalURL, nil
}

func (s *ShortenerService) GetStats(shortCode string) (int64, error) {
	ctx := context.Background()
	return s.redisRepo.GetClickCount(ctx, shortCode)
}

func (s *ShortenerService) GetMyLinks(userID int64, page, limit int) ([]model.Link, error) {
	ctx := context.Background()

	links, err := s.sqlRepo.GetUserLinks(userID, page, limit)
	if err != nil {
		return nil, err
	}

	// 为每个链接从 Redis 获取点击数
	for i := range links {
		clicks, err := s.redisRepo.GetClickCount(ctx, links[i].ShortCode)
		if err != nil {
			log.Printf("Failed to get clicks for %s: %v", links[i].ShortCode, err)
			clicks = 0
		}
		links[i].Clicks = clicks
		links[i].UserID = userID
	}

	return links, nil
}
