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

func TestWorktreeMerge_CleanWorktree(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)

	// Create a worktree
	wt, err := worktree.Create(repo, "feat/merge-test")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize skillshare + add a skill file to commit
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(wt.Path, ".skillshare", "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test"), 0644)

	// Commit in the worktree
	exec.Command("git", "-C", wt.Path, "add", "-A").Run()
	exec.Command("git", "-C", wt.Path, "commit", "-m", "add test skill").Run()

	// Switch to the worktree dir and merge
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wt.Path)

	err = worktreeMerge([]string{"feat/merge-test"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify worktree removed
	wts, err := worktree.List(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wts {
		if w.Branch == "feat/merge-test" {
			t.Error("worktree should have been removed after merge")
		}
	}

	// Verify the commit is in the main branch
	os.Chdir(repo)
	out, err := exec.Command("git", "log", "--oneline").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "add test skill") {
		t.Error("expected merge commit in main branch history")
	}
}

func TestWorktreeMerge_DirtyWorktreeRefused(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)

	wt, err := worktree.Create(repo, "feat/dirty-merge")
	if err != nil {
		t.Fatal(err)
	}
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}

	// Make an uncommitted change
	os.WriteFile(filepath.Join(wt.Path, "uncommitted.txt"), []byte("dirty"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wt.Path)

	err = worktreeMerge([]string{"feat/dirty-merge"})
	if err == nil {
		t.Error("expected error for dirty worktree without --force")
	}
	t.Logf("error: %v", err)

	// Cleanup
	os.Chdir(origDir)
	worktree.Remove(repo, "feat/dirty-merge", true)
}

