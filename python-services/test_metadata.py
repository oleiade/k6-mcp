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

"""Test script to verify metadata extraction functionality."""

import os
from pathlib import Path
from ingest import extract_metadata_from_path, parse_frontmatter, get_k6_docs_path

def test_metadata_extraction():
    """Test metadata extraction from various file paths."""
    base_path = "/Users/theocrevon/Dev/grafana/k6-docs/docs/sources"
    
    test_cases = [
        ("k6/next/examples/basic.md", {"tool": "k6", "version": "next", "category": "examples"}),
        ("k6/v1.0.x/javascript-api/k6.md", {"tool": "k6", "version": "v1.0.x", "category": "javascript-api"}),
        ("k6-studio/introduction.md", {"tool": "k6-studio", "version": "current", "category": "introduction"}),
        ("k6-studio/set-up/installation.md", {"tool": "k6-studio", "version": "current", "category": "set-up"}),
    ]
    
    print("Testing metadata extraction:")
    for file_path, expected in test_cases:
        full_path = os.path.join(base_path, file_path)
        metadata = extract_metadata_from_path(full_path, base_path)
        
        print(f"\nFile: {file_path}")
        print(f"  Tool: {metadata['tool']} (expected: {expected['tool']})")
        print(f"  Version: {metadata['version']} (expected: {expected['version']})")
        print(f"  Category: {metadata['category']} (expected: {expected['category']})")
        
        # Check if matches expected
        matches = all(metadata[key] == expected[key] for key in expected)
        print(f"  ✅ Match: {matches}")

def test_actual_files():
    """Test with actual files from the documentation."""
    try:
        docs_path = get_k6_docs_path()
        print(f"\nTesting with actual files from: {docs_path}")
        
        # Find a few sample files
        sample_files = []
        for root, dirs, files in os.walk(docs_path):
            for file in files[:3]:  # Just test first 3 files in each directory
                if file.endswith('.md'):
                    sample_files.append(os.path.join(root, file))
                    if len(sample_files) >= 5:  # Limit to 5 samples
                        break
            if len(sample_files) >= 5:
                break
        
        for file_path in sample_files:
            metadata = extract_metadata_from_path(file_path, docs_path)
            fm_data = parse_frontmatter(file_path)
            
            print(f"\nFile: {os.path.relpath(file_path, docs_path)}")
            print(f"  Tool: {metadata['tool']}")
            print(f"  Version: {metadata['version']}")
            print(f"  Category: {metadata['category']}")
            print(f"  Title: {fm_data['title'][:50]}..." if fm_data['title'] else "  Title: (none)")
            print(f"  Content length: {len(fm_data['content'])} chars")
            
    except Exception as e:
        print(f"Error testing actual files: {e}")

if __name__ == "__main__":
    test_metadata_extraction()
    test_actual_files()