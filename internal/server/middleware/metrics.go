package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tabular/stag-v2/internal/metrics"
)

// Metrics returns a middleware that records HTTP metrics
func Metrics(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Get endpoint (without params)
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = "unknown"
		}

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		// Record request count
		m.HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()

		// Record request duration
		m.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
	}
}