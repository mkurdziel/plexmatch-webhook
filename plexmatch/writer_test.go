package plexmatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite_NewFile(t *testing.T) {
	dir := t.TempDir()
	content := "# test\ntvdbid: 123\n"

	result, err := AtomicWrite(dir, content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != WriteResultWritten {
		t.Errorf("expected WriteResultWritten, got %d", result)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".plexmatch"))
	if err != nil {
		t.Fatalf("read .plexmatch: %v", err)
	}
	if string(got) != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(got), content)
	}
}

func TestAtomicWrite_Noop(t *testing.T) {
	dir := t.TempDir()
	content := "# test\ntvdbid: 123\n"

	// Write once.
	if _, err := AtomicWrite(dir, content, false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Write again with same content.
	result, err := AtomicWrite(dir, content, false)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if result != WriteResultNoop {
		t.Errorf("expected WriteResultNoop, got %d", result)
	}
}

func TestAtomicWrite_OverwriteChanged(t *testing.T) {
	dir := t.TempDir()

	if _, err := AtomicWrite(dir, "old content\n", false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	result, err := AtomicWrite(dir, "new content\n", false)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if result != WriteResultWritten {
		t.Errorf("expected WriteResultWritten, got %d", result)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".plexmatch"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new content\n" {
		t.Errorf("unexpected content: %q", string(got))
	}
}

func TestAtomicWrite_MissingFolder(t *testing.T) {
	result, err := AtomicWrite("/nonexistent/path/12345", "test", false)
	if result != WriteResultNoFolder {
		t.Errorf("expected WriteResultNoFolder, got %d", result)
	}
	if err == nil {
		t.Error("expected error for missing folder")
	}
}

func TestAtomicWrite_DryRun(t *testing.T) {
	dir := t.TempDir()

	result, err := AtomicWrite(dir, "content\n", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != WriteResultDryRun {
		t.Errorf("expected WriteResultDryRun, got %d", result)
	}

	// File should not exist.
	if _, err := os.Stat(filepath.Join(dir, ".plexmatch")); !os.IsNotExist(err) {
		t.Error("file should not exist in dry-run mode")
	}
}
