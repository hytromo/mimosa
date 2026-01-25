package actions

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActioner_RetagFromCacheTags_Success(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}

	// Create a real test image
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("retag-test-%d", testID), "v1.0.0")

	// Create cache tag pairs for retagging within the same repository
	newTag := fmt.Sprintf("localhost:5000/retag-test-%d:v2.0.0", testID)
	cacheTagPairs := map[string][]cacher.CacheTagPair{
		"default": {
			{CacheTag: originalImage, NewTag: newTag},
		},
	}

	// Perform the retag
	err := actioner.RetagFromCacheTags(cacheTagPairs, false)
	require.NoError(t, err)

	// Verify the new tag exists in the registry
	err = testutils.CheckTagExists(newTag)
	assert.NoError(t, err, "New tag should exist after retag: %s", newTag)
}

func TestActioner_RetagFromCacheTags_DryRun(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}

	// Create a real test image
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("retag-dryrun-%d", testID), "v1.0.0")

	// Create cache tag pairs
	newTag := fmt.Sprintf("localhost:5000/retag-dryrun-%d:v2.0.0", testID)
	cacheTagPairs := map[string][]cacher.CacheTagPair{
		"default": {
			{CacheTag: originalImage, NewTag: newTag},
		},
	}

	// Perform dry run retag
	err := actioner.RetagFromCacheTags(cacheTagPairs, true)
	require.NoError(t, err)

	// Verify the new tag does NOT exist (dry run should not create it)
	err = testutils.CheckTagExists(newTag)
	assert.Error(t, err, "New tag should NOT exist after dry run: %s", newTag)
}

func TestActioner_RetagFromCacheTags_EmptyPairs(t *testing.T) {
	actioner := &Actioner{}

	// Empty cache tag pairs should return an error
	err := actioner.RetagFromCacheTags(map[string][]cacher.CacheTagPair{}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no cache tag pairs provided")
}

func TestActioner_RetagFromCacheTags_CrossRepoFails(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}

	// Create a real test image
	originalImage := testutils.CreateTestImage(t, fmt.Sprintf("cross-repo-%d", testID), "v1.0.0")

	// Try to retag to a different repository (should fail)
	differentRepoTag := fmt.Sprintf("localhost:5000/different-repo-%d:v1.0.0", testID)
	cacheTagPairs := map[string][]cacher.CacheTagPair{
		"default": {
			{CacheTag: originalImage, NewTag: differentRepoTag},
		},
	}

	err := actioner.RetagFromCacheTags(cacheTagPairs, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retagging across repositories is not supported")
}

func TestActioner_RetagFromCacheTags_MultipleTargets(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}

	// Create test images for multiple targets
	backendImage := testutils.CreateTestImage(t, fmt.Sprintf("backend-%d", testID), "v1.0.0")
	frontendImage := testutils.CreateTestImage(t, fmt.Sprintf("frontend-%d", testID), "v1.0.0")

	// Create cache tag pairs for multiple targets
	backendNewTag := fmt.Sprintf("localhost:5000/backend-%d:v2.0.0", testID)
	frontendNewTag := fmt.Sprintf("localhost:5000/frontend-%d:v2.0.0", testID)
	cacheTagPairs := map[string][]cacher.CacheTagPair{
		"backend": {
			{CacheTag: backendImage, NewTag: backendNewTag},
		},
		"frontend": {
			{CacheTag: frontendImage, NewTag: frontendNewTag},
		},
	}

	// Perform retag
	err := actioner.RetagFromCacheTags(cacheTagPairs, false)
	require.NoError(t, err)

	// Verify both new tags exist
	err = testutils.CheckTagExists(backendNewTag)
	assert.NoError(t, err, "Backend tag should exist: %s", backendNewTag)

	err = testutils.CheckTagExists(frontendNewTag)
	assert.NoError(t, err, "Frontend tag should exist: %s", frontendNewTag)
}

func TestActioner_CheckRegistryCacheExists_CacheHit(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}
	testHash := fmt.Sprintf("testhash%d", testID)

	// Create a test image that will serve as the "cache"
	imageName := fmt.Sprintf("cache-hit-test-%d", testID)
	cacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, cacher.CacheTagPrefix, testHash)

	// First create the base image, then create the cache tag
	baseImage := testutils.CreateTestImage(t, imageName, "base")

	// Create the cache tag by using RetagFromCacheTags
	cacheTagPairs := map[string][]cacher.CacheTagPair{
		"default": {
			{CacheTag: baseImage, NewTag: cacheTag},
		},
	}
	err := actioner.RetagFromCacheTags(cacheTagPairs, false)
	require.NoError(t, err)

	// Now check if cache exists for a tag in the same repo
	tagsByTarget := map[string][]string{
		"default": {fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)},
	}

	exists, cachePairs, err := actioner.CheckRegistryCacheExists(testHash, tagsByTarget)
	require.NoError(t, err)
	assert.True(t, exists, "Cache should exist")
	assert.NotNil(t, cachePairs)
	assert.Len(t, cachePairs["default"], 1)
	assert.Equal(t, cacheTag, cachePairs["default"][0].CacheTag)
}

