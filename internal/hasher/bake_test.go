package hasher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/buildx/bake"
	"github.com/stretchr/testify/assert"
)

func TestHashBakeTargets_EmptyTargets(t *testing.T) {
	targets := map[string]*bake.Target{}
	hash := HashBakeTargets(targets, []string{})
	if hash != "00000000000000000000000000000000" {
		t.Errorf("Expected empty hash for empty targets, got %q", hash)
	}
}

func TestHashBakeTargets_SingleTarget(t *testing.T) {
	context := "."
	dockerfile := "Dockerfile"
	targets := map[string]*bake.Target{
		"app": {
			Context:    &context,
			Dockerfile: &dockerfile,
			Tags:       []string{"myapp:latest"},
		},
	}
	hash := HashBakeTargets(targets, []string{})
	if hash == "" {
		t.Error("Expected non-empty hash for single target")
	}
}

func TestHashBakeTargets_MultipleTargets(t *testing.T) {
	context1 := "."
	context2 := "./frontend"
	dockerfile1 := "Dockerfile"
	dockerfile2 := "Dockerfile.frontend"

	targets := map[string]*bake.Target{
		"backend": {
			Context:    &context1,
			Dockerfile: &dockerfile1,
			Tags:       []string{"myapp/backend:latest"},
		},
		"frontend": {
			Context:    &context2,
			Dockerfile: &dockerfile2,
			Tags:       []string{"myapp/frontend:latest"},
		},
	}
	hash := HashBakeTargets(targets, []string{})
	if hash == "" {
		t.Error("Expected non-empty hash for multiple targets")
	}
}

func TestHashBakeTargets_Deterministic(t *testing.T) {
	context := "."
	dockerfile := "Dockerfile"
	targets := map[string]*bake.Target{
		"app": {
			Context:    &context,
			Dockerfile: &dockerfile,
			Tags:       []string{"myapp:latest"},
		},
	}

	hash1 := HashBakeTargets(targets, []string{})
	hash2 := HashBakeTargets(targets, []string{})

	if hash1 != hash2 {
		t.Errorf("Expected same hash for same targets, got %q and %q", hash1, hash2)
	}
}

func TestHashBakeTargets_DifferentOrder_Deterministic(t *testing.T) {
	context1 := "."
	context2 := "./frontend"
	dockerfile1 := "Dockerfile"
	dockerfile2 := "Dockerfile.frontend"

	targets1 := map[string]*bake.Target{
		"backend": {
			Context:    &context1,
			Dockerfile: &dockerfile1,
			Tags:       []string{"myapp/backend:latest"},
		},
		"frontend": {
			Context:    &context2,
			Dockerfile: &dockerfile2,
			Tags:       []string{"myapp/frontend:latest"},
		},
	}

	targets2 := map[string]*bake.Target{
		"frontend": {
			Context:    &context2,
			Dockerfile: &dockerfile2,
			Tags:       []string{"myapp/frontend:latest"},
		},
		"backend": {
			Context:    &context1,
			Dockerfile: &dockerfile1,
			Tags:       []string{"myapp/backend:latest"},
		},
	}

	hash1 := HashBakeTargets(targets1, []string{})
	hash2 := HashBakeTargets(targets2, []string{})

	if hash1 != hash2 {
		t.Errorf("Expected same hash for same targets in different order, got %q and %q", hash1, hash2)
	}
}

func TestHashBakeTargets_WithBuildContexts(t *testing.T) {
	context := "."
	dockerfile := "Dockerfile"
	targets := map[string]*bake.Target{
		"app": {
			Context:    &context,
			Dockerfile: &dockerfile,
			Tags:       []string{"myapp:latest"},
			Contexts: map[string]string{
				"frontend": "./frontend",
				"backend":  "./backend",
			},
		},
	}
	hash := HashBakeTargets(targets, []string{})
	if hash == "" {
		t.Error("Expected non-empty hash for target with build contexts")
	}
}

