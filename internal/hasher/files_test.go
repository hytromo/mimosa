package hasher

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
	hash := HashFiles([]string{}, 1)
	if hash != "" {
		t.Errorf("Expected empty hash for empty input, got %q", hash)
	}
}

func TestHashFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	file := createTempFileWithContent(t, dir, "hello world")
	hash1 := HashFiles([]string{file}, 1)
	// Hash should be non-empty
	if hash1 == "" {
		t.Error("Expected non-empty hash for single file")
	}
	// Changing file content should change hash
	if err := os.WriteFile(file, []byte("goodbye world"), 0644); err != nil {
		t.Fatalf("Failed to overwrite file: %v", err)
	}
	hash2 := HashFiles([]string{file}, 1)
	if hash1 == hash2 {
		t.Error("Hash did not change after file content changed")
	}
}

func TestHashFiles_MultipleFiles_Deterministic(t *testing.T) {
	dir := t.TempDir()
	file1 := createTempFileWithContent(t, dir, "foo")
	file2 := createTempFileWithContent(t, dir, "bar")
	files := []string{file1, file2}
	hash1 := HashFiles(files, 1)
	// Hash should be same regardless of order
	hash2 := HashFiles([]string{file2, file1}, 1)
	if hash1 != hash2 {
		t.Errorf("Hash should be order-independent, got %q and %q", hash1, hash2)
	}
	// Changing one file changes hash
	if err := os.WriteFile(file1, []byte("baz"), 0644); err != nil {
		t.Fatalf("Failed to overwrite file: %v", err)
	}
	hash3 := HashFiles(files, 1)
	if hash1 == hash3 {
		t.Error("Hash did not change after file content changed")
	}
}

func TestHashFiles_SameContentFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := createTempFileWithContent(t, dir, "same")
	file2 := createTempFileWithContent(t, dir, "same")
	hash1 := HashFiles([]string{file1, file2}, 1)
	hash2 := HashFiles([]string{file2, file1}, 1)
	if hash1 != hash2 {
		t.Errorf("Hash should be same for same content files regardless of order, got %q and %q", hash1, hash2)
	}
}

func TestHashFiles_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doesnotexist.txt")
	// Debug level is set by default in slog
	// Should not panic or error, just skip
	hash := HashFiles([]string{file}, 1)
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
	hash := HashFiles([]string{file}, 1)
	if hash == "" {
		t.Error("Expected non-empty hash for large file")
	}
}

func TestHashFiles_DuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	file := createTempFileWithContent(t, dir, "dup")
	hash1 := HashFiles([]string{file, file}, 1)
	hash2 := HashFiles([]string{file}, 1)
	if hash1 == "" || hash2 == "" {
		t.Error("Expected non-empty hashes")
	}
	// Hashes should be different because the file is included twice
	if hash1 == hash2 {
		t.Error("Hash should differ when file is included twice")
	}
}

func TestHashFiles_MultipleWorkers(t *testing.T) {
	dir := t.TempDir()

	// Create multiple files
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		files[i] = createTempFileWithContent(t, dir, fmt.Sprintf("content %d", i))
	}

	// Test with different numbers of workers
	hash1 := HashFiles(files, 1)
	hash2 := HashFiles(files, 4)
	hash3 := HashFiles(files, 8)

	// All hashes should be the same regardless of worker count
	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hashes should be the same regardless of worker count: %q, %q, %q", hash1, hash2, hash3)
	}
}

func TestHashFiles_ZeroWorkers(t *testing.T) {
	dir := t.TempDir()
	file := createTempFileWithContent(t, dir, "test")

	// Test with zero workers
	hash := HashFiles([]string{file}, 0)
	if hash == "" {
		t.Error("Expected non-empty hash with zero workers")
	}
}

func TestHashFiles_NegativeWorkers(t *testing.T) {
	dir := t.TempDir()
	file := createTempFileWithContent(t, dir, "test")
	HashFiles([]string{file}, -1)
}

