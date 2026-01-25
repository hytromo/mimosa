package cacher

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/hytromo/mimosa/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testHexHashRegistry = "406b7725b0e93838b460e38d30903899"
)

// =============================================================================
// Unit tests for GetCacheTagForRegistry (pure function, no registry needed)
// =============================================================================

func TestRegistryCache_GetCacheTagForRegistry(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		fullTag  string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple tag",
			hash:     testHexHashRegistry,
			fullTag:  "myreg1/myimage:v1.0",
			expected: "index.docker.io/myreg1/myimage:mimosa-content-hash-" + testHexHashRegistry,
			wantErr:  false,
		},
		{
			name:     "tag with registry domain",
			hash:     testHexHashRegistry,
			fullTag:  "docker.io/library/nginx:latest",
			expected: "index.docker.io/library/nginx:mimosa-content-hash-" + testHexHashRegistry,
			wantErr:  false,
		},
		{
			name:     "tag with port",
			hash:     testHexHashRegistry,
			fullTag:  "localhost:5000/myimage:tag",
			expected: "localhost:5000/myimage:mimosa-content-hash-" + testHexHashRegistry,
			wantErr:  false,
		},
		{
			name:     "nested repository path",
			hash:     testHexHashRegistry,
			fullTag:  "gcr.io/my-project/my-app:v1.0",
			expected: "gcr.io/my-project/my-app:mimosa-content-hash-" + testHexHashRegistry,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &RegistryCache{
				Hash:         tt.hash,
				TagsByTarget: make(map[string][]string),
			}

			result, err := rc.GetCacheTagForRegistry(tt.fullTag)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRegistryCache_GetCacheTagForRegistry_InvalidTag(t *testing.T) {
	rc := &RegistryCache{
		Hash:         testHexHashRegistry,
		TagsByTarget: make(map[string][]string),
	}

	// Test with invalid tag format (too many colons)
	result, err := rc.GetCacheTagForRegistry("invalid:tag:format:too:many:colons")
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "failed to parse tag")
}

func TestRegistryCache_GetCacheTagForRegistry_EmptyHash(t *testing.T) {
	rc := &RegistryCache{
		Hash:         "",
		TagsByTarget: make(map[string][]string),
	}

	result, err := rc.GetCacheTagForRegistry("localhost:5000/test:tag")
	require.NoError(t, err)
	// Should create cache tag with empty hash suffix
	assert.Equal(t, "localhost:5000/test:mimosa-content-hash-", result)
}

// =============================================================================
// Unit tests for error handling (no registry needed)
// =============================================================================

func TestRegistryCache_Exists_EmptyTagsByTarget(t *testing.T) {
	rc := &RegistryCache{
		Hash:         testHexHashRegistry,
		TagsByTarget: make(map[string][]string),
	}

	exists, cacheTags, err := rc.Exists()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tags to check")
	assert.False(t, exists)
	assert.Nil(t, cacheTags)
}

func TestRegistryCache_SaveCacheTags_EmptyTagsByTarget(t *testing.T) {
	rc := &RegistryCache{
		Hash:         testHexHashRegistry,
		TagsByTarget: make(map[string][]string),
	}

	err := rc.SaveCacheTags(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tags to save")
}

func TestRegistryCache_Exists_EmptyTagList(t *testing.T) {
	rc := &RegistryCache{
		Hash: testHexHashRegistry,
		TagsByTarget: map[string][]string{
			"target1": {}, // Empty tag list
		},
	}

	exists, cacheTags, err := rc.Exists()
	// Empty tag list means allExist becomes false
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, cacheTags)
}

func TestRegistryCache_Exists_InvalidTagSkipsGracefully(t *testing.T) {
	rc := &RegistryCache{
		Hash: testHexHashRegistry,
		TagsByTarget: map[string][]string{
			"target1": {"invalid:tag:format:too:many:colons"},
		},
	}

	exists, cacheTags, err := rc.Exists()
	// Invalid tags are skipped (logged), allExist becomes false, no error returned
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, cacheTags)
}

