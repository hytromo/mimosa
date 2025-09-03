package configuration

type CommandContainer interface {
	GetCommandToRun() []string
}

type RememberSubcommandOptions struct {
	Enabled      bool
	CommandToRun []string
	DryRun       bool
}

func (r RememberSubcommandOptions) GetCommandToRun() []string {
	return r.CommandToRun
}

// ForgetSubcommandOptions defines the options of the forget subcommand
type ForgetSubcommandOptions struct {
	Enabled      bool
	CommandToRun []string
	Period       string
	AutoYes      bool
	Everything   bool
	DryRun       bool
}

func (f ForgetSubcommandOptions) GetCommandToRun() []string {
	return f.CommandToRun
}

// CacheSubcommandOptions defines the options of the cache subcommand
type CacheSubcommandOptions struct {
	Enabled    bool
	Show       bool
	ToEnvValue bool
}

// AppOptions defines the options of the application
type AppOptions struct {
	Remember RememberSubcommandOptions
	Forget   ForgetSubcommandOptions
	Cache    CacheSubcommandOptions
}

// ParsedCommand is the parsed command from the user input
type ParsedCommand struct {
	// map of target to tags, default target is "default"
	// this is because the "bake" command can support multiple targets
	TagsByTarget map[string][]string
	// the final hash of the command - includes all the needed information to calculate a unique hash (e.g. command, contexts etc)
	Hash string
	// the raw command - we will fallback to actually running this if there is an error during remember mode
	Command []string
}
