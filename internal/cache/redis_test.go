package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestCache(t *testing.T, ttl, negTTL time.Duration) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return New(mr.Addr(), "", 0, ttl, negTTL), mr
}

func TestPositiveEntry(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t, time.Hour, time.Minute)

	if state := mustGet(t, c, ctx, "100001"); state != StateMiss {
		t.Fatalf("expected miss on empty cache, got %v", state)
	}

	if err := c.SetLink(ctx, "100001", "https://example.com"); err != nil {
		t.Fatalf("SetLink: %v", err)
	}

	url, state, err := c.GetLink(ctx, "100001")
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if state != StateHit || url != "https://example.com" {
		t.Fatalf("got (%q, %v), want hit with example.com", url, state)
	}

	ttl := mr.TTL("short:100001")
	if ttl <= 59*time.Minute || ttl > time.Hour {
		t.Fatalf("positive TTL = %v, want ~1h", ttl)
	}
}

func TestNegativeEntry(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t, time.Hour, time.Minute)

	if err := c.SetNegative(ctx, "000000"); err != nil {
		t.Fatalf("SetNegative: %v", err)
	}

	url, state, err := c.GetLink(ctx, "000000")
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if state != StateNegative || url != "" {
		t.Fatalf("got (%q, %v), want negative empty entry", url, state)
	}

	ttl := mr.TTL("short:000000")
	if ttl <= 50*time.Second || ttl > time.Minute {
		t.Fatalf("negative TTL = %v, want ~1m", ttl)
	}
}

func TestKeyPrefix(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t, time.Hour, time.Minute)

	if err := c.SetLink(ctx, "100001", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists("short:100001") {
		t.Fatal(`expected Redis key "short:100001"`)
	}
}

func mustGet(t *testing.T, c *RedisCache, ctx context.Context, code string) State {
	t.Helper()
	_, state, err := c.GetLink(ctx, code)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	return state
}
