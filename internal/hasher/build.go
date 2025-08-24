package hasher

// DockerBuildCommand is a struct that contains the information needed to hash a docker build command
type DockerBuildCommand struct {
	DockerfilePath        string
	DockerignorePath      string
	BuildContexts         map[string]string
	AllRegistryDomains    map[string]string
	CmdWithTagPlaceholder []string
}

func HashBuildCommand(command DockerBuildCommand) string {
	// logic:
	// registryDomainsHash = hash(sortByNameAndUnique(registryDomains))
	// cpu - 1 workers:
	//   for each build context:
	//      buildContextHashN = hash(context files)
	// buildContextsHash = hash(buildContextHash1, buildContextHash2, ..., buildContextHashN)

	// hash(dockerfile, dockerignore, cmdwithtagplaceholder, registryDomainsHash, buildContextsHash)

	return ""
}
