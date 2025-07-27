package argsparser

import (
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1h", time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"3w", 504 * time.Hour, false},
		{"1.5h", 90 * time.Minute, false},
		{"1d2h", 26 * time.Hour, false},
		{"1w3d", (7 + 3) * 24 * time.Hour, false},
		{"1M", 30 * 24 * time.Hour, false},
		{"1y", 365 * 24 * time.Hour, false},
		{"-2h", -2 * time.Hour, false},
		{"", 0, false},
		{"5d2x", 0, true},
		{"1x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v, got %v", tt.input, err, tt.wantErr, got)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParse_Remember(t *testing.T) {
	args := []string{"mimosa", "remember", "--dry-run", "--", "docker", "buildx", "build", "."}
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.Remember.Enabled {
		t.Error("Remember.Enabled should be true")
	}
	if !opts.Remember.DryRun {
		t.Error("Remember.DryRun should be true")
	}
	wantCmd := []string{"docker", "buildx", "build", "."}
	if len(opts.Remember.CommandToRun) != len(wantCmd) {
		t.Errorf("Remember.CommandToRun = %v, want %v", opts.Remember.CommandToRun, wantCmd)
	}
	for i, v := range wantCmd {
		if opts.Remember.CommandToRun[i] != v {
			t.Errorf("Remember.CommandToRun[%d] = %v, want %v", i, opts.Remember.CommandToRun[i], v)
		}
	}
}

func TestParse_Forget(t *testing.T) {
	args := []string{"mimosa", "forget", "--dry-run", "--", "docker", "buildx", "build", "."}
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.Forget.Enabled {
		t.Error("Forget.Enabled should be true")
	}
	if !opts.Forget.DryRun {
		t.Error("Forget.DryRun should be true")
	}
	wantCmd := []string{"docker", "buildx", "build", "."}
	if len(opts.Forget.CommandToRun) != len(wantCmd) {
		t.Errorf("Forget.CommandToRun = %v, want %v", opts.Forget.CommandToRun, wantCmd)
	}
	for i, v := range wantCmd {
		if opts.Forget.CommandToRun[i] != v {
			t.Errorf("Forget.CommandToRun[%d] = %v, want %v", i, opts.Forget.CommandToRun[i], v)
		}
	}
}

func TestParse_Cache(t *testing.T) {
	args := []string{"mimosa", "cache", "--forget", "2d", "--yes", "--show", "--to-env-value", "--purge"}
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.Cache.Enabled {
		t.Error("Cache.Enabled should be true")
	}
	if opts.Cache.Forget != "2d" {
		t.Errorf("Cache.Forget = %v, want %v", opts.Cache.Forget, "2d")
	}
	if !opts.Cache.ForgetYes {
		t.Error("Cache.ForgetYes should be true")
	}
	if !opts.Cache.Show {
		t.Error("Cache.Show should be true")
	}
	if !opts.Cache.ToEnvValue {
		t.Error("Cache.ToEnvValue should be true")
	}
	if !opts.Cache.Purge {
		t.Error("Cache.Purge should be true")
	}
}

func TestParse_UnknownSubcommand(t *testing.T) {
	args := []string{"mimosa", "unknown"}
	_, err := Parse(args)
	if err == nil {
		t.Error("Expected error for unknown subcommand, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "please specify one of the valid subcommands") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParse_NoSubcommand(t *testing.T) {
	args := []string{"mimosa"}
	_, err := Parse(args)
	if err == nil {
		t.Error("Expected error for missing subcommand, got nil")
	}
}

func TestParse_Remember_NoArgs(t *testing.T) {
	args := []string{"mimosa", "remember"}
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.Remember.Enabled {
		t.Error("Remember.Enabled should be true")
	}
	if opts.Remember.DryRun {
		t.Error("Remember.DryRun should be false by default")
	}
	if len(opts.Remember.CommandToRun) != 0 {
		t.Errorf("Remember.CommandToRun = %v, want empty", opts.Remember.CommandToRun)
	}
}

func TestParse_Forget_NoArgs(t *testing.T) {
	args := []string{"mimosa", "forget"}
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.Forget.Enabled {
		t.Error("Forget.Enabled should be true")
	}
	if opts.Forget.DryRun {
		t.Error("Forget.DryRun should be false by default")
	}
	if len(opts.Forget.CommandToRun) != 0 {
		t.Errorf("Forget.CommandToRun = %v, want empty", opts.Forget.CommandToRun)
	}
}

func TestParse_Cache_NoArgs(t *testing.T) {
	args := []string{"mimosa", "cache"}
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.Cache.Enabled {
		t.Error("Cache.Enabled should be true")
	}
	if opts.Cache.Forget != "" {
		t.Errorf("Cache.Forget = %v, want empty", opts.Cache.Forget)
	}
	if opts.Cache.ForgetYes {
		t.Error("Cache.ForgetYes should be false by default")
	}
	if opts.Cache.Show {
		t.Error("Cache.Show should be false by default")
	}
	if opts.Cache.ToEnvValue {
		t.Error("Cache.ToEnvValue should be false by default")
	}
	if opts.Cache.Purge {
		t.Error("Cache.Purge should be false by default")
	}
}

func TestParse_Remember_ParseError(t *testing.T) {
	// --dry-run expects no value, so passing a value triggers a parse error
	args := []string{"mimosa", "remember", "--dry-run=notabool"}
	_, err := Parse(args)
	if err == nil {
		t.Error("Expected error when parsing invalid remember args, got nil")
	}
}

func TestParse_Forget_ParseError(t *testing.T) {
	args := []string{"mimosa", "forget", "--dry-run=notabool"}
	_, err := Parse(args)
	if err == nil {
		t.Error("Expected error when parsing invalid forget args, got nil")
	}
}

func TestParse_Cache_ParseError(t *testing.T) {
	args := []string{"mimosa", "cache", "--forget"}
	// --forget expects a value, so omitting it triggers a parse error
	_, err := Parse(args)
	if err == nil {
		t.Error("Expected error when parsing invalid cache args, got nil")
	}
}