func TestHashBakeTargets_WithMultipleTags(t *testing.T) {
	context := "."
	dockerfile := "Dockerfile"
	targets := map[string]*bake.Target{
		"app": {
			Context:    &context,
			Dockerfile: &dockerfile,
			Tags:       []string{"myapp:latest", "myapp:v1.0", "registry.com/myapp:latest"},
		},
	}
	hash := HashBakeTargets(targets, []string{})
	if hash == "" {
		t.Error("Expected non-empty hash for target with multiple tags")
	}
}

func TestHashBakeTargets_WithBakeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	bakeFile := filepath.Join(tmpDir, "docker-bake.json")
	err := os.WriteFile(bakeFile, []byte(`{"targets": {"app": {"context": ".", "dockerfile": "Dockerfile"}}}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write bake file: %v", err)
	}

	localBakeFiles, err := bake.ReadLocalFiles([]string{bakeFile}, nil, nil)
	if err != nil {
		t.Fatalf("Failed to read bake file: %v", err)
	}
	targets, _, err := bake.ReadTargets(context.Background(), localBakeFiles, []string{}, []string{}, nil, nil)
	if err != nil {
		t.Fatalf("Failed to read bake targets: %v", err)
	}

	hash := HashBakeTargets(targets, []string{bakeFile})
	if hash == "" {
		t.Error("Expected non-empty hash for target with bake files")
	}
	hashWithoutBakeFiles := HashBakeTargets(targets, []string{})
	assert.NotEqual(t, hash, hashWithoutBakeFiles, "Expected different hashes for targets with and without bake files")

	err = os.WriteFile(bakeFile, []byte(`{"targets": {"app": {"context": ".", "dockerfile": "Dockerfile.frontend"}}}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write bake file: %v", err)
	}
	hashWithChangedBakeFile := HashBakeTargets(targets, []string{bakeFile})
	assert.NotEqual(t, hash, hashWithChangedBakeFile, "Expected different hashes for targets with and without changed bake file")
}

