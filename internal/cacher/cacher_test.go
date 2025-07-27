package cacher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/hytromo/mimosa/internal/docker"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/hytromo/mimosa/internal/utils/fileutil"
)

func setupTempCacheDir(t *testing.T) (string, func()) {
	dir := t.TempDir()
	CacheDir = dir
	return dir, func() { _ = os.RemoveAll(dir) }
}

func newTestCache(hash string) *Cache {
	return &Cache{
		FinalHash:       hash,
		InMemoryEntries: orderedmap.NewOrderedMap[string, string](),
	}
}

func readCacheFile(t *testing.T, path string) CacheFile {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	var cf CacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("failed to unmarshal cache file: %v", err)
	}
	return cf
}

func TestCache_Save_DryRun(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("abc123")
	dataFile, err := cache.Save("tag1", true)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := os.Stat(dataFile); !os.IsNotExist(err) {
		t.Errorf("Expected no file created on dry run, but file exists: %v", dataFile)
	}
}

func TestCache_Save_NewFile(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("abc456")
	dataFile, err := cache.Save("tag1", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	cf := readCacheFile(t, dataFile)
	if len(cf.Tags) != 1 || cf.Tags[0] != "tag1" {
		t.Errorf("Expected tags [tag1], got %v", cf.Tags)
	}
	if time.Since(cf.LastUpdatedAt) > time.Minute {
		t.Errorf("LastUpdatedAt not recent: %v", cf.LastUpdatedAt)
	}
}

func TestCache_Save_AppendsUniqueTagsAndKeepsLast10(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("abc789")
	// Save 12 unique tags
	for i := 1; i <= 12; i++ {
		tag := "tag" + fmt.Sprint('A'+i-1)
		_, err := cache.Save(tag, false)
		if err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
	}
	cf := readCacheFile(t, cache.DataPath())
	if len(cf.Tags) != 10 {
		t.Errorf("Expected 10 tags, got %d: %v", len(cf.Tags), cf.Tags)
	}
	want := []string{"tag67", "tag68", "tag69", "tag70", "tag71", "tag72", "tag73", "tag74", "tag75", "tag76"}
	for i, tag := range want {
		if cf.Tags[i] != tag {
			t.Errorf("Tag at %d: got %q, want %q", i, cf.Tags[i], tag)
		}
	}
}

func TestCache_Save_DoesNotDuplicateTags(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("abcdup")
	_, err := cache.Save("tagX", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	_, err = cache.Save("tagX", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	cf := readCacheFile(t, cache.DataPath())
	if len(cf.Tags) != 1 || cf.Tags[0] != "tagX" {
		t.Errorf("Expected tags [tagX], got %v", cf.Tags)
	}
}

func TestCache_Save_ExistingFileWithInvalidJSON(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("abcinvalid")
	dataFile := cache.DataPath()
	err := os.MkdirAll(filepath.Dir(dataFile), 0755)
	if err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	err = os.WriteFile(dataFile, []byte("{invalid json"), 0644)
	if err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	_, err = cache.Save("tagZ", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	cf := readCacheFile(t, dataFile)
	if len(cf.Tags) != 1 || cf.Tags[0] != "tagZ" {
		t.Errorf("Expected tags [tagZ], got %v", cf.Tags)
	}
}

func TestCache_Save_ExistingFileWithEmptyTags(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("abcempt")
	dataFile := cache.DataPath()
	err := fileutil.SaveJSON(dataFile, CacheFile{Tags: []string{""}, LastUpdatedAt: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	_, err = cache.Save("tagY", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	cf := readCacheFile(t, dataFile)
	if len(cf.Tags) != 1 || cf.Tags[0] != "tagY" {
		t.Errorf("Expected tags [tagY], got %v", cf.Tags)
	}
}

func TestCache_Remove_DryRun(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("rmtest")
	dataFile, err := cache.Save("tag1", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := os.Stat(dataFile); err != nil {
		t.Fatalf("Expected file to exist before remove: %v", err)
	}
	err = cache.Remove(true)
	if err != nil {
		t.Fatalf("Remove (dry run) returned error: %v", err)
	}
	if _, err := os.Stat(dataFile); err != nil {
		// File should still exist
		t.Errorf("Expected file to still exist after dry run remove, got error: %v", err)
	}
}

func TestCache_Remove_Actual(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("rmtest2")
	dataFile, err := cache.Save("tag1", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := os.Stat(dataFile); err != nil {
		t.Fatalf("Expected file to exist before remove: %v", err)
	}
	err = cache.Remove(false)
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Stat(dataFile); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted after remove, but it exists or another error: %v", err)
	}
}

func TestCache_Exists(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("existtest")
	// Should not exist yet
	if cache.Exists() {
		t.Errorf("Expected Exists to be false before saving")
	}
	_, err := cache.Save("tag1", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if !cache.Exists() {
		t.Errorf("Expected Exists to be true after saving")
	}
	// Remove and check again
	err = cache.Remove(false)
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if cache.Exists() {
		t.Errorf("Expected Exists to be false after remove")
	}
}

func TestCache_LatestTag(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("latesttag")
	_, err := cache.Save("tag1", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	_, err = cache.Save("tag2", false)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	tag, err := cache.LatestTag()
	if err != nil {
		t.Fatalf("LatestTag returned error: %v", err)
	}
	if tag != "tag2" {
		t.Errorf("Expected latest tag to be 'tag2', got %q", tag)
	}
}

func TestCache_LatestTag_Empty(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := newTestCache("emptytag")
	dataFile := cache.DataPath()
	err := fileutil.SaveJSON(dataFile, CacheFile{Tags: []string{}, LastUpdatedAt: time.Now()})
	if err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}
	tag, err := cache.LatestTag()
	if err != nil {
		t.Fatalf("LatestTag returned error: %v", err)
	}
	if tag != "" {
		t.Errorf("Expected empty latest tag, got %q", tag)
	}
}

func TestForgetCacheEntriesOlderThan(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	// Create 3 cache files: one old, two new
	oldCache := newTestCache("oldcache")
	oldFile := oldCache.DataPath()
	err := fileutil.SaveJSON(oldFile, CacheFile{Tags: []string{"old"}, LastUpdatedAt: time.Now().Add(-2 * time.Hour)})
	if err != nil {
		t.Fatalf("Failed to write json")
	}

	newCache := newTestCache("newcache")
	newFile := newCache.DataPath()
	err = fileutil.SaveJSON(newFile, CacheFile{Tags: []string{"new"}, LastUpdatedAt: time.Now()})
	if err != nil {
		t.Fatalf("Failed to write json")
	}

	newCache2 := newTestCache("newcache2")
	newFile2 := newCache2.DataPath()
	err = fileutil.SaveJSON(newFile2, CacheFile{Tags: []string{"new2"}, LastUpdatedAt: time.Now()})
	if err != nil {
		t.Fatalf("Failed to write json")
	}

	ForgetCacheEntriesOlderThan(time.Now().Add(-1 * time.Hour))

	// Old file should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("Expected old cache file to be deleted, but it exists or another error: %v", err)
	}
	// New files should remain
	if _, err := os.Stat(newFile); err != nil {
		t.Errorf("Expected new cache file to exist, got error: %v", err)
	}
	if _, err := os.Stat(newFile2); err != nil {
		t.Errorf("Expected new cache file 2 to exist, got error: %v", err)
	}
}

func TestForgetCacheEntriesOlderThan_NoFiles(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	// Should not panic or error if no files exist
	ForgetCacheEntriesOlderThan(time.Now())
}

func TestGetCache_WithTempDockerfileAndContext(t *testing.T) {
	dir, cleanup := setupTempCacheDir(t)
	defer cleanup()

	// Create a Dockerfile in the temp dir
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	err := os.WriteFile(dockerfilePath, []byte("FROM busybox\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	cmd := docker.ParsedBuildCommand{
		CmdWithTagPlaceholder: []string{"docker", "build", "-t", "some:TAG", "."},
		RegistryDomain:        "docker.io",
		ContextPath:           dir,
		DockerignorePath:      "",
		DockerfilePath:        dockerfilePath,
	}

	cache, err := GetCache(cmd)
	if err != nil {
		t.Fatalf("GetCache returned error: %v", err)
	}
	if cache.FinalHash == "" {
		t.Errorf("Expected FinalHash to be set, got empty string")
	}
	if cache.InMemoryEntries == nil {
		t.Errorf("Expected InMemoryEntries to be initialized")
	}
}

func TestGetCache_WithDockerignoreAndDockerfile(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	// Create dummy files
	dockerignore := filepath.Join(CacheDir, ".dockerignore")
	dockerfile := filepath.Join(CacheDir, "Dockerfile")
	err := os.WriteFile(dockerignore, []byte("node_modules"), 0644)
	if err != nil {
		t.Fatalf("Failed to write .dockerignore: %v", err)
	}
	err = os.WriteFile(dockerfile, []byte("FROM busybox"), 0644)
	if err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	cmd := docker.ParsedBuildCommand{
		CmdWithTagPlaceholder: []string{"docker", "build", "-t", "some:TAG", "."},
		RegistryDomain:        "docker.io",
		ContextPath:           CacheDir,
		DockerignorePath:      dockerignore,
		DockerfilePath:        dockerfile,
	}

	cache, err := GetCache(cmd)
	if err != nil {
		t.Fatalf("GetCache returned error: %v", err)
	}
	if cache.FinalHash == "" {
		t.Errorf("Expected FinalHash to be set, got empty string")
	}
}

func TestGetCache_ErrorOnMissingFile(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cmd := docker.ParsedBuildCommand{
		CmdWithTagPlaceholder: []string{"docker", "build", "-t", "some:TAG", "."},
		RegistryDomain:        "docker.io",
		ContextPath:           "/nonexistent/path",
		DockerignorePath:      "/nonexistent/.dockerignore",
		DockerfilePath:        "/nonexistent/Dockerfile",
	}

	_, err := GetCache(cmd)
	if err == nil {
		t.Errorf("Expected error due to missing files, got nil")
	}
}

func TestCache_GetInMemoryEntry_Found(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	// Prepare a hash and its Z85 encoding
	hash := "0123456789abcdef0123456789abcdef"
	z85Hash, err := hasher.HexToZ85(hash)
	if err != nil {
		t.Fatalf("Failed to convert hash to Z85: %v", err)
	}

	cache := &Cache{
		FinalHash:       hash,
		InMemoryEntries: orderedmap.NewOrderedMap[string, string](),
	}
	cache.InMemoryEntries.Set(z85Hash, "mytag")

	val, ok := cache.GetInMemoryEntry()
	if !ok {
		t.Errorf("Expected to find in-memory entry, but did not")
	}
	if val != "mytag" {
		t.Errorf("Expected value 'mytag', got %q", val)
	}
}

func TestCache_GetInMemoryEntry_NotFound(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := &Cache{
		FinalHash:       "deadbeefdeadbeefdeadbeefdeadbeef",
		InMemoryEntries: orderedmap.NewOrderedMap[string, string](),
	}
	// No entry set
	val, ok := cache.GetInMemoryEntry()
	if ok {
		t.Errorf("Expected not to find in-memory entry, but got one: %q", val)
	}
}

func TestCache_GetInMemoryEntry_InvalidHash(t *testing.T) {
	_, cleanup := setupTempCacheDir(t)
	defer cleanup()

	cache := &Cache{
		FinalHash:       "not-a-hex-hash",
		InMemoryEntries: orderedmap.NewOrderedMap[string, string](),
	}
	// Should not panic, just return false
	val, ok := cache.GetInMemoryEntry()
	if ok {
		t.Errorf("Expected not to find in-memory entry for invalid hash, but got: %q", val)
	}
}
