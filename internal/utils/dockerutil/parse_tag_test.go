package dockerutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	// Test valid tag
	result, err := ParseTag("alpine:latest")
	require.NoError(t, err)
	assert.Equal(t, "index.docker.io", result.Registry)
	assert.Equal(t, "latest", result.Tag)
	assert.Equal(t, "library/alpine", result.ImageName)

	// Test valid digest
	result, err = ParseTag("alpine@sha256:3c7497bf0c7af934282c735445aa1e9093a105f9e6ab4f6127df97cdd9e5e8f1")
	require.NoError(t, err)
	assert.Equal(t, "index.docker.io", result.Registry)
	assert.Equal(t, "sha256:3c7497bf0c7af934282c735445aa1e9093a105f9e6ab4f6127df97cdd9e5e8f1", result.Tag)

	// Test invalid reference
	result, err = ParseTag("invalid:tag:format")
	assert.Error(t, err)
	assert.Equal(t, "invalid image reference", err.Error())
	assert.Equal(t, ParsedTag{}, result)

	// Test ECR
	result, err = ParseTag("1234567890.dkr.ecr.us-east-1.amazonaws.com/mimosa:latest")
	require.NoError(t, err)
	assert.Equal(t, "1234567890.dkr.ecr.us-east-1.amazonaws.com", result.Registry)
	assert.Equal(t, "latest", result.Tag)
	assert.Equal(t, "mimosa", result.ImageName)

	// Test GCP
	result, err = ParseTag("us-docker.pkg.dev/ubuntu-os-cloud/ubuntu-os-cloud/ubuntu:latest")
	require.NoError(t, err)
	assert.Equal(t, "us-docker.pkg.dev", result.Registry)
	assert.Equal(t, "latest", result.Tag)
	assert.Equal(t, "ubuntu-os-cloud/ubuntu-os-cloud/ubuntu", result.ImageName)
}
