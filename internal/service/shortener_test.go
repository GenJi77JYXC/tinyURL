package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/cache"
	"github.com/GenJi77JYXC/tinyurl/internal/model"
	"github.com/alicebob/miniredis/v2"
)

// fakeStore is an in-memory LinkStore for service-level tests.
type fakeStore struct {
	mu       sync.Mutex
	nextID   int64
	byCode   map[string]string
	getCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{byCode: map[string]string{}}
}

func (f *fakeStore) NextID(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return f.nextID, nil
}

func (f *fakeStore) InsertLink(_ context.Context, id int64, code, originalURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byCode[code] = originalURL
	return nil
}

func (f *fakeStore) GetURLByCode(_ context.Context, code string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if u, ok := f.byCode[code]; ok {
		return u, nil
	}
	return "", model.ErrNotFound
}

func newTestService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	c := cache.New(mr.Addr(), "", 0, time.Hour, time.Minute)
	st := newFakeStore()
	return New(st, c, "http://localhost:8080"), st
}

// Shorten -> Resolve: first resolve must be a Redis hit (no DB query),
// proving the creation-time cache warm-up works.
func TestShortenThenResolveFromCache(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)

	shortURL, link, err := svc.Shorten(ctx, "https://93.184.216.34/a") // public IP literal: no DNS needed
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	if link.ID != 1 || link.ShortCode != "100001" {
		t.Fatalf("unexpected link: %+v", link)
	}
	if want := "http://localhost:8080/s/100001"; shortURL != want {
		t.Fatalf("shortURL = %q, want %q", shortURL, want)
	}

	got, err := svc.Resolve(ctx, "100001")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "https://93.184.216.34/a" {
		t.Fatalf("resolved = %q", got)
	}
	if st.getCalls != 0 {
		t.Fatalf("expected cache hit (0 DB calls), got %d DB calls", st.getCalls)
	}
}

// On cache miss the DB row is read and Redis is back-filled, so the next
// resolve bypasses the DB again.
func TestResolveDBHitBackfillsCache(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)
	st.byCode["100002"] = "https://93.184.216.34/b"

	got, err := svc.Resolve(ctx, "100002")
	if err != nil || got != "https://93.184.216.34/b" {
		t.Fatalf("first Resolve = (%q, %v)", got, err)
	}
	if st.getCalls != 1 {
		t.Fatalf("first resolve DB calls = %d, want 1", st.getCalls)
	}

	got, err = svc.Resolve(ctx, "100002")
	if err != nil || got != "https://93.184.216.34/b" {
		t.Fatalf("second Resolve = (%q, %v)", got, err)
	}
	if st.getCalls != 1 {
		t.Fatalf("after back-fill DB calls = %d, want still 1", st.getCalls)
	}
}

// A missing code installs a negative marker: repeated misses must not keep
// hammering PostgreSQL (cache penetration protection).
func TestResolveNotFoundNegativeCached(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)

	for i := 0; i < 3; i++ {
		if _, err := svc.Resolve(ctx, "000000"); !errors.Is(err, model.ErrNotFound) {
			t.Fatalf("Resolve #%d = %v, want ErrNotFound", i+1, err)
		}
	}
	if st.getCalls != 1 {
		t.Fatalf("DB calls for missing code = %d, want 1 (negative cache)", st.getCalls)
	}
}

func TestResolveMalformedCode(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)

	for _, code := range []string{"abc", "short!", "1000011"} {
		if _, err := svc.Resolve(ctx, code); !errors.Is(err, model.ErrNotFound) {
			t.Fatalf("Resolve(%q) = %v, want ErrNotFound", code, err)
		}
	}
	if st.getCalls != 0 {
		t.Fatalf("malformed codes must not touch the DB, got %d calls", st.getCalls)
	}
}

// A public IPv6 literal must pass validation without panicking (regression:
// As4 used to be called before the Is4 guard).
func TestValidatePublicIPv6(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	if err := ValidateLongURL(ctx, "https://[2606:2800:220:1:248:1893:25c8:1946]/"); err != nil {
		t.Fatalf("public IPv6 literal rejected: %v", err)
	}
	// Through the full Shorten path too (fake store accepts it).
	_, _, err := svc.Shorten(ctx, "https://[2606:2800:220:1:248:1893:25c8:1946]/path")
	if err != nil {
		t.Fatalf("Shorten with public IPv6 failed: %v", err)
	}
}

func TestShortenRejectsUnsafeURLs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	cases := []struct {
		name string
		url  string
		want error
	}{
		{"not a url", "not-a-url", ErrInvalidURL},
		{"bad scheme", "ftp://example.com/file", ErrInvalidScheme},
		{"loopback ipv4", "http://127.0.0.1:8080/admin", ErrBlockedHost},
		{"loopback ipv6", "http://[::1]/admin", ErrBlockedHost},
		{"private 10/8", "http://10.0.0.5/", ErrBlockedHost},
		{"link-local metadata", "http://169.254.169.254/latest/meta-data", ErrBlockedHost},
		{"localhost hostname", "http://localhost:6379/", ErrBlockedHost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := svc.Shorten(ctx, tc.url)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Shorten(%q) = %v, want %v", tc.url, err, tc.want)
			}
		})
	}
}
