import os
import chromadb
from langchain_community.vectorstores import Chroma
from langchain_huggingface import HuggingFaceEmbeddings

# Disable ChromaDB telemetry to avoid telemetry errors
os.environ["ANONYMIZED_TELEMETRY"] = "False"

def verify_chromadb():
    """Verify ChromaDB instance and show statistics"""
    
    # Connect to ChromaDB
    chroma_client = chromadb.HttpClient(host="localhost", port=8000)
    print(f"✅ Successfully connected to ChromaDB at localhost:8000")
    
    # List all collections
    collections = chroma_client.list_collections()
    print(f"Available collections: {len(collections)}")
    for collection in collections:
        print(f"  - {collection.name}")
    
    # Get the k6_docs collection
    if not collections:
        print("No collections found! Make sure the ingestion script ran successfully.")
        return
    
    # Find k6_docs collection
    k6_collection = None
    for collection in collections:
        if collection.name == "k6_docs":
            k6_collection = collection
            break
    
    if not k6_collection:
        print("k6_docs collection not found!")
        return
    
    # Get collection info
    collection_info = k6_collection.get()
    print(f"\nk6_docs collection statistics:")
    print(f"  - Total documents: {len(collection_info['ids'])}")
    print(f"  - Sample IDs: {collection_info['ids'][:5]}")
    
    # Show sample documents
    print(f"\nSample documents:")
    for i in range(min(3, len(collection_info['documents']))):
        doc = collection_info['documents'][i]
        metadata = collection_info['metadatas'][i] if collection_info['metadatas'] else {}
        print(f"  Document {i+1}:")
        print(f"    ID: {collection_info['ids'][i]}")
        print(f"    Content preview: {doc[:200]}...")
        print(f"    Metadata: {metadata}")
        print("")

def interactive_search():
    """Interactive search function"""
    
    # Initialize the same embedding function used during ingestion
    embedding_function = HuggingFaceEmbeddings(model_name="all-MiniLM-L6-v2")
    
    # Connect to ChromaDB
    chroma_client = chromadb.HttpClient(host="localhost", port=8000)
    
    # Initialize Chroma vector store
    vectorstore = Chroma(
        collection_name="k6_docs",
        embedding_function=embedding_function,
        client=chroma_client
    )
    
    print("\n=== Interactive Search ===")
    print("Enter your search queries (type 'quit' to exit):")
    
    while True:
        query = input("\nSearch query: ").strip()
        
        if query.lower() in ['quit', 'exit', 'q']:
            break
        
        if not query:
            continue
        
        # Search for similar documents
        results = vectorstore.similarity_search_with_score(query, k=3)
        
        print(f"\nFound {len(results)} results for '{query}':")
        for i, (doc, score) in enumerate(results, 1):
            print(f"\n  Result {i} (similarity score: {score:.4f}):")
            print(f"    Source: {doc.metadata.get('source', 'Unknown')}")
            print(f"    Content: {doc.page_content[:300]}...")

if __name__ == "__main__":
    print("=== ChromaDB Verification Tool ===\n")
    
    try:
        # First, verify the database
        verify_chromadb()
        
        # Then offer interactive search
        while True:
            print("\nOptions:")
            print("1. Search the documentation")
            print("2. Show database statistics again")
            print("3. Quit")
            
            choice = input("\nEnter your choice (1-3): ").strip()
            
            if choice == "1":
                interactive_search()
            elif choice == "2":
                verify_chromadb()
            elif choice == "3":
                print("Goodbye!")
                break
            else:
                print("Invalid choice. Please enter 1, 2, or 3.")
    
    except Exception as e:
        print(f"Error: {e}")
        print("Make sure ChromaDB is running and accessible at localhost:8000") 