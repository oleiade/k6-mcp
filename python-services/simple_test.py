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

"""Simple test for metadata extraction."""

import sys
sys.path.append('.')

def test_path_extraction():
    """Test just the path extraction logic."""
    from ingest import extract_metadata_from_path
    
    base_path = "/Users/theocrevon/Dev/grafana/k6-docs/docs/sources"
    
    test_cases = [
        "k6/next/examples/basic.md",
        "k6/v1.0.x/javascript-api/k6.md", 
        "k6-studio/introduction.md",
    ]
    
    print("Testing metadata extraction:")
    for rel_path in test_cases:
        full_path = f"{base_path}/{rel_path}"
        metadata = extract_metadata_from_path(full_path, base_path)
        print(f"{rel_path}: tool={metadata['tool']}, version={metadata['version']}, category={metadata['category']}")

if __name__ == "__main__":
    test_path_extraction()