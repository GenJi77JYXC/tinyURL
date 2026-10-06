// Package config loads all runtime configuration from environment variables
// (optionally seeded from a local .env file) and validates the result once
// at startup, so the rest of the codebase consumes a typed struct instead of
// scattering os.Getenv calls.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every runtime setting for the service.
type Config struct {
	// HTTP
	GinMode        string        // gin debug / release / test
	HTTPAddr       string        // listen address, e.g. ":8080"
	BaseURL        string        // public base used to build short URLs
	TrustedProxies []string      // CIDRs / IPs allowed to set X-Forwarded-For
	ShutdownTO     time.Duration // graceful shutdown budget

	// Storage
	DatabaseURL string // PostgreSQL DSN
	RedisAddr   string
	RedisPass   string
	RedisDB     int

	// Cache
	CacheTTL time.Duration // positive cache entry lifetime (plan: 1h)
	NegTTL   time.Duration // negative cache entry lifetime (anti-penetration)

	// Rate limiting (token bucket, per client IP, applied to POST /shorten)
	RateRPS   float64 // sustained tokens per second
	RateBurst int     // token bucket capacity
}

// Load reads configuration from the environment (.env file is optional).
func Load() (*Config, error) {
	// A missing .env is fine in containers — real env vars take over.
	_ = godotenv.Load()

	cfg := &Config{
		GinMode:    getenv("GIN_MODE", "debug"),
		HTTPAddr:   getenv("HTTP_ADDR", ":"+getenv("PORT", "8080")),
		BaseURL:    strings.TrimRight(getenv("BASE_URL", "http://localhost:8080"), "/"),
		ShutdownTO: getdur("SHUTDOWN_TIMEOUT", 10*time.Second),

		DatabaseURL: getenv("DATABASE_URL", "postgres://tinyurl:tinyurl@localhost:5432/tinyurl?sslmode=disable"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getenv("REDIS_PASSWORD", ""),
		RedisDB:     getint("REDIS_DB", 0),

		CacheTTL: getdur("CACHE_TTL", time.Hour),
		NegTTL:   getdur("NEGATIVE_CACHE_TTL", time.Minute),

		RateRPS:   getfloat("RATE_LIMIT_RPS", 10), // rate.Every(100ms) ≈ 10 QPS
		RateBurst: getint("RATE_LIMIT_BURST", 5),
	}

	for _, p := range strings.Split(getenv("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			cfg.TrustedProxies = append(cfg.TrustedProxies, p)
		}
	}

	if cfg.RateRPS <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_RPS must be > 0, got %v", cfg.RateRPS)
	}
	if cfg.RateBurst < 1 {
		return nil, fmt.Errorf("RATE_LIMIT_BURST must be >= 1, got %d", cfg.RateBurst)
	}
	if cfg.CacheTTL <= 0 || cfg.NegTTL <= 0 {
		return nil, fmt.Errorf("cache TTLs must be > 0")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getint(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getfloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func getdur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}
