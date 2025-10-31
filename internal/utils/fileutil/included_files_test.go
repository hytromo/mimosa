package fileutil

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	err := os.MkdirAll(path, 0755)
	if err != nil {
		t.Fatalf("failed to mkdir %s: %v", path, err)
	}
}

func TestIncludedFiles_NoDockerignore_AllFilesIncluded(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "A")
	mustWriteFile(t, filepath.Join(dir, "b.txt"), "B")
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustWriteFile(t, filepath.Join(dir, "sub", "c.txt"), "C")

	got, _ := IncludedFiles(dir, "")
	want := []string{
		abs(t, filepath.Join(dir, "a.txt")),
		abs(t, filepath.Join(dir, "b.txt")),
		abs(t, filepath.Join(dir, "sub", "c.txt")),
	}
	assertUnorderedEqual(t, got, want)
}

func TestIncludedFiles_Dockerignore_ExcludesFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "A")
	mustWriteFile(t, filepath.Join(dir, "b.txt"), "B")
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustWriteFile(t, filepath.Join(dir, "sub", "c.txt"), "C")
	di := filepath.Join(dir, ".dockerignore")
	mustWriteFile(t, di, "b.txt\nsub/\n")

	got, _ := IncludedFiles(dir, di)
	want := []string{
		abs(t, filepath.Join(dir, "a.txt")),
		abs(t, filepath.Join(dir, ".dockerignore")),
	}
	assertUnorderedEqual(t, got, want)
}

func TestIncludedFiles_Dockerignore_Negation(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "A")
	mustWriteFile(t, filepath.Join(dir, "b.txt"), "B")
	mustWriteFile(t, filepath.Join(dir, "c.txt"), "C")
	di := filepath.Join(dir, ".dockerignore")
	mustWriteFile(t, di, "*.txt\n!b.txt\n")

	got, _ := IncludedFiles(dir, di)
	want := []string{
		abs(t, filepath.Join(dir, "b.txt")),
		abs(t, filepath.Join(dir, ".dockerignore")),
	}
	assertUnorderedEqual(t, got, want)
}

func TestIncludedFiles_Dockerignore_ExcludeAll(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "A")
	di := filepath.Join(dir, ".dockerignore")
	mustWriteFile(t, di, "**\n")

	got, _ := IncludedFiles(dir, di)
	if len(got) != 0 {
		t.Errorf("Expected no files, got %v", got)
	}
}

func TestIncludedFiles_Dockerignore_EmptyAndComments(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "A")
	di := filepath.Join(dir, ".dockerignore")
	mustWriteFile(t, di, "\n# comment\n\n")

	got, _ := IncludedFiles(dir, di)
	want := []string{
		abs(t, filepath.Join(dir, "a.txt")),
		abs(t, filepath.Join(dir, ".dockerignore")),
	}
	assertUnorderedEqual(t, got, want)
}

func TestIncludedFiles_Dockerignore_NonexistentPath_Errors(t *testing.T) {
	dir := t.TempDir()
	_, err := IncludedFiles(dir, filepath.Join(dir, "no-such-file"))
	if err == nil {
		t.Errorf("Expected error for missing .dockerignore, got none")
	}
}

func TestIncludedFiles_DirectoriesNotIncluded(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustWriteFile(t, filepath.Join(dir, "sub", "file.txt"), "X")

	got, _ := IncludedFiles(dir, "")
	for _, f := range got {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		if info.IsDir() {
			t.Errorf("Directory %s should not be included", f)
		}
	}
}

func TestIncludedFiles_AbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "A")
	got, _ := IncludedFiles(dir, "")
	for _, f := range got {
		if !filepath.IsAbs(f) {
			t.Errorf("Path %s is not absolute", f)
		}
	}
}

func TestIncludedFiles_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	mustWriteFile(t, target, "T")
	link := filepath.Join(dir, "link.txt")
	err := os.Symlink(target, link)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	got, _ := IncludedFiles(dir, "")
	want := []string{
		abs(t, target),
		abs(t, link),
	}
	assertUnorderedEqual(t, got, want)
}

// --- helpers ---

func abs(t *testing.T, path string) string {
	t.Helper()
	a, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs failed: %v", err)
	}
	return a
}

func assertUnorderedEqual(t *testing.T, got, want []string) {
	t.Helper()
	gotMap := make(map[string]struct{}, len(got))
	for _, g := range got {
		gotMap[g] = struct{}{}
	}
	for _, w := range want {
		if _, ok := gotMap[w]; !ok {
			t.Errorf("Missing expected file: %s", w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Got %d files, want %d. Got: %v, want: %v", len(got), len(want), got, want)
	}
	if !reflect.DeepEqual(sorted(got), sorted(want)) {
		t.Errorf("Files mismatch.\nGot:  %v\nWant: %v", got, want)
	}
}

func sorted(ss []string) []string {
	cp := append([]string(nil), ss...)
	// simple bubble sort for short slices
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[j] < cp[i] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	return cp
}
