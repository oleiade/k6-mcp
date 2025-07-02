#!/usr/bin/env -S uv run --script
# /// script
# dependencies = [
#     "chromadb>=1.0.13",
#     "python-frontmatter>=1.1.0",
#     "langchain>=0.3.26",
#     "langchain-community>=0.3.26",
#     "langchain-huggingface>=0.3.0",
#     "gitpython>=3.1.40",
#     "unstructured>=0.18.1",
#     "sentence-transformers>=4.1.0"
# ]
# ///

import os
import tempfile
import shutil
import re
from pathlib import Path
from typing import Dict, List, Optional, Tuple
from git import Repo
from langchain_community.document_loaders import DirectoryLoader, UnstructuredMarkdownLoader
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import Chroma
from langchain_huggingface import HuggingFaceEmbeddings
from langchain.schema import Document
import chromadb
import frontmatter

# Disable ChromaDB telemetry to avoid telemetry errors
os.environ["ANONYMIZED_TELEMETRY"] = "False"

def get_k6_docs_path() -> str:
    """Get the path to the k6-docs directory."""
    docs_path = os.path.expanduser("~/Dev/grafana/k6-docs/docs/sources")
    
    if not os.path.exists(docs_path):
        raise FileNotFoundError(f"Documentation sources not found at {docs_path}")
    
    print(f"Using k6-docs at {docs_path}")
    return docs_path

def extract_metadata_from_path(file_path: str, base_path: str) -> Dict[str, str]:
    """Extract tool, version, and category metadata from file path."""
    # Get relative path from base sources directory
    rel_path = os.path.relpath(file_path, base_path)
    path_parts = Path(rel_path).parts
    
    metadata = {
        "file_path": rel_path,
        "tool": "unknown",
        "version": "unknown",
        "category": "unknown"
    }
    
    if len(path_parts) == 0:
        return metadata
    
    # Extract tool information
    if path_parts[0] == "k6":
        metadata["tool"] = "k6"
        # Extract version if present
        if len(path_parts) > 1:
            version_part = path_parts[1]
            if version_part == "next":
                metadata["version"] = "next"
            elif re.match(r"v\d+\.\d+\.x", version_part):
                metadata["version"] = version_part
            else:
                metadata["category"] = version_part
                
            # Extract category
            if len(path_parts) > 2 and metadata["version"] != "unknown":
                metadata["category"] = path_parts[2]
            elif len(path_parts) > 1 and metadata["version"] == "unknown":
                metadata["category"] = path_parts[1]
                
    elif path_parts[0] == "k6-studio":
        metadata["tool"] = "k6-studio"
        metadata["version"] = "current"
        # Extract category
        if len(path_parts) > 1:
            metadata["category"] = path_parts[1]
            
    elif path_parts[0] == "next":
        metadata["tool"] = "k6"
        metadata["version"] = "next"
        if len(path_parts) > 1:
            metadata["category"] = path_parts[1]
            
    elif re.match(r"v\d+\.\d+\.x", path_parts[0]):
        metadata["tool"] = "k6"
        metadata["version"] = path_parts[0]
        if len(path_parts) > 1:
            metadata["category"] = path_parts[1]
    
    return metadata

def parse_frontmatter(file_path: str) -> Dict[str, str]:
    """Parse front matter from markdown file."""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            post = frontmatter.load(f)
            return {
                "title": post.metadata.get("title", ""),
                "description": post.metadata.get("description", ""),
                "weight": str(post.metadata.get("weight", "")),
                "content": post.content
            }
    except Exception as e:
        print(f"Warning: Could not parse front matter for {file_path}: {e}")
        # Fallback to reading the file normally
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()
                return {"title": "", "description": "", "weight": "", "content": content}
        except Exception:
            return {"title": "", "description": "", "weight": "", "content": ""}

