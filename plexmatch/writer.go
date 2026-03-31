package plexmatch

import (
	"fmt"
	"os"
	"path/filepath"
)

const plexmatchFilename = ".plexmatch"

// WriteResult describes the outcome of a write attempt.
type WriteResult int

const (
	WriteResultWritten  WriteResult = iota // File was written (new or changed)
	WriteResultNoop                        // File exists with same content
	WriteResultDryRun                      // Would have written but dry-run is enabled
	WriteResultNoFolder                    // Target folder does not exist
)

// AtomicWrite writes content to <seriesPath>/.plexmatch atomically.
// It writes to a temp file, fsyncs, then renames into place.
// Returns WriteResultNoop if the existing file already matches content.
// Returns WriteResultNoFolder if the series directory doesn't exist.
func AtomicWrite(seriesPath, content string, dryRun bool) (WriteResult, error) {
	// Check folder exists.
	info, err := os.Stat(seriesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return WriteResultNoFolder, fmt.Errorf("series path does not exist: %s", seriesPath)
		}
		return WriteResultNoFolder, fmt.Errorf("stat series path: %w", err)
	}
	if !info.IsDir() {
		return WriteResultNoFolder, fmt.Errorf("series path is not a directory: %s", seriesPath)
	}

	target := filepath.Join(seriesPath, plexmatchFilename)

	// Check if content is already up to date.
	existing, err := os.ReadFile(target)
	if err == nil && string(existing) == content {
		return WriteResultNoop, nil
	}

	if dryRun {
		return WriteResultDryRun, nil
	}

	// Write to temp file in the same directory for atomic rename.
	tmp, err := os.CreateTemp(seriesPath, ".plexmatch.tmp.*")
	if err != nil {
		return 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on any error.
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return 0, fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return 0, fmt.Errorf("fsync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		return 0, fmt.Errorf("rename temp to target: %w", err)
	}

	// Rename succeeded; prevent deferred cleanup.
	tmpPath = ""

	return WriteResultWritten, nil
}
