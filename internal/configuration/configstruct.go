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
	DryRun       bool
}

func (f ForgetSubcommandOptions) GetCommandToRun() []string {
	return f.CommandToRun
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
