package orchestrator

import (
	"errors"
	"testing"

	"github.com/hytromo/mimosa/internal/configuration"
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

func (m *MockActions) RetagFromCacheTags(cacheTagsByTarget map[string]string, newTagsByTarget map[string][]string, dryRun bool) error {
	args := m.Called(cacheTagsByTarget, newTagsByTarget, dryRun)
	return args.Error(0)
}

func (m *MockActions) CheckRegistryCacheExists(hash string, tagsByTarget map[string][]string) (bool, map[string]string, error) {
	args := m.Called(hash, tagsByTarget)
	var cacheTags map[string]string
	if args.Get(1) != nil {
		cacheTags = args.Get(1).(map[string]string)
	}
	return args.Bool(0), cacheTags, args.Error(2)
}

func (m *MockActions) SaveRegistryCacheTags(hash string, tagsByTarget map[string][]string, dryRun bool) error {
	args := m.Called(hash, tagsByTarget, dryRun)
	return args.Error(0)
}

func TestRun_NoSubcommandsEnabled(t *testing.T) {
	mockActions := &MockActions{}

	err := HandleRememberOrForgetSubcommands(configuration.RememberSubcommandOptions{}, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.Error(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_CacheExists(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	cacheTagsByTarget := map[string]string{
		"default": "myreg1/myimage:mimosa-content-hash-" + TestHash,
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(true, cacheTagsByTarget, nil)
	mockActions.On("RetagFromCacheTags", cacheTagsByTarget, parsedCommand.TagsByTarget, false).Return(nil)

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_CacheMiss(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(false, nil, nil)
	mockActions.On("RunCommand", false, parsedCommand.Command).Return(0)
	mockActions.On("SaveRegistryCacheTags", TestHash, parsedCommand.TagsByTarget, false).Return(nil)

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_CheckError_Fallback(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(false, nil, errors.New("check error"))
	mockActions.On("RunCommand", false, parsedCommand.Command).Return(0)
	mockActions.On("ExitProcessWithCode", 0).Return()

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_RetagFails_Fallback(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	cacheTagsByTarget := map[string]string{
		"default": "myreg1/myimage:mimosa-content-hash-" + TestHash,
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(true, cacheTagsByTarget, nil)
	mockActions.On("RetagFromCacheTags", cacheTagsByTarget, parsedCommand.TagsByTarget, false).Return(errors.New("retag error"))
	mockActions.On("RunCommand", false, parsedCommand.Command).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retag error")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_CommandFails(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(false, nil, nil)
	mockActions.On("RunCommand", false, parsedCommand.Command).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error running command - exit code: 1")
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_SaveCacheTagsFails_Continues(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(false, nil, nil)
	mockActions.On("RunCommand", false, parsedCommand.Command).Return(0)
	mockActions.On("SaveRegistryCacheTags", TestHash, parsedCommand.TagsByTarget, false).Return(errors.New("save error"))

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	// SaveRegistryCacheTags errors are logged as warnings but don't fail the command
	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_RegistryCache_MultipleTargets(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "buildx", "bake", "-f", "docker-bake.hcl"},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:    TestHash,
		Command: []string{"docker", "buildx", "bake", "-f", "docker-bake.hcl"},
		TagsByTarget: map[string][]string{
			"frontend": {"frontend:latest"},
			"backend":  {"backend:latest"},
		},
	}

	cacheTagsByTarget := map[string]string{
		"frontend": "frontend:mimosa-content-hash-" + TestHash,
		"backend":  "backend:mimosa-content-hash-" + TestHash,
	}

	mockActions.On("ParseCommand", []string{"docker", "buildx", "bake", "-f", "docker-bake.hcl"}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(true, cacheTagsByTarget, nil)
	mockActions.On("RetagFromCacheTags", cacheTagsByTarget, parsedCommand.TagsByTarget, false).Return(nil)

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}

func TestRun_RememberEnabled_ParseCommandError_Fallback(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"invalid", "command"},
		DryRun:       false,
	}

	mockActions := &MockActions{}

	mockActions.On("ParseCommand", []string{"invalid", "command"}).Return(configuration.ParsedCommand{
		Command: []string{"invalid", "command"},
	}, errors.New("parse error"))
	mockActions.On("RunCommand", false, []string{"invalid", "command"}).Return(1)
	mockActions.On("ExitProcessWithCode", 1).Return()

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse error")
	mockActions.AssertExpectations(t)
}

func TestRun_DryRunMode(t *testing.T) {
	rememberOptions := configuration.RememberSubcommandOptions{
		Enabled:      true,
		CommandToRun: []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		DryRun:       true,
	}

	mockActions := &MockActions{}

	parsedCommand := configuration.ParsedCommand{
		Hash:         TestHash,
		Command:      []string{"docker", "build", "-t", "myreg1/myimage:v1", "."},
		TagsByTarget: map[string][]string{"default": {"myreg1/myimage:v1"}},
	}

	mockActions.On("ParseCommand", []string{"docker", "build", "-t", "myreg1/myimage:v1", "."}).Return(parsedCommand, nil)
	mockActions.On("CheckRegistryCacheExists", TestHash, parsedCommand.TagsByTarget).Return(false, nil, nil)
	mockActions.On("RunCommand", true, parsedCommand.Command).Return(0)
	mockActions.On("SaveRegistryCacheTags", TestHash, parsedCommand.TagsByTarget, true).Return(nil)

	err := HandleRememberOrForgetSubcommands(rememberOptions, configuration.ForgetSubcommandOptions{}, mockActions)

	assert.NoError(t, err)
	mockActions.AssertExpectations(t)
}
