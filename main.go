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

	return Config{
		Port:        *port,
		WorkerCount: *workers,
		LogFile:     *logFile,
		Shutdown:    defaultShutdownTimeout,
	}
}

func main() {
	cfg := parseFlags()

	if cfg.LogFile == "" {
		log.Fatalf("log file name must not be empty")
	}

	logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to create or open log file: %v", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("starting job scheduler on port %s with %d workers (log file: %s)", cfg.Port, cfg.WorkerCount, cfg.LogFile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := api.NewServer(cfg.Port, log.Default())

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Fatalf("API server error: %v", err)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("failed to shutdown API server: %v", err)
	}

	if err := <-errCh; err != nil {
		log.Fatalf("API server exited with error: %v", err)
	}

	fmt.Println("job-scheduler-fampay service exited cleanly")
}
