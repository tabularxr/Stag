# STAG v2 - Spatial Topology & Anchor Graph

A high-performance spatial data ingestion and query service built in Go, designed for real-time AR/VR applications.

## Features

- **Real-time WebSocket streaming** for live spatial updates
- **Mesh diffing algorithm** for bandwidth optimization
- **Mesh deduplication** with content-based hashing
- **ArangoDB backend** for graph-based spatial relationships
- **Prometheus metrics** for monitoring
- **Docker support** for easy deployment

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- ArangoDB 3.11+

### Running Locally

1. **Start ArangoDB**:
   ```bash
   make docker-up-db
   ```

2. **Set environment variables**:
   ```bash
   export ARANGO_PASSWORD=stagpassword
   ```

3. **Run STAG**:
   ```bash
   make run
   ```

### Running with Docker

```bash
make docker-up
```

This starts ArangoDB, STAG, and Prometheus.

## API Endpoints

### HTTP Endpoints

- `POST /api/v1/ingest` - Ingest spatial events
- `GET /api/v1/query` - Query spatial data
- `GET /api/v1/anchors/{id}` - Get specific anchor
- `GET /api/v1/metrics` - Get system metrics
- `GET /health` - Health check

### WebSocket Endpoint

- `GET /api/v1/ws?session_id={session_id}` - Real-time streaming

## Data Model

### Spatial Event
```json
{
  "session_id": "uuid",
  "event_id": "uuid",
  "timestamp": 1234567890,
  "anchors": [...],
  "meshes": [...]
}
```

### Mesh with Delta Support
```json
{
  "id": "mesh-id",
  "anchor_id": "anchor-id",
  "is_delta": true,
  "base_mesh_id": "base-mesh-id",
  "delta_data": "base64-encoded-delta",
  "compression_level": 5,
  "timestamp": 1234567890
}
```

## Mesh Diffing

STAG v2 includes an efficient mesh diffing system:

1. **Content-based deduplication**: Identical meshes are stored only once
2. **Delta compression**: Only changes between mesh versions are transmitted
3. **Automatic reconstruction**: Delta meshes are resolved on query

## Configuration

Configure via environment variables:

- `STAG_SERVER_PORT` - Server port (default: 8080)
- `STAG_DATABASE_URL` - ArangoDB URL (default: http://localhost:8529)
- `STAG_DATABASE_PASSWORD` - ArangoDB password (required)
- `STAG_LOG_LEVEL` - Log level (default: info)

## Development

```bash
# Install dependencies
make deps

# Run tests
make test

# Run with hot reload
make dev

# Build binary
make build
```

## Monitoring

Prometheus metrics available at `/metrics`:

- `stag_http_requests_total` - HTTP request count
- `stag_ws_connections_active` - Active WebSocket connections
- `stag_meshes_total` - Processed meshes count
- `stag_mesh_dedup_saved_bytes` - Bytes saved through deduplication

## License

See LICENSE file.