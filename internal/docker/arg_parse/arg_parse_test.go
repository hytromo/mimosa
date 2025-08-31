package argparse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractRegistryDomain_ValidTags(t *testing.T) {
	testCases := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "Docker Hub default",
			tag:      "myapp:latest",
			expected: "index.docker.io",
		},
		{
			name:     "Docker Hub with username",
			tag:      "username/myapp:latest",
			expected: "index.docker.io",
		},
		{
			name:     "Docker Hub with organization",
			tag:      "organization/myapp:latest",
			expected: "index.docker.io",
		},
		{
			name:     "Custom registry",
			tag:      "registry.example.com/myapp:latest",
			expected: "registry.example.com",
		},
		{
			name:     "Custom registry with port",
			tag:      "registry.example.com:5000/myapp:latest",
			expected: "registry.example.com:5000",
		},
		{
			name:     "Custom registry with subdomain",
			tag:      "sub.registry.example.com/myapp:latest",
			expected: "sub.registry.example.com",
		},
		{
			name:     "Custom registry with path",
			tag:      "registry.example.com/path/myapp:latest",
			expected: "registry.example.com",
		},
		{
			name:     "Custom registry with multiple subdomains",
			tag:      "prod.registry.company.com/myapp:latest",
			expected: "prod.registry.company.com",
		},
		{
			name:     "IP address registry",
			tag:      "192.168.1.100:5000/myapp:latest",
			expected: "192.168.1.100:5000",
		},
		{
			name:     "Localhost registry",
			tag:      "localhost:5000/myapp:latest",
			expected: "localhost:5000",
		},
		{
			name:     "ECR registry",
			tag:      "123456789012.dkr.ecr.us-west-2.amazonaws.com/myapp:latest",
			expected: "123456789012.dkr.ecr.us-west-2.amazonaws.com",
		},
		{
			name:     "GCR registry",
			tag:      "gcr.io/myproject/myapp:latest",
			expected: "gcr.io",
		},
		{
			name:     "ACR registry",
			tag:      "myregistry.azurecr.io/myapp:latest",
			expected: "myregistry.azurecr.io",
		},
		{
			name:     "Quay registry",
			tag:      "quay.io/myapp:latest",
			expected: "quay.io",
		},
		{
			name:     "Harbor registry",
			tag:      "harbor.example.com/project/myapp:latest",
			expected: "harbor.example.com",
		},
		{
			name:     "Nexus registry",
			tag:      "nexus.example.com:8081/myapp:latest",
			expected: "nexus.example.com:8081",
		},
		{
			name:     "GitLab registry",
			tag:      "registry.gitlab.com/mygroup/myapp:latest",
			expected: "registry.gitlab.com",
		},
		{
			name:     "GitHub registry",
			tag:      "ghcr.io/myuser/myapp:latest",
			expected: "ghcr.io",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractRegistryDomain(tc.tag)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractRegistryDomain_InvalidTags(t *testing.T) {
	testCases := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "Empty tag",
			tag:      "",
			expected: "docker.io",
		},
		{
			name:     "Invalid format",
			tag:      "invalid:tag:format",
			expected: "docker.io",
		},
		{
			name:     "No tag",
			tag:      "myapp",
			expected: "index.docker.io",
		},
		{
			name:     "Just colon",
			tag:      ":",
			expected: "docker.io",
		},
		{
			name:     "Multiple colons",
			tag:      "myapp:tag:extra",
			expected: "docker.io",
		},
		{
			name:     "Special characters",
			tag:      "my@pp:latest",
			expected: "docker.io",
		},
		{
			name:     "Spaces",
			tag:      "my app:latest",
			expected: "docker.io",
		},
		{
			name:     "Unicode characters",
			tag:      "myapp-测试:latest",
			expected: "docker.io",
		},
		{
			name:     "Very long tag",
			tag:      strings.Repeat("a", 1000) + ":latest",
			expected: "docker.io",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractRegistryDomain(tc.tag)
			assert.Equal(t, tc.expected, result, "Should fall back to docker.io for invalid tags")
		})
	}
}

