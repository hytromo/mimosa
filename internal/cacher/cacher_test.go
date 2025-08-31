package cacher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testHexHash  = "406b7725b0e93838b460e38d30903899"
	testZ85Hash  = "kX>M&U<1bbV%.{NfPXnZ"
	testHexHash2 = "993080d3e8e460b838e3b0e5727b6406"
	testZ85Hash2 = "Nj%fu>&ucYiodm8A^Is3"
)

func TestCache_DataPath(t *testing.T) {
	tempDir := t.TempDir()
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tempDir,
	}
	expectedPath := filepath.Join(tempDir, fmt.Sprintf("%s.json", testHexHash))
	assert.Equal(t, expectedPath, cache.DataPath())
}

func TestCache_GetLatestTagByTarget(t *testing.T) {
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tempDir,
	}

	// Test case 1: File doesn't exist
	_, err := cache.GetLatestTagByTarget()
	assert.Error(t, err)

	// Test case 2: Valid cache file
	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"target1": {"tag1", "tag2", "tag3"},
			"target2": {"tagA", "tagB"},
		},
		LastUpdatedAt: time.Now(),
	}

	assert.False(t, cache.ExistsInFilesystem())
	cache.Save(cacheFile.TagsByTarget, false)
	assert.True(t, cache.ExistsInFilesystem())

	result, err := cache.GetLatestTagByTarget()
	require.NoError(t, err)

	expected := map[string]string{
		"target1": "tag3",
		"target2": "tagB",
	}
	assert.Equal(t, expected, result)
}

func TestCache_Remove(t *testing.T) {
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tempDir,
	}

	// Create a test file
	err := os.WriteFile(cache.DataPath(), []byte("{}"), 0644)
	require.NoError(t, err)
	assert.True(t, cache.ExistsInFilesystem())

	// Test case 1: Dry run - file should still exist
	err = cache.Remove(true)
	assert.NoError(t, err)
	assert.True(t, cache.ExistsInFilesystem())

	// Test case 2: Actual removal
	err = cache.Remove(false)
	assert.NoError(t, err)
	assert.False(t, cache.ExistsInFilesystem())
}

func TestCache_GetInMemoryEntry(t *testing.T) {
	tmpDir := t.TempDir()

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tmpDir,
	}

	// Test case 1: No in-memory entries
	cache.InMemoryEntries = GetAllInMemoryEntries()
	entry, exists := cache.GetInMemoryEntry()
	assert.False(t, exists)
	assert.Equal(t, CacheFile{}, entry)

	// Test case 2: With in-memory entries
	z85Hash, err := hasher.HexToZ85(testHexHash)
	require.NoError(t, err)

	inMemoryEntries := GetAllInMemoryEntries()
	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"default": {"latest"},
		},
		LastUpdatedAt: time.Now(),
	}
	inMemoryEntries.Set(z85Hash, cacheFile)

	cache.InMemoryEntries = inMemoryEntries
	entry, exists = cache.GetInMemoryEntry()
	assert.True(t, exists)
	assert.Equal(t, cacheFile, entry)
}

func TestCache_Exists(t *testing.T) {
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tempDir,
	}
	cache.InMemoryEntries = GetAllInMemoryEntries()

	// Test case 1: Neither in-memory nor filesystem
	assert.False(t, cache.Exists())

	// Test case 2: Exists in filesystem
	cache.Save(map[string][]string{}, false)
	assert.True(t, cache.Exists())

	cache.Remove(false)
	assert.False(t, cache.Exists())

	// Test case 3: Exists in memory
	z85Hash, err := hasher.HexToZ85(testHexHash)
	require.NoError(t, err)

	inMemoryEntries := GetAllInMemoryEntries()
	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"default": {"latest"},
		},
		LastUpdatedAt: time.Now(),
	}
	inMemoryEntries.Set(z85Hash, cacheFile)

	cache.InMemoryEntries = inMemoryEntries
	assert.True(t, cache.Exists())
}

