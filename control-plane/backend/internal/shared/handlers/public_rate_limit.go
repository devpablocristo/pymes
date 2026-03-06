package handlers

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type publicRateLimiter struct {
	mu    sync.Mutex
	hits  map[string][]time.Time
	limit int
}

func NewPublicRateLimit(limit int) gin.HandlerFunc {
	if limit <= 0 {
		limit = 30
	}
	limiter := &publicRateLimiter{
		hits:  make(map[string][]time.Time),
		limit: limit,
	}

	return func(c *gin.Context) {
		key := clientIP(c)
		now := time.Now().UTC()

		limiter.mu.Lock()
		defer limiter.mu.Unlock()

		windowStart := now.Add(-1 * time.Minute)
		history := limiter.hits[key]
		filtered := make([]time.Time, 0, len(history)+1)
		for _, ts := range history {
			if ts.After(windowStart) {
				filtered = append(filtered, ts)
			}
		}

		if len(filtered) >= limiter.limit {
			limiter.hits[key] = filtered
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		filtered = append(filtered, now)
		limiter.hits[key] = filtered
		c.Next()
	}
}

func clientIP(c *gin.Context) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		if raw := strings.TrimSpace(c.GetHeader(header)); raw != "" {
			if header == "X-Forwarded-For" {
				parts := strings.Split(raw, ",")
				if len(parts) > 0 {
					return strings.TrimSpace(parts[0])
				}
			}
			return raw
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if c.ClientIP() != "" {
		return c.ClientIP()
	}
	return "unknown"
}
