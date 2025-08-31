package testutils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// GenerateTestID generates a unique test identifier to avoid conflicts between tests
func GenerateTestID() string {
	// Generate 8 random bytes and encode as hex
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(fmt.Sprintf("failed to generate test ID: %v", err))
	}
	return fmt.Sprintf("%x", bytes)
}

// TestRegistry represents a Docker registry for testing
type TestRegistry struct {
	Port int
	Name string
	Url  string
}

// SharedRegistryManager manages a single shared registry instance for all Docker tests
type SharedRegistryManager struct {
	registry *TestRegistry
	mu       sync.Mutex
	initOnce sync.Once
}

var (
	sharedManager = &SharedRegistryManager{}
)

// GetSharedRegistry returns the shared registry instance, creating it if necessary
func GetSharedRegistry() (*TestRegistry, error) {
	return sharedManager.GetRegistry()
}

// GetRegistry returns the shared registry instance, creating it if necessary
func (srm *SharedRegistryManager) GetRegistry() (*TestRegistry, error) {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	var initErr error
	srm.initOnce.Do(func() {
		registry, err := startRegistry()
		if err != nil {
			initErr = fmt.Errorf("failed to start shared registry: %w", err)
			return
		}
		srm.registry = registry
		fmt.Printf("Shared test registry started on port %d with name %s\n", registry.Port, registry.Name)
	})

	if initErr != nil {
		return nil, initErr
	}

	return srm.registry, nil
}

// CleanupSharedRegistry cleans up the shared registry
func CleanupSharedRegistry() {
	sharedManager.Cleanup()
}

// Cleanup cleans up the shared registry
func (srm *SharedRegistryManager) Cleanup() {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	if srm.registry != nil {
		srm.registry.Cleanup(nil)
		srm.registry = nil
	}
}

// SetupTestRegistry is a helper function for tests that need a registry
func SetupTestRegistry(t *testing.T) *TestRegistry {
	registry, err := GetSharedRegistry()
	if err != nil {
		t.Fatalf("Failed to get shared registry: %v", err)
	}
	return registry
}

// startRegistry starts a single Docker registry
func startRegistry() (*TestRegistry, error) {
	// Generate a random port between 5000-65535
	portRange := big.NewInt(60535) // 65535 - 5000
	randomPort, err := rand.Int(rand.Reader, portRange)
	if err != nil {
		return nil, err
	}
	port := int(randomPort.Int64()) + 5000

	// Generate a unique container name
	name := fmt.Sprintf("mimosa_registry_%d", port)
	url := fmt.Sprintf("localhost:%d", port)

	// Start the registry
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"-p", fmt.Sprintf("%d:5000", port),
		"--name", name,
		"registry:3")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start registry: %s", string(output))
	}

	// Wait for registry to be ready
	timeoutSeconds := 30
	timeout := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for time.Now().Before(timeout) {
		resp, err := http.Get(fmt.Sprintf("http://%s/v2/", url))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return &TestRegistry{
					Port: port,
					Name: name,
					Url:  url,
				}, nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("registry failed to start within %d seconds", timeoutSeconds)
}

// Cleanup stops and removes the test registry container
func (tr *TestRegistry) Cleanup(t *testing.T) {
	if tr.Name == "" {
		return
	}

	killCmd := exec.Command("docker", "kill", "-s", "9", tr.Name)
	killOutput, killErr := killCmd.CombinedOutput()
	if killErr != nil {
		if t != nil {
			t.Logf("Failed to stop/kill registry container: %s, %s", string(killOutput), string(killOutput))
		} else {
			fmt.Printf("Failed to stop/kill registry container: %s, %s\n", string(killOutput), string(killOutput))
		}
	}

	if t != nil {
		t.Logf("Shared test registry cleaned up: %s", tr.Name)
	} else {
		fmt.Printf("Shared test registry cleaned up: %s\n", tr.Name)
	}
}
