package spatial

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/google/uuid"

	"github.com/tabular/stag-v2/internal/database"
	"github.com/tabular/stag-v2/internal/metrics"
	"github.com/tabular/stag-v2/pkg/api"
	"github.com/tabular/stag-v2/pkg/errors"
	"github.com/tabular/stag-v2/pkg/logger"
)

// Repository handles spatial data operations
type Repository struct {
	db               *database.Connection
	logger           logger.Logger
	metrics          *metrics.Metrics
	meshHashCache    map[string]string // hash -> mesh ID
	compressionCache map[string][]byte // mesh ID -> compressed data
	cacheExpiry      time.Duration
}

// NewRepository creates a new spatial repository
func NewRepository(db *database.Connection, logger logger.Logger, metrics *metrics.Metrics) *Repository {
	return &Repository{
		db:               db,
		logger:           logger,
		metrics:          metrics,
		meshHashCache:    make(map[string]string),
		compressionCache: make(map[string][]byte),
		cacheExpiry:      5 * time.Minute,
	}
}

// Ingest processes and stores spatial events
func (r *Repository) Ingest(ctx context.Context, event *api.SpatialEvent) error {
	startTime := time.Now()
	defer func() {
		r.metrics.DBOperationDuration.WithLabelValues("ingest", "spatial_event").
			Observe(time.Since(startTime).Seconds())
	}()

	// Process anchors
	for _, anchor := range event.Anchors {
		if err := r.ingestAnchor(ctx, &anchor); err != nil {
			r.metrics.DBOperationsTotal.WithLabelValues("ingest", "anchors", "error").Inc()
			return fmt.Errorf("failed to ingest anchor %s: %w", anchor.ID, err)
		}
		r.metrics.AnchorsTotal.WithLabelValues(event.SessionID, "ingest").Inc()
	}

	// Process meshes
	for _, mesh := range event.Meshes {
		processedMesh, saved, err := r.processMeshForStorage(ctx, &mesh)
		if err != nil {
			r.metrics.DBOperationsTotal.WithLabelValues("ingest", "meshes", "error").Inc()
			return fmt.Errorf("failed to process mesh %s: %w", mesh.ID, err)
		}

		if err := r.ingestMesh(ctx, processedMesh); err != nil {
			r.metrics.DBOperationsTotal.WithLabelValues("ingest", "meshes", "error").Inc()
			return fmt.Errorf("failed to ingest mesh %s: %w", mesh.ID, err)
		}

		// Track deduplication savings
		if saved > 0 {
			r.metrics.MeshDedupSavedBytes.WithLabelValues(event.SessionID).Add(float64(saved))
		}

		meshType := "full"
		if mesh.IsDelta {
			meshType = "delta"
		}
		r.metrics.MeshesTotal.WithLabelValues(event.SessionID, meshType, "ingest").Inc()
	}

	r.metrics.DBOperationsTotal.WithLabelValues("ingest", "spatial_event", "success").Inc()
	return nil
}

// ingestAnchor stores an anchor in the database
func (r *Repository) ingestAnchor(ctx context.Context, anchor *api.Anchor) error {
	col, err := r.db.Database().Collection(ctx, database.AnchorsCollection)
	if err != nil {
		return errors.DatabaseError(fmt.Sprintf("failed to get collection: %v", err))
	}

	// Use UPSERT to handle updates
	query := `
		UPSERT { id: @id }
		INSERT @anchor
		UPDATE @anchor
		IN @@collection
		RETURN NEW
	`

	bindVars := map[string]interface{}{
		"id":         anchor.ID,
		"anchor":     anchor,
		"@collection": database.AnchorsCollection,
	}

	cursor, err := r.db.Database().Query(ctx, query, bindVars)
	if err != nil {
		return errors.DatabaseError(fmt.Sprintf("failed to upsert anchor: %v", err))
	}
	defer cursor.Close()

	return nil
}