func TestExtractRegistryDomain_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "Single character registry",
			tag:      "a/myapp:latest",
			expected: "index.docker.io",
		},
		{
			name:     "Single character image",
			tag:      "registry.example.com/a:latest",
			expected: "docker.io",
		},
		{
			name:     "Very long registry name",
			tag:      "very-long-registry-name-with-many-subdomains.example.com/myapp:latest",
			expected: "very-long-registry-name-with-many-subdomains.example.com",
		},
		{
			name:     "Registry with underscore",
			tag:      "registry_example.com/myapp:latest",
			expected: "registry_example.com",
		},
		{
			name:     "Registry with hyphen",
			tag:      "registry-example.com/myapp:latest",
			expected: "registry-example.com",
		},
		{
			name:     "Registry with numbers",
			tag:      "registry123.example.com/myapp:latest",
			expected: "registry123.example.com",
		},
		{
			name:     "Image with numbers",
			tag:      "registry.example.com/app123:latest",
			expected: "registry.example.com",
		},
		{
			name:     "Image with underscore",
			tag:      "registry.example.com/my_app:latest",
			expected: "registry.example.com",
		},
		{
			name:     "Image with hyphen",
			tag:      "registry.example.com/my-app:latest",
			expected: "registry.example.com",
		},
		{
			name:     "Tag with numbers",
			tag:      "registry.example.com/myapp:123",
			expected: "registry.example.com",
		},
		{
			name:     "Tag with underscore",
			tag:      "registry.example.com/myapp:latest_tag",
			expected: "registry.example.com",
		},
		{
			name:     "Tag with hyphen",
			tag:      "registry.example.com/myapp:latest-tag",
			expected: "registry.example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractRegistryDomain(tc.tag)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractRegistryDomain_RealWorldExamples(t *testing.T) {
	testCases := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "Alpine Linux",
			tag:      "alpine:latest",
			expected: "index.docker.io",
		},
		{
			name:     "Ubuntu",
			tag:      "ubuntu:20.04",
			expected: "index.docker.io",
		},
		{
			name:     "Nginx",
			tag:      "nginx:1.21",
			expected: "index.docker.io",
		},
		{
			name:     "PostgreSQL",
			tag:      "postgres:13",
			expected: "index.docker.io",
		},
		{
			name:     "Redis",
			tag:      "redis:6-alpine",
			expected: "index.docker.io",
		},
		{
			name:     "Node.js",
			tag:      "node:16-alpine",
			expected: "index.docker.io",
		},
		{
			name:     "Python",
			tag:      "python:3.9-slim",
			expected: "index.docker.io",
		},
		{
			name:     "MySQL",
			tag:      "mysql:8.0",
			expected: "index.docker.io",
		},
		{
			name:     "MongoDB",
			tag:      "mongo:5.0",
			expected: "index.docker.io",
		},
		{
			name:     "Elasticsearch",
			tag:      "elasticsearch:7.17.0",
			expected: "index.docker.io",
		},
		{
			name:     "Kubernetes",
			tag:      "k8s.gcr.io/pause:3.2",
			expected: "k8s.gcr.io",
		},
		{
			name:     "Istio",
			tag:      "docker.io/istio/proxyv2:1.12.0",
			expected: "index.docker.io",
		},
		{
			name:     "Prometheus",
			tag:      "prom/prometheus:v2.30.0",
			expected: "index.docker.io",
		},
		{
			name:     "Grafana",
			tag:      "grafana/grafana:8.2.0",
			expected: "index.docker.io",
		},
		{
			name:     "Jenkins",
			tag:      "jenkins/jenkins:lts",
			expected: "index.docker.io",
		},
		{
			name:     "GitLab Runner",
			tag:      "gitlab/gitlab-runner:latest",
			expected: "index.docker.io",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractRegistryDomain(tc.tag)
			assert.Equal(t, tc.expected, result)
		})
	}
}
