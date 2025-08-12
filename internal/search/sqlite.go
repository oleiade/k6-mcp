package search

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB opens (or creates) the SQLite database at the given path and ensures
// the FTS5 table exists with the intended tokenizer options. It drops any
// existing table named `chunks` to keep indexing idempotent per run.
func InitSQLiteDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Recreate FTS5 table for chunks with a single content column.
	// Use unicode61 tokenizer with extra token characters useful for code.
	if _, err := db.Exec(`DROP TABLE IF EXISTS chunks;`); err != nil {
		return nil, err
	}
	_, err = db.Exec(`
        CREATE VIRTUAL TABLE IF NOT EXISTS chunks
        USING fts5(
            title,
            content,
            path,
            tokenize = 'unicode61 remove_diacritics 2 tokenchars ''_/:#@-$'''
        );
    `)
	if err != nil {
		return nil, err
	}
	return db, nil
}
