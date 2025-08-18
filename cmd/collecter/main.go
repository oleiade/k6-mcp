package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oleiade/k6-mcp/internal"
)

const (
	typesRepo    = "https://github.com/DefinitelyTyped/DefinitelyTyped.git"
	typesRepoDir = "DefinitelyTyped"
)

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	destDir := filepath.Join(workDir, internal.DistFolderName, internal.DistDefinitionsFolderName, internal.DistTypesFolderName, internal.DistK6FolderName)

	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		log.Printf("Removing existing dist definitions directory: %s", destDir)
		os.RemoveAll(destDir)
	}

	err = cloneTypesRepository(typesRepo, destDir)
	if err != nil {
		log.Fatalf("Failed to clone types repository: %v", err)
	}

	if err := cleanUpTypesRepository(destDir); err != nil {
		log.Fatalf("Failed to clean up types repository: %v", err)
	}
}

// cloneTypesRepository clones the types repository and sets the sparse checkout to the types source path
// repoURL is the URL of the types repository
// repoDir is the directory to clone the types repository to
func cloneTypesRepository(repoURL, repoDir string) error {
	cmd := exec.Command("git", "clone", "--filter=blob:none", "--sparse", repoURL, repoDir)
	var cloneStderr bytes.Buffer
	cmd.Stderr = &cloneStderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to clone types repository; reason: %s", cloneStderr.String())
	}

	cmd = exec.Command("git", "-C", repoDir, "sparse-checkout", "set", "types/k6")
	var sparseStderr bytes.Buffer
	cmd.Stderr = &sparseStderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set sparse checkout; reason: %s", sparseStderr.String())
	}

	// Move the checked-out subtree (types/k6) up to repoDir so that repoDir mirrors the k6 types folder
	srcDir := filepath.Join(repoDir, "types", "k6")
	tmpDir := repoDir + ".tmp"
	if err := os.Rename(srcDir, tmpDir); err != nil {
		return fmt.Errorf("failed to move %s to temporary location %s: %w", srcDir, tmpDir, err)
	}
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to clear repository directory %s: %w", repoDir, err)
	}
	if err := os.Rename(tmpDir, repoDir); err != nil {
		return fmt.Errorf("failed to move temporary directory back to %s: %w", repoDir, err)
	}

	return nil
}

func cleanUpTypesRepository(repoDir string) error {
	// First pass: remove any file that does not end with .d.ts
	removeNonDTS := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), internal.DistDTSFileSuffix) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove file %s: %w", path, err)
			}
		}
		return nil
	}

	if err := filepath.WalkDir(repoDir, removeNonDTS); err != nil {
		return err
	}

	// Second pass: gather directories and prune empty ones from deepest to root
	var directories []string
	collectDirs := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			directories = append(directories, path)
		}
		return nil
	}

	if err := filepath.WalkDir(repoDir, collectDirs); err != nil {
		return err
	}

	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, dir := range directories {
		_ = os.Remove(dir) // remove only if empty
	}

	return nil
}
