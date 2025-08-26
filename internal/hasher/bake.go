package hasher

import (
	"fmt"
	"slices"

	"github.com/docker/buildx/bake"
	argparse "github.com/hytromo/mimosa/internal/docker/arg_parse"
	fileresolution "github.com/hytromo/mimosa/internal/docker/file_resolution"
	log "github.com/sirupsen/logrus"
)

func constructTemplatedDockerBuildCommand(target *bake.Target) []string {
	args := []string{"docker", "buildx", "build"}

	// Add annotations
	for _, annotation := range target.Annotations {
		args = append(args, "--annotation", annotation)
	}

	// Add attestations
	for _, attest := range target.Attest {
		args = append(args, "--attest", attest.String())
	}

	// Add build contexts
	for name, context := range target.Contexts {
		args = append(args, "--build-context", fmt.Sprintf("%s=%s", name, context))
	}

	// Add build args
	for key, value := range target.Args {
		if value != nil {
			args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, *value))
		}
	}

	// Add cache-from options
	for _, cacheFrom := range target.CacheFrom {
		args = append(args, "--cache-from", cacheFrom.String())
	}

	// Add cache-to options
	for _, cacheTo := range target.CacheTo {
		args = append(args, "--cache-to", cacheTo.String())
	}

	// Add dockerfile
	if target.Dockerfile != nil {
		args = append(args, "--file", *target.Dockerfile)
	}

	// Add labels
	for key, value := range target.Labels {
		if value != nil {
			args = append(args, "--label", fmt.Sprintf("%s=%s", key, *value))
		}
	}

	// Add network mode
	if target.NetworkMode != nil {
		args = append(args, "--network", *target.NetworkMode)
	}

	// Add no-cache
	if target.NoCache != nil && *target.NoCache {
		args = append(args, "--no-cache")
	}

	// Add no-cache-filter
	for _, filter := range target.NoCacheFilter {
		args = append(args, "--no-cache-filter", filter)
	}

	// Add platforms
	for _, platform := range target.Platforms {
		args = append(args, "--platform", platform)
	}

	// Add pull
	if target.Pull != nil && *target.Pull {
		args = append(args, "--pull")
	}

	// Add secrets
	for _, secret := range target.Secrets {
		args = append(args, "--secret", secret.String())
	}

	// Add shm-size
	if target.ShmSize != nil {
		args = append(args, "--shm-size", *target.ShmSize)
	}

	// Add SSH keys
	for _, ssh := range target.SSH {
		args = append(args, "--ssh", ssh.String())
	}

	// Add target
	if target.Target != nil {
		args = append(args, "--target", *target.Target)
	}

	// Add ulimits
	for _, ulimit := range target.Ulimits {
		args = append(args, "--ulimit", ulimit)
	}

	// Add entitlements
	for _, entitlement := range target.Entitlements {
		args = append(args, "--allow", entitlement)
	}

	// Add extra hosts
	for host, ip := range target.ExtraHosts {
		if ip != nil {
			args = append(args, "--add-host", fmt.Sprintf("%s:%s", host, *ip))
		}
	}

	// Add outputs
	for _, output := range target.Outputs {
		args = append(args, "--output", output.String())
	}

	// Add tags as placeholders
	for range target.Tags {
		args = append(args, "--tag", "TAG")
	}

	// Add context path
	if target.Context != nil {
		args = append(args, *target.Context)
	} else {
		// Default context is current directory
		args = append(args, ".")
	}

	return args
}

// TODO: need to include the bake files themselves in this calculation
func HashBakeTargets(targets map[string]*bake.Target) string {
	// each target is basically its own docker build
	// we need to reuse HashBuildCommand for each target and sum the hashes:

	hashes := []string{}
	for _, target := range targets {
		dockerIgnorePath := fileresolution.FindDockerignorePath(*target.Context, *target.Dockerfile)
		allRegistryDomains := []string{}
		for _, tag := range target.Tags {
			allRegistryDomains = append(allRegistryDomains, argparse.ExtractRegistryDomain(tag))
		}

		correspondingDockerBuildCommand := DockerBuildCommand{
			DockerfilePath:        *target.Dockerfile,
			DockerignorePath:      dockerIgnorePath,
			BuildContexts:         target.Contexts,
			AllRegistryDomains:    allRegistryDomains,
			CmdWithTagPlaceholder: constructTemplatedDockerBuildCommand(target),
		}

		log.Debugf("Corresponding docker build command for target %s: %v", target.Name, correspondingDockerBuildCommand)

		hash := HashBuildCommand(correspondingDockerBuildCommand)
		hashes = append(hashes, hash)
	}

	slices.Sort(hashes)

	return HashStrings(hashes)
}
