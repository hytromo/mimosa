package cacher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_DataPath(t *testing.T) {
	tempDir := t.TempDir()
	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: tempDir,
	}
	expectedPath := filepath.Join(tempDir, "406b7725b0e93838b460e38d30903899.json")
	assert.Equal(t, expectedPath, cache.DataPath())
}

func TestCache_GetLatestTagByTarget(t *testing.T) {
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
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

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	err = os.WriteFile(cache.DataPath(), data, 0644)
	require.NoError(t, err)

	result, err := cache.GetLatestTagByTarget()
	require.NoError(t, err)

	expected := map[string]string{
		"target1": "tag3",
		"target2": "tagB",
	}
	assert.Equal(t, expected, result)
}

func TestCache_ExistsInFilesystem(t *testing.T) {
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: tempDir,
	}

	// Test case 1: File doesn't exist
	assert.False(t, cache.ExistsInFilesystem())

	// Test case 2: File exists
	err := os.WriteFile(cache.DataPath(), []byte("{}"), 0644)
	require.NoError(t, err)
	assert.True(t, cache.ExistsInFilesystem())
}

func TestCache_Remove(t *testing.T) {
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
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
	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: CacheDir, // Use default cache dir for this test
	}

	// Test case 1: No in-memory entries
	cache.InMemoryEntries = GetAllInMemoryEntries()
	entry, exists := cache.GetInMemoryEntry()
	assert.False(t, exists)
	assert.Equal(t, CacheFile{}, entry)

	// Test case 2: With in-memory entries
	z85Hash, err := hasher.HexToZ85("406b7725b0e93838b460e38d30903899")
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
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: tempDir,
	}
	cache.InMemoryEntries = GetAllInMemoryEntries()

	// Test case 1: Neither in-memory nor filesystem
	assert.False(t, cache.Exists())

	// Test case 2: Exists in filesystem
	err := os.WriteFile(cache.DataPath(), []byte("{}"), 0644)
	require.NoError(t, err)
	assert.True(t, cache.Exists())

	// Test case 3: Exists in memory
	z85Hash, err := hasher.HexToZ85("406b7725b0e93838b460e38d30903899")
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
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
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
	// Create a temporary cache directory
	tempDir := t.TempDir()

	// Create test cache files
	oldTime := time.Now().Add(-24 * time.Hour)
	newTime := time.Now()

	oldCache := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"old"}},
		LastUpdatedAt: oldTime,
	}

	newCache := CacheFile{
		TagsByTarget:  map[string][]string{"default": {"new"}},
		LastUpdatedAt: newTime,
	}

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
}

func TestGetAllInMemoryEntries(t *testing.T) {
	// Test case 1: No environment variable
	os.Unsetenv(EnvVarName)
	entries := GetAllInMemoryEntries()
	assert.Equal(t, 0, entries.Len())

	// Test case 2: Valid environment variable
	z85Hash, err := hasher.HexToZ85("406b7725b0e93838b460e38d30903899")
	require.NoError(t, err)

	envValue := z85Hash + " default:latest"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 1, entries.Len())

	entry, exists := entries.Get("406b7725b0e93838b460e38d30903899")
	assert.True(t, exists)
	assert.NotEmpty(t, entry.TagsByTarget["default"])
	assert.Equal(t, "latest", entry.TagsByTarget["default"][0])

	// Test case 3: Multiple targets
	envValue = z85Hash + " target1:tag1,target2:tag2"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 1, entries.Len())

	entry, exists = entries.Get("406b7725b0e93838b460e38d30903899")
	assert.True(t, exists)
	assert.Equal(t, "tag1", entry.TagsByTarget["target1"][0])
	assert.Equal(t, "tag2", entry.TagsByTarget["target2"][0])

	// Test case 4: Multiple cache entries
	z85Hash2, err := hasher.HexToZ85("993080d3e8e460b838e3b0e5727b6406")
	require.NoError(t, err)

	envValue = z85Hash + " default:latest\n" + z85Hash2 + " default:new"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 2, entries.Len())

	// Test case 5: Invalid entries (should be ignored)
	envValue = "invalid-key default:latest\n" + z85Hash + " default:valid"
	os.Setenv(EnvVarName, envValue)

	entries = GetAllInMemoryEntries()
	assert.Equal(t, 1, entries.Len())

	entry, exists = entries.Get("406b7725b0e93838b460e38d30903899")
	assert.True(t, exists)
	assert.Equal(t, "valid", entry.TagsByTarget["default"][0])

	// Clean up
	os.Unsetenv(EnvVarName)
}

func TestGetDiskCacheToMemoryEntries(t *testing.T) {
	// Create a temporary cache directory
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
	err = os.WriteFile(filepath.Join(tempDir, "406b7725b0e93838b460e38d30903899.json"), oldData, 0644)
	require.NoError(t, err)

	newData, err := json.Marshal(newCache)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "993080d3e8e460b838e3b0e5727b6406.json"), newData, 0644)
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
	z85NewHash, err := hasher.HexToZ85("993080d3e8e460b838e3b0e5727b6406")
	require.NoError(t, err)

	value, exists := entries.Get(z85NewHash)
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

func TestCacheFile_MarshalUnmarshal(t *testing.T) {
	original := CacheFile{
		TagsByTarget: map[string][]string{
			"target1": {"tag1", "tag2"},
			"target2": {"tagA"},
		},
		LastUpdatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var unmarshaled CacheFile
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, original.TagsByTarget, unmarshaled.TagsByTarget)
	assert.Equal(t, original.LastUpdatedAt.Unix(), unmarshaled.LastUpdatedAt.Unix())
}

func TestGetLatestTagByTargetEmptyTagsSlice(t *testing.T) {
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: tempDir,
	}

	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"target1": {},
		},
		LastUpdatedAt: time.Now(),
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	err = os.WriteFile(cache.DataPath(), data, 0644)
	require.NoError(t, err)
	_, err = cache.GetLatestTagByTarget()
	assert.NoError(t, err)
}

func TestGetDiskCacheToMemoryEntriesEmptyTagsSlice(t *testing.T) {
	// Create a temporary cache directory
	tempDir := t.TempDir()

	// Create a cache file with empty tags slice
	cacheFile := CacheFile{
		TagsByTarget: map[string][]string{
			"default": {}, // Empty slice - this should not cause panic
		},
		LastUpdatedAt: time.Now(),
	}

	data, err := json.Marshal(cacheFile)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "406b7725b0e93838b460e38d30903899.json"), data, 0644)
	require.NoError(t, err)

	entries := GetDiskCacheToMemoryEntries(tempDir)
	assert.Equal(t, 0, entries.Len())
}

func TestSaveInvalidJsonInExistingFile(t *testing.T) {
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: tempDir,
	}

	// Create a corrupted cache file
	err := os.WriteFile(cache.DataPath(), []byte("invalid json content"), 0644)
	require.NoError(t, err)

	// Try to save new tags
	tagsByTarget := map[string][]string{
		"target1": {"newtag"},
	}

	err = cache.Save(tagsByTarget, false)
	// This should not fail, but the corrupted data might be lost
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
	// Create a temporary cache directory
	tempDir := t.TempDir()

	cache := &Cache{
		Hash:     "406b7725b0e93838b460e38d30903899",
		CacheDir: tempDir,
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
