package service

import (
	"context"
	"fmt"
	"log"

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

func (s *ShortenerService) Shorten(url string, userID int64) (string, error) {
	ctx := context.Background()

	// 1. 插入 SQLite，获取自增 ID（同时关联 userID）
	res, err := s.sqlRepo.Db.Exec(
		"INSERT INTO links (original_url, short_code, user_id) VALUES (?, '', ?)",
		url, userID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert link: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("failed to get last insert id: %w", err)
	}

	// 2. 根据 ID 生成短码（Base62）
	shortCode := util.Base62Encode(id)

	// 3. 更新 SQLite 中的 short_code
	_, err = s.sqlRepo.Db.Exec(
		"UPDATE links SET short_code = ? WHERE id = ?",
		shortCode, id,
	)
	if err != nil {
		return "", fmt.Errorf("failed to update short_code: %w", err)
	}

	// 4. 同时写入 Redis 缓存（带过期时间）
	err = s.redisRepo.SetShortLink(ctx, shortCode, url)
	if err != nil {
		// 缓存失败不影响核心功能，但记录日志
		log.Printf("Redis set failed for shortCode %s: %v", shortCode, err)
	}

	// 5. 返回完整的短链接
	fullShortURL := fmt.Sprintf("%s/%s", s.baseURL, shortCode)
	return fullShortURL, nil
}

func (s *ShortenerService) GetRedirectURL(shortCode string) (string, error) {
	ctx := context.Background()

	// 先查 Redis
	url, err := s.redisRepo.GetOriginalURL(ctx, shortCode)
	if err == nil && url != "" {
		// 命中缓存，增加点击
		s.redisRepo.IncrementClick(ctx, shortCode)
		return url, nil
	}

	// 未命中 → 查 SQLite
	url, err = s.sqlRepo.GetOriginalURL(shortCode)
	if err != nil {
		return "", err
	}

	// 回写 Redis 缓存
	s.redisRepo.SetShortLink(ctx, shortCode, url)

	// 增加点击
	s.redisRepo.IncrementClick(ctx, shortCode)

	return url, nil
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
