package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ipLimiter keeps one token-bucket limiter per client IP.
//
// Default config mirrors the Day 3 plan:
//
//	rate.NewLimiter(rate.Every(100*time.Millisecond), 5)
//
// i.e. a sustained 10 QPS average with a burst allowance of 5, per IP.
type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	rps     rate.Limit
	burst   int
}

type entry struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewIPLimiter builds the limiter and starts a background sweeper that evicts
// idle IP entries until ctx is canceled, so the map cannot grow without bound.
func NewIPLimiter(rps float64, burst int) *ipLimiter {
	l := &ipLimiter{
		entries: make(map[string]*entry),
		rps:     rate.Limit(rps),
		burst:   burst,
	}
	return l
}

// RunSweeper periodically removes entries unused for longer than ttl.
func (l *ipLimiter) RunSweeper(interval, ttl time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			l.mu.Lock()
			for ip, e := range l.entries {
				if now.Sub(e.seen) > ttl {
					delete(l.entries, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

func (l *ipLimiter) forIP(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[ip]
	if !ok {
		e = &entry{lim: rate.NewLimiter(l.rps, l.burst)}
		l.entries[ip] = e
	}
	e.seen = time.Now()
	return e.lim
}

// RateLimit returns the gin middleware that enforces the per-IP bucket.
func (l *ipLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.forIP(c.ClientIP()).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, slow down",
			})
			return
		}
		c.Next()
	}
}
