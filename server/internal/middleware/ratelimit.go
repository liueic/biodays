package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens float64
	last   time.Time
	seen   time.Time
}

type RateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	perMinute   int
	perSecond   float64
	burst       float64
	lastSweep   time.Time
	lastSweepMu sync.Mutex
}

func NewRateLimiter(perMinute int) *RateLimiter {
	return &RateLimiter{
		buckets:   make(map[string]*bucket),
		perMinute: perMinute,
		perSecond: float64(perMinute) / 60.0,
		burst:     float64(perMinute),
	}
}

func (l *RateLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.buckets[key]
	if !ok {
		entry = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = entry
	} else {
		elapsed := now.Sub(entry.last).Seconds()
		if elapsed > 0 {
			entry.tokens = min(l.burst, entry.tokens+elapsed*l.perSecond)
			entry.last = now
		}
	}
	entry.seen = now

	if entry.tokens >= 1 {
		entry.tokens--
		return true
	}
	return false
}

func (l *RateLimiter) sweep(now time.Time) {
	l.lastSweepMu.Lock()
	if now.Sub(l.lastSweep) < 5*time.Minute {
		l.lastSweepMu.Unlock()
		return
	}
	l.lastSweep = now
	l.lastSweepMu.Unlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.buckets {
		if now.Sub(entry.seen) > 10*time.Minute {
			delete(l.buckets, key)
		}
	}
}

func RateLimit(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if c.Request.Method == http.MethodOptions || path == "/healthz" || path == "/readyz" || strings.HasPrefix(path, "/swagger") {
			c.Next()
			return
		}

		now := time.Now()
		limiter.sweep(now)

		c.Header("X-RateLimit-Limit", strconv.Itoa(limiter.perMinute))
		if !limiter.Allow(c.ClientIP(), now) {
			c.Header("Retry-After", "60")
			c.Header("Cache-Control", "no-store")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMITED",
					"message": "rate limit exceeded; retry later",
				},
			})
			return
		}
		c.Next()
	}
}
