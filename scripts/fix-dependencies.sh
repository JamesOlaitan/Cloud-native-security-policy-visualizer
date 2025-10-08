#!/bin/bash
set -e

echo "=== Fixing AccessGraph Dependencies ==="
echo ""

# Fix Go dependencies
echo "1. Cleaning Go module cache..."
go clean -modcache

echo "2. Downloading Go dependencies..."
go mod download

echo "3. Tidying Go modules..."
go mod tidy

echo "4. Verifying Go modules..."
go mod verify

echo ""
echo "5. Installing UI dependencies..."
cd ui
npm install
cd ..

echo ""
echo "=== Dependencies fixed successfully! ==="
echo ""
echo "You can now run:"
echo "  make lint    # Run linter"
echo "  make test    # Run tests"
echo "  make build   # Build binaries"
echo "  make ui      # Build UI"

