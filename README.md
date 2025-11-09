# Job Scheduler Service

A high-performance, distributed job scheduling service built in Go that executes HTTP webhooks on CRON schedules. Designed to handle thousands of concurrent job executions with configurable reliability guarantees.

## 🚀 Features

- **CRON-based Scheduling**: Support for 6-field CRON expressions (including seconds)
- **HTTP Webhook Execution**: Execute jobs via HTTP POST requests to specified endpoints
- **Delivery Guarantees**: Support for AT_LEAST_ONCE and AT_MOST_ONCE execution semantics
- **High Performance**: Optimized executor capable of handling 1000+ jobs/second
- **RESTful API**: Complete CRUD operations for job management
- **Execution History**: Track job execution results, response times, and success rates
- **Real-time Statistics**: Monitor job performance and system health
- **Graceful Shutdown**: Clean service termination with proper resource cleanup
- **In-Memory Storage**: Fast, lightweight storage (easily extensible to databases)

## 📋 Prerequisites

- Go 1.21 or higher
- Git
- curl or similar HTTP client (for testing)
- jq (optional, for pretty JSON output)

## 🛠️ Installation

```bash
# Clone the repository
git clone https://github.com/nikunj165/job-scheduler-fampay.git
cd job-scheduler-fampay

# Install dependencies
go mod tidy

# Build the service
go build -o job-scheduler-fampay
```

## 🏃 Running the Service

### Quick Start

```bash
# Run with default settings
go run .

# Or run the built binary
./job-scheduler-fampay
```

### Configuration Options

```bash
# Run with custom configuration
go run . \
  --port=8080 \
  --workers=100 \
  --logfile=scheduler.log \
  --optimized

# Environment variables
export JOB_TIMEOUT_SECONDS=90  # Set job execution timeout
./job-scheduler-fampay --optimized
```

**Command-line Flags:**
- `--port`: HTTP server port (default: 8080)
- `--workers`: Number of executor workers (default: 10, optimized: 1000)
- `--logfile`: Path to log file (default: stdout)
- `--optimized`: Use optimized executor for high-throughput scenarios

## 📡 API Endpoints

### Health Check
```bash
curl http://localhost:8080/healthz
```

### Create a Job
```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "*/30 * * * * *",
    "api": "https://httpbin.org/post",
    "type": "ATLEAST_ONCE"
  }'
```

### Get All Jobs
```bash
# List all jobs with pagination
curl "http://localhost:8080/api/v1/jobs?limit=10&offset=0"

# Filter by status
curl "http://localhost:8080/api/v1/jobs?status=ACTIVE"
```

### Get Job Details
```bash
curl http://localhost:8080/api/v1/jobs/{job_id}
```

### Update Job
```bash
curl -X PATCH http://localhost:8080/api/v1/jobs/{job_id} \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "0 */5 * * * *",
    "status": "INACTIVE"
  }'
```

### Delete Job
```bash
curl -X DELETE http://localhost:8080/api/v1/jobs/{job_id}
```

### Get Job Executions
```bash
# Returns last 10 executions
curl http://localhost:8080/api/v1/jobs/{job_id}/executions
```

### Get Job Statistics
```bash
curl http://localhost:8080/api/v1/jobs/{job_id}/stats
```

### Get System Statistics
```bash
# Get scheduler stats for date range
curl "http://localhost:8080/api/v1/stats?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z"
```

## 🧪 Testing

### Run Unit Tests
```bash
# Run all tests
go test ./... -count=1

# Run with race detector
go test -race ./... -count=1

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Test Suite
```bash
# Run comprehensive test suite (format, vet, unit tests)
./scripts/test_service.sh

# Run integration tests
RUN_INTEGRATION=true ./scripts/test_service.sh
```

### Run Integration Tests Only
```bash
# Start the service first
go run . &

# Run integration tests
./scripts/integration_test.sh

# Stop the service
pkill -f "go run ."
```

## 🏗️ Architecture

### Core Components

1. **API Layer** (`/api`)
   - HTTP handlers for RESTful endpoints
   - Request validation and response formatting
   - Middleware for CORS, logging, and error handling

2. **Scheduler** (`/scheduler`)
   - Polls active jobs at configurable intervals
   - Determines which jobs are due for execution
   - Dispatches jobs to the executor

3. **Executor** (`/executor`)
   - Worker pool for concurrent job execution
   - HTTP client with configurable timeouts
   - Records execution results and metrics

4. **CRON Parser** (`/cron`)
   - Validates 6-field CRON expressions
   - Calculates next execution times
   - Supports standard CRON syntax with seconds

5. **Repository** (`/repository`)
   - In-memory storage implementation
   - Thread-safe operations with mutex locks
   - Easily extensible to database backends

### Job Execution Flow

```
1. Client creates job via API
2. Scheduler polls for active jobs
3. Scheduler identifies due jobs
4. Executor receives job from scheduler
5. Executor makes HTTP POST request
6. Executor records execution result
7. Client queries execution history
```

## 📊 Performance

### Standard Executor
- 10 workers by default
- Suitable for moderate workloads
- 30-second request timeout

### Optimized Executor
- 1000 workers
- 10,000 job queue capacity
- HTTP/2 with connection pooling
- 120-second request timeout
- Capable of 1000+ jobs/second

## 🔧 Development

### Project Structure
```
job-scheduler-fampay/
├── api/              # HTTP handlers and routing
├── cron/             # CRON expression parser
├── executor/         # Job execution engine
├── models/           # Data models
├── repository/       # Storage layer
├── scheduler/        # Job scheduling logic
├── scripts/          # Test and utility scripts
└── main.go          # Application entry point
```

### Adding New Features

1. **Database Support**: Implement the `JobRepository` interface
2. **Authentication**: Add middleware to the API router
3. **Metrics**: Integrate Prometheus or similar monitoring
4. **Message Queue**: Replace in-memory queue with RabbitMQ/Kafka

## 📈 Monitoring

The service logs important events including:
- Job creation/updates/deletions
- Job dispatches with timing information
- Execution results and failures
- System errors and warnings

Configure logging output with the `--logfile` flag.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
