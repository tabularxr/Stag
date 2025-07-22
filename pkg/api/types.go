package api

import (
	"encoding/json"
	"time"
)

// SpatialEvent represents a batch of spatial data from a session
type SpatialEvent struct {
	SessionID string   `json:"session_id" binding:"required"`
	EventID   string   `json:"event_id" binding:"required"`
	Timestamp int64    `json:"timestamp" binding:"required"`
	Anchors   []Anchor `json:"anchors"`
	Meshes    []Mesh   `json:"meshes"`
}

// Anchor represents a spatial anchor with pose and metadata
type Anchor struct {
	ID        string                 `json:"id" binding:"required"`
	SessionID string                 `json:"session_id" binding:"required"`
	Pose      Pose                   `json:"pose" binding:"required"`
	Timestamp int64                  `json:"timestamp" binding:"required"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Pose represents position and orientation in 3D space
type Pose struct {
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Z        float64   `json:"z"`
	Rotation []float64 `json:"rotation"` // Quaternion [x, y, z, w]
}

// Mesh represents 3D geometry data
type Mesh struct {
	ID               string `json:"id" binding:"required"`
	AnchorID         string `json:"anchor_id" binding:"required"`
	Vertices         []byte `json:"vertices,omitempty"`         // Compressed vertex data
	Faces            []byte `json:"faces,omitempty"`            // Compressed face indices
	Normals          []byte `json:"normals,omitempty"`          // Optional compressed normals
	Hash             string `json:"hash,omitempty"`              // Hash for deduplication
	IsDelta          bool   `json:"is_delta"`                    // Whether this is a delta mesh
	BaseMeshID       string `json:"base_mesh_id,omitempty"`     // Reference to base mesh if delta
	DeltaData        []byte `json:"delta_data,omitempty"`       // Delta information
	CompressionLevel int    `json:"compression_level" binding:"min=0,max=9"`
	Timestamp        int64  `json:"timestamp" binding:"required"`
}

// QueryParams defines parameters for spatial queries
type QueryParams struct {
	SessionID      string  `form:"session_id"`
	AnchorID       string  `form:"anchor_id"`
	Radius         float64 `form:"radius"`         // Radius in meters for spatial query
	Since          int64   `form:"since"`          // Unix timestamp in milliseconds
	Until          int64   `form:"until"`          // Unix timestamp in milliseconds
	Limit          int     `form:"limit"`          // Max number of results
	IncludeMeshes  bool    `form:"include_meshes"` // Whether to include mesh data
	IncludeDeleted bool    `form:"include_deleted"` // Whether to include deleted anchors
}

// QueryResponse contains the results of a spatial query
type QueryResponse struct {
	Anchors []Anchor `json:"anchors"`
	Meshes  []Mesh   `json:"meshes,omitempty"`
	Count   int      `json:"count"`
	HasMore bool     `json:"has_more"`
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp"`
	TraceID   string          `json:"trace_id,omitempty"`
}

// WebSocket message types
const (
	WSTypeAnchorUpdate = "anchor_update"
	WSTypeMeshUpdate   = "mesh_update"
	WSTypePing         = "ping"
	WSTypePong         = "pong"
	WSTypeError        = "error"
	WSTypeSubscribe    = "subscribe"
	WSTypeUnsubscribe  = "unsubscribe"
)

// AnchorUpdate represents an anchor position update
type AnchorUpdate struct {
	ID       string                 `json:"id"`
	Pose     PoseData               `json:"pose"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PoseData represents pose in WebSocket messages
type PoseData struct {
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Z        float64   `json:"z"`
	Rotation []float64 `json:"rotation"`
}

// MeshUpdate represents mesh geometry update
type MeshUpdate struct {
	ID               string `json:"id"`
	AnchorID         string `json:"anchor_id"`
	Vertices         string `json:"vertices"`         // Base64 encoded
	Faces            string `json:"faces"`            // Base64 encoded
	Normals          string `json:"normals,omitempty"` // Base64 encoded
	CompressionLevel int    `json:"compression_level"`
	IsDelta          bool   `json:"is_delta"`
	BaseMeshID       string `json:"base_mesh_id,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Database  string    `json:"database"`
}

// MetricsInfo represents metrics information
type MetricsInfo struct {
	ActiveConnections int     `json:"active_connections"`
	TotalAnchors      int64   `json:"total_anchors"`
	TotalMeshes       int64   `json:"total_meshes"`
	StorageSize       int64   `json:"storage_size_bytes"`
	CompressionRatio  float64 `json:"compression_ratio"`
}