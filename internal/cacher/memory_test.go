package cacher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hytromo/mimosa/internal/hasher"
	log "github.com/sirupsen/logrus"
)

func TestGetAllInMemoryEntries_EmptyEnv(t *testing.T) {
	_ = os.Unsetenv("MIMOSA_CACHE")
	entries := GetAllInMemoryEntries()
	if entries.Len() != 0 {
		t.Errorf("Expected empty map, got %d entries", entries.Len())
	}
}

func TestGetAllInMemoryEntries_ValidEntries(t *testing.T) {
	z85, _ := hasher.HexToZ85("0123456789abcdef0123456789abcdef")
	_ = os.Setenv("MIMOSA_CACHE", z85+" tag1\n"+z85+" tag2")
	entries := GetAllInMemoryEntries()
	if entries.Len() != 1 {
		t.Errorf("Expected 1 entry, got %d", entries.Len())
	}
	val, ok := entries.Get(z85)
	if !ok || val != "tag2" {
		t.Errorf("Expected tag2 for %s, got %v", z85, val)
	}
}

func TestGetAllInMemoryEntries_InvalidLines(t *testing.T) {
	_ = os.Setenv("MIMOSA_CACHE", "badline\nkeyonly \nkey value extra\n")
	entries := GetAllInMemoryEntries()
	if entries.Len() != 0 {
		t.Errorf("Expected 0 entries, got %d", entries.Len())
		for key, value := range entries.AllFromFront() {
			t.Errorf("Invalid entry: %s -> %s", key, value)
		}
	}
}

func TestGetAllInMemoryEntries_InvalidZ85Key(t *testing.T) {
	// Should log warning, but not panic
	_ = os.Setenv("MIMOSA_CACHE", "notz85 tag1")
	// Set log level to debug to trigger debug output
	log.SetLevel(log.DebugLevel)
	_ = GetAllInMemoryEntries()
}

func TestGetDiskCacheToMemoryEntries_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	entries := GetDiskCacheToMemoryEntries()
	if entries.Len() != 0 {
		t.Errorf("Expected 0 entries, got %d", entries.Len())
	}
}

func TestGetDiskCacheToMemoryEntries_ValidFiles(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	hash := "0123456789abcdef0123456789abcdef"
	cf := CacheFile{Tags: []string{"tag1", "tag2"}, LastUpdatedAt: time.Now()}
	data, _ := json.Marshal(cf)
	_ = os.WriteFile(filepath.Join(dir, hash+".json"), data, 0644)
	entries := GetDiskCacheToMemoryEntries()
	z85, _ := hasher.HexToZ85(hash)
	val, ok := entries.Get(z85)
	if !ok || val != "tag2" {
		t.Errorf("Expected tag2 for %s, got %v", z85, val)
	}
}

func TestGetDiskCacheToMemoryEntries_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	hash := "badjsonhashbadjsonhashbadjsonha"
	_ = os.WriteFile(filepath.Join(dir, hash+".json"), []byte("{invalid json"), 0644)
	entries := GetDiskCacheToMemoryEntries()
	if entries.Len() != 0 {
		t.Errorf("Expected 0 entries, got %d", entries.Len())
	}
}

func TestGetDiskCacheToMemoryEntries_NonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	_ = os.WriteFile(filepath.Join(dir, "notjson.txt"), []byte("ignore me"), 0644)
	entries := GetDiskCacheToMemoryEntries()
	if entries.Len() != 0 {
		t.Errorf("Expected 0 entries, got %d", entries.Len())
	}
}

func TestGetDiskCacheToMemoryEntries_InvalidHash(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	// Not a valid hex string for hash
	_ = os.WriteFile(filepath.Join(dir, "nothex.json"), []byte(`{"tags":["tag1"],"last_updated_at":"2023-01-01T00:00:00Z"}`), 0644)
	_ = GetDiskCacheToMemoryEntries()
	// Should not panic, just skip
}

func TestGetDiskCacheToMemoryEntries_EmptyTags(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	hash := "0123456789abcdef0123456789abcdef"
	cf := CacheFile{Tags: []string{}, LastUpdatedAt: time.Now()}
	data, _ := json.Marshal(cf)
	_ = os.WriteFile(filepath.Join(dir, hash+".json"), data, 0644)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic due to empty tags, but did not panic")
		}
	}()
	_ = GetDiskCacheToMemoryEntries()
}

func TestGetDiskCacheToMemoryEntries_MultipleFilesSameHash(t *testing.T) {
	dir := t.TempDir()
	CacheDir = dir
	hash := "0123456789abcdef0123456789abcdef"
	hash2 := "0123456789abcdef0123456789abcdee"
	cf1 := CacheFile{Tags: []string{"tag1"}, LastUpdatedAt: time.Now().Add(-time.Hour)}
	cf2 := CacheFile{Tags: []string{"tag2"}, LastUpdatedAt: time.Now()}
	data1, _ := json.Marshal(cf1)
	data2, _ := json.Marshal(cf2)
	_ = os.WriteFile(filepath.Join(dir, hash+".json"), data1, 0644)
	_ = os.WriteFile(filepath.Join(dir, hash2+".json"), data2, 0644)
	entries := GetDiskCacheToMemoryEntries()
	if entries.Len() != 2 {
		t.Errorf("Expected 2 entries, got %d", entries.Len())
	}
}

func TestGetDiskCacheToMemoryEntries_WalkError(t *testing.T) {
	// Simulate by setting CacheDir to a non-existent path
	CacheDir = "/nonexistent/path/shouldnotexist"
	_ = GetDiskCacheToMemoryEntries()
	// Should not panic
}
