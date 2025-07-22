package server

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tabular/stag-v2/internal/config"
	"github.com/tabular/stag-v2/internal/metrics"
	"github.com/tabular/stag-v2/internal/server/handlers"
	"github.com/tabular/stag-v2/internal/server/middleware"
	"github.com/tabular/stag-v2/internal/server/websocket"
	"github.com/tabular/stag-v2/internal/spatial"
	"github.com/tabular/stag-v2/pkg/logger"
)

// Version is the service version
const Version = "2.0.0"

// New creates a new server instance
func New(cfg *config.Config, repository *spatial.Repository, logger logger.Logger, metrics *metrics.Metrics) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Metrics(metrics))

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // TODO: Configure for production
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(repository, logger, metrics)
	go wsHub.Run()

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(Version)
	ingestHandler := handlers.NewIngestHandler(repository, logger)
	queryHandler := handlers.NewQueryHandler(repository, logger)
	wsHandler := handlers.NewWebSocketHandler(wsHub, logger)

	// Health check endpoint
	router.GET("/health", healthHandler.Health)

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		router.GET(cfg.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Ingestion
		v1.POST("/ingest", ingestHandler.Ingest)

		// Queries
		v1.GET("/query", queryHandler.Query)
		v1.GET("/anchors/:id", queryHandler.GetAnchor)

		// WebSocket
		v1.GET("/ws", wsHandler.HandleWebSocket)

		// Metrics
		v1.GET("/metrics", func(c *gin.Context) {
			info, err := repository.GetMetrics(c.Request.Context())
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get metrics"})
				return
			}
			info.ActiveConnections = wsHub.GetActiveConnections()
			c.JSON(200, info)
		})
	}

	return router
}