// processMeshForStorage handles mesh deduplication and delta processing
func (r *Repository) processMeshForStorage(ctx context.Context, mesh *api.Mesh) (*api.Mesh, int64, error) {
	var savedBytes int64

	// If it's a delta mesh, validate and store as-is
	if mesh.IsDelta {
		if mesh.BaseMeshID == "" {
			return nil, 0, errors.ValidationError("delta mesh missing base_mesh_id")
		}
		// Store delta data in the vertices field for consistency
		if len(mesh.DeltaData) > 0 {
			mesh.Vertices = mesh.DeltaData
			mesh.Faces = nil
			mesh.Normals = nil
		}
		return mesh, 0, nil
	}

	// Compute hash for deduplication
	hash := r.computeMeshHash(mesh)
	mesh.Hash = hash

	// Check if we've seen this mesh before
	if existingMeshID, exists := r.meshHashCache[hash]; exists {
		// Mesh already exists, just reference it
		r.logger.Debugf("Mesh %s is duplicate of %s", mesh.ID, existingMeshID)
		
		// Calculate saved bytes
		savedBytes = int64(len(mesh.Vertices) + len(mesh.Faces) + len(mesh.Normals))
		
		// Replace with reference
		mesh.ID = existingMeshID
		return mesh, savedBytes, nil
	}

	// Add to cache
	r.meshHashCache[hash] = mesh.ID

	return mesh, 0, nil
}

// ingestMesh stores a mesh in the database
func (r *Repository) ingestMesh(ctx context.Context, mesh *api.Mesh) error {
	col, err := r.db.Database().Collection(ctx, database.MeshesCollection)
	if err != nil {
		return errors.DatabaseError(fmt.Sprintf("failed to get collection: %v", err))
	}

	// Check if mesh already exists (for deduplication)
	var existingMesh api.Mesh
	_, err = col.ReadDocument(ctx, mesh.ID, &existingMesh)
	if err == nil {
		// Mesh already exists, skip
		return nil
	} else if !driver.IsNotFound(err) {
		return errors.DatabaseError(fmt.Sprintf("failed to check existing mesh: %v", err))
	}

	// Insert new mesh
	_, err = col.CreateDocument(ctx, mesh)
	if err != nil {
		return errors.DatabaseError(fmt.Sprintf("failed to create mesh: %v", err))
	}

	// Update storage metrics
	meshSize := int64(len(mesh.Vertices) + len(mesh.Faces) + len(mesh.Normals))
	r.metrics.StorageSizeBytes.WithLabelValues("meshes").Add(float64(meshSize))

	return nil
}

// Query retrieves spatial data based on parameters
func (r *Repository) Query(ctx context.Context, params *api.QueryParams) (*api.QueryResponse, error) {
	startTime := time.Now()
	defer func() {
		r.metrics.DBOperationDuration.WithLabelValues("query", "spatial").
			Observe(time.Since(startTime).Seconds())
	}()

	// Build AQL query
	query, bindVars := r.buildQuery(params)

	cursor, err := r.db.Database().Query(ctx, query, bindVars)
	if err != nil {
		r.metrics.DBOperationsTotal.WithLabelValues("query", "spatial", "error").Inc()
		return nil, errors.DatabaseError(fmt.Sprintf("failed to execute query: %v", err))
	}
	defer cursor.Close()

	var anchors []api.Anchor
	for {
		var anchor api.Anchor
		_, err := cursor.ReadDocument(ctx, &anchor)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return nil, errors.DatabaseError(fmt.Sprintf("failed to read anchor: %v", err))
		}
		anchors = append(anchors, anchor)
	}

	response := &api.QueryResponse{
		Anchors: anchors,
		Count:   len(anchors),
		HasMore: len(anchors) >= params.Limit,
	}

	// Load meshes if requested
	if params.IncludeMeshes && len(anchors) > 0 {
		meshes, err := r.loadMeshesForAnchors(ctx, anchors)
		if err != nil {
			return nil, err
		}
		response.Meshes = meshes
	}

	r.metrics.DBOperationsTotal.WithLabelValues("query", "spatial", "success").Inc()
	return response, nil
}

// buildQuery constructs an AQL query based on parameters
func (r *Repository) buildQuery(params *api.QueryParams) (string, map[string]interface{}) {
	conditions := []string{}
	bindVars := map[string]interface{}{
		"@collection": database.AnchorsCollection,
	}

	// Session filter
	if params.SessionID != "" {
		conditions = append(conditions, "doc.session_id == @session_id")
		bindVars["session_id"] = params.SessionID
	}

	// Time range filter
	if params.Since > 0 {
		conditions = append(conditions, "doc.timestamp >= @since")
		bindVars["since"] = params.Since
	}
	if params.Until > 0 {
		conditions = append(conditions, "doc.timestamp <= @until")
		bindVars["until"] = params.Until
	}

	// Spatial filter
	if params.AnchorID != "" && params.Radius > 0 {
		// First get the reference anchor
		conditions = append(conditions, `
			LET refAnchor = FIRST(
				FOR a IN @@collection
				FILTER a.id == @anchor_id
				RETURN a
			)
			FILTER refAnchor != null
			FILTER GEO_DISTANCE([refAnchor.pose.x, refAnchor.pose.y], [doc.pose.x, doc.pose.y]) <= @radius
		`)
		bindVars["anchor_id"] = params.AnchorID
		bindVars["radius"] = params.Radius * 1000 // Convert to millimeters
	}

	// Build query
	query := "FOR doc IN @@collection"
	if len(conditions) > 0 {
		query += "\nFILTER " + conditions[0]
		for _, cond := range conditions[1:] {
			query += "\nAND " + cond
		}
	}

	// Sort and limit
	query += "\nSORT doc.timestamp DESC"
	if params.Limit > 0 {
		query += fmt.Sprintf("\nLIMIT %d", params.Limit)
		bindVars["limit"] = params.Limit
	} else {
		query += "\nLIMIT 100" // Default limit
	}

	query += "\nRETURN doc"

	return query, bindVars
}

