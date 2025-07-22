package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tabular/stag-v2/pkg/logger"
)

// Logger returns a middleware that logs HTTP requests
func Logger(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Log request
		if raw != "" {
			path = path + "?" + raw
		}

		log.WithFields(map[string]interface{}{
			"method":     c.Request.Method,
			"path":       path,
			"status":     statusCode,
			"latency_ms": latency.Milliseconds(),
			"client_ip":  c.ClientIP(),
			"error":      c.Errors.String(),
		}).Info("HTTP request")
	}
}