func TestCache_Save(t *testing.T) {
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tempDir,
	}

	// Test case 1: Dry run
	tagsByTarget := map[string][]string{
		"target1": {"tag1", "tag2"},
		"target2": {"tagA"},
	}

	err := cache.Save(tagsByTarget, true)
	assert.NoError(t, err)
	assert.False(t, cache.ExistsInFilesystem())

	// Test case 2: Actual save
	err = cache.Save(tagsByTarget, false)
	assert.NoError(t, err)
	assert.True(t, cache.ExistsInFilesystem())

	// Verify the saved content
	data, err := os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var savedCache CacheFile
	err = json.Unmarshal(data, &savedCache)
	require.NoError(t, err)

	assert.Equal(t, tagsByTarget, savedCache.TagsByTarget)
	assert.False(t, savedCache.LastUpdatedAt.IsZero())

	// Test case 3: Append to existing cache
	newTags := map[string][]string{
		"target1": {"tag3"},
		"target3": {"tagX"},
	}

	err = cache.Save(newTags, false)
	assert.NoError(t, err)

	// Verify appended content
	data, err = os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var updatedCache CacheFile
	err = json.Unmarshal(data, &updatedCache)
	require.NoError(t, err)

	expected := map[string][]string{
		"target1": {"tag1", "tag2", "tag3"},
		"target2": {"tagA"},
		"target3": {"tagX"},
	}
	assert.Equal(t, expected, updatedCache.TagsByTarget)

	// Test case 4: Limit to 10 tags per target
	manyTags := make([]string, 15)
	for i := range manyTags {
		manyTags[i] = "tag" + string(rune('A'+i))
	}

	overflowTags := map[string][]string{
		"target1": manyTags,
	}

	err = cache.Save(overflowTags, false)
	assert.NoError(t, err)

	data, err = os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var limitedCache CacheFile
	err = json.Unmarshal(data, &limitedCache)
	require.NoError(t, err)

	assert.Len(t, limitedCache.TagsByTarget["target1"], 10)
	assert.Equal(t, "tagO", limitedCache.TagsByTarget["target1"][9]) // Last tag should be 'O'
}

func TestForgetCacheEntriesOlderThan(t *testing.T) {
	tempDir := t.TempDir()

	// Create test cache files
	oldTime := time.Now().Add(-24 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)

	oldCache := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"old"}},
		LastUpdatedAt: oldTime,
	}

	newCache := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"new"}},
		LastUpdatedAt: newTime,
	}

	// manual saving in order to control the last updated at time
	// Save old cache
	oldData, err := json.Marshal(oldCache)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "old-hash.json"), oldData, 0644)
	require.NoError(t, err)

	// Save new cache
	newData, err := json.Marshal(newCache)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "new-hash.json"), newData, 0644)
	require.NoError(t, err)

	// Create a non-json file (should be ignored)
	err = os.WriteFile(filepath.Join(tempDir, "ignore.txt"), []byte("ignore"), 0644)
	require.NoError(t, err)

	// Test forgetting entries older than 12 hours ago
	forgetTime := time.Now().Add(-12 * time.Hour)
	err = ForgetCacheEntriesOlderThan(forgetTime, tempDir)
	assert.NoError(t, err)

	// Verify old cache was deleted
	_, err = os.Stat(filepath.Join(tempDir, "old-hash.json"))
	assert.True(t, os.IsNotExist(err))

	// Verify new cache still exists
	_, err = os.Stat(filepath.Join(tempDir, "new-hash.json"))
	assert.NoError(t, err)

	// Verify non-json file still exists
	_, err = os.Stat(filepath.Join(tempDir, "ignore.txt"))
	assert.NoError(t, err)

	// Test forgetting entries older than 10 minutes ago
	forgetTime = time.Now().Add(-10 * time.Minute)
	err = ForgetCacheEntriesOlderThan(forgetTime, tempDir)
	assert.NoError(t, err)

	// Verify new cache is also deleted
	_, err = os.Stat(filepath.Join(tempDir, "new-hash.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestGetAllInMemoryEntries(t *testing.T) {
	// Test case 1: No environment variable
	originalEnvValue := os.Getenv(EnvVarName)
	defer os.Setenv(EnvVarName, originalEnvValue)

	os.Unsetenv(EnvVarName)
	entries := GetAllInMemoryEntries()
	assert.Equal(t, 0, entries.Len())

	envValue := testZ85Hash + " default:latest"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 1, entries.Len())

	entry, exists := entries.Get(testZ85Hash)
	assert.True(t, exists)
	assert.NotEmpty(t, entry.TagsByTarget["default"])
	assert.Equal(t, "latest", entry.TagsByTarget["default"][0])

	// Test case 3: Multiple targets
	envValue = testZ85Hash + " target1:tag1,target2:tag2"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 1, entries.Len())

	entry, exists = entries.Get(testZ85Hash)
	assert.True(t, exists)
	assert.Equal(t, "tag1", entry.TagsByTarget["target1"][0])
	assert.Equal(t, "tag2", entry.TagsByTarget["target2"][0])

	// Test case 4: Multiple cache entries
	envValue = testZ85Hash + " default:latest\n" + testZ85Hash2 + " default:new"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 2, entries.Len())
}