// loadMeshesForAnchors loads meshes associated with anchors
func (r *Repository) loadMeshesForAnchors(ctx context.Context, anchors []api.Anchor) ([]api.Mesh, error) {
	anchorIDs := make([]string, len(anchors))
	for i, anchor := range anchors {
		anchorIDs[i] = anchor.ID
	}

	query := `
		FOR doc IN @@collection
		FILTER doc.anchor_id IN @anchor_ids
		RETURN doc
	`

	bindVars := map[string]interface{}{
		"@collection": database.MeshesCollection,
		"anchor_ids":  anchorIDs,
	}

	cursor, err := r.db.Database().Query(ctx, query, bindVars)
	if err != nil {
		return nil, errors.DatabaseError(fmt.Sprintf("failed to query meshes: %v", err))
	}
	defer cursor.Close()

	var meshes []api.Mesh
	for {
		var mesh api.Mesh
		_, err := cursor.ReadDocument(ctx, &mesh)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return nil, errors.DatabaseError(fmt.Sprintf("failed to read mesh: %v", err))
		}
		meshes = append(meshes, mesh)
	}

	// Resolve delta meshes
	resolvedMeshes := make([]api.Mesh, 0, len(meshes))
	for _, mesh := range meshes {
		if mesh.IsDelta {
			resolved, err := r.resolveDeltaMesh(ctx, &mesh)
			if err != nil {
				r.logger.Warnf("Failed to resolve delta mesh %s: %v", mesh.ID, err)
				continue
			}
			resolvedMeshes = append(resolvedMeshes, *resolved)
		} else {
			resolvedMeshes = append(resolvedMeshes, mesh)
		}
	}

	return resolvedMeshes, nil
}

// resolveDeltaMesh reconstructs a full mesh from delta
func (r *Repository) resolveDeltaMesh(ctx context.Context, deltaMesh *api.Mesh) (*api.Mesh, error) {
	if !deltaMesh.IsDelta || deltaMesh.BaseMeshID == "" {
		return deltaMesh, nil
	}

	// Load base mesh
	col, err := r.db.Database().Collection(ctx, database.MeshesCollection)
	if err != nil {
		return nil, errors.DatabaseError(fmt.Sprintf("failed to get collection: %v", err))
	}

	var baseMesh api.Mesh
	_, err = col.ReadDocument(ctx, deltaMesh.BaseMeshID, &baseMesh)
	if err != nil {
		return nil, errors.DatabaseError(fmt.Sprintf("failed to load base mesh: %v", err))
	}

	// If base mesh is also a delta, resolve it first
	if baseMesh.IsDelta {
		resolvedBase, err := r.resolveDeltaMesh(ctx, &baseMesh)
		if err != nil {
			return nil, err
		}
		baseMesh = *resolvedBase
	}

	// Apply delta to base mesh
	// In a real implementation, this would decode the delta data and apply it
	// For now, we'll just return the base mesh with updated ID
	result := baseMesh
	result.ID = deltaMesh.ID
	result.Timestamp = deltaMesh.Timestamp
	
	return &result, nil
}

