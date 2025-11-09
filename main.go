package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"job-scheduler-fampay/api"
	"job-scheduler-fampay/executor"
	"job-scheduler-fampay/repository"
	"job-scheduler-fampay/scheduler"
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
	Optimized   bool
}

func parseFlags() Config {
	port := flag.String("port", defaultPort, "HTTP port to listen on")
	workers := flag.Int("workers", defaultWorkerCount, "Number of worker goroutines")
	logFile := flag.String("logfile", defaultLogFile, "Path to log file")
	optimized := flag.Bool("optimized", false, "Use optimized executor for high throughput (1000+ jobs/sec)")

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
		Optimized:   *optimized,
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
	repo := repository.NewMemoryRepository()

	// Choose executor based on optimization flag
	var jobExecutor scheduler.Executor

	if cfg.Optimized {
		logger.Printf("Using OPTIMIZED executor for high throughput")
		optCfg := executor.DefaultOptimizedConfig()
		optCfg.WorkerCount = cfg.WorkerCount

		// Allow configuring timeout via environment variable
		if timeoutStr := os.Getenv("JOB_TIMEOUT_SECONDS"); timeoutStr != "" {
			if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
				optCfg.RequestTimeout = time.Duration(timeout) * time.Second
				logger.Printf("Using custom job timeout: %v seconds", timeout)
			}
		}

		optimizedExec := executor.NewOptimizedJobExecutor(repo, optCfg)
		optimizedExec.Start(ctx)
		defer optimizedExec.Stop()
		jobExecutor = optimizedExec
	} else {
		logger.Printf("Using standard executor")
		execCfg := executor.DefaultJobExecutorConfig()
		execCfg.WorkerCount = cfg.WorkerCount

		// Allow configuring timeout via environment variable
		if timeoutStr := os.Getenv("JOB_TIMEOUT_SECONDS"); timeoutStr != "" {
			if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
				execCfg.RequestTimeout = time.Duration(timeout) * time.Second
				logger.Printf("Using custom job timeout: %v seconds", timeout)
			}
		}

		standardExec := executor.NewJobExecutor(repo, execCfg)
		standardExec.Start(ctx)
		defer standardExec.Stop()
		jobExecutor = standardExec
	}

	schedCfg := scheduler.DefaultConfig()
	jobScheduler := scheduler.New(repo, jobExecutor, logger, schedCfg)
	jobScheduler.Start(ctx)
	defer jobScheduler.Stop()

	server := api.NewServer(cfg.Port, logger, repo)

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("HTTP server starting on port %s", cfg.Port)
		errCh <- server.Start()
	}()

	select {
	case <-ctx.Done():
		logger.Println("received shutdown signal (SIGINT/SIGTERM)")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}

	logger.Printf("initiating graceful shutdown (timeout: %v)", cfg.Shutdown)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown API server: %w", err)
	}

	// Wait for server goroutine to finish
	if err := <-errCh; err != nil {
		return fmt.Errorf("API server exited with error: %w", err)
	}

	logger.Println("graceful shutdown completed successfully")
	return nil
}

func main() {
	cfg := parseFlags()

	logFile, logger, err := setupLogger(cfg.LogFile)
	if err != nil {
		log.Fatalf("failed to configure logger: %v", err)
	}
	defer logFile.Close()

	logger.Printf("=== Job Scheduler Service Starting ===")
	logger.Printf("Configuration: port=%s, workers=%d, logfile=%s, optimized=%v",
		cfg.Port, cfg.WorkerCount, cfg.LogFile, cfg.Optimized)
	logger.Printf("Process ID: %d", os.Getpid())
	logger.Printf("Registering signal handlers for graceful shutdown (SIGINT, SIGTERM)")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Fatalf("service stopped with error: %v", err)
	}

	logger.Println("=== Job Scheduler Service Stopped Successfully ===")
}
