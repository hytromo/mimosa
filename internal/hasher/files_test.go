package hasher

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
)

func createTempFileWithContent(t *testing.T, dir, content string) string {
	t.Helper()
	tmpfile, err := os.CreateTemp(dir, "testfile-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	defer func() {
		_ = tmpfile.Close()
	}()

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	return tmpfile.Name()
}

func TestHashFiles_EmptyInput(t *testing.T) {
	hash, err := HashFiles([]string{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if hash != "" {
		t.Errorf("Expected empty hash for empty input, got %q", hash)
	}
}

func TestHashFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	file := createTempFileWithContent(t, dir, "hello world")
	hash1, err := HashFiles([]string{file})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Hash should be non-empty
	if hash1 == "" {
		t.Error("Expected non-empty hash for single file")
	}
	// Changing file content should change hash
	if err := os.WriteFile(file, []byte("goodbye world"), 0644); err != nil {
		t.Fatalf("Failed to overwrite file: %v", err)
	}
	hash2, err := HashFiles([]string{file})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hash1 == hash2 {
		t.Error("Hash did not change after file content changed")
	}
}

func TestHashFiles_MultipleFiles_Deterministic(t *testing.T) {
	dir := t.TempDir()
	file1 := createTempFileWithContent(t, dir, "foo")
	file2 := createTempFileWithContent(t, dir, "bar")
	files := []string{file1, file2}
	hash1, err := HashFiles(files)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Hash should be same regardless of order
	hash2, err := HashFiles([]string{file2, file1})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("Hash should be order-independent, got %q and %q", hash1, hash2)
	}
	// Changing one file changes hash
	if err := os.WriteFile(file1, []byte("baz"), 0644); err != nil {
		t.Fatalf("Failed to overwrite file: %v", err)
	}
	hash3, err := HashFiles(files)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hash1 == hash3 {
		t.Error("Hash did not change after file content changed")
	}
}

func TestHashFiles_SameContentFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := createTempFileWithContent(t, dir, "same")
	file2 := createTempFileWithContent(t, dir, "same")
	hash1, err := HashFiles([]string{file1, file2})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	hash2, err := HashFiles([]string{file2, file1})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("Hash should be same for same content files regardless of order, got %q and %q", hash1, hash2)
	}
}

func TestHashFiles_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doesnotexist.txt")
	log.SetLevel(log.DebugLevel)
	// Should not panic or error, just skip
	hash, err := HashFiles([]string{file})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Should be empty since no file was hashed
	if hash != "00000000000000000000000000000000" {
		t.Fatalf("Expected zero-ed hash for non-existent file, got %q", hash)
	}
}

func TestHashFiles_LargeFile(t *testing.T) {
	dir := t.TempDir()
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	file := createTempFileWithContent(t, dir, string(largeContent))
	hash, err := HashFiles([]string{file})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hash == "" {
		t.Error("Expected non-empty hash for large file")
	}
}

func TestHashFiles_DuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	file := createTempFileWithContent(t, dir, "dup")
	hash1, err := HashFiles([]string{file, file})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	hash2, err := HashFiles([]string{file})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hash1 == "" || hash2 == "" {
		t.Error("Expected non-empty hashes")
	}
	// Hashes should be different because the file is included twice
	if hash1 == hash2 {
		t.Error("Hash should differ when file is included twice")
	}
}