// computeMeshHash calculates a hash for mesh deduplication
func (r *Repository) computeMeshHash(mesh *api.Mesh) string {
	h := sha256.New()
	h.Write(mesh.Vertices)
	h.Write(mesh.Faces)
	if len(mesh.Normals) > 0 {
		h.Write(mesh.Normals)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ProcessWebSocketMessage handles incoming WebSocket messages
func (r *Repository) ProcessWebSocketMessage(ctx context.Context, msg *api.WSMessage) error {
	switch msg.Type {
	case api.WSTypeAnchorUpdate:
		return r.processAnchorUpdate(ctx, msg)
	case api.WSTypeMeshUpdate:
		return r.processMeshUpdate(ctx, msg)
	default:
		return nil
	}
}

// processAnchorUpdate handles anchor update messages
func (r *Repository) processAnchorUpdate(ctx context.Context, msg *api.WSMessage) error {
	var update api.AnchorUpdate
	if err := json.Unmarshal(msg.Data, &update); err != nil {
		return errors.ValidationError(fmt.Sprintf("invalid anchor update: %v", err))
	}

	anchor := api.Anchor{
		ID:        update.ID,
		SessionID: msg.SessionID,
		Pose: api.Pose{
			X:        update.Pose.X,
			Y:        update.Pose.Y,
			Z:        update.Pose.Z,
			Rotation: update.Pose.Rotation,
		},
		Timestamp: msg.Timestamp,
		Metadata:  update.Metadata,
	}

	return r.ingestAnchor(ctx, &anchor)
}

// processMeshUpdate handles mesh update messages
func (r *Repository) processMeshUpdate(ctx context.Context, msg *api.WSMessage) error {
	var update api.MeshUpdate
	if err := json.Unmarshal(msg.Data, &update); err != nil {
		return errors.ValidationError(fmt.Sprintf("invalid mesh update: %v", err))
	}

	// Decode base64 data
	vertices, err := base64.StdEncoding.DecodeString(update.Vertices)
	if err != nil {
		return errors.ValidationError(fmt.Sprintf("invalid vertices encoding: %v", err))
	}

	var faces []byte
	if update.Faces != "" {
		faces, err = base64.StdEncoding.DecodeString(update.Faces)
		if err != nil {
			return errors.ValidationError(fmt.Sprintf("invalid faces encoding: %v", err))
		}
	}

	var normals []byte
	if update.Normals != "" {
		normals, err = base64.StdEncoding.DecodeString(update.Normals)
		if err != nil {
			return errors.ValidationError(fmt.Sprintf("invalid normals encoding: %v", err))
		}
	}

	mesh := api.Mesh{
		ID:               update.ID,
		AnchorID:         update.AnchorID,
		Vertices:         vertices,
		Faces:            faces,
		Normals:          normals,
		IsDelta:          update.IsDelta,
		BaseMeshID:       update.BaseMeshID,
		CompressionLevel: update.CompressionLevel,
		Timestamp:        msg.Timestamp,
	}

	// If it's a delta mesh, vertices contain the delta data
	if update.IsDelta {
		mesh.DeltaData = vertices
	}

	// Process and ingest
	processedMesh, saved, err := r.processMeshForStorage(ctx, &mesh)
	if err != nil {
		return err
	}

	if err := r.ingestMesh(ctx, processedMesh); err != nil {
		return err
	}

	if saved > 0 {
		r.metrics.MeshDedupSavedBytes.WithLabelValues(msg.SessionID).Add(float64(saved))
	}

	return nil
}

// GetMetrics returns current metrics
func (r *Repository) GetMetrics(ctx context.Context) (*api.MetricsInfo, error) {
	// Count anchors
	anchorCount, err := r.countDocuments(ctx, database.AnchorsCollection)
	if err != nil {
		return nil, err
	}

	// Count meshes
	meshCount, err := r.countDocuments(ctx, database.MeshesCollection)
	if err != nil {
		return nil, err
	}

	// Estimate storage size (simplified)
	storageSize := anchorCount*500 + meshCount*50000 // Rough estimates

	return &api.MetricsInfo{
		ActiveConnections: 0, // Will be set by WebSocket hub
		TotalAnchors:      anchorCount,
		TotalMeshes:       meshCount,
		StorageSize:       storageSize,
		CompressionRatio:  0.6, // Placeholder
	}, nil
}

// countDocuments counts documents in a collection
func (r *Repository) countDocuments(ctx context.Context, collectionName string) (int64, error) {
	query := "RETURN COUNT(FOR doc IN @@collection RETURN 1)"
	bindVars := map[string]interface{}{
		"@collection": collectionName,
	}

	cursor, err := r.db.Database().Query(ctx, query, bindVars)
	if err != nil {
		return 0, errors.DatabaseError(fmt.Sprintf("failed to count documents: %v", err))
	}
	defer cursor.Close()

	var count int64
	_, err = cursor.ReadDocument(ctx, &count)
	if err != nil {
		return 0, errors.DatabaseError(fmt.Sprintf("failed to read count: %v", err))
	}

	return count, nil
}