package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/tabular/stag-v2/internal/config"
	"github.com/tabular/stag-v2/internal/database"
	"github.com/tabular/stag-v2/internal/metrics"
	"github.com/tabular/stag-v2/internal/server"
	"github.com/tabular/stag-v2/internal/spatial"
	"github.com/tabular/stag-v2/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()
	log.Info("Starting STAG v2...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warnf("Invalid log level %s, using info", cfg.LogLevel)
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Initialize metrics
	metricsCollector := metrics.New()

	// Connect to ArangoDB
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize spatial repository
	repository := spatial.NewRepository(db, log, metricsCollector)

	// Set Gin mode
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create server
	srv := server.New(cfg, repository, log, metricsCollector)

	// Start server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Infof("Server starting on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server stopped")
}

func init() {
	// Set configuration defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("database.url", "http://localhost:8529")
	viper.SetDefault("database.name", "stag")
	viper.SetDefault("database.username", "root")
	viper.SetDefault("log_level", "info")

	// Bind environment variables
	viper.SetEnvPrefix("STAG")
	viper.AutomaticEnv()
}