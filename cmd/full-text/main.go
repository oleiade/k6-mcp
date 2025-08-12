package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Chunk struct {
	Title   string
	Content string
	Path    string
}

// --- Markdown Parsing + Chunking ---

func parseMarkdown(path string) ([]Chunk, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(src))

	var chunks []Chunk
	var currentTitle string
	var buffer strings.Builder
	const maxWords = 300 // target ~500–1000 tokens

	flush := func() {
		if buffer.Len() > 0 {
			chunks = append(chunks, Chunk{
				Title:   currentTitle,
				Content: strings.TrimSpace(buffer.String()),
				Path:    path,
			})
			buffer.Reset()
		}
	}

	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch node := n.(type) {
		case *ast.Heading:
			if node.Level <= 3 { // start new chunk on H1–H3
				flush()
				currentTitle = string(node.Text(src))
			} else {
				buffer.WriteString("\n" + string(node.Text(src)) + "\n")
			}
		case *ast.Paragraph:
			buffer.WriteString("\n" + string(node.Text(src)) + "\n")
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			buffer.WriteString("\nCode:\n" + string(node.Text(src)) + "\n")
		case *ast.List:
			buffer.WriteString("\n" + string(node.Text(src)) + "\n")
		}

		// If chunk is too long, split
		if wordCount(buffer.String()) > maxWords {
			flush()
		}

		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(doc)
	flush()
	return chunks, nil
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

// --- SQLite (FTS5) ---

func initDB(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal(err)
	}

	// Create FTS5 table for chunks
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunks
		USING fts5(title, content, path, tokenize='porter');
	`)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func insertChunk(db *sql.DB, c Chunk) {
	_, err := db.Exec(`INSERT INTO chunks (title, content, path) VALUES (?, ?, ?)`,
		c.Title, c.Content, c.Path)
	if err != nil {
		log.Fatal(err)
	}
}

func searchChunks(db *sql.DB, query string, limit int) []Chunk {
	rows, err := db.Query(`
		SELECT title, content, path
		FROM chunks
		WHERE chunks MATCH ?
		LIMIT ?`, query, limit)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var results []Chunk
	for rows.Next() {
		var c Chunk
		rows.Scan(&c.Title, &c.Content, &c.Path)
		results = append(results, c)
	}
	return results
}

// --- Main Program ---

func main() {
	db := initDB("docs.db")

	// Index all Markdown files in ./docs
	filepath.WalkDir("./docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			chunks, err := parseMarkdown(path)
			if err != nil {
				log.Printf("Failed parsing %s: %v", path, err)
				return nil
			}
			for _, c := range chunks {
				insertChunk(db, c)
			}
		}
		return nil
	})

	// Example query
	results := searchChunks(db, "ramping load", 5)
	for _, r := range results {
		fmt.Printf("[%s] %s\n%s\n\n", r.Path, r.Title, r.Content)
	}
}
