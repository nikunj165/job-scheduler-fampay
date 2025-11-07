package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Server wraps an http.Server and exposes lifecycle helpers for the API service.
type Server struct {
	logger *log.Logger
	http   *http.Server
}

// NewServer constructs a Server configured to listen on the provided port with the
// built-in router that exposes a health check endpoint.
func NewServer(port string, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}

	handler := NewHandler(logger, nil)
	router := NewRouter(handler, RouterOptions{})

	return &Server{
		logger: logger,
		http: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20, // 1 MB
		},
	}
}

// Start begins serving HTTP requests. It blocks until the server stops.
func (s *Server) Start() error {
	s.logger.Printf("API server listening on %s", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	s.logger.Println("API server stopped")
	return nil
}

// Shutdown attempts a graceful shutdown of the HTTP server within the provided context.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Println("API server shutting down")
	return s.http.Shutdown(ctx)
}
