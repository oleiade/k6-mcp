# Run the k6-mcp server
run: index
    go run -tags fts5 ./cmd/k6-mcp

# Install the k6-mcp server
install: index
    go install -tags fts5 ./cmd/k6-mcp


# Build the k6-mcp server
build: index
    go build -tags fts5  -o k6-mcp ./cmd/k6-mcp

release: index
    go build -tags 'fts5 sqlite_fts5' -trimpath -ldflags '-s -w' -o k6-mcp ./cmd/k6-mcp

# Index the k6 documentation into the database. Argument is the path to the k6 documentation folder (e.g. /Users/myself/dev/k6-docs).
index:
    go run -tags fts5 ./cmd/indexer 
