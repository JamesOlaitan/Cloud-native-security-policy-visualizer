#!/bin/bash
set -e

echo "=== AccessGraph Integration Test ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    rm -f test-integration.db
}

trap cleanup EXIT

# Test 1: Build binaries
echo "Test 1: Building binaries..."
make build
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Build successful${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Test 2: Ingest demo data
echo ""
echo "Test 2: Ingesting demo1 snapshot..."
SQLITE_PATH=test-integration.db ./bin/accessgraph-ingest \
    --aws sample/aws \
    --k8s sample/k8s \
    --snapshot demo1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Ingestion successful${NC}"
else
    echo -e "${RED}✗ Ingestion failed${NC}"
    exit 1
fi

# Test 3: List snapshots
echo ""
echo "Test 3: Listing snapshots..."
SQLITE_PATH=test-integration.db ./bin/accessgraph-cli snapshots ls

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Snapshot list successful${NC}"
else
    echo -e "${RED}✗ Snapshot list failed${NC}"
    exit 1
fi

# Test 4: Verify node count
echo ""
echo "Test 4: Verifying database has nodes..."
node_count=$(sqlite3 test-integration.db "SELECT COUNT(*) FROM nodes WHERE snapshot_id='demo1';" 2>/dev/null || echo "0")

if [ "$node_count" -gt "0" ]; then
    echo -e "${GREEN}✓ Database contains ${node_count} nodes${NC}"
else
    echo -e "${RED}✗ No nodes found in database${NC}"
    exit 1
fi

# Test 5: Ingest demo2 with Terraform
echo ""
echo "Test 5: Ingesting demo2 snapshot with Terraform..."
SQLITE_PATH=test-integration.db ./bin/accessgraph-ingest \
    --aws sample/aws \
    --k8s sample/k8s \
    --tf sample/terraform/plan.json \
    --snapshot demo2

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Terraform ingestion successful${NC}"
else
    echo -e "${RED}✗ Terraform ingestion failed${NC}"
    exit 1
fi

# Test 6: Snapshot diff
echo ""
echo "Test 6: Computing snapshot diff..."
SQLITE_PATH=test-integration.db ./bin/accessgraph-cli snapshots diff --a demo1 --b demo2

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Snapshot diff successful${NC}"
else
    echo -e "${RED}✗ Snapshot diff failed${NC}"
    exit 1
fi

# Test 7: Run offline test
echo ""
echo "Test 7: Testing offline mode..."
go test -v ./internal/config -run TestOfflineMode

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Offline mode test passed${NC}"
else
    echo -e "${RED}✗ Offline mode test failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}=== All integration tests passed! ===${NC}"

