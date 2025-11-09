# Job Scheduler Service

A high-performance, distributed job scheduling service built in Go that executes HTTP webhooks on CRON schedules.

## Features

- **CRON-based Scheduling**: 6-field CRON expressions with second precision
- **HTTP Webhook Execution**: POST requests to configured endpoints
- **Delivery Guarantees**: AT_LEAST_ONCE and AT_MOST_ONCE semantics
- **High Performance**: Optimized executor supporting 1000+ jobs/second
- **RESTful API**: Complete CRUD operations for job management
- **Execution History**: Track execution results and performance
- **Observability**: Prometheus metrics and Grafana dashboards
- **Graceful Shutdown**: Clean service termination

## Quick Start

### Using Docker (Recommended)

```bash
# Clone and start
git clone https://github.com/nikunj165/job-scheduler-fampay.git
cd job-scheduler-fampay
docker-compose up -d

# Access services
- API: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)
```

### Using Go

```bash
# Clone and build
git clone https://github.com/nikunj165/job-scheduler-fampay.git
cd job-scheduler-fampay
go mod tidy
go build

# Run
./job-scheduler-fampay --port=8080
```

## API Usage

### Create a Job
```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "*/30 * * * * *",
    "api": "https://httpcan.org/post",
    "type": "ATLEAST_ONCE"
  }'
```

### List Jobs
```bash
curl "http://localhost:8080/api/v1/jobs?limit=10&status=ACTIVE"
```

### Get Job Details
```bash
curl http://localhost:8080/api/v1/jobs/{job_id}
```

### Update Job
```bash
curl -X PUT http://localhost:8080/api/v1/jobs/{job_id} \
  -H "Content-Type: application/json" \
  -d '{"schedule": "0 */5 * * * *", "status": "INACTIVE"}'
```

### Delete Job
```bash
curl -X DELETE http://localhost:8080/api/v1/jobs/{job_id}
```

### Get Execution History
```bash
curl http://localhost:8080/api/v1/jobs/{job_id}/executions
```

### Get Statistics
```bash
# Job stats
curl http://localhost:8080/api/v1/jobs/{job_id}/stats

# System stats
curl "http://localhost:8080/api/v1/stats?from=2024-01-01T00:00:00Z"
```

## Configuration

### Command-line Flags
- `--port`: HTTP port (default: 8080)
- `--workers`: Worker count (default: 1000)
- `--logfile`: Log file path (default: scheduler.log)
- `--optimized`: Enable optimized executor

### Environment Variables
- `JOB_TIMEOUT_SECONDS`: Job timeout (default: 120)
- `API_RATE_LIMIT_REQUESTS`: Rate limit (default: 1000)
- `API_RATE_LIMIT_WINDOW_SECONDS`: Rate limit window (default: 1)

### Examples

```bash
# Standard executor
go run . --port=8080 --workers=10

# Optimized executor with custom timeout
JOB_TIMEOUT_SECONDS=90 go run . --optimized

# Docker with custom config
docker run -p 8080:8080 \
  -e API_RATE_LIMIT_REQUESTS=5000 \
  -e JOB_TIMEOUT_SECONDS=90 \
  job-scheduler-fampay:latest
```

## Testing

### Unit Tests
```bash
# Run all tests
go test ./...

# With race detector
go test -race -count=1 ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests
```bash
# Start service and run tests
./scripts/test_service.sh

# With integration tests
RUN_INTEGRATION=true ./scripts/test_service.sh
```

### Performance Tests
```bash
# Start optimized service
go run . --optimized &

# Run load test
./scripts/load_test.sh

# Custom load
TOTAL_JOBS=5000 ./scripts/load_test.sh

# Run benchmark
./scripts/benchmark.sh
```

## Monitoring

### Prometheus Metrics

Available at `http://localhost:8080/metrics`:

- `jobs_created_total`, `jobs_deleted_total`, `jobs_active`
- `job_executions_total{status}` - Success/failure counters
- `job_execution_duration_seconds` - Execution duration histogram
- `job_execution_response_time_ms` - Response time histogram
- `scheduler_jobs_dispatched_total` - Dispatch counter
- `executor_queue_depth` - Current queue size
- `http_requests_total{method,endpoint,status}` - API request metrics

### Grafana

Pre-configured dashboard showing:
- Job creation and execution rates
- HTTP request latencies (p50, p95, p99)
- Success vs failure rates
- Queue depth and worker utilization

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP
       ▼
┌─────────────────────────────────────┐
│         API Layer (Gin)             │
│  • CRUD operations                  │
│  • Request validation               │
│  • Metrics middleware               │
└─────────────┬───────────────────────┘
              │
              ▼
┌─────────────────────────────────────┐
│    Repository (In-Memory)           │
│  • Thread-safe storage              │
│  • Job & execution management       │
└─────┬───────────────────┬───────────┘
      │                   │
      ▼                   ▼
┌─────────────┐    ┌─────────────┐
│  Scheduler  │    │  Executor   │
│  • Poll     │───▶│  • Workers  │
│  • Dispatch │    │  • HTTP     │
└─────────────┘    └─────────────┘
```

### Components

- **API**: HTTP handlers, routing, middleware
- **Scheduler**: Polls active jobs, dispatches due jobs
- **Executor**: Worker pool for concurrent HTTP execution
- **CRON Parser**: Schedule validation and next-run calculation
- **Repository**: Thread-safe in-memory storage

## Performance

| Executor   | Workers | Queue  | Throughput       |
|------------|---------|--------|------------------|
| Standard   | 10      | 100    | ~100-200 jobs/s  |
| Optimized  | 1000    | 10,000 | 1000+ jobs/s     |

The optimized executor includes:
- HTTP/2 with connection pooling
- Asynchronous execution recording
- Real-time metrics reporting
- Non-blocking job submission

## Project Structure

```
job-scheduler-fampay/
├── api/              # HTTP handlers and routing
├── cron/             # CRON expression parser
├── executor/         # Job execution engine
├── metrics/          # Prometheus instrumentation
├── models/           # Data models
├── repository/       # Storage layer
├── scheduler/        # Job scheduling logic
├── scripts/          # Test scripts
├── grafana/          # Grafana configuration
├── prometheus/       # Prometheus configuration
├── Dockerfile        # Container image
├── docker-compose.yml
└── main.go
```

## Development

### Prerequisites
- Go 1.23+
- Docker & Docker Compose (optional)
- jq (for testing scripts)

### Build
```bash
go build -o job-scheduler-fampay
```

### Format & Lint
```bash
gofmt -w .
go vet ./...
```

### Run Tests
```bash
./scripts/test_service.sh
```

## Docker Commands

```bash
# Build image
docker build -t job-scheduler-fampay:latest .

# Run standard
docker-compose up -d scheduler

# Run optimized
docker-compose --profile optimized up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down

# Clean up volumes
docker-compose down -v
```

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request
