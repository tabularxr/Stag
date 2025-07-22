package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tabular/stag-v2/pkg/api"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{
		version: version,
	}
}

// Health returns the service health status
func (h *HealthHandler) Health(c *gin.Context) {
	response := api.HealthResponse{
		Status:    "healthy",
		Version:   h.version,
		Timestamp: time.Now(),
		Database:  "connected",
	}

	c.JSON(http.StatusOK, response)
}