func TestHashFiles_MixedExistentAndNonExistent(t *testing.T) {
	dir := t.TempDir()
	existingFile := createTempFileWithContent(t, dir, "exists")
	nonExistentFile := filepath.Join(dir, "doesnotexist.txt")

	hash := HashFiles([]string{existingFile, nonExistentFile}, 1)
	if hash == "" {
		t.Error("Expected non-empty hash when at least one file exists")
	}
}

func TestHashFiles_AllNonExistent(t *testing.T) {
	dir := t.TempDir()
	nonExistentFile1 := filepath.Join(dir, "doesnotexist1.txt")
	nonExistentFile2 := filepath.Join(dir, "doesnotexist2.txt")

	hash := HashFiles([]string{nonExistentFile1, nonExistentFile2}, 1)
	if hash != "00000000000000000000000000000000" {
		t.Errorf("Expected zero-ed hash for all non-existent files, got %q", hash)
	}
}

func TestHashFiles_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyFile := createTempFileWithContent(t, dir, "")

	hash := HashFiles([]string{emptyFile}, 1)
	if hash == "" {
		t.Error("Expected non-empty hash for empty file")
	}
}

func TestHashFiles_SpecialCharactersInPath(t *testing.T) {
	dir := t.TempDir()

	// Create a file with special characters in the name
	specialName := "file with spaces and special chars!@#$%^&*().txt"
	specialFile := filepath.Join(dir, specialName)
	if err := os.WriteFile(specialFile, []byte("special content"), 0644); err != nil {
		t.Fatalf("Failed to create file with special name: %v", err)
	}

	hash := HashFiles([]string{specialFile}, 1)
	if hash == "" {
		t.Error("Expected non-empty hash for file with special characters in name")
	}
}

func TestHashFiles_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()

	// Create multiple files
	files := make([]string, 5)
	for i := 0; i < 5; i++ {
		files[i] = createTempFileWithContent(t, dir, fmt.Sprintf("content %d", i))
	}

	// Test concurrent access to HashFiles
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			hash := HashFiles(files, 2)
			if hash == "" {
				t.Error("Expected non-empty hash in concurrent access")
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestJoinHashes_EmptySlice(t *testing.T) {
	result := joinHashes([][]byte{})
	if len(result) != 0 {
		t.Errorf("Expected empty result for empty slice, got %d bytes", len(result))
	}
}

func TestJoinHashes_SingleHash(t *testing.T) {
	hash := []byte{1, 2, 3, 4}
	result := joinHashes([][]byte{hash})
	if len(result) != 4 {
		t.Errorf("Expected 4 bytes, got %d", len(result))
	}
	for i, b := range hash {
		if result[i] != b {
			t.Errorf("Expected byte %d to be %d, got %d", i, b, result[i])
		}
	}
}

func TestJoinHashes_MultipleHashes(t *testing.T) {
	hash1 := []byte{1, 2, 3}
	hash2 := []byte{4, 5, 6}
	hash3 := []byte{7, 8, 9}

	result := joinHashes([][]byte{hash1, hash2, hash3})
	expected := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

	if len(result) != len(expected) {
		t.Errorf("Expected %d bytes, got %d", len(expected), len(result))
	}
	for i, b := range expected {
		if result[i] != b {
			t.Errorf("Expected byte %d to be %d, got %d", i, b, result[i])
		}
	}
}

func TestJoinHashes_EmptyHashes(t *testing.T) {
	hash1 := []byte{}
	hash2 := []byte{1, 2, 3}
	hash3 := []byte{}

	result := joinHashes([][]byte{hash1, hash2, hash3})
	expected := []byte{1, 2, 3}

	if len(result) != len(expected) {
		t.Errorf("Expected %d bytes, got %d", len(expected), len(result))
	}
	for i, b := range expected {
		if result[i] != b {
			t.Errorf("Expected byte %d to be %d, got %d", i, b, result[i])
		}
	}
}
