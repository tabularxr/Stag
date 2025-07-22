package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tabular/stag-v2/internal/spatial"
	"github.com/tabular/stag-v2/pkg/api"
	"github.com/tabular/stag-v2/pkg/errors"
	"github.com/tabular/stag-v2/pkg/logger"
)

// IngestHandler handles spatial data ingestion
type IngestHandler struct {
	repository *spatial.Repository
	logger     logger.Logger
}

// NewIngestHandler creates a new ingest handler
func NewIngestHandler(repository *spatial.Repository, logger logger.Logger) *IngestHandler {
	return &IngestHandler{
		repository: repository,
		logger:     logger,
	}
}

// Ingest handles POST /api/v1/ingest
func (h *IngestHandler) Ingest(c *gin.Context) {
	var event api.SpatialEvent

	// Bind and validate request
	if err := c.ShouldBindJSON(&event); err != nil {
		h.logger.Warnf("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Additional validation
	if event.SessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id is required",
		})
		return
	}

	if event.EventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "event_id is required",
		})
		return
	}

	// Process the event
	if err := h.repository.Ingest(c.Request.Context(), &event); err != nil {
		// Check if it's an API error
		if apiErr, ok := errors.IsAPIError(err); ok {
			c.JSON(apiErr.StatusCode, gin.H{
				"error": apiErr.Message,
				"code":  apiErr.Code,
			})
			return
		}

		// Generic error
		h.logger.Errorf("Failed to ingest event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to ingest event",
		})
		return
	}

	// Success response
	c.JSON(http.StatusOK, gin.H{
		"message": "Event ingested successfully",
		"event_id": event.EventID,
		"anchors_count": len(event.Anchors),
		"meshes_count": len(event.Meshes),
	})
}