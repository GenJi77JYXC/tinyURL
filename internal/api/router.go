package api

import (
	"log/slog"

	"github.com/GenJi77JYXC/tinyurl/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter wires every route:
//
//	GET  /healthz   liveness probe
//	GET  /metrics   Prometheus scrape endpoint
//	POST /shorten   create a short link (per-IP token-bucket rate limited)
//	GET  /s/{code}  302 redirect
func NewRouter(h *Handler, cfg *config.Config, limiter *ipLimiter) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		slog.Warn("invalid trusted proxies configuration, falling back to direct peer IP", "error", err)
		_ = r.SetTrustedProxies(nil)
	}

	r.GET("/healthz", h.Healthz)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.POST("/shorten", limiter.RateLimit(), h.Shorten)
	r.GET("/s/:code", h.Redirect)

	return r
}