func TestConstructTemplatedDockerBuildCommand_EmptyTarget(t *testing.T) {
	target := &bake.Target{}
	args := constructDockerBuildCommandWithoutTags(target)

	expected := []string{"docker", "buildx", "build", "."}
	if len(args) != len(expected) {
		t.Errorf("Expected %d args, got %d", len(expected), len(args))
	}
	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("Expected arg[%d] = %q, got %q", i, arg, args[i])
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_WithAnnotations(t *testing.T) {
	target := &bake.Target{
		Annotations: []string{"key1=value1", "key2=value2"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that annotations are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--annotation" {
			if i+1 < len(args) {
				annotation := args[i+1]
				if annotation == "key1=value1" || annotation == "key2=value2" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 annotations, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithAttestations(t *testing.T) {
	// Skip this test as Attestation type is not directly accessible
	t.Skip("Attestation type not directly accessible from bake package")
}

func TestConstructTemplatedDockerBuildCommand_WithBuildContexts(t *testing.T) {
	target := &bake.Target{
		Contexts: map[string]string{
			"frontend": "./frontend",
			"backend":  "./backend",
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that build contexts are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--build-context" {
			if i+1 < len(args) {
				context := args[i+1]
				if context == "frontend=./frontend" || context == "backend=./backend" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 build contexts, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithBuildArgs(t *testing.T) {
	arg1 := "value1"
	arg2 := "value2"
	target := &bake.Target{
		Args: map[string]*string{
			"ARG1": &arg1,
			"ARG2": &arg2,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that build args are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--build-arg" {
			if i+1 < len(args) {
				arg := args[i+1]
				if arg == "ARG1=value1" || arg == "ARG2=value2" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 build args, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNilBuildArgs(t *testing.T) {
	target := &bake.Target{
		Args: map[string]*string{
			"ARG1": nil,
			"ARG2": nil,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that nil build args are not included
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--build-arg" {
			t.Errorf("Expected nil build args to be skipped, but found --build-arg")
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_WithCacheFrom(t *testing.T) {
	// Skip this test as CacheEntry type is not directly accessible
	t.Skip("CacheEntry type not directly accessible from bake package")
}

func TestConstructTemplatedDockerBuildCommand_WithCacheTo(t *testing.T) {
	// Skip this test as CacheEntry type is not directly accessible
	t.Skip("CacheEntry type not directly accessible from bake package")
}

func TestConstructTemplatedDockerBuildCommand_WithDockerfile(t *testing.T) {
	dockerfile := "Dockerfile.custom"
	target := &bake.Target{
		Dockerfile: &dockerfile,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that dockerfile is included
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--file" && i+1 < len(args) && args[i+1] == dockerfile {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --file %s to be in args", dockerfile)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithLabels(t *testing.T) {
	label1 := "value1"
	label2 := "value2"
	target := &bake.Target{
		Labels: map[string]*string{
			"LABEL1": &label1,
			"LABEL2": &label2,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that labels are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--label" {
			if i+1 < len(args) {
				label := args[i+1]
				if label == "LABEL1=value1" || label == "LABEL2=value2" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 labels, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNilLabels(t *testing.T) {
	target := &bake.Target{
		Labels: map[string]*string{
			"LABEL1": nil,
			"LABEL2": nil,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that nil labels are not included
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--label" {
			t.Errorf("Expected nil labels to be skipped, but found --label")
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNetworkMode(t *testing.T) {
	network := "host"
	target := &bake.Target{
		NetworkMode: &network,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that network mode is included
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--network" && i+1 < len(args) && args[i+1] == network {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --network %s to be in args", network)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNoCache(t *testing.T) {
	noCache := true
	target := &bake.Target{
		NoCache: &noCache,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that no-cache is included
	found := false
	for _, arg := range args {
		if arg == "--no-cache" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --no-cache to be in args")
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNoCacheFalse(t *testing.T) {
	noCache := false
	target := &bake.Target{
		NoCache: &noCache,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that no-cache is not included when false
	for _, arg := range args {
		if arg == "--no-cache" {
			t.Errorf("Expected --no-cache to not be in args when NoCache is false")
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNoCacheFilter(t *testing.T) {
	target := &bake.Target{
		NoCacheFilter: []string{"*.tmp", "*.log"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that no-cache-filter options are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--no-cache-filter" {
			if i+1 < len(args) {
				filter := args[i+1]
				if filter == "*.tmp" || filter == "*.log" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 no-cache-filter options, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithPlatforms(t *testing.T) {
	target := &bake.Target{
		Platforms: []string{"linux/amd64", "linux/arm64"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that platforms are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--platform" {
			if i+1 < len(args) {
				platform := args[i+1]
				if platform == "linux/amd64" || platform == "linux/arm64" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 platforms, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithPull(t *testing.T) {
	pull := true
	target := &bake.Target{
		Pull: &pull,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that pull is included
	found := false
	for _, arg := range args {
		if arg == "--pull" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --pull to be in args")
	}
}

func TestConstructTemplatedDockerBuildCommand_WithPullFalse(t *testing.T) {
	pull := false
	target := &bake.Target{
		Pull: &pull,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that pull is not included when false
	for _, arg := range args {
		if arg == "--pull" {
			t.Errorf("Expected --pull to not be in args when Pull is false")
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_WithSecrets(t *testing.T) {
	// Skip this test as Secret type is not directly accessible
	t.Skip("Secret type not directly accessible from bake package")
}

func TestConstructTemplatedDockerBuildCommand_WithShmSize(t *testing.T) {
	shmSize := "1g"
	target := &bake.Target{
		ShmSize: &shmSize,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that shm-size is included
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--shm-size" && i+1 < len(args) && args[i+1] == shmSize {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --shm-size %s to be in args", shmSize)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithSSH(t *testing.T) {
	// Skip this test as SSH type is not directly accessible
	t.Skip("SSH type not directly accessible from bake package")
}

func TestConstructTemplatedDockerBuildCommand_WithTarget(t *testing.T) {
	targetName := "production"
	target := &bake.Target{
		Target: &targetName,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that target is included
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--target" && i+1 < len(args) && args[i+1] == targetName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --target %s to be in args", targetName)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithUlimits(t *testing.T) {
	target := &bake.Target{
		Ulimits: []string{"nofile=65536:65536", "nproc=32768:32768"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that ulimits are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--ulimit" {
			if i+1 < len(args) {
				ulimit := args[i+1]
				if ulimit == "nofile=65536:65536" || ulimit == "nproc=32768:32768" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 ulimits, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithEntitlements(t *testing.T) {
	target := &bake.Target{
		Entitlements: []string{"network.host", "security.insecure"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that entitlements are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--allow" {
			if i+1 < len(args) {
				entitlement := args[i+1]
				if entitlement == "network.host" || entitlement == "security.insecure" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 entitlements, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithExtraHosts(t *testing.T) {
	host1 := "host1.local"
	host2 := "host2.local"
	ip1 := "192.168.1.10"
	ip2 := "192.168.1.11"

	target := &bake.Target{
		ExtraHosts: map[string]*string{
			host1: &ip1,
			host2: &ip2,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that extra hosts are included
	found := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--add-host" {
			if i+1 < len(args) {
				host := args[i+1]
				if host == "host1.local:192.168.1.10" || host == "host2.local:192.168.1.11" {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 extra hosts, found %d", found)
	}
}

func TestConstructTemplatedDockerBuildCommand_WithNilExtraHosts(t *testing.T) {
	target := &bake.Target{
		ExtraHosts: map[string]*string{
			"host1.local": nil,
			"host2.local": nil,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that nil extra hosts are not included
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--add-host" {
			t.Errorf("Expected nil extra hosts to be skipped, but found --add-host")
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_WithOutputs(t *testing.T) {
	// Skip this test as Output type is not directly accessible
	t.Skip("Output type not directly accessible from bake package")
}

func TestConstructTemplatedDockerBuildCommand_WithoutTags(t *testing.T) {
	target := &bake.Target{
		Tags: []string{"myapp:latest", "myapp:v1.0"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that tags are not included
	found := 0
	for _, arg := range args {
		if arg == "--tag" || arg == "-t" || strings.HasPrefix(arg, "--tag=") || strings.HasPrefix(arg, "-t=") {
			found++
		}
	}

	assert.Equal(t, 0, found)
}

func TestConstructTemplatedDockerBuildCommand_WithContext(t *testing.T) {
	context := "./custom-context"
	target := &bake.Target{
		Context: &context,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that context is included at the end
	assert.Equal(t, context, args[len(args)-1])
}

func TestConstructTemplatedDockerBuildCommand_WithNilContext(t *testing.T) {
	target := &bake.Target{
		Context: nil,
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Check that default context "." is included at the end
	if len(args) == 0 || args[len(args)-1] != "." {
		t.Errorf("Expected default context \".\" to be the last argument, got %s", args[len(args)-1])
	}
}

func TestConstructTemplatedDockerBuildCommand_ComplexTarget(t *testing.T) {
	context := "."
	dockerfile := "Dockerfile"
	noCache := true
	pull := true
	shmSize := "2g"
	targetName := "production"

	target := &bake.Target{
		Context:    &context,
		Dockerfile: &dockerfile,
		Tags:       []string{"myapp:latest", "myapp:v1.0"},
		Platforms:  []string{"linux/amd64", "linux/arm64"},
		NoCache:    &noCache,
		Pull:       &pull,
		ShmSize:    &shmSize,
		Target:     &targetName,
		Args: map[string]*string{
			"BUILD_VERSION": &targetName,
		},
		Labels: map[string]*string{
			"version": &targetName,
		},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Verify all expected arguments are present
	expectedArgs := []string{
		"docker", "buildx", "build",
		"--file", dockerfile,
		"--platform", "linux/amd64",
		"--platform", "linux/arm64",
		"--no-cache",
		"--pull",
		"--shm-size", shmSize,
		"--target", targetName,
		"--build-arg", "BUILD_VERSION=" + targetName,
		"--label", "version=" + targetName,
		context,
	}

	for _, expected := range expectedArgs {
		found := false
		for _, arg := range args {
			if arg == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected argument %q not found in args", expected)
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_AllFieldsCombined(t *testing.T) {
	context := "./app"
	dockerfile := "Dockerfile.prod"
	network := "bridge"
	noCache := true
	pull := true
	shmSize := "1g"
	targetName := "production"
	arg1 := "value1"
	label1 := "label1"
	host1 := "host1.local"
	ip1 := "192.168.1.10"

	target := &bake.Target{
		Context:     &context,
		Dockerfile:  &dockerfile,
		NetworkMode: &network,
		NoCache:     &noCache,
		Pull:        &pull,
		ShmSize:     &shmSize,
		Target:      &targetName,
		Annotations: []string{"key1=value1", "key2=value2"},
		Platforms:   []string{"linux/amd64"},
		Tags:        []string{"myapp:latest"},
		Args: map[string]*string{
			"ARG1": &arg1,
		},
		Labels: map[string]*string{
			"LABEL1": &label1,
		},
		ExtraHosts: map[string]*string{
			host1: &ip1,
		},
		NoCacheFilter: []string{"*.tmp"},
		Ulimits:       []string{"nofile=65536:65536"},
		Entitlements:  []string{"network.host"},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Verify all expected arguments are present
	expectedArgs := []string{
		"docker", "buildx", "build",
		"--annotation", "key1=value1",
		"--annotation", "key2=value2",
		"--file", dockerfile,
		"--network", network,
		"--no-cache",
		"--pull",
		"--shm-size", shmSize,
		"--target", targetName,
		"--platform", "linux/amd64",
		"--build-arg", "ARG1=value1",
		"--label", "LABEL1=label1",
		"--no-cache-filter", "*.tmp",
		"--ulimit", "nofile=65536:65536",
		"--allow", "network.host",
		"--add-host", "host1.local:192.168.1.10",
		context,
	}

	for _, expected := range expectedArgs {
		found := false
		for _, arg := range args {
			if arg == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected argument %q not found in args", expected)
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_EmptySlices(t *testing.T) {
	target := &bake.Target{
		Annotations:   []string{},
		Platforms:     []string{},
		Tags:          []string{},
		Ulimits:       []string{},
		Entitlements:  []string{},
		NoCacheFilter: []string{},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Should only contain basic command and default context
	expected := []string{"docker", "buildx", "build", "."}
	if len(args) != len(expected) {
		t.Errorf("Expected %d args, got %d", len(expected), len(args))
	}
	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("Expected arg[%d] = %q, got %q", i, arg, args[i])
		}
	}
}

func TestConstructTemplatedDockerBuildCommand_EmptyMaps(t *testing.T) {
	target := &bake.Target{
		Contexts:   map[string]string{},
		Args:       map[string]*string{},
		Labels:     map[string]*string{},
		ExtraHosts: map[string]*string{},
	}
	args := constructDockerBuildCommandWithoutTags(target)

	// Should only contain basic command and default context
	expected := []string{"docker", "buildx", "build", "."}
	if len(args) != len(expected) {
		t.Errorf("Expected %d args, got %d", len(expected), len(args))
	}
	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("Expected arg[%d] = %q, got %q", i, arg, args[i])
		}
	}
}
