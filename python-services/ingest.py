import os
import tempfile
import shutil
from git import Repo
from langchain_community.document_loaders import DirectoryLoader, UnstructuredMarkdownLoader
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import Chroma
from langchain_huggingface import HuggingFaceEmbeddings
import chromadb

# Disable ChromaDB telemetry to avoid telemetry errors
os.environ["ANONYMIZED_TELEMETRY"] = "False"

def clone_k6_docs():
    """Clone the k6-docs repository to a temporary directory."""
    print("Cloning k6-docs repository...")
    temp_dir = tempfile.mkdtemp(prefix="k6-docs-")
    try:
        # Clone the repository
        repo = Repo.clone_from("https://github.com/grafana/k6-docs.git", temp_dir)
        docs_path = os.path.join(temp_dir, "docs", "sources")
        
        if not os.path.exists(docs_path):
            raise FileNotFoundError(f"Documentation sources not found at {docs_path}")
        
        print(f"Successfully cloned k6-docs to {temp_dir}")
        return temp_dir, docs_path
    except Exception as e:
        # Clean up on failure
        if os.path.exists(temp_dir):
            shutil.rmtree(temp_dir)
        raise e

# --- 1. Clone k6 documentation and load documents ---
try:
    temp_dir, docs_path = clone_k6_docs()
    
    # Load all .md files from the cloned documentation directory
    loader = DirectoryLoader(docs_path, glob="**/*.md", loader_cls=UnstructuredMarkdownLoader)
    documents = loader.load()
    
    if not documents:
        print("No markdown documents found. Check your directory path.")
        exit(1)
    
    print(f"Loaded {len(documents)} documents.")
except Exception as e:
    print(f"Error cloning or loading k6 documentation: {e}")
    exit(1)

# --- 2. Chunk the documents into smaller pieces ---
# This is crucial for getting relevant results.
text_splitter = RecursiveCharacterTextSplitter(chunk_size=1000, chunk_overlap=200)
docs_chunks = text_splitter.split_documents(documents)

if not docs_chunks:
    print("Could not split documents into chunks.")
    exit()

print(f"Split documents into {len(docs_chunks)} chunks.")

# --- 3. Create embeddings and store them in ChromaDB ---
# This will download a model to your machine (e.g., 'all-MiniLM-L6-v2')
# and use it to create the embeddings.
embedding_function = HuggingFaceEmbeddings(model_name="all-MiniLM-L6-v2")

# Connect to the ChromaDB Docker container
# Create proper ChromaDB client
chroma_client = chromadb.HttpClient(host="localhost", port=8000)

vectorstore = Chroma.from_documents(
    documents=docs_chunks,
    embedding=embedding_function,
    collection_name="k6_docs",
    client=chroma_client
)

print("Successfully loaded documents into ChromaDB.")
print("Connected to ChromaDB container at localhost:8000")

# --- 4. Clean up temporary directory ---
try:
    print(f"Cleaning up temporary directory: {temp_dir}")
    shutil.rmtree(temp_dir)
    print("Cleanup completed successfully.")
except Exception as e:
    print(f"Warning: Failed to clean up temporary directory {temp_dir}: {e}")
    print("You may need to manually remove this directory.")