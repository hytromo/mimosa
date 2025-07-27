package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestExtractBuildFlags(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		wantTag, wantFile string
	}{
		{"long flags", []string{"build", "--tag", "foo:bar", "--file", "Dockerfile.dev"}, "foo:bar", "Dockerfile.dev"},
		{"short flags", []string{"build", "-t", "foo:bar", "-f", "Dockerfile.dev"}, "foo:bar", "Dockerfile.dev"},
		{"eq flags", []string{"build", "--tag=foo:bar", "--file=Dockerfile.dev"}, "foo:bar", "Dockerfile.dev"},
		{"short eq flags", []string{"build", "-t=foo:bar", "-f=Dockerfile.dev"}, "foo:bar", "Dockerfile.dev"},
		{"mixed", []string{"build", "-t", "foo:bar", "--file=Dockerfile.dev"}, "foo:bar", "Dockerfile.dev"},
		{"missing", []string{"build"}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, file, _ := extractBuildFlags(tt.args)
			if tag != tt.wantTag || file != tt.wantFile {
				t.Errorf("got (%q, %q), want (%q, %q)", tag, file, tt.wantTag, tt.wantFile)
			}
		})
	}
}

func TestFindContextPath(t *testing.T) {
	args := []string{"build", "-t", "foo:bar", ".", "--file", "Dockerfile"}
	ctx, err := findContextPath(args)
	if err != nil || ctx != "." {
		t.Errorf("expected '.', got %q, err=%v", ctx, err)
	}
	args = []string{"build", "-t", "foo:bar", "-f", "Dockerfile"}
	_, err = findContextPath(args)
	if err == nil {
		t.Error("expected error for missing context path")
	}
}

func TestResolveDockerfilePath(t *testing.T) {
	cwd := t.TempDir()
	abs := filepath.Join(cwd, "Dockerfile")
	got := resolveDockerfilePath(cwd, "")
	if got != abs {
		t.Errorf("expected %q, got %q", abs, got)
	}
	got = resolveDockerfilePath(cwd, "Dockerfile.custom")
	want := filepath.Join(cwd, "Dockerfile.custom")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestFindDockerignorePath(t *testing.T) {
	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile")
	di := filepath.Join(dir, ".dockerignore")
	_ = os.WriteFile(df, []byte("FROM scratch"), 0644)
	_ = os.WriteFile(di, []byte("*.txt"), 0644)
	got := findDockerignorePath(dir, df)
	if got != di {
		t.Errorf("expected %q, got %q", di, got)
	}
	// dockerignore in Dockerfile dir with custom name
	df2 := filepath.Join(dir, "Dockerfile.custom")
	_ = os.WriteFile(df2, []byte("FROM scratch"), 0644)
	di2 := df2 + ".dockerignore"
	_ = os.WriteFile(di2, []byte("*.md"), 0644)
	got2 := findDockerignorePath(dir, df2)
	if got2 != di2 {
		t.Errorf("expected %q, got %q", di2, got2)
	}

	// no dockerignore
	_ = os.Remove(di)
	got3 := findDockerignorePath(dir, filepath.Join(dir, "NoDockerfile"))
	if got3 != "" {
		t.Errorf("expected empty, got %q", got3)
	}
}

func TestBuildCmdWithTagPlaceholder(t *testing.T) {
	tests := [][]string{
		{"docker", "build", "--tag", "foo:bar", "."},
		{"docker", "build", "-t", "foo:bar", "."},
		{"docker", "build", "--tag=foo:bar", "."},
		{"docker", "build", "-t=foo:bar", "."},
	}
	for _, in := range tests {
		out := buildCmdWithTagPlaceholder(in)
		found := false
		for i := range out {
			if (in[i] == "--tag" || in[i] == "-t") && i+1 < len(out) {
				if out[i+1] == "TAG" {
					found = true
				}
			}
			if strings.HasPrefix(in[i], "--tag=") && out[i] == "--tag=TAG" {
				found = true
			}
			if strings.HasPrefix(in[i], "-t=") && out[i] == "-t=TAG" {
				found = true
			}
		}
		if !found {
			t.Errorf("tag placeholder not found in %v -> %v", in, out)
		}
	}
}

func TestExtractRegistryDomain(t *testing.T) {
	tests := []struct {
		tag, want string
	}{
		{"ubuntu:latest", "docker.io"},
		{"library/ubuntu:latest", "docker.io"},
		{"gcr.io/myproj/img:tag", "gcr.io"},
		{"myregistry:5000/foo/bar:tag", "myregistry:5000"},
		{"127.0.0.1:5000/foo/bar:tag", "127.0.0.1:5000"},
	}
	for _, tt := range tests {
		got := extractRegistryDomain(tt.tag)
		if got != tt.want {
			t.Errorf("tag %q: got %q, want %q", tt.tag, got, tt.want)
		}
	}
}

func TestParseBuildCommand(t *testing.T) {
	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile")
	_ = os.WriteFile(df, []byte("FROM scratch"), 0644)
	di := filepath.Join(dir, ".dockerignore")
	_ = os.WriteFile(di, []byte("*.txt"), 0644)
	cmd := []string{"docker", "build", "--tag", "foo/bar:tag", dir}
	log.SetLevel(log.DebugLevel)
	parsed, err := ParseBuildCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.FinalTag != "foo/bar:tag" {
		t.Errorf("FinalTag: got %q", parsed.FinalTag)
	}
	if parsed.ContextPath != dir {
		t.Errorf("ContextPath: got %q, want %q", parsed.ContextPath, dir)
	}
	if parsed.DockerfilePath != filepath.Join(dir, "Dockerfile") {
		t.Errorf("DockerfilePath: got %q", parsed.DockerfilePath)
	}
	if parsed.DockerignorePath != di {
		t.Errorf("DockerignorePath: got %q", parsed.DockerignorePath)
	}
	if parsed.Executable != "docker" {
		t.Errorf("Executable: got %q", parsed.Executable)
	}
	if parsed.RegistryDomain != "docker.io" {
		t.Errorf("RegistryDomain: got %q", parsed.RegistryDomain)
	}
	// error cases
	_, err = ParseBuildCommand([]string{"podman", "build", "--tag", "foo", dir})
	if err == nil || !strings.Contains(err.Error(), "only 'docker' executable") {
		t.Errorf("expected error for non-docker")
	}
	_, err = ParseBuildCommand([]string{"docker", "run", "foo"})
	if err == nil || !strings.Contains(err.Error(), "only image building") {
		t.Errorf("expected error for non-build")
	}
	_, err = ParseBuildCommand([]string{"docker", "build", dir})
	if err == nil || !strings.Contains(err.Error(), "cannot find image tag") {
		t.Errorf("expected error for missing tag")
	}
}

func TestRunCommand_DryRun(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	old := log.StandardLogger().Out
	defer func() { log.SetOutput(old) }()
	// Should not actually run the command
	code := RunCommand([]string{"sh", "-c", "exit 25"}, true)
	if code != 0 {
		// with dry run we always expect 0
		t.Errorf("expected 0, got %d", code)
	}
}

func TestRunCommand_Success(t *testing.T) {
	code := RunCommand([]string{"echo", "hello"}, false)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

func TestRunCommand_Failure(t *testing.T) {
	code := RunCommand([]string{"false"}, false)
	if code == 0 {
		t.Errorf("expected nonzero exit code, got %d", code)
	}
}
