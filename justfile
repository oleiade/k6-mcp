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

# Initialize the repository's submodules
initialize: chroma ingest

# Run chroma vector database in the background
chroma:
    docker compose -f docker-compose.chroma.yml up -d

# Run milvus vector database in the background
milvus:
    docker compose -f docker-compose.milvus.yml up -d

# Ingest the k6 documentation into the chroma vector database
ingest:
    cd python-services && ./ingest.py

# Verify that the chroma vector database ingestion was successful
verify:
    cd python-services && ./verify_chroma.py

# Reset the chroma vector database
reset:
    docker compose -f docker-compose.chroma.yml down
    docker volume rm k6-mcp_chroma_data