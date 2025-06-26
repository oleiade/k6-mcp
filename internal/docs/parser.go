package docs

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
)

const (
	frontmatterColonSeparator = ":"
	frontmatterPartsCount     = 2
	minCategoryPartsCount     = 3
)

// DocumentContent represents the parsed content of a documentation file.
type DocumentContent struct {
	Frontmatter map[string]string
	Title       string
	Content     string
	RawContent  string
}

// Parser handles parsing of markdown documentation files.
type Parser struct{}

// NewParser creates a new documentation parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseDocument parses a markdown file and extracts its content and metadata.
func (p *Parser) ParseDocument(filePath string) (*DocumentContent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, &Error{Type: ErrorTypeIO, Message: "failed to open document", Err: err}
	}
	defer func() {
		_ = file.Close() // Ignore close error as we're already handling read errors
	}()
	
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, &Error{Type: ErrorTypeIO, Message: "failed to read document", Err: err}
	}
	
	doc := &DocumentContent{
		RawContent:  string(content),
		Frontmatter: make(map[string]string),
	}
	
	if err := p.parseFrontmatter(doc, content); err != nil {
		return nil, err
	}
	
	p.extractTitle(doc)
	p.extractContent(doc)
	
	return doc, nil
}

func (p *Parser) parseFrontmatter(doc *DocumentContent, content []byte) error {
	if !bytes.HasPrefix(content, []byte("---")) {
		doc.Content = string(content)
		return nil
	}
	
	return p.parseFrontmatterSections(doc, content)
}

func (p *Parser) parseFrontmatterSections(doc *DocumentContent, content []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var inFrontmatter bool
	var contentStart int
	lineNum := 0
	
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		
		if p.handleFrontmatterDelimiter(line, lineNum, &inFrontmatter, &contentStart) {
			break
		}
		
		if inFrontmatter {
			p.parseFrontmatterLine(line, doc)
		}
	}
	
	if contentStart > 0 && contentStart < len(content) {
		doc.Content = string(content[contentStart:])
	} else {
		doc.Content = string(content)
	}
	
	return nil
}

func (p *Parser) handleFrontmatterDelimiter(line string, lineNum int, inFrontmatter *bool, contentStart *int) bool {
	if lineNum == 1 && line == "---" {
		*inFrontmatter = true
		return false
	}
	
	if *inFrontmatter && line == "---" {
		*contentStart = lineNum * len(line) // Approximate calculation
		return true
	}
	
	return false
}

func (p *Parser) parseFrontmatterLine(line string, doc *DocumentContent) {
	if !strings.Contains(line, frontmatterColonSeparator) {
		return
	}
	
	parts := strings.SplitN(line, frontmatterColonSeparator, frontmatterPartsCount)
	if len(parts) != frontmatterPartsCount {
		return
	}
	
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `"'`)
	doc.Frontmatter[key] = value
}

func (p *Parser) extractTitle(doc *DocumentContent) {
	if title, exists := doc.Frontmatter["title"]; exists {
		doc.Title = title
		return
	}
	
	lines := strings.Split(doc.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			doc.Title = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			return
		}
	}
	
	doc.Title = "Untitled"
}

func (p *Parser) extractContent(doc *DocumentContent) {
	doc.Content = strings.TrimSpace(doc.Content)
}