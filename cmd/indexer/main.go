package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/oleiade/k6-mcp/internal/search"
)

const (
	defaultDatabaseName = "index.db"
)

const (
	k6DocsRepo     = "https://github.com/grafana/k6-docs.git"
	docsSourcePath = "docs/sources/k6"
	databaseName   = "index.db"
	distDir        = "dist"
)

type Version struct {
	Original string
	Major    int
	Minor    int
}

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	tempDir, err := os.MkdirTemp("", "k6-docs-*")
	if err != nil {
		log.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			log.Printf("Warning: Failed to clean up temporary directory %s: %v", tempDir, removeErr)
		}
	}()

	log.Printf("Cloning k6 documentation repository...")
	if err := cloneRepository(k6DocsRepo, tempDir); err != nil {
		log.Fatalf("Failed to clone k6-docs repository: %v", err)
	}

	docsDir := filepath.Join(tempDir, docsSourcePath)
	latestVersion, err := findLatestVersion(docsDir)
	if err != nil {
		log.Fatalf("Failed to find latest version: %v", err)
	}

	log.Printf("Using k6 documentation version: %s", latestVersion)
	docsPath := filepath.Join(docsDir, latestVersion)

	distPath := filepath.Join(workDir, distDir)
	if err := os.MkdirAll(distPath, 0o755); err != nil {
		log.Fatalf("Failed to create dist directory: %v", err)
	}

	databasePath := filepath.Join(distPath, databaseName)
	log.Printf("Generating SQLite database at: %s", databasePath)

	db, err := search.InitSQLiteDB(databasePath)
	if err != nil {
		log.Fatalf("Failed to initialize SQLite database: %v", err)
	}
	defer db.Close()

	indexer := search.NewSQLiteIndexer(db)
	count, err := indexer.IndexDirectory(docsPath)
	if err != nil {
		log.Fatalf("Failed to index documents: %v", err)
	}

	log.Printf("Successfully generated database with %d documents at: %s", count, databasePath)
}

func cloneRepository(repoURL, targetDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findLatestVersion(docsDir string) (string, error) {
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read docs directory: %w", err)
	}

	var versions []Version
	versionRegex := regexp.MustCompile(`^v(\d+)\.(\d+)\.x$`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == "next" {
			continue
		}

		matches := versionRegex.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		major, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		minor, err := strconv.Atoi(matches[2])
		if err != nil {
			continue
		}

		versions = append(versions, Version{
			Original: name,
			Major:    major,
			Minor:    minor,
		})
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no valid version directories found")
	}

	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Major != versions[j].Major {
			return versions[i].Major > versions[j].Major
		}
		return versions[i].Minor > versions[j].Minor
	})

	return versions[0].Original, nil
}

// func main() {
// 	docsPath := flag.String("docs-path", "", "Path to the directory containing the documents to index")
// 	distPath := flag.String("dist-path", "", "Path to the directory to save the index database to")
// 	databaseName := flag.String("database-name", defaultDatabaseName, "Name of the database to save the index to")
// 	flag.Parse()

// 	if strings.TrimSpace(*docsPath) == "" {
// 		log.Fatal("--docs-path is required and cannot be empty")
// 	}

// 	if strings.TrimSpace(*distPath) == "" {
// 		log.Fatal("--dist-path is required and cannot be empty")
// 	}

// 	// Resolve and validate paths
// 	resolvedDocsPath, err := resolvePath(*docsPath)
// 	if err != nil {
// 		log.Fatalf("Failed to resolve docs path: %v", err)
// 	}

// 	resolvedDistPath, err := resolvePath(*distPath)
// 	if err != nil {
// 		log.Fatalf("Failed to resolve dist path: %v", err)
// 	}

// 	// Ensure docs path exists and is a directory
// 	if err := ensureDirectoryExists(resolvedDocsPath, false); err != nil {
// 		log.Fatalf("Invalid docs path: %v", err)
// 	}

// 	// Ensure dist path exists (create if needed) with safe permissions
// 	if err := ensureDirectoryExists(resolvedDistPath, true); err != nil {
// 		log.Fatalf("Invalid dist path: %v", err)
// 	}

