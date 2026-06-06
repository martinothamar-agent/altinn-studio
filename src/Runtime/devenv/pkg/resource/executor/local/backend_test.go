package localbackend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"altinn.studio/devenv/pkg/resource"
	"altinn.studio/devenv/pkg/resource/executor"
)

func TestBackendSupportsLocalResources(t *testing.T) {
	t.Parallel()

	backend := New()
	for _, res := range []resource.Resource{
		&resource.LocalFile{},
		&resource.GitCheckout{},
	} {
		if !backend.Supports(res) {
			t.Fatalf("Supports(%T) = false", res)
		}
	}
	if backend.Supports(&resource.OCIArtifact{}) {
		t.Fatalf("Supports(OCIArtifact) = true, want false")
	}
}

func TestApplyLocalFileWritesContentAtomically(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "config.yml")
	file := &resource.LocalFile{Name: "config", Path: path, Content: []byte("content"), Mode: 0o600}

	if _, err := New().Apply(t.Context(), executor.BackendContext{GraphID: "test"}, file); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("content = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 0600", got)
	}
}

func TestApplyGitCheckoutClonesBranch(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initGitRepo(t, repo)
	checkoutPath := filepath.Join(t.TempDir(), "checkout")
	checkout := &resource.GitCheckout{
		Name:    "chart",
		RepoURL: repo,
		Ref:     "main",
		Path:    checkoutPath,
	}

	if _, err := New().Apply(t.Context(), executor.BackendContext{GraphID: "test"}, checkout); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(checkoutPath, "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("checked out content = %q", data)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("add", ".")
	run("commit", "-m", "init")
}
