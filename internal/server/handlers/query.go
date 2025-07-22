package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tabular/stag-v2/internal/spatial"
	"github.com/tabular/stag-v2/pkg/api"
	"github.com/tabular/stag-v2/pkg/errors"
	"github.com/tabular/stag-v2/pkg/logger"
)

// QueryHandler handles spatial queries
type QueryHandler struct {
	repository *spatial.Repository
	logger     logger.Logger
}

// NewQueryHandler creates a new query handler
func NewQueryHandler(repository *spatial.Repository, logger logger.Logger) *QueryHandler {
	return &QueryHandler{
		repository: repository,
		logger:     logger,
	}
}

// Query handles GET /api/v1/query
func (h *QueryHandler) Query(c *gin.Context) {
	var params api.QueryParams

	// Bind query parameters
	if err := c.ShouldBindQuery(&params); err != nil {
		h.logger.Warnf("Invalid query parameters: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid query parameters",
			"details": err.Error(),
		})
		return
	}

	// Validate parameters
	if params.SessionID == "" && params.AnchorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Either session_id or anchor_id must be provided",
		})
		return
	}

	if params.AnchorID != "" && params.Radius <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "radius must be provided when using anchor_id",
		})
		return
	}

	// Set default limit
	if params.Limit <= 0 {
		params.Limit = 100
	} else if params.Limit > 1000 {
		params.Limit = 1000
	}

	// Execute query
	response, err := h.repository.Query(c.Request.Context(), &params)
	if err != nil {
		// Check if it's an API error
		if apiErr, ok := errors.IsAPIError(err); ok {
			c.JSON(apiErr.StatusCode, gin.H{
				"error": apiErr.Message,
				"code":  apiErr.Code,
			})
			return
		}

		// Generic error
		h.logger.Errorf("Failed to execute query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to execute query",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetAnchor handles GET /api/v1/anchors/:id
func (h *QueryHandler) GetAnchor(c *gin.Context) {
	anchorID := c.Param("id")
	if anchorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "anchor ID is required",
		})
		return
	}

	// Query for specific anchor
	params := &api.QueryParams{
		AnchorID: anchorID,
		Limit:    1,
	}

	response, err := h.repository.Query(c.Request.Context(), params)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			c.JSON(apiErr.StatusCode, gin.H{
				"error": apiErr.Message,
				"code":  apiErr.Code,
			})
			return
		}

		h.logger.Errorf("Failed to get anchor: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get anchor",
		})
		return
	}

	if len(response.Anchors) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Anchor not found",
		})
		return
	}

	c.JSON(http.StatusOK, response.Anchors[0])
}