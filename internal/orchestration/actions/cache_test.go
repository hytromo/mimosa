package actions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/hasher"
	logger "github.com/hytromo/mimosa/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetCacheEntry(t *testing.T) {
	actioner := &Actioner{}
	hash := "test-hash-123"

	cache := actioner.GetCacheEntry(hash)

	assert.Equal(t, hash, cache.Hash)
	assert.NotNil(t, cache.InMemoryEntries)
}

func TestRemoveCacheEntry(t *testing.T) {
	tests := []struct {
		name   string
		dryRun bool
	}{
		{
			name:   "dry run",
			dryRun: true,
		},
		{
			name:   "not dry run",
			dryRun: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actioner := &Actioner{}
			cache := cacher.Cache{
				Hash: "test-hash",
			}

			// This will either log (dry run) or try to remove a non-existent file
			err := actioner.RemoveCacheEntry(cache, tt.dryRun)

			// Should not panic and should return nil for dry run
			if tt.dryRun {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSaveCache(t *testing.T) {
	tests := []struct {
		name         string
		tagsByTarget map[string][]string
		dryRun       bool
	}{
		{
			name: "simple tags",
			tagsByTarget: map[string][]string{
				"default": {"image:latest"},
			},
			dryRun: true, // Use dry run to avoid file system issues
		},
		{
			name: "multiple targets",
			tagsByTarget: map[string][]string{
				"app":     {"app:latest"},
				"backend": {"backend:v1.0"},
			},
			dryRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actioner := &Actioner{}
			cache := cacher.Cache{
				Hash:            "test-hash",
				CacheDir:        t.TempDir(),
				InMemoryEntries: cacher.GetAllInMemoryEntries(),
			}

			// Should not panic and should return nil for dry run
			err := actioner.SaveCache(cache, tt.tagsByTarget, tt.dryRun)
			assert.NoError(t, err)
		})
	}
}

func TestForgetCacheEntriesOlderThan(t *testing.T) {
	tests := []struct {
		name        string
		duration    string
		autoApprove bool
		expectError bool
	}{
		{
			name:        "empty duration with auto approve",
			duration:    "",
			autoApprove: true,
			expectError: false,
		},
		{
			name:        "valid duration with auto approve",
			duration:    "1h",
			autoApprove: true,
			expectError: false,
		},
		{
			name:        "valid duration without auto approve",
			duration:    "30m",
			autoApprove: false,
			expectError: false, // Will prompt for input, but we can't test that easily
		},
		{
			name:        "invalid duration",
			duration:    "invalid-duration",
			autoApprove: true,
			expectError: false, // parseDuration returns 0 for invalid input without error
		},
		{
			name:        "complex duration",
			duration:    "1d2h30m",
			autoApprove: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actioner := &Actioner{}

			err := actioner.ForgetCacheEntriesOlderThan(tt.duration, tt.autoApprove)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				// For non-auto-approve cases, we can't easily test the user input
				// but we can verify it doesn't panic
				assert.NotPanics(t, func() {
					err := actioner.ForgetCacheEntriesOlderThan(tt.duration, tt.autoApprove)
					assert.NoError(t, err)
				})
			}
		})
	}
}

func TestPrintCacheDir(t *testing.T) {
	// This test verifies that PrintCacheDir doesn't panic
	// The actual output depends on the cacher.CacheDir which is set at runtime
	actioner := &Actioner{}

	// Should not panic
	assert.NotPanics(t, func() {
		actioner.PrintCacheDir()
	})
}

func TestPrintCacheToEnvValue(t *testing.T) {
	// This test verifies that PrintCacheToEnvValue actually produces output
	actioner := &Actioner{}

	// Set up test environment variable with some cache data
	originalEnv := os.Getenv("MIMOSA_CACHE")
	defer func() { _ = os.Setenv("MIMOSA_CACHE", originalEnv) }()

	// Set some test cache data (format: "z85hash image:tag")
	z85Hash, err := hasher.HexToZ85("406b7725b0e93838b460e38d30903899")
	assert.NoError(t, err)
	_ = os.Setenv("MIMOSA_CACHE", z85Hash+" testimage:latest")

	// Capture the CleanLog output
	handler := logger.GetCleanLogHandler()
	if handler == nil {
		t.Fatal("Could not get CleanLog handler")
	}

	var output strings.Builder
	originalWriter := handler.GetWriter()
	handler.SetWriter(&output)
	defer func() {
		handler.SetWriter(originalWriter)
	}()

	// Should not panic and should produce output
	assert.NotPanics(t, func() {
		actioner.PrintCacheToEnvValue(cacher.CacheDir)
	})

	// Check that some output was produced
	outputStr := output.String()
	assert.NotEmpty(t, outputStr, "PrintCacheToEnvValue should produce output")
}

func TestPrintCacheToEnvValue_WithDiskAndEnvEntries(t *testing.T) {
	// This test verifies the case where both disk and env cache entries exist
	actioner := &Actioner{}

	// Create a temporary cache directory with disk entries
	tempDir := t.TempDir()

	// Create a disk cache file
	diskHash := "406b7725b0e93838b460e38d30903899"
	diskCache := cacher.CacheFile{
		TagsByTarget:  map[string][]string{"default": {"diskimage:latest"}},
		LastUpdatedAt: time.Now(),
	}
	diskData, err := json.Marshal(diskCache)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, diskHash+".json"), diskData, 0644)
	assert.NoError(t, err)

	// Set up environment variable with different cache data
	originalEnv := os.Getenv("MIMOSA_CACHE")
	defer func() { _ = os.Setenv("MIMOSA_CACHE", originalEnv) }()

	// Set env cache data that's different from disk cache
	z85Hash, err := hasher.HexToZ85("993080d3e8e460b838e3b0e5727b6406")
	assert.NoError(t, err)
	_ = os.Setenv("MIMOSA_CACHE", z85Hash+" envimage:latest")

	// Capture the CleanLog output
	handler := logger.GetCleanLogHandler()
	if handler == nil {
		t.Fatal("Could not get CleanLog handler")
	}

	var output strings.Builder
	originalWriter := handler.GetWriter()
	handler.SetWriter(&output)
	defer func() {
		handler.SetWriter(originalWriter)
	}()

	// Should not panic and should produce output
	assert.NotPanics(t, func() {
		actioner.PrintCacheToEnvValue(tempDir)
	})

	// Check that some output was produced
	outputStr := output.String()
	assert.NotEmpty(t, outputStr, "PrintCacheToEnvValue should produce output")

	// Should contain both disk and env entries
	assert.Contains(t, outputStr, "diskimage:latest", "Should contain disk cache entry")
	assert.Contains(t, outputStr, "envimage", "Should contain env cache entry")
}