// 	// Sanitize and validate database name to avoid path traversal
// 	safeDBName, err := sanitizeDatabaseName(*databaseName)
// 	if err != nil {
// 		log.Fatalf("Invalid database name: %v", err)
// 	}

// 	databasePath := filepath.Join(resolvedDistPath, safeDBName)

// 	// Extra safety: ensure resulting DB path remains within dist directory
// 	if !isPathWithin(resolvedDistPath, databasePath) {
// 		log.Fatalf("Unsafe database path resolution: %s not within %s", databasePath, resolvedDistPath)
// 	}

// 	// Avoid indexing a directory that contains the destination DB file
// 	if isPathWithin(resolvedDocsPath, resolvedDistPath) {
// 		log.Fatalf("dist-path (%s) must not be inside docs-path (%s) to avoid indexing the database file", resolvedDistPath, resolvedDocsPath)
// 	}

// 	db, err := search.InitSQLiteDB(databasePath)
// 	if err != nil {
// 		log.Fatalf("Failed to initialize SQLite database: %v", err)
// 	}
// 	defer db.Close()

// 	indexer := search.NewSQLiteIndexer(db)
// 	count, err := indexer.IndexDirectory(resolvedDocsPath)
// 	if err != nil {
// 		log.Fatalf("Failed to index documents: %v", err)
// 	}
// 	log.Printf("Indexed %d documents, %s is ready to use", count, databasePath)
// }

// // resolvePath expands environment variables and user (~) home, cleans, and absolutizes the path.
// func resolvePath(input string) (string, error) {
// 	if input == "" {
// 		return "", fmt.Errorf("path is empty")
// 	}

// 	// Expand environment variables first (e.g., $HOME, ${VAR})
// 	expanded := os.ExpandEnv(input)

// 	// Expand leading ~ or ~user
// 	withHome, err := expandTilde(expanded)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Clean and convert to absolute
// 	cleaned := filepath.Clean(withHome)
// 	abs, err := filepath.Abs(cleaned)
// 	if err != nil {
// 		return "", err
// 	}

// 	return abs, nil
// }

// // expandTilde handles ~ and ~user expansions on Unix-like systems.
// func expandTilde(path string) (string, error) {
// 	if path == "" || path[0] != '~' {
// 		return path, nil
// 	}

// 	// Only '~' or '~/' -> current user's home
// 	if len(path) == 1 || path[1] == '/' || path[1] == '\\' {
// 		home, err := os.UserHomeDir()
// 		if err != nil {
// 			return "", fmt.Errorf("cannot determine user home directory: %w", err)
// 		}
// 		return filepath.Join(home, path[2:]), nil
// 	}

// 	// Handle ~user
// 	sep := strings.IndexAny(path, "/\\")
// 	var userPart, rest string
// 	if sep == -1 {
// 		userPart = path[1:]
// 		rest = ""
// 	} else {
// 		userPart = path[1:sep]
// 		rest = path[sep:]
// 	}

// 	u, err := user.Lookup(userPart)
// 	if err != nil {
// 		return "", fmt.Errorf("cannot lookup user %q: %w", userPart, err)
// 	}
// 	return filepath.Join(u.HomeDir, rest), nil
// }

// // ensureDirectoryExists verifies that the path exists and is a directory.
// // If createIfMissing is true, it will create the directory tree with 0700 permissions.
// func ensureDirectoryExists(dirPath string, createIfMissing bool) error {
// 	info, err := os.Stat(dirPath)
// 	if err == nil {
// 		if !info.IsDir() {
// 			return fmt.Errorf("%s exists but is not a directory", dirPath)
// 		}
// 		return nil
// 	}
// 	if !os.IsNotExist(err) {
// 		return err
// 	}
// 	if !createIfMissing {
// 		return fmt.Errorf("directory does not exist: %s", dirPath)
// 	}
// 	// Create with restrictive permissions
// 	if mkErr := os.MkdirAll(dirPath, 0o700); mkErr != nil {
// 		return fmt.Errorf("failed to create directory %s: %w", dirPath, mkErr)
// 	}
// 	return nil
// }