func TestRegistryCache_SaveCacheTags_InvalidTagSkipsGracefully(t *testing.T) {
	rc := &RegistryCache{
		Hash: testHexHashRegistry,
		TagsByTarget: map[string][]string{
			"target1": {"invalid:tag:format:too:many:colons"},
		},
	}

	// Invalid tags are skipped, no retag operations attempted
	err := rc.SaveCacheTags(false)
	assert.NoError(t, err)
}

func TestRegistryCache_SaveCacheTags_DryRun_LogsButDoesNothing(t *testing.T) {
	rc := &RegistryCache{
		Hash: testHexHashRegistry,
		TagsByTarget: map[string][]string{
			"default": {"myreg1/myimage:v1.0"},
		},
	}

	// Dry run should not fail and should not actually create tags
	err := rc.SaveCacheTags(true)
	assert.NoError(t, err)
}

// =============================================================================
// Integration tests (require local registry at localhost:5000)
// =============================================================================

func TestRegistryCache_Exists_CacheHit(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("cachehit%d", testID)

	// Create a test image
	imageName := fmt.Sprintf("exists-cache-hit-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	// Create the cache tag manually
	cacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, CacheTagPrefix, testHash)
	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"default": {originalTag},
		},
	}

	// First save the cache tag
	err := rc.SaveCacheTags(false)
	require.NoError(t, err)

	// Verify cache tag was created
	err = testutils.CheckTagExists(cacheTag)
	require.NoError(t, err, "Cache tag should exist: %s", cacheTag)

	// Now check if cache exists
	exists, cachePairs, err := rc.Exists()
	require.NoError(t, err)
	assert.True(t, exists, "Cache should exist")
	require.NotNil(t, cachePairs)
	require.Len(t, cachePairs["default"], 1)
	assert.Equal(t, cacheTag, cachePairs["default"][0].CacheTag)
	assert.Equal(t, originalTag, cachePairs["default"][0].NewTag)
}

func TestRegistryCache_Exists_CacheMiss(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("cachemiss%d", testID)

	// Create a test image but don't create cache tag
	imageName := fmt.Sprintf("exists-cache-miss-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"default": {originalTag},
		},
	}

	// Check if cache exists (it shouldn't)
	exists, cachePairs, err := rc.Exists()
	require.NoError(t, err)
	assert.False(t, exists, "Cache should not exist")
	assert.Nil(t, cachePairs)
}

func TestRegistryCache_Exists_PartialCacheMiss_DifferentTargets(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("partial%d", testID)

	// Create two test images in DIFFERENT repos
	imageName1 := fmt.Sprintf("partial-cache-1-%d", testID)
	imageName2 := fmt.Sprintf("partial-cache-2-%d", testID)
	tag1 := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName1)
	tag2 := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName2)
	testutils.CreateTestImage(t, imageName1, "v1.0.0")
	testutils.CreateTestImage(t, imageName2, "v1.0.0")

	// Create cache tag only for first image
	cacheTag1 := fmt.Sprintf("localhost:5000/%s:%s%s", imageName1, CacheTagPrefix, testHash)
	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"target1": {tag1}, // Only target1 for initial save
		},
	}
	err := rc.SaveCacheTags(false)
	require.NoError(t, err)

	// Verify first cache tag exists
	err = testutils.CheckTagExists(cacheTag1)
	require.NoError(t, err)

	// Now check with both targets - should return false because target2's cache doesn't exist
	rc.TagsByTarget = map[string][]string{
		"target1": {tag1},
		"target2": {tag2},
	}

	exists, cachePairs, err := rc.Exists()
	require.NoError(t, err)
	assert.False(t, exists, "Cache should not exist when not all targets have cache")
	assert.Nil(t, cachePairs)
}

