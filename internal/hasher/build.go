package hasher

import (
	"math"
	"runtime"
	"slices"
)

// DockerBuildCommand is a struct that contains the information needed to hash a docker build command
type DockerBuildCommand struct {
	DockerfilePath        string
	DockerignorePath      string
	BuildContexts         map[string]string
	AllRegistryDomains    []string
	CmdWithTagPlaceholder []string
}

func registryDomainsHash(registryDomains []string) string {
	slices.Sort(registryDomains)
	return HashStrings(registryDomains)
}

func HashBuildCommand(command DockerBuildCommand) string {
	// logic:
	// registryDomainsHash = hash(sortByNameAndUnique(registryDomains))
	// cpu - 1 workers:
	//   for each build context:
	//      buildContextHashN = hash(context files)
	// buildContextsHash = hash(buildContextHash1, buildContextHash2, ..., buildContextHashN)

	// hash(dockerfile, dockerignore, cmdwithtagplaceholder, registryDomainsHash, buildContextsHash)

	registryDomainsHash := registryDomainsHash(command.AllRegistryDomains)

	// up to num of CPUs-1
	nWorkers := math.Max(float64(runtime.NumCPU()-1), 1)
	allLocalContexts := []string{}

	return registryDomainsHash
}
