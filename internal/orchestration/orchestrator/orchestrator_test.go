package orchestrator

import (
	"errors"
	"log"
	"testing"
	"time"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/hytromo/mimosa/internal/cacher"
	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/hasher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	TestHash = "40e764c8623a830fe8cc77c52b4902c7"
)

// MockActions is a mock implementation of the Actions interface
type MockActions struct {
	mock.Mock
}

func (m *MockActions) ParseCommand(command []string) (configuration.ParsedCommand, error) {
	args := m.Called(command)
	return args.Get(0).(configuration.ParsedCommand), args.Error(1)
}

func (m *MockActions) RunCommand(dryRun bool, command []string) int {
	args := m.Called(dryRun, command)
	return args.Int(0)
}

func (m *MockActions) ExitProcessWithCode(code int) {
	m.Called(code)
}

func (m *MockActions) GetCacheEntry(hash string) cacher.Cache {
	args := m.Called(hash)
	return args.Get(0).(cacher.Cache)
}

func (m *MockActions) RemoveCacheEntry(cacheEntry cacher.Cache, dryRun bool) error {
	args := m.Called(cacheEntry, dryRun)
	return args.Error(0)
}

func (m *MockActions) SaveCache(cacheEntry cacher.Cache, tagsByTarget map[string][]string, dryRun bool) error {
	args := m.Called(cacheEntry, tagsByTarget, dryRun)
	return args.Error(0)
}

func (m *MockActions) ForgetCacheEntriesOlderThan(duration string, autoApprove bool) error {
	args := m.Called(duration, autoApprove)
	return args.Error(0)
}

func (m *MockActions) PrintCacheDir() {
	m.Called()
}

func (m *MockActions) PrintCacheToEnvValue(cacheDir string) {
	m.Called(cacheDir)
}

func (m *MockActions) Retag(cacheEntry cacher.Cache, parsedCommand configuration.ParsedCommand, dryRun bool) error {
	args := m.Called(cacheEntry, parsedCommand, dryRun)
	return args.Error(0)
}

// createTestCache creates a cache instance for testing
func createTestCache(hexHash string, shouldExist bool) cacher.Cache {
	inMemoryEntries := orderedmap.NewOrderedMap[string, cacher.CacheFile]()

	z85Hash, err := hasher.HexToZ85(hexHash)
	if err != nil {
		log.Fatalf("Failed to convert hex hash to z85: %v", err)
	}

	if shouldExist {
		inMemoryEntries.Set(z85Hash, cacher.CacheFile{
			TagsByTarget:  map[string][]string{"default": {"latest"}},
			LastUpdatedAt: time.Now(),
		})
	}

	return cacher.Cache{
		Hash:            hexHash,
		InMemoryEntries: inMemoryEntries,
	}
}

