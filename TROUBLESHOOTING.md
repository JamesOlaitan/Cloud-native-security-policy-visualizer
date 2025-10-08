# Troubleshooting Guide

## Common Issues and Solutions

### Linter Errors

If you encounter linter errors like `undefined: yaml` or standard library errors, try:

```bash
# Clean and refresh Go dependencies
./scripts/fix-dependencies.sh

# Or manually:
go clean -modcache
go mod download
go mod tidy
go mod verify
```

### UI Build Errors

If you see TypeScript or module errors in the UI:

```bash
# Clean and reinstall UI dependencies
cd ui
rm -rf node_modules package-lock.json yarn.lock
npm install
cd ..

# Or using yarn
cd ui
rm -rf node_modules yarn.lock
yarn install
cd ..
```

### Missing cytoscape-dagre Module

The project includes type declarations for `cytoscape-dagre` in `ui/src/cytoscape-dagre.d.ts`. If you still see errors:

```bash
cd ui
npm install --save-dev @types/node
npm install
cd ..
```

### golangci-lint Errors

If golangci-lint fails to run:

```bash
# Install/update golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Or on macOS with Homebrew
brew install golangci-lint
```

### Coverage Test Failures

If tests fail to achieve 70% coverage:

```bash
# Run tests with verbose output to see which packages need more tests
go test -v -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Docker Build Issues

If Docker builds fail:

```bash
# Clean Docker build cache
docker system prune -af

# Rebuild without cache
docker compose build --no-cache

# Check Docker daemon is running
docker info
```

### OPA Policy Errors

If OPA policies fail to load:

```bash
# Test OPA policies
docker run --rm -v $(pwd)/policy:/policy openpolicyagent/opa test /policy

# Validate individual policy files
docker run --rm -v $(pwd)/policy:/policy openpolicyagent/opa check /policy/wildcards.rego
```

## Environment Issues

### Go Version Mismatch

Ensure you're using Go 1.22 or later:

```bash
go version

# If version is too old, install from https://go.dev/dl/
```

### Node.js Version Mismatch

Ensure you're using Node.js 18 or later:

```bash
node --version

# If version is too old, install from https://nodejs.org/
# Or use nvm:
nvm install 18
nvm use 18
```

### Path Issues

Ensure Go and Node binaries are in your PATH:

```bash
# Check Go
which go
echo $GOPATH

# Check Node
which node
which npm
```

## Still Having Issues?

1. Check the [GitHub Issues](https://github.com/jamesolaitan/accessgraph/issues) for similar problems
2. Review the [Implementation Summary](docs/implementation_summary.md) for architecture details
3. Open a new issue with:
   - Your OS and version
   - Go version (`go version`)
   - Node version (`node --version`)
   - Full error output
   - Steps to reproduce

## Quick Reset

If all else fails, try a complete reset:

```bash
# Clean everything
make clean
rm -rf ui/node_modules ui/dist
go clean -modcache

# Reinstall dependencies
./scripts/fix-dependencies.sh

# Rebuild
make build
make ui
```