func TestRegistryCache_Exists_MultipleTargets(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("multitarget%d", testID)

	// Create test images for multiple targets
	backendName := fmt.Sprintf("backend-exist-%d", testID)
	frontendName := fmt.Sprintf("frontend-exist-%d", testID)
	backendTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", backendName)
	frontendTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", frontendName)
	testutils.CreateTestImage(t, backendName, "v1.0.0")
	testutils.CreateTestImage(t, frontendName, "v1.0.0")

	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"backend":  {backendTag},
			"frontend": {frontendTag},
		},
	}

	// Save cache tags for both
	err := rc.SaveCacheTags(false)
	require.NoError(t, err)

	// Verify both cache tags exist
	backendCacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", backendName, CacheTagPrefix, testHash)
	frontendCacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", frontendName, CacheTagPrefix, testHash)
	err = testutils.CheckTagExists(backendCacheTag)
	require.NoError(t, err, "Backend cache tag should exist")
	err = testutils.CheckTagExists(frontendCacheTag)
	require.NoError(t, err, "Frontend cache tag should exist")

	// Check if all caches exist
	exists, cachePairs, err := rc.Exists()
	require.NoError(t, err)
	assert.True(t, exists, "All caches should exist")
	require.NotNil(t, cachePairs)
	assert.Len(t, cachePairs, 2)
	assert.Len(t, cachePairs["backend"], 1)
	assert.Len(t, cachePairs["frontend"], 1)
}

func TestRegistryCache_SaveCacheTags_Success(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("savesuccess%d", testID)

	// Create a test image
	imageName := fmt.Sprintf("save-success-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"default": {originalTag},
		},
	}

	// Save cache tag
	err := rc.SaveCacheTags(false)
	require.NoError(t, err)

	// Verify cache tag was created
	expectedCacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, CacheTagPrefix, testHash)
	err = testutils.CheckTagExists(expectedCacheTag)
	assert.NoError(t, err, "Cache tag should exist: %s", expectedCacheTag)
}

func TestRegistryCache_SaveCacheTags_DryRun_DoesNotCreateTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("savedryrun%d", testID)

	// Create a test image
	imageName := fmt.Sprintf("save-dryrun-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"default": {originalTag},
		},
	}

	// Save cache tag with dry run
	err := rc.SaveCacheTags(true)
	require.NoError(t, err)

	// Verify cache tag was NOT created
	expectedCacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, CacheTagPrefix, testHash)
	err = testutils.CheckTagExists(expectedCacheTag)
	assert.Error(t, err, "Cache tag should NOT exist after dry run: %s", expectedCacheTag)
}

func TestRegistryCache_SaveCacheTags_NonExistentSourceFails(t *testing.T) {
	testID := rand.IntN(10000000000)

	rc := &RegistryCache{
		Hash: "somehash",
		TagsByTarget: map[string][]string{
			"default": {fmt.Sprintf("localhost:5000/nonexistent-%d:v1.0.0", testID)},
		},
	}

	// Should fail because source tag doesn't exist
	err := rc.SaveCacheTags(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create some cache tags")
}

func TestRegistryCache_SaveCacheTags_MultipleTags(t *testing.T) {
	testID := rand.IntN(10000000000)
	testHash := fmt.Sprintf("multitag%d", testID)

	// Create test images with multiple tags
	imageName := fmt.Sprintf("multi-tag-%d", testID)
	tag1 := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	tag2 := fmt.Sprintf("localhost:5000/%s:v2.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")
	testutils.CreateTestImage(t, imageName, "v2.0.0")

	rc := &RegistryCache{
		Hash: testHash,
		TagsByTarget: map[string][]string{
			"default": {tag1, tag2},
		},
	}

	// Save cache tags
	err := rc.SaveCacheTags(false)
	require.NoError(t, err)

	// Both cache tags should point to the same hash
	cacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, CacheTagPrefix, testHash)
	err = testutils.CheckTagExists(cacheTag)
	assert.NoError(t, err, "Cache tag should exist: %s", cacheTag)
}
