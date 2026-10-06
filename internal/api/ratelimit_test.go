package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIPRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 1 request/second sustained, burst capacity 2.
	limiter := NewIPLimiter(1, 2)
	r := gin.New()
	r.GET("/ping", limiter.RateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	do := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = ip + ":54321"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// First IP: two burst tokens pass, the third is rejected with 429.
	if code := do("1.2.3.4"); code != http.StatusOK {
		t.Fatalf("request 1 = %d, want 200", code)
	}
	if code := do("1.2.3.4"); code != http.StatusOK {
		t.Fatalf("request 2 = %d, want 200", code)
	}
	if code := do("1.2.3.4"); code != http.StatusTooManyRequests {
		t.Fatalf("request 3 = %d, want 429", code)
	}

	// A different IP has its own independent bucket.
	if code := do("5.6.7.8"); code != http.StatusOK {
		t.Fatalf("other IP = %d, want 200 (per-IP buckets must be independent)", code)
	}
}
