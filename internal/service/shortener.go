// Package service contains the core business logic, independent of the HTTP
// transport: short-code allocation/encoding and the cache-aside redirect path.
package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/cache"
	"github.com/GenJi77JYXC/tinyurl/internal/metrics"
	"github.com/GenJi77JYXC/tinyurl/internal/model"
	"github.com/GenJi77JYXC/tinyurl/pkg/base62"
)

// LinkStore is the persistence port (implemented by internal/repo).
type LinkStore interface {
	NextID(ctx context.Context) (int64, error)
	InsertLink(ctx context.Context, id int64, code, originalURL string) error
	GetURLByCode(ctx context.Context, code string) (string, error) // model.ErrNotFound when absent
}

// LinkCache is the cache port (implemented by internal/cache).
type LinkCache interface {
	GetLink(ctx context.Context, code string) (string, cache.State, error)
	SetLink(ctx context.Context, code, originalURL string) error
	SetNegative(ctx context.Context, code string) error
}

// Service implements short-link creation and resolution.
type Service struct {
	store   LinkStore
	cache   LinkCache
	baseURL string
}

// New builds a Service.
func New(store LinkStore, cache LinkCache, baseURL string) *Service {
	return &Service{store: store, cache: cache, baseURL: strings.TrimRight(baseURL, "/")}
}

// Shorten validates the target URL, allocates a sequence ID, base62-encodes it
// into a fixed 6-char code and persists the link. The positive cache entry is
// warmed best-effort. It returns the full short URL.
func (s *Service) Shorten(ctx context.Context, rawURL string) (string, model.Link, error) {
	if err := ValidateLongURL(ctx, rawURL); err != nil {
		return "", model.Link{}, err
	}
	rawURL = strings.TrimSpace(rawURL)

	id, err := s.store.NextID(ctx)
	if err != nil {
		return "", model.Link{}, err
	}

	// Isolate the encoding latency so the histogram reflects CPU work only.
	var code string
	encStart := time.Now()
	code, encErr := base62.Encode(uint64(id))
	if encErr != nil {
		metrics.EncodeDuration.WithLabelValues("error").Observe(time.Since(encStart).Seconds())
		return "", model.Link{}, encErr
	}
	metrics.EncodeDuration.WithLabelValues("ok").Observe(time.Since(encStart).Seconds())

	if err := s.store.InsertLink(ctx, id, code, rawURL); err != nil {
		return "", model.Link{}, err
	}

	// Warm cache; never fail a creation because of Redis.
	if err := s.cache.SetLink(ctx, code, rawURL); err != nil {
		slog.Warn("cache warm-up failed, degrading", "code", code, "error", err)
	}

	link := model.Link{ID: id, ShortCode: code, OriginalURL: rawURL}
	return s.baseURL + "/s/" + code, link, nil
}

// Resolve implements the redirect path:
//
//	Redis positive -> return immediately
//	Redis negative -> not found, no DB query
//	Redis miss/err -> query PostgreSQL; on hit back-fill Redis (TTL 1h),
//	                  on miss install a short-lived negative marker.
//
// Redis errors never break a request: the flow transparently falls back to PG.
func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	if !isWellFormedCode(code) {
		metrics.ResolveTotal.WithLabelValues("not_found").Inc()
		return "", model.ErrNotFound
	}

	if url, state, err := s.cache.GetLink(ctx, code); err == nil {
		switch state {
		case cache.StateHit:
			metrics.ResolveTotal.WithLabelValues("cache_hit").Inc()
			return url, nil
		case cache.StateNegative:
			metrics.ResolveTotal.WithLabelValues("not_found").Inc()
			return "", model.ErrNotFound
		}
	} else {
		slog.Warn("cache lookup failed, falling back to db", "code", code, "error", err)
	}

	originalURL, err := s.store.GetURLByCode(ctx, code)
	if errors.Is(err, model.ErrNotFound) {
		metrics.ResolveTotal.WithLabelValues("not_found").Inc()
		if cacheErr := s.cache.SetNegative(ctx, code); cacheErr != nil {
			slog.Warn("negative cache write failed", "code", code, "error", cacheErr)
		}
		return "", model.ErrNotFound
	}
	if err != nil {
		return "", err
	}

	metrics.ResolveTotal.WithLabelValues("db_hit").Inc()
	if cacheErr := s.cache.SetLink(ctx, code, originalURL); cacheErr != nil {
		slog.Warn("cache back-fill failed", "code", code, "error", cacheErr)
	}
	return originalURL, nil
}

func isWellFormedCode(code string) bool {
	if len(code) != base62.CodeLength {
		return false
	}
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}
