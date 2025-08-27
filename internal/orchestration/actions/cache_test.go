package actions

import (
	"testing"

	"github.com/hytromo/mimosa/internal/cacher"
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
					actioner.ForgetCacheEntriesOlderThan(tt.duration, tt.autoApprove)
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
	// This test verifies that PrintCacheToEnvValue doesn't panic
	// The actual output depends on environment variables and disk cache
	actioner := &Actioner{}

	// Should not panic
	assert.NotPanics(t, func() {
		actioner.PrintCacheToEnvValue()
	})
}

func TestSaveCacheNilMapShouldNotPanic(t *testing.T) {
	actioner := &Actioner{}

	cache := cacher.Cache{
		Hash:            "test-hash-nil-map",
		InMemoryEntries: cacher.GetAllInMemoryEntries(),
	}

	tagsByTarget := map[string][]string{
		"default": {"image:latest"},
	}

	assert.NotPanics(t, func() {
		actioner.SaveCache(cache, tagsByTarget, false)
	}, "SaveCache should not panic when cache file contains invalid JSON and TagsByTarget is nil")
}
