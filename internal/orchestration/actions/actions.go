package actions

import (
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
)

type Actions interface {
	// hashing
	ParseCommand(command []string) (configuration.ParsedCommand, error)

	// command execution
	RunCommand(dryRun bool, command []string) int
	ExitProcessWithCode(code int)

	// caching
	GetCacheEntry(hash string) cacher.Cache
	RemoveCacheEntry(cacheEntry cacher.Cache, dryRun bool) error
	SaveCache(cacheEntry cacher.Cache, tagsByTarget map[string][]string, dryRun bool) error
	ForgetCacheEntriesOlderThan(duration string, autoApprove bool) error
	PrintCacheDir()
	PrintCacheToEnvValue()

	// docker
	Retag(cacheEntry cacher.Cache, parsedCommand configuration.ParsedCommand, dryRun bool) error
}

// Actioner is a concrete implementation of the Actions interface
type Actioner struct {
}

func New() *Actioner {
	return &Actioner{}
}
