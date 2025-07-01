# Initialize the repository's submodules
initialize:
    git submodule update --init --recursive

# Run the k6-mcp server
run:
    go run ./cmd/k6-mcp

# Run chroma vector database in the background
chroma:
    docker-compose up -d chroma

# Ingest the k6 documentation into the chroma vector database
ingest:
    cd python-services && poetry run python ingest.py

# Verify that the chroma vector database ingestion was successful
verify:
    cd python-services && poetry run python verify_chroma.py
