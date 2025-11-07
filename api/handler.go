package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes HTTP endpoints for the API service.
type Handler struct {
	logger *log.Logger
}

// NewHandler constructs a new Handler instance.
func NewHandler(logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{logger: logger}
}

// Health returns the current health status of the service.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
