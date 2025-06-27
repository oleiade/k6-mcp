import os
from langchain_community.document_loaders import DirectoryLoader, UnstructuredMarkdownLoader
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import Chroma
from langchain_huggingface import HuggingFaceEmbeddings
import chromadb

# Disable ChromaDB telemetry to avoid telemetry errors
os.environ["ANONYMIZED_TELEMETRY"] = "False"

# --- 1. Load your k6 documentation ---
# This will load all .md files from the specified directory.
loader = DirectoryLoader('../k6-docs/docs/sources', glob="**/*.md", loader_cls=UnstructuredMarkdownLoader)
documents = loader.load()

if not documents:
    print("No markdown documents found. Check your directory path.")
    exit()

print(f"Loaded {len(documents)} documents.")

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