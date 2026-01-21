package cacher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testHexHashRegistry = "406b7725b0e93838b460e38d30903899"
)

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
			expected: "index.docker.io/myreg1/myimage:mimosa-content-hash-" + testHexHashRegistry, // go-containerregistry normalizes docker.io to index.docker.io
			wantErr:  false,
		},
		{
			name:     "tag with registry domain",
			hash:     testHexHashRegistry,
			fullTag:  "docker.io/library/nginx:latest",
			expected: "index.docker.io/library/nginx:mimosa-content-hash-" + testHexHashRegistry, // go-containerregistry normalizes docker.io to index.docker.io
			wantErr:  false,
		},
		{
			name:     "tag with port",
			hash:     testHexHashRegistry,
			fullTag:  "localhost:5000/myimage:tag",
			expected: "localhost:5000/myimage:mimosa-content-hash-" + testHexHashRegistry,
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

func TestRegistryCache_Exists_EmptyTags(t *testing.T) {
	rc := &RegistryCache{
		Hash:         testHexHashRegistry,
		TagsByTarget: make(map[string][]string),
	}

	exists, cacheTags, err := rc.Exists()
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Nil(t, cacheTags)
}

func TestRegistryCache_SaveCacheTags_EmptyTags(t *testing.T) {
	rc := &RegistryCache{
		Hash:         testHexHashRegistry,
		TagsByTarget: make(map[string][]string),
	}

	err := rc.SaveCacheTags(false)
	assert.Error(t, err)
}

func TestRegistryCache_SaveCacheTags_DryRun(t *testing.T) {
	rc := &RegistryCache{
		Hash: testHexHashRegistry,
		TagsByTarget: map[string][]string{
			"default": {"myreg1/myimage:v1.0"},
		},
	}

	// Dry run should not fail
	err := rc.SaveCacheTags(true)
	assert.NoError(t, err)
}
