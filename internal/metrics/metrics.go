package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	
	// WebSocket metrics
	WSConnectionsActive *prometheus.GaugeVec
	WSMessagesTotal     *prometheus.CounterVec
	
	// Database metrics
	DBOperationsTotal   *prometheus.CounterVec
	DBOperationDuration *prometheus.HistogramVec
	
	// Business metrics
	AnchorsTotal         *prometheus.CounterVec
	MeshesTotal          *prometheus.CounterVec
	CompressionRatio     *prometheus.GaugeVec
	StorageSizeBytes     *prometheus.GaugeVec
	MeshDedupSavedBytes  *prometheus.CounterVec
}

// New creates a new metrics instance
func New() *Metrics {
	return &Metrics{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stag_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "stag_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		
		// WebSocket metrics
		WSConnectionsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "stag_ws_connections_active",
				Help: "Number of active WebSocket connections",
			},
			[]string{"session_id"},
		),
		WSMessagesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stag_ws_messages_total",
				Help: "Total number of WebSocket messages",
			},
			[]string{"direction", "type", "status"},
		),
		
		// Database metrics
		DBOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stag_db_operations_total",
				Help: "Total number of database operations",
			},
			[]string{"operation", "collection", "status"},
		),
		DBOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "stag_db_operation_duration_seconds",
				Help:    "Database operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "collection"},
		),
		
		// Business metrics
		AnchorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stag_anchors_total",
				Help: "Total number of anchors processed",
			},
			[]string{"session_id", "operation"},
		),
		MeshesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stag_meshes_total",
				Help: "Total number of meshes processed",
			},
			[]string{"session_id", "type", "operation"},
		),
		CompressionRatio: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "stag_compression_ratio",
				Help: "Current compression ratio",
			},
			[]string{"session_id"},
		),
		StorageSizeBytes: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "stag_storage_size_bytes",
				Help: "Total storage size in bytes",
			},
			[]string{"type"},
		),
		MeshDedupSavedBytes: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stag_mesh_dedup_saved_bytes",
				Help: "Bytes saved through mesh deduplication",
			},
			[]string{"session_id"},
		),
	}
}