func TestGetDiskCacheToMemoryEntries(t *testing.T) {
	tempDir := t.TempDir()

	// Create test cache files with proper hex hashes
	oldTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	oldCache := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"old"}},
		LastUpdatedAt: oldTime,
	}

	newCache := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"new"}},
		LastUpdatedAt: newTime,
	}

	multiTargetCache := CacheFile{
		TagsByTarget: map[string][]string{
			"target1": {"tag1"},
			"target2": {"tag2"},
		},
		LastUpdatedAt: newTime,
	}

	// Save cache files with proper hex hashes
	oldData, err := json.Marshal(oldCache)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("%s.json", testHexHash)), oldData, 0644)
	require.NoError(t, err)

	newData, err := json.Marshal(newCache)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("%s.json", testHexHash2)), newData, 0644)
	require.NoError(t, err)

	multiData, err := json.Marshal(multiTargetCache)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "1234567890abcdef1234567890abcdef.json"), multiData, 0644)
	require.NoError(t, err)

	// Create a non-json file (should be ignored)
	err = os.WriteFile(filepath.Join(tempDir, "ignore.txt"), []byte("ignore"), 0644)
	require.NoError(t, err)

	// Test getting disk cache entries
	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 3, entries.Len())

	// Test single target format
	value, exists := entries.Get(testZ85Hash2)
	assert.True(t, exists)
	assert.Equal(t, "new", value)

	// Test multi-target format
	z85MultiHash, err := hasher.HexToZ85("1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	value, exists = entries.Get(z85MultiHash)
	assert.True(t, exists)
	assert.Contains(t, value, "target1:tag1")
	assert.Contains(t, value, "target2:tag2")
}

func TestGetLatestTagByTargetEmptyTagsSlice(t *testing.T) {
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: t.TempDir(),
	}

	err := cache.Save(map[string][]string{
		"target1": {},
	}, false)

	require.NoError(t, err)

	tagsByTarget, err := cache.GetLatestTagByTarget()
	assert.NoError(t, err)
	assert.Empty(t, tagsByTarget)
}

func TestGetDiskCacheToMemoryEntriesEmptyTagsSlice(t *testing.T) {
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: tempDir,
	}

	err := cache.Save(map[string][]string{
		"target1": {},
	}, false)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 0, entries.Len())
	_, exists := entries.Get(testZ85Hash)
	assert.False(t, exists)
}

func TestSaveInvalidJsonInExistingFile(t *testing.T) {
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: t.TempDir(),
	}

	// Create a corrupted cache file
	err := os.WriteFile(cache.DataPath(), []byte("invalid json content"), 0644)
	require.NoError(t, err)

	// Try to save new tags
	tagsByTarget := map[string][]string{
		"target1": {"newtag"},
	}

	err = cache.Save(tagsByTarget, false)
	// This should not fail, but the corrupted data will be lost
	assert.NoError(t, err)

	// assert cache file exists and has the correct tags
	assert.True(t, cache.ExistsInFilesystem())

	data, err := os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var cacheFile CacheFile

	err = json.Unmarshal(data, &cacheFile)
	require.NoError(t, err)

	assert.Equal(t, "newtag", cacheFile.TagsByTarget["target1"][0])
	assert.Equal(t, 1, len(cacheFile.TagsByTarget["target1"]))
}