func TestWorktreeMerge_ForceDirty(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)

	wt, err := worktree.Create(repo, "feat/force-dirty-merge")
	if err != nil {
		t.Fatal(err)
	}
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}

	// Commit something so the branch has a commit to merge
	os.MkdirAll(filepath.Join(wt.Path, ".skillshare", "skills", "skill-a"), 0755)
	os.WriteFile(filepath.Join(wt.Path, ".skillshare", "skills", "skill-a", "SKILL.md"), []byte("# A"), 0644)
	exec.Command("git", "-C", wt.Path, "add", "-A").Run()
	exec.Command("git", "-C", wt.Path, "commit", "-m", "add skill a").Run()

	// Make an uncommitted change
	os.WriteFile(filepath.Join(wt.Path, "uncommitted.txt"), []byte("dirty"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wt.Path)

	err = worktreeMerge([]string{"feat/force-dirty-merge", "--force"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify merge succeeded despite dirty worktree
	wts, _ := worktree.List(repo)
	for _, w := range wts {
		if w.Branch == "feat/force-dirty-merge" {
			t.Error("worktree should have been removed after force merge")
		}
	}
}

func TestWorktreeMerge_SyncPreviewDetectsNewRepairNeed(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)
	mockTarget := filepath.Join(tmp, "mock-project", ".claude", "skills")
	if err := os.MkdirAll(mockTarget, 0o755); err != nil {
		t.Fatal(err)
	}

	writeWorktreeMergeProjectConfig(t, repo, mockTarget)

	wt, err := worktree.Create(repo, "feat/merge-preview")
	if err != nil {
		t.Fatal(err)
	}
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}
	writeWorktreeMergeProjectConfig(t, wt.Path, mockTarget)

	localSkillDir := filepath.Join(mockTarget, "mock-skill")
	if err := os.MkdirAll(localSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte("# Mock"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	if err := os.Chdir(wt.Path); err != nil {
		t.Fatal(err)
	}
	if err := worktreeCollect([]string{"claude", "--force"}); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "-C", wt.Path, "add", "-A").Run()
	exec.Command("git", "-C", wt.Path, "commit", "-m", "add mock skill").Run()

	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	output := stripANSIWarnings(captureStdout(t, func() {
		if mergeErr := worktreeMerge([]string{"feat/merge-preview"}); mergeErr != nil {
			t.Fatalf("worktreeMerge failed: %v", mergeErr)
		}
	}))

	if !strings.Contains(output, "sync -p") {
		t.Fatalf("expected merge output to mention sync guidance, got:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(output), "claude") {
		t.Fatalf("expected merge output to mention the affected target, got:\n%s", output)
	}

	if _, err := os.Stat(filepath.Join(mockTarget, "mock-skill", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected target skill link to remain broken until a real sync runs; stat err=%v", err)
	}

	wts, err := worktree.List(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wts {
		if w.Branch == "feat/merge-preview" {
			t.Error("worktree should have been removed after merge")
		}
	}
}

func TestWorktreeMerge_SyncPreviewWarnsOnPreExistingLocalDrift(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)
	mockTarget := filepath.Join(tmp, "mock-project", ".claude", "skills")
	if err := os.MkdirAll(mockTarget, 0o755); err != nil {
		t.Fatal(err)
	}

	writeWorktreeMergeProjectConfig(t, repo, mockTarget)

	wt, err := worktree.Create(repo, "feat/merge-preview-local")
	if err != nil {
		t.Fatal(err)
	}
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}
	writeWorktreeMergeProjectConfig(t, wt.Path, mockTarget)

	localSkillDir := filepath.Join(mockTarget, "mock-skill")
	if err := os.MkdirAll(localSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte("# Mock"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	if err := os.Chdir(wt.Path); err != nil {
		t.Fatal(err)
	}
	if err := worktreeCollect([]string{"claude", "--force"}); err != nil {
		t.Fatal(err)
	}

	extraLocalDir := filepath.Join(mockTarget, "other-local")
	if err := os.MkdirAll(extraLocalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extraLocalDir, "SKILL.md"), []byte("# Local"), 0o644); err != nil {
		t.Fatal(err)
	}

	exec.Command("git", "-C", wt.Path, "add", "-A").Run()
	exec.Command("git", "-C", wt.Path, "commit", "-m", "add mock skill").Run()

	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	output := stripANSIWarnings(captureStdout(t, func() {
		if mergeErr := worktreeMerge([]string{"feat/merge-preview-local"}); mergeErr != nil {
			t.Fatalf("worktreeMerge failed: %v", mergeErr)
		}
	}))

	if !strings.Contains(strings.ToLower(output), "unmanaged local content") {
		t.Fatalf("expected pre-merge local drift warning, got:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(output), "other-local") && !strings.Contains(strings.ToLower(output), "claude") {
		t.Fatalf("expected warning context for affected target, got:\n%s", output)
	}
	if strings.Contains(output, "repair the merged skill link") {
		t.Fatalf("expected broader warning path instead of narrow repair guidance, got:\n%s", output)
	}
}

func TestWorktreeMerge_SyncPreviewFailureIsReported(t *testing.T) {
	tmp := t.TempDir()
	repo := setupTestRepo(t, tmp)
	mockTarget := filepath.Join(tmp, "mock-project", ".claude", "skills")
	if err := os.MkdirAll(mockTarget, 0o755); err != nil {
		t.Fatal(err)
	}

	writeWorktreeMergeProjectConfig(t, repo, mockTarget)
	if err := os.RemoveAll(filepath.Join(repo, ".skillshare", "skills")); err != nil {
		t.Fatal(err)
	}

	wt, err := worktree.Create(repo, "feat/merge-preview-failure")
	if err != nil {
		t.Fatal(err)
	}
	if err := initWorktreeSkillshare(wt.Path, false); err != nil {
		t.Fatal(err)
	}
	writeWorktreeMergeProjectConfig(t, wt.Path, mockTarget)

	localSkillDir := filepath.Join(mockTarget, "mock-skill")
	if err := os.MkdirAll(localSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte("# Mock"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	if err := os.Chdir(wt.Path); err != nil {
		t.Fatal(err)
	}
	if err := worktreeCollect([]string{"claude", "--force"}); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "-C", wt.Path, "add", "-A").Run()
	exec.Command("git", "-C", wt.Path, "commit", "-m", "add mock skill").Run()

	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	output := stripANSIWarnings(captureStdout(t, func() {
		if mergeErr := worktreeMerge([]string{"feat/merge-preview-failure"}); mergeErr != nil {
			t.Fatalf("worktreeMerge failed: %v", mergeErr)
		}
	}))

	if !strings.Contains(strings.ToLower(output), "could not preview project sync state before merge") {
		t.Fatalf("expected preview failure warning, got:\n%s", output)
	}
}

func writeWorktreeMergeProjectConfig(t *testing.T, root, targetPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.ProjectConfig{
		Targets: []config.ProjectTargetEntry{
			{
				Name: "claude",
				Skills: &config.ResourceTargetConfig{
					Path: targetPath,
					Mode: "merge",
				},
			},
		},
		Audit: config.AuditConfig{
			BlockThreshold: "CRITICAL",
		},
	}
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
}