// // sanitizeDatabaseName restricts the database file name to a safe basename and allowed characters.
// func sanitizeDatabaseName(name string) (string, error) {
// 	if strings.TrimSpace(name) == "" {
// 		return "", fmt.Errorf("database name cannot be empty")
// 	}
// 	// Take only the last element to prevent any path components
// 	base := filepath.Base(name)
// 	if base != name {
// 		// Informative error rather than silently altering the name
// 		return "", fmt.Errorf("database name must not contain path separators: %q", name)
// 	}
// 	// Allow letters, numbers, dots, underscores, and dashes
// 	allowed := regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
// 	if !allowed.MatchString(base) {
// 		return "", fmt.Errorf("database name contains invalid characters: %q", name)
// 	}
// 	return base, nil
// }

// // isPathWithin checks whether candidate is within the base directory.
// func isPathWithin(baseDir, candidate string) bool {
// 	baseClean := filepath.Clean(baseDir)
// 	candidateClean := filepath.Clean(candidate)
// 	rel, err := filepath.Rel(baseClean, candidateClean)
// 	if err != nil {
// 		return false
// 	}
// 	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
// }

// package main

// import (
// 	"flag"
// 	"fmt"
// 	search2 "github.com/oleiade/k6-mcp/internal/search"
// 	"log"
// 	"os"
// 	"os/user"
// 	"path/filepath"
// 	"regexp"
// 	"strings"
// )

// const (
// 	defaultDatabaseName = "index.db"
// )

// func main() {
// 	docsPath := flag.String("docs-path", "", "Path to the directory containing the documents to index")
// 	distPath := flag.String("dist-path", "", "Path to the directory to save the index database to")
// 	databaseName := flag.String("database-name", defaultDatabaseName, "Name of the database to save the index to")
// 	flag.Parse()

// 	if strings.TrimSpace(*docsPath) == "" {
// 		log.Fatal("--docs-path is required and cannot be empty")
// 	}

// 	if strings.TrimSpace(*distPath) == "" {
// 		log.Fatal("--dist-path is required and cannot be empty")
// 	}

// 	// Resolve and validate paths
// 	resolvedDocsPath, err := resolvePath(*docsPath)
// 	if err != nil {
// 		log.Fatalf("Failed to resolve docs path: %v", err)
// 	}

// 	resolvedDistPath, err := resolvePath(*distPath)
// 	if err != nil {
// 		log.Fatalf("Failed to resolve dist path: %v", err)
// 	}

// 	// Ensure docs path exists and is a directory
// 	if err := ensureDirectoryExists(resolvedDocsPath, false); err != nil {
// 		log.Fatalf("Invalid docs path: %v", err)
// 	}

// 	// Ensure dist path exists (create if needed) with safe permissions
// 	if err := ensureDirectoryExists(resolvedDistPath, true); err != nil {
// 		log.Fatalf("Invalid dist path: %v", err)
// 	}

// 	// Sanitize and validate database name to avoid path traversal
// 	safeDBName, err := sanitizeDatabaseName(*databaseName)
// 	if err != nil {
// 		log.Fatalf("Invalid database name: %v", err)
// 	}

// 	databasePath := filepath.Join(resolvedDistPath, safeDBName)

// 	// Extra safety: ensure resulting DB path remains within dist directory
// 	if !isPathWithin(resolvedDistPath, databasePath) {
// 		log.Fatalf("Unsafe database path resolution: %s not within %s", databasePath, resolvedDistPath)
// 	}

// 	// Avoid indexing a directory that contains the destination DB file
// 	if isPathWithin(resolvedDocsPath, resolvedDistPath) {
// 		log.Fatalf("dist-path (%s) must not be inside docs-path (%s) to avoid indexing the database file", resolvedDistPath, resolvedDocsPath)
// 	}

// 	db, err := search2.InitSQLiteDB(databasePath)
// 	if err != nil {
// 		log.Fatalf("Failed to initialize SQLite database: %v", err)
// 	}
// 	defer db.Close()