func TestPrintCacheToEnvValue_EnvEntryExistsInDisk(t *testing.T) {
	// This test verifies the case where env cache entry exists but is also in disk cache
	actioner := &Actioner{}

	// Create a temporary cache directory with disk entries
	tempDir := t.TempDir()

	// Create a disk cache file
	diskHash := "406b7725b0e93838b460e38d30903899"
	diskCache := cacher.CacheFile{
		TagsByTarget:  map[string][]string{"default": {"sameimage:latest"}},
		LastUpdatedAt: time.Now(),
	}
	diskData, err := json.Marshal(diskCache)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, diskHash+".json"), diskData, 0644)
	assert.NoError(t, err)

	// Set up environment variable with the same hash as disk cache
	originalEnv := os.Getenv("MIMOSA_CACHE")
	defer func() { _ = os.Setenv("MIMOSA_CACHE", originalEnv) }()

	// Set env cache data with the same hash as disk cache
	z85Hash, err := hasher.HexToZ85(diskHash)
	assert.NoError(t, err)
	_ = os.Setenv("MIMOSA_CACHE", z85Hash+" sameimage:latest")

	// Capture the CleanLog output
	handler := logger.GetCleanLogHandler()
	if handler == nil {
		t.Fatal("Could not get CleanLog handler")
	}

	var output strings.Builder
	originalWriter := handler.GetWriter()
	handler.SetWriter(&output)
	defer func() {
		handler.SetWriter(originalWriter)
	}()

	// Should not panic and should produce output
	assert.NotPanics(t, func() {
		actioner.PrintCacheToEnvValue(tempDir)
	})

	// Check that some output was produced
	outputStr := output.String()
	assert.NotEmpty(t, outputStr, "PrintCacheToEnvValue should produce output")

	// Should contain disk entry but not duplicate env entry
	assert.Contains(t, outputStr, "sameimage:latest", "Should contain disk cache entry")
	// The env entry should not be printed again since it exists in disk cache
}
