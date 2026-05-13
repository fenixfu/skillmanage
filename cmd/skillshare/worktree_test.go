package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/worktree"
)

// setupTestRepo creates a git repo in dir and returns the path.
func setupTestRepo(t *testing.T, tmp string) string {
	t.Helper()
	repo := filepath.Join(tmp, "repo")
	exec.Command("git", "init", repo).Run()
	gc := func(k, v string) { exec.Command("git", "-C", repo, "config", k, v).Run() }
	gc("user.email", "test@test.com")
	gc("user.name", "Test")
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# test"), 0644)
	exec.Command("git", "-C", repo, "add", "README.md").Run()
	exec.Command("git", "-C", repo, "commit", "-m", "initial").Run()
	return repo
}

func TestWorktreeTarget_Integration(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)

	// Create a worktree
	wt, err := worktree.Create(repo, "feat/test-target")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize skillshare in the worktree
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}

	// Change to worktree directory for the test
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wt.Path)

	// Test: target add
	homeDir := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(homeDir, "project", ".claude", "skills"), 0755)
	t.Setenv("HOME", homeDir)

	err = worktreeTargetAdd([]string{"claude", "~/project/.claude/skills"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify config.yaml was updated
	cfg, err := config.LoadProject(wt.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(cfg.Targets))
	}
	if cfg.Targets[0].Name != "claude" {
		t.Errorf("expected target name 'claude', got %q", cfg.Targets[0].Name)
	}
	sc := cfg.Targets[0].SkillsConfig()
	if !strings.Contains(sc.Path, filepath.Join("home", "project", ".claude", "skills")) {
		t.Errorf("expected ~-expanded path, got %q", sc.Path)
	}

	// Test: target list (sanity check — just verify no error)
	err = worktreeTargetList(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test: target remove
	err = worktreeTargetRemove([]string{"claude"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify target was removed
	cfg, err = config.LoadProject(wt.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 0 {
		t.Fatalf("expected 0 targets after remove, got %d", len(cfg.Targets))
	}

	// Test: error outside worktree
	os.Chdir(repo)
	err = worktreeTargetAdd([]string{"claude", "/tmp/foo"})
	if err == nil {
		t.Error("expected error when running outside worktree")
	}

	// Cleanup
	os.Chdir(origDir)
	worktree.Remove(repo, "feat/test-target", true)
}
