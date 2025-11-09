package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Job metrics
	JobsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_created_total",
		Help: "Total number of jobs created",
	})

	JobsDeletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_deleted_total",
		Help: "Total number of jobs deleted",
	})

	JobsActiveGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "jobs_active",
		Help: "Current number of active jobs",
	})

	// Execution metrics
	JobExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_executions_total",
			Help: "Total number of job executions",
		},
		[]string{"status"}, // success, failure
	)

	JobExecutionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "job_execution_duration_seconds",
		Help:    "Job execution duration in seconds",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
	})

	JobExecutionResponseTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "job_execution_response_time_ms",
		Help:    "HTTP response time for job execution in milliseconds",
		Buckets: prometheus.ExponentialBuckets(10, 2, 12), // 10ms to ~40s
	})

	// Scheduler metrics
	SchedulerPollsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_polls_total",
		Help: "Total number of scheduler polls",
	})

	SchedulerJobsDispatchedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_jobs_dispatched_total",
		Help: "Total number of jobs dispatched by scheduler",
	})

	SchedulerPollDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "scheduler_poll_duration_seconds",
		Help:    "Scheduler poll duration in seconds",
		Buckets: prometheus.LinearBuckets(0.001, 0.001, 10), // 1ms to 10ms
	})

	// Executor metrics
	ExecutorQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "executor_queue_depth",
		Help: "Current depth of executor job queue",
	})

	ExecutorWorkersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "executor_workers_active",
		Help: "Number of active executor workers",
	})

	// API metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	HTTPRateLimitExceeded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "http_rate_limit_exceeded_total",
		Help: "Total number of rate limit exceeded responses",
	})

	// Repository metrics
	RepositoryOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "repository_operations_total",
			Help: "Total number of repository operations",
		},
		[]string{"operation", "status"}, // create, read, update, delete / success, error
	)

	RepositoryOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "repository_operation_duration_seconds",
			Help:    "Repository operation duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.00001, 2, 10), // 10μs to ~10ms
		},
		[]string{"operation"},
	)
)

