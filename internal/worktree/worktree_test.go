package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo initializes a git repo in dir with at least one commit.
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v\n%s", err, string(out))
	}
	// Configure git user for commits
	gitCfg := func(key, value string) {
		c := exec.Command("git", "-C", dir, "config", key, value)
		c.Run()
	}
	gitCfg("user.email", "test@test.com")
	gitCfg("user.name", "Test")
	// Create a file so there's something to commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", dir, "add", "README.md")
	cmd.Run()
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "initial")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}
}

func TestList_SingleMainWorktree(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	wts, err := List(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	wt := wts[0]
	if wt.Branch == "" {
		t.Error("expected non-empty branch")
	}
	if wt.Path == "" {
		t.Error("expected non-empty path")
	}
	if wt.Commitish == "" {
		t.Error("expected non-empty commitish")
	}
	t.Logf("branch=%q path=%q commitish=%q", wt.Branch, wt.Path, wt.Commitish)
}

func TestCreate_NewFeatureBranch(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	wt, err := Create(repo, "feat/my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if wt.Branch != "feat/my-skill" {
		t.Errorf("expected branch 'feat/my-skill', got %q", wt.Branch)
	}
	if wt.Path == "" {
		t.Error("expected non-empty path")
	}
	if wt.Commitish == "" {
		t.Error("expected non-empty commitish")
	}
	// Verify the worktree directory exists on disk
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Errorf("worktree path %q does not exist", wt.Path)
	}
	// Verify .git is a file (worktree marker) not a directory
	gitFile := filepath.Join(wt.Path, ".git")
	info, err := os.Lstat(gitFile)
	if err != nil {
		t.Fatalf("cannot stat .git in worktree: %v", err)
	}
	if info.Mode().IsDir() {
		t.Error("worktree .git should be a file, not a directory")
	}
	t.Logf("branch=%q path=%q commitish=%q", wt.Branch, wt.Path, wt.Commitish)
}

func TestCreate_NonGitRepo(t *testing.T) {
	tmp := t.TempDir()
	nonRepo := filepath.Join(tmp, "not-a-repo")
	if err := os.MkdirAll(nonRepo, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Create(nonRepo, "feat/foo")
	if err == nil {
		t.Fatal("expected error for non-git repo, got nil")
	}
	t.Logf("error: %v", err)
}

func TestList_MultipleWorktrees(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	// Initially one worktree (main)
	wts, err := List(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree initially, got %d", len(wts))
	}

	// Create a feature worktree
	_, err = Create(repo, "feat/bar")
	if err != nil {
		t.Fatal(err)
	}

	wts, err = List(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees after create, got %d", len(wts))
	}

	// Verify both branches are present
	branches := make(map[string]bool)
	for _, wt := range wts {
		branches[wt.Branch] = true
	}
	if !branches["feat/bar"] {
		t.Error("expected feat/bar branch in list")
	}
}

func TestRemove_CleanWorktree(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	wt, err := Create(repo, "feat/clean")
	if err != nil {
		t.Fatal(err)
	}

	err = Remove(repo, "feat/clean", false)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the worktree directory is gone
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}

	// Verify it's no longer in the list
	wts, err := List(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wts {
		if w.Branch == "feat/clean" {
			t.Error("removed worktree should not appear in list")
		}
	}
}

func TestRemove_DirtyWithoutForce(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	_, err := Create(repo, "feat/dirty")
	if err != nil {
		t.Fatal(err)
	}

	// List worktrees to get the path
	wts, _ := List(repo)
	var wtPath string
	for _, w := range wts {
		if w.Branch == "feat/dirty" {
			wtPath = w.Path
			break
		}
	}

	// Make an uncommitted change in the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	err = Remove(repo, "feat/dirty", false)
	if err == nil {
		t.Fatal("expected error for dirty worktree without force, got nil")
	}
	t.Logf("error: %v", err)

	// Worktree should still exist
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("dirty worktree should not be removed without force")
	}
}

func TestRemove_DirtyWithForce(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	wt, err := Create(repo, "feat/force-dirty")
	if err != nil {
		t.Fatal(err)
	}

	// Make an uncommitted change
	if err := os.WriteFile(filepath.Join(wt.Path, "dirty.txt"), []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	err = Remove(repo, "feat/force-dirty", true)
	if err != nil {
		t.Fatal(err)
	}

	// Worktree should be gone
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("dirty worktree should be removed with force")
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	setupGitRepo(t, repo)

	_, err := Create(repo, "feat/foo")
	if err != nil {
		t.Fatal(err)
	}
	// Second create for same branch should fail
	_, err = Create(repo, "feat/foo")
	if err == nil {
		t.Fatal("expected error for already-existing branch, got nil")
	}
	t.Logf("error: %v", err)
}
