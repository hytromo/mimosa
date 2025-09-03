package hasher

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/docker/buildx/bake"
	"github.com/hytromo/mimosa/internal/configuration"
	argparse "github.com/hytromo/mimosa/internal/docker/arg_parse"
	fileresolution "github.com/hytromo/mimosa/internal/docker/file_resolution"
	log "github.com/sirupsen/logrus"
)

func constructDockerBuildCommandWithoutTags(target *bake.Target) []string {
	args := []string{"docker", "buildx", "build"}

	// Add annotations
	if target.Annotations != nil {
		for _, annotation := range target.Annotations {
			args = append(args, "--annotation", annotation)
		}
	}

	// Add attestations
	if target.Attest != nil {
		for _, attest := range target.Attest {
			args = append(args, "--attest", attest.String())
		}
	}

	// Add build contexts
	if target.Contexts != nil {
		for name, context := range target.Contexts {
			args = append(args, "--build-context", fmt.Sprintf("%s=%s", name, context))
		}
	}

	// Add build args
	if target.Args != nil {
		for key, value := range target.Args {
			if value != nil {
				args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, *value))
			}
		}
	}

	// Add cache-from options
	if target.CacheFrom != nil {
		for _, cacheFrom := range target.CacheFrom {
			args = append(args, "--cache-from", cacheFrom.String())
		}
	}

	// Add cache-to options
	if target.CacheTo != nil {
		for _, cacheTo := range target.CacheTo {
			args = append(args, "--cache-to", cacheTo.String())
		}
	}

	// Add dockerfile
	if target.Dockerfile != nil {
		args = append(args, "--file", *target.Dockerfile)
	}

	// Add labels
	if target.Labels != nil {
		for key, value := range target.Labels {
			if value != nil {
				args = append(args, "--label", fmt.Sprintf("%s=%s", key, *value))
			}
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
	if target.NoCacheFilter != nil {
		for _, filter := range target.NoCacheFilter {
			args = append(args, "--no-cache-filter", filter)
		}
	}

	// Add platforms
	if target.Platforms != nil {
		for _, platform := range target.Platforms {
			args = append(args, "--platform", platform)
		}
	}

	// Add pull
	if target.Pull != nil && *target.Pull {
		args = append(args, "--pull")
	}

	// Add secrets
	if target.Secrets != nil {
		for _, secret := range target.Secrets {
			args = append(args, "--secret", secret.String())
		}
	}

	// Add shm-size
	if target.ShmSize != nil {
		args = append(args, "--shm-size", *target.ShmSize)
	}

	// Add SSH keys
	if target.SSH != nil {
		for _, ssh := range target.SSH {
			args = append(args, "--ssh", ssh.String())
		}
	}

	// Add target
	if target.Target != nil {
		args = append(args, "--target", *target.Target)
	}

	// Add ulimits
	if target.Ulimits != nil {
		for _, ulimit := range target.Ulimits {
			args = append(args, "--ulimit", ulimit)
		}
	}

	// Add entitlements
	if target.Entitlements != nil {
		for _, entitlement := range target.Entitlements {
			args = append(args, "--allow", entitlement)
		}
	}

	// Add extra hosts
	if target.ExtraHosts != nil {
		for host, ip := range target.ExtraHosts {
			if ip != nil {
				args = append(args, "--add-host", fmt.Sprintf("%s:%s", host, *ip))
			}
		}
	}

	// Add outputs
	if target.Outputs != nil {
		for _, output := range target.Outputs {
			args = append(args, "--output", output.String())
		}
	}

	// Add context path
	if target.Context != nil {
		args = append(args, *target.Context)
	} else {
		// Default context is current directory
		args = append(args, ".")
	}

	// tags are skipped on purpose - we do not take them into account when hashing the command
	return args
}

func HashBakeTargets(targets map[string]*bake.Target, bakeFiles []string) string {
	// each target is basically its own docker build - so we reuse HashBuildCommand for each target and sum the hashes:

	hashes := []string{}
	for targetName, target := range targets {
		if target.Context == nil || target.Dockerfile == nil {
			continue
		}

		dockerIgnorePath := fileresolution.ResolveAbsoluteDockerIgnorePath(*target.Context, *target.Dockerfile)
		allRegistryDomains := []string{}
		for _, tag := range target.Tags {
			allRegistryDomains = append(allRegistryDomains, argparse.ExtractRegistryDomain(tag))
		}

		// copy target.Contexts to allContexts as shortly as possible:
		allContexts := make(map[string]string)
		for k, v := range target.Contexts {
			allContexts[k] = v
		}
		allContexts[configuration.MainBuildContextName] = *target.Context

		// if dockerfile already not absolute, then it is relative to the context
		absoluteDockerfilePath := *target.Dockerfile
		var err error
		if !filepath.IsAbs(absoluteDockerfilePath) {
			absoluteDockerfilePath, err = filepath.Abs(filepath.Join(*target.Context, *target.Dockerfile))
			if err != nil {
				log.Errorf("Error getting absolute path for dockerfile: %v", err)
			}
		}

		correspondingDockerBuildCommand := DockerBuildCommand{
			DockerfilePath:         absoluteDockerfilePath,
			DockerignorePath:       dockerIgnorePath,
			BuildContexts:          allContexts,
			AllRegistryDomains:     allRegistryDomains,
			CmdWithoutTagArguments: constructDockerBuildCommandWithoutTags(target),
		}

		log.Debugf("Corresponding docker build command for target %s: %#v\n", targetName, correspondingDockerBuildCommand)

		hash := HashBuildCommand(correspondingDockerBuildCommand)
		hashes = append(hashes, hash)
	}

	hashes = append(hashes, HashFiles(bakeFiles, 1))

	slices.Sort(hashes)

	return HashStrings(hashes)
}
