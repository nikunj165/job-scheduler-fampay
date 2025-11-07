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

