# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o job-scheduler-fampay .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 scheduler && \
    adduser -D -u 1000 -G scheduler scheduler

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/job-scheduler-fampay .

# Change ownership
RUN chown -R scheduler:scheduler /app

# Switch to non-root user
USER scheduler

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Default command (can be overridden)
ENTRYPOINT ["./job-scheduler-fampay"]

# Default arguments
CMD ["--port=8080", "--workers=1000", "--logfile=/dev/stdout"]

