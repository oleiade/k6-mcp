# Initialize the repository's submodules
initialize: chroma ingest 

# Run the k6-mcp server
run:
    go run ./cmd/k6-mcp

# Run chroma vector database in the background
chroma:
    docker compose -f docker-compose.chroma.yml up -d

# Run milvus vector database in the background
milvus:
    docker compose -f docker-compose.milvus.yml up -d

# Ingest the k6 documentation into the chroma vector database
ingest:
    cd python-services && poetry install && poetry run python ingest.py

# Verify that the chroma vector database ingestion was successful
verify:
    cd python-services && poetry install && poetry run python verify_chroma.py

# Reset the chroma vector database
reset:
    docker compose -f docker-compose.chroma.yml down
    docker volume rm k6-mcp_chroma_data