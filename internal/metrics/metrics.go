// Package metrics defines every Prometheus metric exposed at /metrics.
//
// Three metric families are collected (Day 4 checklist):
//
//	Histogram: shorturl_encode_duration_seconds — base62 encoding latency (P99)
//	Counter:   shorturl_resolve_total{state}    — cache_hit / db_hit / not_found
//	           (hit rate and cache-penetration rate are derived in PromQL)
//	Gauge:     shorturl_goroutines, shorturl_redis_pool_usage_ratio
package metrics

import (
	"context"
	"runtime"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/cache"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// EncodeDuration tracks the base62 encoding latency. Exponential buckets
	// start at 0.5ms and double 12 times -> up to ~2s, covering micro to ms range.
	EncodeDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "shorturl_encode_duration_seconds",
		Help:    "Base62 encoding latency in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.0005, 2, 12),
	}, []string{"result"})

	// ResolveTotal counts redirect resolutions by cache outcome:
	// cache_hit (Redis), db_hit (Redis miss, PG hit) or not_found (both miss).
	ResolveTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "shorturl_resolve_total",
		Help: "Total redirect lookups labeled by resolution outcome.",
	}, []string{"state"})

	// Goroutines is the live goroutine count.
	Goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "shorturl_goroutines",
		Help: "Number of live goroutines.",
	})

	// RedisPoolUsage is (total - idle conns) / total conns, i.e. the fraction
	// of the Redis pool currently in use.
	RedisPoolUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "shorturl_redis_pool_usage_ratio",
		Help: "Fraction of the Redis connection pool currently in use.",
	})
)

// Register exposes all custom metrics on the default registry (which already
// includes the standard Go runtime and process collectors).
func Register() {
	prometheus.MustRegister(EncodeDuration, ResolveTotal, Goroutines, RedisPoolUsage)
}

// CollectRuntime periodically refreshes runtime/gauges until ctx is canceled.
func CollectRuntime(ctx context.Context, c *cache.RedisCache, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	collect := func() {
		Goroutines.Set(float64(runtime.NumGoroutine()))

		stats := c.PoolStats()
		if stats.TotalConns > 0 {
			RedisPoolUsage.Set(float64(stats.TotalConns-stats.IdleConns) / float64(stats.TotalConns))
		} else {
			RedisPoolUsage.Set(0)
		}
	}

	collect()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}