// 	indexer := search2.NewSQLiteIndexer(db)
// 	count, err := indexer.IndexDirectory(resolvedDocsPath)
// 	if err != nil {
// 		log.Fatalf("Failed to index documents: %v", err)
// 	}
// 	log.Printf("Indexed %d documents, %s is ready to use", count, databasePath)
// }

// // resolvePath expands environment variables and user (~) home, cleans, and absolutizes the path.
// func resolvePath(input string) (string, error) {
// 	if input == "" {
// 		return "", fmt.Errorf("path is empty")
// 	}

// 	// Expand environment variables first (e.g., $HOME, ${VAR})
// 	expanded := os.ExpandEnv(input)

// 	// Expand leading ~ or ~user
// 	withHome, err := expandTilde(expanded)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Clean and convert to absolute
// 	cleaned := filepath.Clean(withHome)
// 	abs, err := filepath.Abs(cleaned)
// 	if err != nil {
// 		return "", err
// 	}

// 	return abs, nil
// }

// // expandTilde handles ~ and ~user expansions on Unix-like systems.
// func expandTilde(path string) (string, error) {
// 	if path == "" || path[0] != '~' {
// 		return path, nil
// 	}

// 	// Only '~' or '~/' -> current user's home
// 	if len(path) == 1 || path[1] == '/' || path[1] == '\\' {
// 		home, err := os.UserHomeDir()
// 		if err != nil {
// 			return "", fmt.Errorf("cannot determine user home directory: %w", err)
// 		}
// 		return filepath.Join(home, path[2:]), nil
// 	}

// 	// Handle ~user
// 	sep := strings.IndexAny(path, "/\\")
// 	var userPart, rest string
// 	if sep == -1 {
// 		userPart = path[1:]
// 		rest = ""
// 	} else {
// 		userPart = path[1:sep]
// 		rest = path[sep:]
// 	}

// 	u, err := user.Lookup(userPart)
// 	if err != nil {
// 		return "", fmt.Errorf("cannot lookup user %q: %w", userPart, err)
// 	}
// 	return filepath.Join(u.HomeDir, rest), nil
// }

// // ensureDirectoryExists verifies that the path exists and is a directory.
// // If createIfMissing is true, it will create the directory tree with 0700 permissions.
// func ensureDirectoryExists(dirPath string, createIfMissing bool) error {
// 	info, err := os.Stat(dirPath)
// 	if err == nil {
// 		if !info.IsDir() {
// 			return fmt.Errorf("%s exists but is not a directory", dirPath)
// 		}
// 		return nil
// 	}
// 	if !os.IsNotExist(err) {
// 		return err
// 	}
// 	if !createIfMissing {
// 		return fmt.Errorf("directory does not exist: %s", dirPath)
// 	}
// 	// Create with restrictive permissions
// 	if mkErr := os.MkdirAll(dirPath, 0o700); mkErr != nil {
// 		return fmt.Errorf("failed to create directory %s: %w", dirPath, mkErr)
// 	}
// 	return nil
// }

// // sanitizeDatabaseName restricts the database file name to a safe basename and allowed characters.
// func sanitizeDatabaseName(name string) (string, error) {
// 	if strings.TrimSpace(name) == "" {
// 		return "", fmt.Errorf("database name cannot be empty")
// 	}
// 	// Take only the last element to prevent any path components
// 	base := filepath.Base(name)
// 	if base != name {
// 		// Informative error rather than silently altering the name
// 		return "", fmt.Errorf("database name must not contain path separators: %q", name)
// 	}
// 	// Allow letters, numbers, dots, underscores, and dashes
// 	allowed := regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
// 	if !allowed.MatchString(base) {
// 		return "", fmt.Errorf("database name contains invalid characters: %q", name)
// 	}
// 	return base, nil
// }

// // isPathWithin checks whether candidate is within the base directory.
// func isPathWithin(baseDir, candidate string) bool {
// 	baseClean := filepath.Clean(baseDir)
// 	candidateClean := filepath.Clean(candidate)
// 	rel, err := filepath.Rel(baseClean, candidateClean)
// 	if err != nil {
// 		return false
// 	}
// 	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
// }
