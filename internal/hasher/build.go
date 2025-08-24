package hasher

// DockerBuildCommand is a struct that contains the information needed to hash a docker build command
type DockerBuildCommand struct {
	DockerfilePath          string
	DockerignorePath        string
	AdditionalBuildContexts map[string]string
	AllRegistryDomains      map[string]string
	CmdWithTagPlaceholder   []string
}

func HashBuildCommand(command DockerBuildCommand) string {
	return ""
}