func TestRun_NoSubcommandsEnabled(t *testing.T) {
	appOptions := configuration.AppOptions{}
	mockActions := &MockActions{}

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_CacheExists_RetagSucceeds_SaveCacheSucceeds_Duplicate(t *testing.T) {
	// This test now verifies that when cache exists in memory, retag is called
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("Retag", cache, parsedCommand, false).Return(nil)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}
func TestRun_RememberEnabled_NoCacheExists_CommandFails(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, false)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", false, []string{"docker", "build", "."}).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error running command - exit code: 1")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_ParseCommandError_FallbackFails(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"invalid", "command"},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ParseCommand", []string{"invalid", "command"}).Return(configuration.ParsedCommand{
		Command: []string{"invalid", "command"},
	}, errors.New("parse error"))
	mockActions.On("RunCommand", false, []string{"invalid", "command"}).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_NoCacheExists_CommandSucceeds(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, false)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", false, []string{"docker", "build", "."}).Return(0)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_CacheExists_RetagSucceeds_SaveCacheFails(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	// Since cache exists in memory, it will go to the "retag" branch
	mockActions.On("Retag", cache, parsedCommand, false).Return(nil)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, false).Return(errors.New("save error"))

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_CacheExists_RetagFails(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	// Since cache exists in memory, it will go to the "retag" branch
	mockActions.On("Retag", cache, parsedCommand, false).Return(errors.New("retag error"))
	mockActions.On("RunCommand", false, []string{"docker", "build", "."}).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retag error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_NoCacheExists_CommandSucceeds_SaveCacheFails(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, false)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", false, []string{"docker", "build", "."}).Return(0)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, false).Return(errors.New("save cache error"))

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save cache error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_ParseCommandError_FallbackSucceeds(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"invalid", "command"},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ParseCommand", []string{"invalid", "command"}).Return(configuration.ParsedCommand{
		Command: []string{"invalid", "command"},
	}, errors.New("parse error"))
	mockActions.On("RunCommand", false, []string{"invalid", "command"}).Return(0)
	mockActions.On("ExitProcessWithCode", 0).Return()

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_WithDifferentTags(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "-t", "myapp:v1", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, false)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myapp:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myapp:v1"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myapp:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", false, []string{"docker", "build", "-t", "myapp:v1", "."}).Return(0)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"myapp:v1"}}, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_WithMultipleTargets(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "buildx", "build", "--target", "frontend", "--target", "backend", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:    TestHash,
		Command: []string{"docker", "buildx", "build", "--target", "frontend", "--target", "backend", "."},
		TagsByTarget: map[string][]string{
			"frontend": {"frontend:latest"},
			"backend":  {"backend:latest"},
		},
	}

	mockActions.On("ParseCommand", []string{"docker", "buildx", "build", "--target", "frontend", "--target", "backend", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("Retag", cache, parsedCommand, false).Return(nil)
	mockActions.On("SaveCache", cache, map[string][]string{
		"frontend": {"frontend:latest"},
		"backend":  {"backend:latest"},
	}, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_ForgetEnabled(t *testing.T) {
	appOptions := configuration.AppOptions{
		Forget: configuration.ForgetSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RemoveCacheEntry", cache, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_ForgetEnabled_RemoveError(t *testing.T) {
	appOptions := configuration.AppOptions{
		Forget: configuration.ForgetSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RemoveCacheEntry", cache, false).Return(errors.New("remove error"))

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remove error")
	mockActions.AssertExpectations(t)
}

func TestRun_ForgetEnabled_EmptyCommand(t *testing.T) {
	appOptions := configuration.AppOptions{
		Forget: configuration.ForgetSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ParseCommand", []string{}).Return(configuration.ParsedCommand{
		Command: []string{},
	}, errors.New("empty command error"))

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty command error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_EmptyCommand(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ParseCommand", []string{}).Return(configuration.ParsedCommand{
		Command: []string{},
	}, errors.New("empty command error"))
	mockActions.On("RunCommand", false, []string{}).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty command error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_WithNilCache(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	// Create an empty cache (no in-memory entries)
	cache := cacher.Cache{
		Hash:            TestHash,
		InMemoryEntries: orderedmap.NewOrderedMap[string, cacher.CacheFile](),
	}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", false, []string{"docker", "build", "."}).Return(0)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_WithLongCommand(t *testing.T) {
	longCommand := []string{"docker", "build", "--build-arg", "VERY_LONG_ARGUMENT=" + string(make([]byte, 1000)), "."}

	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: longCommand,
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, false)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      longCommand,
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", longCommand).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", false, longCommand).Return(0)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_CacheEnabled_Forget(t *testing.T) {
	appOptions := configuration.AppOptions{
		Cache: configuration.CacheSubcommandOptions{
			Enabled:   true,
			Forget:    "24h",
			ForgetYes: true,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ForgetCacheEntriesOlderThan", "24h", true).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_CacheEnabled_ForgetError(t *testing.T) {
	appOptions := configuration.AppOptions{
		Cache: configuration.CacheSubcommandOptions{
			Enabled:   true,
			Forget:    "24h",
			ForgetYes: true,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ForgetCacheEntriesOlderThan", "24h", true).Return(errors.New("forget error"))

	err := Run(appOptions, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forget error")
	mockActions.AssertExpectations(t)
}

func TestRun_CacheEnabled_Purge(t *testing.T) {
	appOptions := configuration.AppOptions{
		Cache: configuration.CacheSubcommandOptions{
			Enabled: true,
			Purge:   true,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("ForgetCacheEntriesOlderThan", "", false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_CacheEnabled_Show(t *testing.T) {
	appOptions := configuration.AppOptions{
		Cache: configuration.CacheSubcommandOptions{
			Enabled: true,
			Show:    true,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("PrintCacheDir").Return()

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_CacheEnabled_ToEnvValue(t *testing.T) {
	appOptions := configuration.AppOptions{
		Cache: configuration.CacheSubcommandOptions{
			Enabled:    true,
			ToEnvValue: true,
		},
	}

	mockActions := &MockActions{}

	mockActions.On("PrintCacheToEnvValue").Return()

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_CacheEnabled_NoSpecificAction(t *testing.T) {
	appOptions := configuration.AppOptions{
		Cache: configuration.CacheSubcommandOptions{
			Enabled: true,
		},
	}

	mockActions := &MockActions{}

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberAndForgetBothEnabled_PrioritizesForget(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
		Forget: configuration.ForgetSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       false,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	// Since Forget is enabled, it takes priority over Remember, so we expect Forget behavior
	mockActions.On("RemoveCacheEntry", cache, false).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_DryRunMode(t *testing.T) {
	appOptions := configuration.AppOptions{
		Remember: configuration.RememberSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       true,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, false)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RunCommand", true, []string{"docker", "build", "."}).Return(0)
	mockActions.On("SaveCache", cache, map[string][]string{"default": {"latest"}}, true).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_ForgetDryRunMode(t *testing.T) {
	appOptions := configuration.AppOptions{
		Forget: configuration.ForgetSubcommandOptions{
			Enabled:      true,
			CommandToRun: []string{"docker", "build", "."},
			DryRun:       true,
		},
	}

	mockActions := &MockActions{}
	cache := createTestCache(TestHash, true)

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "."},
		TagsByTarget: map[string][]string{"default": {"latest"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "."}).Return(parsedCommand, nil)
	mockActions.On("GetCacheEntry", TestHash).Return(cache)
	mockActions.On("RemoveCacheEntry", cache, true).Return(nil)

	err := Run(appOptions, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}
