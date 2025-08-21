today := `date -u +%Y-%m-%dT%H:%M:%SZ`
commit := `git rev-parse --short HEAD`

# Run the k6-mcp server
run: prepare
    @go run -tags 'fts5 sqlite_fts5' ./cmd/k6-mcp

# Install the k6-mcp server
install version="dev": prepare
    @go install \
        -tags 'fts5 sqlite_fts5' \
        -ldflags "-s -w -X github.com/oleiade/k6-mcp/internal/buildinfo.Version={{version}} \
                  -X github.com/oleiade/k6-mcp/internal/buildinfo.Commit={{commit}} \
                  -X github.com/oleiade/k6-mcp/internal/buildinfo.Date={{today}}" \
        ./cmd/k6-mcp

# Build the k6-mcp server
build version="dev": prepare
    @go build \
        -tags 'fts5 sqlite_fts5' \
        -ldflags "-s -w -X github.com/oleiade/k6-mcp/internal/buildinfo.Version={{version}} \
                  -X github.com/oleiade/k6-mcp/internal/buildinfo.Commit={{commit}} \
                  -X github.com/oleiade/k6-mcp/internal/buildinfo.Date={{today}}" \
        -o k6-mcp \
        ./cmd/k6-mcp

release version="dev": prepare
    @go build \
        -tags 'fts5 sqlite_fts5' \
        -trimpath \
        -ldflags "-s -w -X github.com/oleiade/k6-mcp/internal/buildinfo.Version={{version}} \
                  -X github.com/oleiade/k6-mcp/internal/buildinfo.Commit={{commit}} \
                  -X github.com/oleiade/k6-mcp/internal/buildinfo.Date={{today}}" \
        -o k6-mcp \
        ./cmd/k6-mcp

# Prepare the k6-mcp server for distribution.
prepare:
    @go run -tags 'fts5 sqlite_fts5' ./cmd/prepare

# Clean the dist folder.
clean:
    @rm -rf dist
    @rm -rf k6-mcp
    @rm -rf prepare

# Index the k6 documentation into the database. Argument is the path to the k6 documentation folder (e.g. /Users/myself/dev/k6-docs).
index:
    go run -tags 'fts5 sqlite_fts5' ./cmd/prepare --index-only

# Collect the type definitions from the DefinitelyTyped repository into the dist folder.
collect:
    go run ./cmd/prepare --collect-only