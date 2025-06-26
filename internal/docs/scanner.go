package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)


// DocumentInfo contains metadata about a k6 documentation file.
type DocumentInfo struct {
	Path        string
	URI         string
	Title       string
	Version     string
	Category    string
	Subcategory string
}

// Scanner discovers and catalogs k6 documentation files.
type Scanner struct {
	docsPath string
}

// NewScanner creates a new scanner with the specified documentation directory path.
func NewScanner(docsPath string) *Scanner {
	return &Scanner{
		docsPath: docsPath,
	}
}

// ScanDocuments scans the documentation directory and returns metadata for all found documents.
func (s *Scanner) ScanDocuments() ([]DocumentInfo, error) {
	var docs []DocumentInfo
	
	sourcesPath := filepath.Join(s.docsPath, "docs", "sources", "k6")
	
	err := filepath.Walk(sourcesPath, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		
		docInfo, err := s.parseDocumentInfo(path, sourcesPath)
		if err != nil {
			return err
		}
		
		docs = append(docs, docInfo)
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to walk documentation directory: %w", err)
	}
	
	return docs, nil
}

func (s *Scanner) parseDocumentInfo(filePath, basePath string) (DocumentInfo, error) {
	relPath, err := filepath.Rel(basePath, filePath)
	if err != nil {
		return DocumentInfo{}, fmt.Errorf("failed to get relative path: %w", err)
	}
	
	parts := strings.Split(relPath, string(filepath.Separator))
	
	doc := DocumentInfo{
		Path: filePath,
		URI:  "k6-docs://" + strings.Join(parts, "/"),
	}
	
	if len(parts) > 0 {
		doc.Version = parts[0]
	}
	
	if len(parts) > 1 {
		doc.Category = parts[1]
	}
	
	if len(parts) > minCategoryPartsCount-1 {
		doc.Subcategory = parts[2]
	}
	
	filename := filepath.Base(filePath)
	doc.Title = strings.TrimSuffix(filename, ".md")
	if doc.Title == "_index" {
		switch {
		case doc.Subcategory != "":
			doc.Title = doc.Subcategory
		case doc.Category != "":
			doc.Title = doc.Category
		default:
			doc.Title = doc.Version
		}
	}
	
	return doc, nil
}

// GetDocumentByURI retrieves document information for a specific URI.
func (s *Scanner) GetDocumentByURI(uri string) (*DocumentInfo, error) {
	if !strings.HasPrefix(uri, "k6-docs://") {
		return nil, &Error{Type: ErrorTypeValidation, Message: "invalid URI format"}
	}
	
	relPath := strings.TrimPrefix(uri, "k6-docs://")
	fullPath := filepath.Join(s.docsPath, "docs", "sources", "k6", relPath)
	
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, &Error{Type: ErrorTypeNotFound, Message: "document not found"}
	}
	
	doc, err := s.parseDocumentInfo(fullPath, filepath.Join(s.docsPath, "docs", "sources", "k6"))
	if err != nil {
		return nil, err
	}
	
	return &doc, nil
}