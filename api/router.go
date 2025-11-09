package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Create Gin router with default middleware
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Apply custom middleware
	router.Use(corsMiddleware())
	router.Use(prometheusMiddleware())
	router.Use(rateLimitMiddleware(handler.logger, opts.RateLimitRequests, opts.RateLimitWindow))

	// Health check endpoint
	router.GET("/healthz", handler.Health)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Job management endpoints
		jobs := v1.Group("/jobs")
		{
			jobs.POST("", handler.CreateJob)       // Create a new job
			jobs.GET("", handler.GetAllJobs)       // Get all jobs (with pagination)
			jobs.GET("/:id", handler.GetJob)       // Get a specific job
			jobs.PUT("/:id", handler.UpdateJob)    // Update a job
			jobs.DELETE("/:id", handler.DeleteJob) // Delete a job

			// Job-specific operations
			jobs.GET("/:id/executions", handler.GetJobExecutions) // Get execution history
			jobs.GET("/:id/stats", handler.GetJobStats)           // Get job statistics
		}

		// Scheduler statistics
		v1.GET("/scheduler/stats", handler.GetSchedulerStats)
	}

	return router
}

// SetupRoutes configures all routes for the API server
func SetupRoutes(logger *log.Logger) *gin.Engine {
	handler := NewHandler(logger, nil) // Repository will be initialized in handler
	return NewRouter(handler, RouterOptions{})
}
