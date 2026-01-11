package service

import (
	"context"
	"fmt"

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

func (s *ShortenerService) Shorten(url string) (string, error) {
	// 先插入 SQLite 获取 ID
	res, err := s.sqlRepo.Db.Exec("INSERT INTO links (original_url, short_code) VALUES (?, '')", url)
	if err != nil {
		return "", err
	}
	id, _ := res.LastInsertId()

	shortCode := util.Base62Encode(id)

	// 更新 SQLite
	_, err = s.sqlRepo.Db.Exec("UPDATE links SET short_code = ? WHERE id = ?", shortCode, id)
	if err != nil {
		return "", err
	}

	// 同时写入 Redis 缓存
	ctx := context.Background()
	err = s.redisRepo.SetShortLink(ctx, shortCode, url)
	if err != nil {
		// 缓存失败不影响核心功能，但可日志
		fmt.Printf("Redis set failed: %v\n", err)
	}

	return fmt.Sprintf("%s/%s", s.baseURL, shortCode), nil
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