func TestActioner_CheckRegistryCacheExists_CacheMiss(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}
	testHash := fmt.Sprintf("nonexistent%d", testID)

	// Check for a cache tag that doesn't exist
	tagsByTarget := map[string][]string{
		"default": {fmt.Sprintf("localhost:5000/nonexistent-%d:v1.0.0", testID)},
	}

	exists, cachePairs, err := actioner.CheckRegistryCacheExists(testHash, tagsByTarget)
	require.NoError(t, err)
	assert.False(t, exists, "Cache should not exist for non-existent image")
	assert.Nil(t, cachePairs)
}

func TestActioner_CheckRegistryCacheExists_EmptyTags(t *testing.T) {
	actioner := &Actioner{}

	// Empty tags should return an error
	exists, cachePairs, err := actioner.CheckRegistryCacheExists("somehash", map[string][]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tags to check")
	assert.False(t, exists)
	assert.Nil(t, cachePairs)
}

func TestActioner_SaveRegistryCacheTags_Success(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}
	testHash := fmt.Sprintf("savehash%d", testID)

	// Create a real test image
	imageName := fmt.Sprintf("save-cache-test-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	// Save cache tags
	tagsByTarget := map[string][]string{
		"default": {originalTag},
	}

	err := actioner.SaveRegistryCacheTags(testHash, tagsByTarget, false)
	require.NoError(t, err)

	// Verify the cache tag was created
	expectedCacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, cacher.CacheTagPrefix, testHash)
	err = testutils.CheckTagExists(expectedCacheTag)
	assert.NoError(t, err, "Cache tag should exist: %s", expectedCacheTag)
}

func TestActioner_SaveRegistryCacheTags_DryRun(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}
	testHash := fmt.Sprintf("dryhash%d", testID)

	// Create a real test image
	imageName := fmt.Sprintf("save-dryrun-test-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	// Save cache tags with dry run
	tagsByTarget := map[string][]string{
		"default": {originalTag},
	}

	err := actioner.SaveRegistryCacheTags(testHash, tagsByTarget, true)
	require.NoError(t, err)

	// Verify the cache tag was NOT created (dry run)
	expectedCacheTag := fmt.Sprintf("localhost:5000/%s:%s%s", imageName, cacher.CacheTagPrefix, testHash)
	err = testutils.CheckTagExists(expectedCacheTag)
	assert.Error(t, err, "Cache tag should NOT exist after dry run: %s", expectedCacheTag)
}

func TestActioner_SaveRegistryCacheTags_EmptyTags(t *testing.T) {
	actioner := &Actioner{}

	// Empty tags should return an error
	err := actioner.SaveRegistryCacheTags("somehash", map[string][]string{}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tags to save")
}

func TestActioner_SaveRegistryCacheTags_NonExistentSourceTag(t *testing.T) {
	testID := rand.IntN(10000000000)
	actioner := &Actioner{}

	// Try to save cache for a source tag that doesn't exist
	tagsByTarget := map[string][]string{
		"default": {fmt.Sprintf("localhost:5000/nonexistent-source-%d:v1.0.0", testID)},
	}

	err := actioner.SaveRegistryCacheTags("somehash", tagsByTarget, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create some cache tags")
}

func TestActioner_EndToEnd_CacheHitFlow(t *testing.T) {
	// This test simulates a complete cache hit flow:
	// 1. Build image and save cache tag
	// 2. Check if cache exists (should be true)
	// 3. Retag from cache to new tag

	testID := rand.IntN(10000000000)
	actioner := &Actioner{}
	testHash := fmt.Sprintf("e2ehash%d", testID)

	// Step 1: Create image and save cache
	imageName := fmt.Sprintf("e2e-test-%d", testID)
	originalTag := fmt.Sprintf("localhost:5000/%s:v1.0.0", imageName)
	testutils.CreateTestImage(t, imageName, "v1.0.0")

	tagsByTarget := map[string][]string{
		"default": {originalTag},
	}

	err := actioner.SaveRegistryCacheTags(testHash, tagsByTarget, false)
	require.NoError(t, err)

	// Step 2: Check cache exists
	exists, cachePairs, err := actioner.CheckRegistryCacheExists(testHash, tagsByTarget)
	require.NoError(t, err)
	assert.True(t, exists, "Cache should exist after saving")
	require.NotNil(t, cachePairs)
	require.Len(t, cachePairs["default"], 1)

	// Step 3: Retag from cache to new tag
	err = actioner.RetagFromCacheTags(cachePairs, false)
	require.NoError(t, err)

	// Verify the original tag still exists
	err = testutils.CheckTagExists(originalTag)
	assert.NoError(t, err, "Original tag should still exist: %s", originalTag)
}
