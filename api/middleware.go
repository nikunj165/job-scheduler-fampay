package api

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"job-scheduler-fampay/metrics"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		headers.Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, X-Requested-With")
		headers.Set("Access-Control-Max-Age", "600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func rateLimitMiddleware(logger *log.Logger, limit int, window time.Duration) gin.HandlerFunc {
	if logger == nil {
		logger = log.Default()
	}

	rl := &rateLimiter{
		limit:       limit,
		window:      window,
		windowStart: time.Now(),
		requests:    make(map[string]int),
	}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if clientIP == "" {
			clientIP = "unknown"
		}

		if !rl.Allow(clientIP) {
			logger.Printf("rate limit exceeded for %s %s (ip=%s)", c.Request.Method, c.Request.URL.Path, clientIP)
			metrics.HTTPRateLimitExceeded.Inc()
			c.Header("Retry-After", rl.window.String())
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		c.Next()
	}
}

// prometheusMiddleware tracks HTTP request metrics
func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

type rateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	windowStart time.Time
	requests    map[string]int
}

func (rl *rateLimiter) Allow(clientIP string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if now.Sub(rl.windowStart) >= rl.window {
		rl.windowStart = now
		rl.requests = make(map[string]int)
	}

	if rl.requests[clientIP] >= rl.limit {
		return false
	}

	rl.requests[clientIP]++
	return true
}
