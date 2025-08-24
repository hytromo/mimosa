package actions

import (
	"github.com/hytromo/mimosa/internal/cacher"
)

type ParsedCommand struct {
	// map of target to tags, default target is "default"
	// this is because the "bake" command can support multiple targets
	TagsByTarget map[string][]string
	// the final hash of the command - includes all the needed information to calculate a unique hash (e.g. command, contexts etc)
	Hash string
	// the raw command - we will fallback to actually running this if there is an error during remember mode
	Command []string
}

type Actions interface {
	// hashing
	ParseCommand(command []string) (ParsedCommand, error)

	// command execution
	RunCommand(command []string) int
	ExitProcessWithCode(code int)

	// caching
	GetCacheEntry(hash string) cacher.Cache
	RemoveCacheEntry(cacheEntry cacher.Cache, dryRun bool) error
	SaveCache(cacheEntry cacher.Cache, tagsByTarget map[string][]string, dryRun bool) error
	ForgetCacheEntriesOlderThan(duration string, autoApprove bool) error
	PrintCacheDir()
	PrintCacheToEnvValue()

	// docker
	Retag(cacheEntry cacher.Cache, parsedCommand ParsedCommand, dryRun bool) error
}

// Actioner is a concrete implementation of the Actions interface
type Actioner struct {
}

func NewActioner() *Actioner {
	return &Actioner{}
}
