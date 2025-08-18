# Run the k6-mcp server
run: prepare
    go run -tags fts5 ./cmd/k6-mcp

# Install the k6-mcp server
install: prepare
    go install -tags fts5 ./cmd/k6-mcp


# Build the k6-mcp server
build: prepare
    go build -tags fts5  -o k6-mcp ./cmd/k6-mcp

release: prepare
    go build -tags 'fts5 sqlite_fts5' -trimpath -ldflags '-s -w' -o k6-mcp ./cmd/k6-mcp

# Prepare the k6-mcp server for distribution.
prepare: index collect

# Index the k6 documentation into the database. Argument is the path to the k6 documentation folder (e.g. /Users/myself/dev/k6-docs).
index:
    go run -tags fts5 ./cmd/indexer 

# Collect the type definitions from the DefinitelyTyped repository into the dist folder.
collect:
    go run ./cmd/collecter