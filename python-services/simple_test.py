#!/usr/bin/env python3
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