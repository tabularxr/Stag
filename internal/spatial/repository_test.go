package spatial

import (
	"context"
	"testing"
	"time"

	"github.com/tabular/stag-v2/pkg/api"
)

func TestMeshDeduplication(t *testing.T) {
	// This test would require a mock database connection
	// For now, we'll test the hash computation

	repo := &Repository{
		meshHashCache: make(map[string]string),
	}

	// Create identical meshes
	mesh1 := &api.Mesh{
		ID:       "mesh1",
		AnchorID: "anchor1",
		Vertices: []byte{1, 2, 3, 4, 5, 6},
		Faces:    []byte{0, 1, 2},
		Timestamp: time.Now().UnixMilli(),
	}

	mesh2 := &api.Mesh{
		ID:       "mesh2",
		AnchorID: "anchor1",
		Vertices: []byte{1, 2, 3, 4, 5, 6},
		Faces:    []byte{0, 1, 2},
		Timestamp: time.Now().UnixMilli(),
	}

	// Compute hashes
	hash1 := repo.computeMeshHash(mesh1)
	hash2 := repo.computeMeshHash(mesh2)

	// Hashes should be identical
	if hash1 != hash2 {
		t.Errorf("Expected identical hashes, got %s and %s", hash1, hash2)
	}

	// Different mesh should have different hash
	mesh3 := &api.Mesh{
		ID:       "mesh3",
		AnchorID: "anchor1",
		Vertices: []byte{7, 8, 9, 10, 11, 12},
		Faces:    []byte{0, 1, 2},
		Timestamp: time.Now().UnixMilli(),
	}

	hash3 := repo.computeMeshHash(mesh3)
	if hash1 == hash3 {
		t.Errorf("Expected different hashes, but both are %s", hash1)
	}
}

func TestDeltaMeshValidation(t *testing.T) {
	repo := &Repository{
		meshHashCache: make(map[string]string),
	}

	// Delta mesh without base should fail
	deltaMesh := &api.Mesh{
		ID:         "delta1",
		AnchorID:   "anchor1",
		IsDelta:    true,
		BaseMeshID: "", // Missing base
		DeltaData:  []byte{1, 2, 3},
		Timestamp:  time.Now().UnixMilli(),
	}

	_, _, err := repo.processMeshForStorage(context.Background(), deltaMesh)
	if err == nil {
		t.Error("Expected error for delta mesh without base_mesh_id")
	}

	// Valid delta mesh
	deltaMesh.BaseMeshID = "base1"
	processed, _, err := repo.processMeshForStorage(context.Background(), deltaMesh)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Delta data should be in vertices field
	if len(processed.Vertices) == 0 {
		t.Error("Expected delta data in vertices field")
	}
}