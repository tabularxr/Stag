package database

import (
	"context"
	"fmt"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	
	"github.com/tabular/stag-v2/internal/config"
)

const (
	// Collection names
	AnchorsCollection  = "anchors"
	MeshesCollection   = "meshes"
	TopologyEdges      = "topology_edges"
	TopologyGraph      = "topology"
)

// Connection wraps the ArangoDB connection
type Connection struct {
	client   driver.Client
	database driver.Database
}

// Connect establishes connection to ArangoDB
func Connect(cfg config.DatabaseConfig) (*Connection, error) {
	// Create HTTP connection
	conn, err := http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{cfg.URL},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP connection: %w", err)
	}

	// Create client
	client, err := driver.NewClient(driver.ClientConfig{
		Connection:     conn,
		Authentication: driver.BasicAuthentication(cfg.Username, cfg.Password),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Get or create database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var db driver.Database
	exists, err := client.DatabaseExists(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to check database existence: %w", err)
	}

	if !exists {
		db, err = client.CreateDatabase(ctx, cfg.Database, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create database: %w", err)
		}
	} else {
		db, err = client.Database(ctx, cfg.Database)
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}
	}

	return &Connection{
		client:   client,
		database: db,
	}, nil
}

// Database returns the database handle
func (c *Connection) Database() driver.Database {
	return c.database
}

// Client returns the client handle
func (c *Connection) Client() driver.Client {
	return c.client
}

// Close closes the database connection
func (c *Connection) Close() error {
	// ArangoDB Go driver doesn't require explicit connection closing
	return nil
}

// CreateCollection creates a collection if it doesn't exist
func (c *Connection) CreateCollection(ctx context.Context, name string, options *driver.CreateCollectionOptions) (driver.Collection, error) {
	exists, err := c.database.CollectionExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection existence: %w", err)
	}

	if exists {
		return c.database.Collection(ctx, name)
	}

	return c.database.CreateCollection(ctx, name, options)
}

// CreateGraph creates a graph if it doesn't exist
func (c *Connection) CreateGraph(ctx context.Context, name string, options *driver.CreateGraphOptions) (driver.Graph, error) {
	exists, err := c.database.GraphExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check graph existence: %w", err)
	}

	if exists {
		return c.database.Graph(ctx, name)
	}

	return c.database.CreateGraph(ctx, name, options)
}