func TestSaveDuplicateTags(t *testing.T) {
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: t.TempDir(),
	}

	// Save the same tag multiple times
	tagsByTarget := map[string][]string{
		"target1": {"duplicate"},
	}

	err := cache.Save(tagsByTarget, false)
	assert.NoError(t, err)

	// Save the same tag again
	err = cache.Save(tagsByTarget, false)
	assert.NoError(t, err)

	latestTags, err := cache.GetLatestTagByTarget()
	assert.NoError(t, err)

	assert.Equal(t, "duplicate", latestTags["target1"])

	// Read the raw file to check for duplicates
	data, err := os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var cacheFile CacheFile
	err = json.Unmarshal(data, &cacheFile)
	require.NoError(t, err)

	// assert only a single tag is present
	assert.Equal(t, 1, len(cacheFile.TagsByTarget["target1"]))
	assert.Equal(t, "duplicate", cacheFile.TagsByTarget["target1"][0])
}

func TestGetLatestTagByTargetWithInvalidJson(t *testing.T) {
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: t.TempDir(),
	}

	// Create a cache file with invalid JSON
	err := os.WriteFile(cache.DataPath(), []byte("invalid json content"), 0644)
	require.NoError(t, err)

	_, err = cache.GetLatestTagByTarget()
	assert.Error(t, err)
}

func TestSaveWithDirectoryCreationError(t *testing.T) {
	// Test Save when directory creation fails
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: "/invalid/path/that/cannot/be/created", // This should cause MkdirAll to fail
	}

	tagsByTarget := map[string][]string{
		"target1": {"tag1"},
	}

	err := cache.Save(tagsByTarget, false)
	assert.Error(t, err)
}

func TestSaveWithFileWriteError(t *testing.T) {
	// Test Save when file writing fails
	// Create a read-only directory
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	err := os.MkdirAll(readOnlyDir, 0444) // Read-only permissions
	require.NoError(t, err)

	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: readOnlyDir,
	}

	tagsByTarget := map[string][]string{
		"target1": {"tag1"},
	}

	err = cache.Save(tagsByTarget, false)
	assert.Error(t, err)
}

func TestForgetCacheEntriesOlderThanWithWalkError(t *testing.T) {
	// Test ForgetCacheEntriesOlderThan with a non-existent directory
	err := ForgetCacheEntriesOlderThan(time.Now(), "/non/existent/directory")
	assert.Error(t, err)
}

func TestForgetCacheEntriesOlderThanWithInvalidJson(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cache file with invalid JSON
	err := os.WriteFile(filepath.Join(tempDir, "invalid.json"), []byte("invalid json"), 0644)
	require.NoError(t, err)

	// This should not fail, but should log an error
	err = ForgetCacheEntriesOlderThan(time.Now().Add(-1*time.Hour), tempDir)
	assert.NoError(t, err)
}

func TestForgetCacheEntriesOlderThanWithDeleteError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cache file
	cacheFile := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"old"}},
		LastUpdatedAt: time.Now().Add(-24 * time.Hour), // Old file
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	cachePath := filepath.Join(tempDir, "old-hash.json")
	err = os.WriteFile(cachePath, data, 0644)
	require.NoError(t, err)

	// Make the file read-only to prevent deletion
	err = os.Chmod(cachePath, 0444)
	require.NoError(t, err)

	// This should not fail
	err = ForgetCacheEntriesOlderThan(time.Now().Add(-12*time.Hour), tempDir)
	assert.NoError(t, err)
}

func TestGetAllInMemoryEntriesWithMalformedLine(t *testing.T) {
	// Test GetAllInMemoryEntries with malformed lines
	envValue := "key-only\nkey value extra\n default:latest"
	os.Setenv(EnvVarName, envValue)

	entries := GetAllInMemoryEntries()
	assert.Equal(t, 0, entries.Len())

	// Clean up
	os.Unsetenv(EnvVarName)
}

func TestGetAllInMemoryEntriesWithEmptyTarget(t *testing.T) {
	// Test GetAllInMemoryEntries with empty target
	z85Hash, err := hasher.HexToZ85(testHexHash)
	require.NoError(t, err)

	envValue := z85Hash + " :latest" // Empty target name
	os.Setenv(EnvVarName, envValue)

	entries := GetAllInMemoryEntries()
	assert.Equal(t, 1, entries.Len())

	entry, exists := entries.Get(z85Hash)
	assert.True(t, exists)
	// When target name is empty, it should be stored as empty string
	assert.Contains(t, entry.TagsByTarget, "")
	assert.Len(t, entry.TagsByTarget[""], 1)
	assert.Equal(t, "latest", entry.TagsByTarget[""][0])

	// Clean up
	os.Unsetenv(EnvVarName)
}

