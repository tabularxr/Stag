package database

import (
	"context"
	"fmt"
	"time"

	"github.com/arangodb/go-driver"
)

// Migrate runs database migrations
func Migrate(conn *Connection) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create collections
	if err := createCollections(ctx, conn); err != nil {
		return fmt.Errorf("failed to create collections: %w", err)
	}

	// Create indexes
	if err := createIndexes(ctx, conn); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	// Create graph
	if err := createGraph(ctx, conn); err != nil {
		return fmt.Errorf("failed to create graph: %w", err)
	}

	return nil
}

func createCollections(ctx context.Context, conn *Connection) error {
	// Create anchors collection
	_, err := conn.CreateCollection(ctx, AnchorsCollection, &driver.CreateCollectionOptions{
		Type: driver.CollectionTypeDocument,
	})
	if err != nil {
		return fmt.Errorf("failed to create anchors collection: %w", err)
	}

	// Create meshes collection
	_, err = conn.CreateCollection(ctx, MeshesCollection, &driver.CreateCollectionOptions{
		Type: driver.CollectionTypeDocument,
	})
	if err != nil {
		return fmt.Errorf("failed to create meshes collection: %w", err)
	}

	// Create topology edges collection
	_, err = conn.CreateCollection(ctx, TopologyEdges, &driver.CreateCollectionOptions{
		Type: driver.CollectionTypeEdge,
	})
	if err != nil {
		return fmt.Errorf("failed to create topology edges collection: %w", err)
	}

	return nil
}

func createIndexes(ctx context.Context, conn *Connection) error {
	// Get collections
	anchorsCol, err := conn.Database().Collection(ctx, AnchorsCollection)
	if err != nil {
		return fmt.Errorf("failed to get anchors collection: %w", err)
	}

	meshesCol, err := conn.Database().Collection(ctx, MeshesCollection)
	if err != nil {
		return fmt.Errorf("failed to get meshes collection: %w", err)
	}

	// Create indexes for anchors
	// Index on session_id for fast session queries
	_, _, err = anchorsCol.EnsurePersistentIndex(ctx, []string{"session_id"}, &driver.EnsurePersistentIndexOptions{
		Name:   "idx_session_id",
		Unique: false,
		Sparse: false,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create session_id index: %w", err)
	}

	// Index on timestamp for time-based queries
	_, _, err = anchorsCol.EnsurePersistentIndex(ctx, []string{"timestamp"}, &driver.EnsurePersistentIndexOptions{
		Name:   "idx_timestamp",
		Unique: false,
		Sparse: false,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create timestamp index: %w", err)
	}

	// Geo index on pose for spatial queries
	_, _, err = anchorsCol.EnsureGeoIndex(ctx, []string{"pose.x", "pose.y"}, &driver.EnsureGeoIndexOptions{
		Name:    "idx_geo_pose",
		GeoJSON: false,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create geo index: %w", err)
	}

	// Create indexes for meshes
	// Index on anchor_id for fast lookups
	_, _, err = meshesCol.EnsurePersistentIndex(ctx, []string{"anchor_id"}, &driver.EnsurePersistentIndexOptions{
		Name:   "idx_anchor_id",
		Unique: false,
		Sparse: false,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create anchor_id index: %w", err)
	}

	// Index on hash for deduplication
	_, _, err = meshesCol.EnsureHashIndex(ctx, []string{"hash"}, &driver.EnsureHashIndexOptions{
		Name:   "idx_mesh_hash",
		Unique: false,
		Sparse: true,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create hash index: %w", err)
	}

	// Index on base_mesh_id for delta queries
	_, _, err = meshesCol.EnsurePersistentIndex(ctx, []string{"base_mesh_id"}, &driver.EnsurePersistentIndexOptions{
		Name:   "idx_base_mesh_id",
		Unique: false,
		Sparse: true,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create base_mesh_id index: %w", err)
	}

	return nil
}

func createGraph(ctx context.Context, conn *Connection) error {
	// Define edge definitions
	edgeDefinitions := []driver.EdgeDefinition{
		{
			Collection: TopologyEdges,
			From:       []string{AnchorsCollection},
			To:         []string{AnchorsCollection},
		},
	}

	// Create graph
	_, err := conn.CreateGraph(ctx, TopologyGraph, &driver.CreateGraphOptions{
		EdgeDefinitions: edgeDefinitions,
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("failed to create topology graph: %w", err)
	}

	return nil
}