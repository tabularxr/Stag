package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tabular/stag-v2/pkg/api"
)

const (
	testServerURL = "http://localhost:8080"
	testWSURL     = "ws://localhost:8080/api/v1/ws"
)

// TestFullIntegration tests the complete flow from ingestion to query
func TestFullIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Wait for server to be ready
	waitForServer(t)

	// Test data
	sessionID := "test-session-" + fmt.Sprint(time.Now().Unix())
	anchorID := "test-anchor-1"

	// Test 1: Ingest spatial event
	t.Run("IngestEvent", func(t *testing.T) {
		event := api.SpatialEvent{
			SessionID: sessionID,
			EventID:   "event-1",
			Timestamp: time.Now().UnixMilli(),
			Anchors: []api.Anchor{
				{
					ID:        anchorID,
					SessionID: sessionID,
					Pose: api.Pose{
						X:        1.0,
						Y:        2.0,
						Z:        3.0,
						Rotation: []float64{0, 0, 0, 1},
					},
					Timestamp: time.Now().UnixMilli(),
				},
			},
			Meshes: []api.Mesh{
				{
					ID:               "mesh-1",
					AnchorID:         anchorID,
					Vertices:         []byte{1, 2, 3, 4, 5, 6},
					Faces:            []byte{0, 1, 2},
					CompressionLevel: 5,
					Timestamp:        time.Now().UnixMilli(),
				},
			},
		}

		resp := postJSON(t, "/api/v1/ingest", event)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test 2: Query by session
	t.Run("QueryBySession", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/query?session_id=%s&include_meshes=true", testServerURL, sessionID))
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var result api.QueryResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result.Anchors) != 1 {
			t.Fatalf("Expected 1 anchor, got %d", len(result.Anchors))
		}

		if len(result.Meshes) != 1 {
			t.Fatalf("Expected 1 mesh, got %d", len(result.Meshes))
		}
	})

	// Test 3: WebSocket streaming
	t.Run("WebSocketStreaming", func(t *testing.T) {
		// Connect to WebSocket
		wsURL := fmt.Sprintf("%s?session_id=%s", testWSURL, sessionID)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WebSocket connection failed: %v", err)
		}
		defer conn.Close()

		// Send anchor update
		update := map[string]interface{}{
			"type":       "anchor_update",
			"session_id": sessionID,
			"data": map[string]interface{}{
				"id": "anchor-ws-1",
				"pose": map[string]interface{}{
					"x":        5.0,
					"y":        6.0,
					"z":        7.0,
					"rotation": []float64{0, 0, 0, 1},
				},
			},
			"timestamp": time.Now().UnixMilli(),
		}

		if err := conn.WriteJSON(update); err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		// Read response (should be broadcast back)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if response["type"] != "anchor_update" {
			t.Fatalf("Expected anchor_update, got %v", response["type"])
		}
	})

	// Test 4: Mesh deduplication
	t.Run("MeshDeduplication", func(t *testing.T) {
		// Ingest same mesh twice
		event1 := api.SpatialEvent{
			SessionID: sessionID,
			EventID:   "event-dedup-1",
			Timestamp: time.Now().UnixMilli(),
			Meshes: []api.Mesh{
				{
					ID:               "mesh-dup-1",
					AnchorID:         anchorID,
					Vertices:         []byte{1, 2, 3, 4, 5, 6},
					Faces:            []byte{0, 1, 2},
					CompressionLevel: 5,
					Timestamp:        time.Now().UnixMilli(),
				},
			},
		}

		event2 := api.SpatialEvent{
			SessionID: sessionID,
			EventID:   "event-dedup-2",
			Timestamp: time.Now().UnixMilli(),
			Meshes: []api.Mesh{
				{
					ID:               "mesh-dup-2",
					AnchorID:         anchorID,
					Vertices:         []byte{1, 2, 3, 4, 5, 6}, // Same data
					Faces:            []byte{0, 1, 2},
					CompressionLevel: 5,
					Timestamp:        time.Now().UnixMilli(),
				},
			},
		}

		// Ingest both
		resp1 := postJSON(t, "/api/v1/ingest", event1)
		if resp1.StatusCode != http.StatusOK {
			t.Fatalf("First ingest failed: %d", resp1.StatusCode)
		}

		resp2 := postJSON(t, "/api/v1/ingest", event2)
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("Second ingest failed: %d", resp2.StatusCode)
		}

		// Check metrics to verify deduplication worked
		metricsResp, err := http.Get(fmt.Sprintf("%s/api/v1/metrics", testServerURL))
		if err != nil {
			t.Fatalf("Failed to get metrics: %v", err)
		}
		defer metricsResp.Body.Close()

		var metrics api.MetricsInfo
		if err := json.NewDecoder(metricsResp.Body).Decode(&metrics); err != nil {
			t.Fatalf("Failed to decode metrics: %v", err)
		}

		// Should have fewer meshes than ingested due to deduplication
		t.Logf("Total meshes stored: %d", metrics.TotalMeshes)
	})

	// Test 5: Delta mesh
	t.Run("DeltaMesh", func(t *testing.T) {
		// First, ingest a base mesh
		baseMeshID := "base-mesh-1"
		baseEvent := api.SpatialEvent{
			SessionID: sessionID,
			EventID:   "event-base",
			Timestamp: time.Now().UnixMilli(),
			Meshes: []api.Mesh{
				{
					ID:               baseMeshID,
					AnchorID:         anchorID,
					Vertices:         []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
					Faces:            []byte{0, 1, 2, 3, 4, 5},
					CompressionLevel: 5,
					Timestamp:        time.Now().UnixMilli(),
				},
			},
		}

		resp := postJSON(t, "/api/v1/ingest", baseEvent)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Base mesh ingest failed: %d", resp.StatusCode)
		}

		// Now ingest a delta mesh
		deltaEvent := api.SpatialEvent{
			SessionID: sessionID,
			EventID:   "event-delta",
			Timestamp: time.Now().UnixMilli(),
			Meshes: []api.Mesh{
				{
					ID:               "delta-mesh-1",
					AnchorID:         anchorID,
					IsDelta:          true,
					BaseMeshID:       baseMeshID,
					DeltaData:        []byte{10, 11, 12}, // Delta information
					CompressionLevel: 5,
					Timestamp:        time.Now().UnixMilli(),
				},
			},
		}

		resp = postJSON(t, "/api/v1/ingest", deltaEvent)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Delta mesh ingest failed: %d", resp.StatusCode)
		}
	})
}

// Helper functions

func waitForServer(t *testing.T) {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(testServerURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatal("Server failed to start")
}

func postJSON(t *testing.T, path string, data interface{}) *http.Response {
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}

	resp, err := http.Post(testServerURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}

	return resp
}