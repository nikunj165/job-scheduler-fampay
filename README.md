# Job Scheduler (FamPay)

## Prerequisites
- Go 1.21+
- Git

## Setup
```bash
git clone https://github.com/nikunj165/job-scheduler-fampay.git
cd job-scheduler-fampay
go mod tidy
```

## Run Locally
```bash
go run . --port=8080 --workers=10 --logfile=scheduler.log
```

Skip flags to use defaults.

### Health Check
```bash
curl http://localhost:8080/healthz
```

## Build
```bash
go build ./...
```

## Run Tests
```bash
go test ./... -count=1
```

## Scheduler & Executor

- The `scheduler` package polls active jobs at startup and hands due runs to the executor.
- The `executor` package performs HTTP requests for each job and records execution results.

Scaffolded tests for these components live in `scheduler/scheduler_test.go` and `executor/executor_test.go`.

Use the helper script to run formatting, vetting, and tests in one go:

```bash
bash scripts/test_service.sh
```

