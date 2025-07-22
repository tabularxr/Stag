#!/bin/bash

set -e

echo "Starting integration tests for STAG v2..."

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Please start Docker Desktop.${NC}"
    exit 1
fi

# Start services
echo "Starting services..."
make docker-down > /dev/null 2>&1 || true
make docker-up

# Wait for ArangoDB to be ready
echo "Waiting for ArangoDB to be ready..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
    if docker exec stag-arangodb curl -f http://localhost:8529/_api/version > /dev/null 2>&1; then
        echo -e "${GREEN}ArangoDB is ready${NC}"
        break
    fi
    attempt=$((attempt + 1))
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo -e "${RED}ArangoDB failed to start${NC}"
    make docker-logs
    exit 1
fi

# Wait for STAG to be ready
echo "Waiting for STAG to be ready..."
attempt=0
while [ $attempt -lt $max_attempts ]; do
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}STAG is ready${NC}"
        break
    fi
    attempt=$((attempt + 1))
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo -e "${RED}STAG failed to start${NC}"
    make docker-logs
    exit 1
fi

# Run integration tests
echo "Running integration tests..."
cd tests
go test -v -run TestFullIntegration ./...
test_result=$?

# Cleanup
echo "Cleaning up..."
cd ..
make docker-down

if [ $test_result -eq 0 ]; then
    echo -e "${GREEN}All integration tests passed!${NC}"
else
    echo -e "${RED}Integration tests failed${NC}"
    exit $test_result
fi