def should_include_directory(dir_path: str, base_path: str) -> bool:
    """Determine if a directory should be included based on content richness."""
    rel_path = os.path.relpath(dir_path, base_path)
    path_parts = Path(rel_path).parts
    
    # Always include k6-studio
    if len(path_parts) > 0 and path_parts[0] == "k6-studio":
        return True
    
    # For k6 versions, check if directory has substantial content
    if len(path_parts) >= 2 and path_parts[0] == "k6" and re.match(r"v\d+\.\d+\.x", path_parts[1]):
        version_dir = os.path.join(base_path, path_parts[0], path_parts[1])
        if not os.path.exists(version_dir):
            return False
            
        # Count markdown files (excluding just _index.md)
        md_files = []
        for root, dirs, files in os.walk(version_dir):
            md_files.extend([f for f in files if f.endswith('.md')])
        
        # Include if more than just _index.md or if it's a recent version
        has_content = len(md_files) > 1 or any(f != '_index.md' for f in md_files)
        is_recent = any(v in path_parts[1] for v in ['v0.5', 'v1.'])
        
        return has_content or is_recent
    
    return True

# --- 1. Load k6 documentation and create enhanced documents ---
try:
    docs_path = get_k6_docs_path()
    
    # Collect all markdown files with metadata
    enhanced_documents = []
    total_files_found = 0
    files_processed = 0
    
    for root, dirs, files in os.walk(docs_path):
        # Filter directories based on content richness
        if not should_include_directory(root, docs_path):
            continue
            
        for file in files:
            if file.endswith('.md'):
                total_files_found += 1
                file_path = os.path.join(root, file)
                
                # Extract metadata from path
                path_metadata = extract_metadata_from_path(file_path, docs_path)
                
                # Parse front matter and content
                fm_data = parse_frontmatter(file_path)
                
                if fm_data["content"].strip():  # Only include files with content
                    # Create enhanced document with metadata
                    doc = Document(
                        page_content=fm_data["content"],
                        metadata={
                            **path_metadata,
                            "title": fm_data["title"],
                            "description": fm_data["description"],
                            "weight": fm_data["weight"],
                            "source": file_path
                        }
                    )
                    enhanced_documents.append(doc)
                    files_processed += 1
    
    if not enhanced_documents:
        print("No markdown documents with content found. Check your directory path.")
        exit(1)
    
    print(f"Found {total_files_found} markdown files, processed {files_processed} with content.")
    documents = enhanced_documents
    
except Exception as e:
    print(f"Error loading k6 documentation: {e}")
    exit(1)

# --- 2. Chunk the documents into smaller pieces with preserved metadata ---
# This is crucial for getting relevant results.
text_splitter = RecursiveCharacterTextSplitter(chunk_size=1000, chunk_overlap=200)
docs_chunks = text_splitter.split_documents(documents)

# Enhance chunks with metadata from parent document
for chunk in docs_chunks:
    # Ensure all metadata is preserved in chunks
    if 'tool' not in chunk.metadata:
        chunk.metadata['tool'] = 'unknown'
    if 'version' not in chunk.metadata:
        chunk.metadata['version'] = 'unknown'
    if 'category' not in chunk.metadata:
        chunk.metadata['category'] = 'unknown'

if not docs_chunks:
    print("Could not split documents into chunks.")
    exit()

print(f"Split documents into {len(docs_chunks)} chunks.")

# Print metadata summary
tool_counts = {}
version_counts = {}
for chunk in docs_chunks:
    tool = chunk.metadata.get('tool', 'unknown')
    version = chunk.metadata.get('version', 'unknown')
    tool_counts[tool] = tool_counts.get(tool, 0) + 1
    version_counts[version] = version_counts.get(version, 0) + 1

print(f"Chunks by tool: {dict(sorted(tool_counts.items()))}")
print(f"Chunks by version: {dict(sorted(version_counts.items()))}")

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

# --- 4. Summary ---
print("\nIngestion completed successfully!")
print(f"Total documents processed: {len(documents)}")
print(f"Total chunks created: {len(docs_chunks)}")
print("Documents are now searchable with tool and version metadata.")