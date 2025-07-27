package configuration

// RememberSubcommandOptions defines the options of the remember subcommand
type RememberSubcommandOptions struct {
	Enabled      bool
	CommandToRun []string
	DryRun       bool
}

// ForgetSubcommandOptions defines the options of the forget subcommand
type ForgetSubcommandOptions struct {
	Enabled      bool
	CommandToRun []string
	DryRun       bool
}

// CacheSubcommandOptions defines the options of the cache subcommand
type CacheSubcommandOptions struct {
	Enabled    bool
	Forget     string
	ForgetYes  bool
	Show       bool
	Purge      bool
	ToEnvValue bool
}

// AppOptions defines the options of the application
type AppOptions struct {
	Remember RememberSubcommandOptions
	Forget   ForgetSubcommandOptions
	Cache    CacheSubcommandOptions
}
