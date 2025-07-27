package fileutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Sample struct for testing
type sample struct {
	Name string
	Age  int
}

func TestSaveJSON_WritesPrettyJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.json")
	data := sample{Name: "Alice", Age: 30}

	err := SaveJSON(tmpFile, data)
	if err != nil {
		t.Fatalf("SaveJSON returned error: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var got sample
	if err := json.Unmarshal(content, &got); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if got != data {
		t.Errorf("File content = %+v, want %+v", got, data)
	}
}

func TestSaveJSON_InvalidPath(t *testing.T) {
	invalidPath := string([]byte{0}) // invalid filename on most OSes
	err := SaveJSON(invalidPath, sample{Name: "Bob", Age: 42})
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestSaveJSON_UnmarshalableData(t *testing.T) {
	ch := make(chan int)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bad.json")
	err := SaveJSON(tmpFile, ch)
	if err == nil {
		t.Error("Expected error for unmarshalable data, got nil")
	}
}
