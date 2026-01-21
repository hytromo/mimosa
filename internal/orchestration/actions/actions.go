package actions

import (
	"github.com/hytromo/mimosa/internal/configuration"
)

type Actions interface {
	// hashing
	ParseCommand(command []string) (configuration.ParsedCommand, error)

	// command execution
	RunCommand(dryRun bool, command []string) int
	ExitProcessWithCode(code int)

	// docker
	RetagFromCacheTags(cacheTagsByTarget map[string]string, newTagsByTarget map[string][]string, dryRun bool) error

	// registry cache
	CheckRegistryCacheExists(hash string, tagsByTarget map[string][]string) (bool, map[string]string, error)
	SaveRegistryCacheTags(hash string, tagsByTarget map[string][]string, dryRun bool) error
}

// Actioner is a concrete implementation of the Actions interface
type Actioner struct {
}

func New() *Actioner {
	return &Actioner{}
}
