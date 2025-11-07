package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"job-scheduler-fampay/api"
)

const (
	defaultPort            = "8080"
	defaultWorkerCount     = 1000
	defaultLogFile         = "scheduler.log"
	defaultShutdownTimeout = 15 * time.Second
)

type Config struct {
	Port        string
	WorkerCount int
	LogFile     string
	Shutdown    time.Duration
}

func parseFlags() Config {
	port := flag.String("port", defaultPort, "HTTP port to listen on")
	workers := flag.Int("workers", defaultWorkerCount, "Number of worker goroutines")
	logFile := flag.String("logfile", defaultLogFile, "Path to log file")

	flag.Parse()

	if *workers <= 0 {
		log.Fatalf("workers must be greater than zero; got %d", *workers)
	}
	if *logFile == "" {
		log.Fatalf("log file name must not be empty")
	}

	return Config{
		Port:        *port,
		WorkerCount: *workers,
		LogFile:     *logFile,
		Shutdown:    defaultShutdownTimeout,
	}
}

func setupLogger(path string) (*os.File, *log.Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	logger := log.New(file, "", log.LstdFlags|log.Lshortfile)
	return file, logger, nil
}

func run(ctx context.Context, cfg Config, logger *log.Logger) error {
	server := api.NewServer(cfg.Port, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	select {
	case <-ctx.Done():
		logger.Println("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown API server: %w", err)
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("API server exited with error: %w", err)
	}

	return nil
}

func main() {
	cfg := parseFlags()

	logFile, logger, err := setupLogger(cfg.LogFile)
	if err != nil {
		log.Fatalf("failed to configure logger: %v", err)
	}
	defer logFile.Close()

	logger.Printf("starting job scheduler on port %s with %d workers (log file: %s)", cfg.Port, cfg.WorkerCount, cfg.LogFile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Fatalf("service stopped with error: %v", err)
	}

	logger.Println("job-scheduler-fampay service exited cleanly")
}
