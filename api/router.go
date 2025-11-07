package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultRateLimitRequests = 60
	defaultRateLimitWindow   = time.Minute
)

// RouterOptions configure the behavior of the API router.
type RouterOptions struct {
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

// NewRouter wires routes to handler functions and wraps them with middleware.
func NewRouter(handler *Handler, opts RouterOptions) *gin.Engine {
	if handler == nil {
		panic("handler must not be nil")
	}

	if opts.RateLimitRequests <= 0 {
		opts.RateLimitRequests = defaultRateLimitRequests
	}
	if opts.RateLimitWindow <= 0 {
		opts.RateLimitWindow = defaultRateLimitWindow
	}

	engine := gin.New()

	if handler.logger != nil {
		engine.Use(gin.LoggerWithWriter(handler.logger.Writer()))
	} else {
		engine.Use(gin.Logger())
	}
	engine.Use(gin.Recovery())
	engine.Use(rateLimitMiddleware(handler.logger, opts.RateLimitRequests, opts.RateLimitWindow))
	engine.Use(corsMiddleware())

	engine.GET("/healthz", handler.Health)

	return engine
}