func TestGetDiskCacheToMemoryEntriesWithWalkError(t *testing.T) {
	// Test GetDiskCacheToMemoryEntries with non-existent directory
	entries := GetDiskCacheToMemoryEntries("/non/existent/directory")
	assert.Equal(t, 0, entries.Len())
}

func TestGetDiskCacheToMemoryEntriesWithReadError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a directory with the same name as a cache file to cause read error
	err := os.MkdirAll(filepath.Join(tempDir, "cache-file.json"), 0755)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 0, entries.Len())
}

func TestGetDiskCacheToMemoryEntriesWithInvalidJson(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cache file with invalid JSON
	err := os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("%s.json", testHexHash)), []byte("invalid json"), 0644)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 0, entries.Len())
}

func TestGetDiskCacheToMemoryEntriesWithInvalidHash(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cache file with invalid hash name
	cacheFile := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"latest"}},
		LastUpdatedAt: time.Now(),
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	// Use invalid hash that can't be converted to Z85
	err = os.WriteFile(filepath.Join(tempDir, "invalid-hash.json"), data, 0644)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 0, entries.Len())
}

func TestGetDiskCacheToMemoryEntriesWithEmptyTags(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cache file with empty tags for all targets
	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"target1": {},
			"target2": {},
		},
		LastUpdatedAt: time.Now(),
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("%s.json", testHexHash)), data, 0644)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 0, entries.Len())
}

func TestGetDiskCacheToMemoryEntriesWithMixedEmptyAndNonEmptyTags(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cache file with mixed empty and non-empty tags
	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"target1": {"tag1", "tag2"},
			"target2": {}, // Empty tags
		},
		LastUpdatedAt: time.Now(),
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("%s.json", testHexHash)), data, 0644)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 1, entries.Len())

	z85Hash, err := hasher.HexToZ85(testHexHash)
	require.NoError(t, err)

	value, exists := entries.Get(z85Hash)
	assert.True(t, exists)
	assert.Contains(t, value, "target1:tag2") // Should only include non-empty targets
}

func TestSaveWithExistingInvalidJsonFile(t *testing.T) {
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: t.TempDir(),
	}

	// Create an existing cache file with invalid JSON
	err := os.WriteFile(cache.DataPath(), []byte("invalid json content"), 0644)
	require.NoError(t, err)

	// Try to save new tags - should handle the invalid JSON gracefully
	tagsByTarget := map[string][]string{
		"target1": {"newtag"},
	}

	err = cache.Save(tagsByTarget, false)
	assert.NoError(t, err)

	// Verify the file was overwritten with valid content
	data, err := os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var cacheFile CacheFile
	err = json.Unmarshal(data, &cacheFile)
	require.NoError(t, err)

	assert.Equal(t, "newtag", cacheFile.TagsByTarget["target1"][0])
}

func TestSaveWithMoreThan10Tags(t *testing.T) {
	cache := &Cache{
		Hash:     testHexHash,
		CacheDir: t.TempDir(),
	}

	// Create initial cache with 5 tags
	initialTags := make([]string, 5)
	for i := range initialTags {
		initialTags[i] = fmt.Sprintf("tag%d", i)
	}

	err := cache.Save(map[string][]string{"target1": initialTags}, false)
	assert.NoError(t, err)

	// Add 8 more tags (total 13, should be limited to 10)
	additionalTags := make([]string, 8)
	for i := range additionalTags {
		additionalTags[i] = fmt.Sprintf("newtag%d", i)
	}

	err = cache.Save(map[string][]string{"target1": additionalTags}, false)
	assert.NoError(t, err)

	// Verify only the last 10 tags are kept
	latestTags, err := cache.GetLatestTagByTarget()
	require.NoError(t, err)

	// Read the raw file to check the number of tags
	data, err := os.ReadFile(cache.DataPath())
	require.NoError(t, err)

	var cacheFile CacheFile
	err = json.Unmarshal(data, &cacheFile)
	require.NoError(t, err)

	assert.Len(t, cacheFile.TagsByTarget["target1"], 10)
	assert.Equal(t, "newtag7", latestTags["target1"]) // Last tag should be newtag7
}
