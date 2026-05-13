package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
	"skillshare/internal/worktree"
)

func cmdWorktree(args []string) error {
	if len(args) < 1 {
		printWorktreeHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "help", "--help", "-h":
		printWorktreeHelp()
		return nil
	case "create":
		return worktreeCreate(subargs)
	case "list":
		return worktreeList(subargs)
	case "select":
		return worktreeSelect(subargs)
	case "remove", "rm":
		return worktreeRemove(subargs)
	case "collect":
		return worktreeCollect(subargs)
	default:
		return fmt.Errorf("unknown worktree subcommand: %s\nRun 'skillshare worktree --help'", subcmd)
	}
}

// requireRepoRoot returns the absolute path to the git repo root,
// or an error if the current directory is not inside a git repo.
func requireRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository (run from your skill-repo root)")
	}
	return strings.TrimSpace(string(out)), nil
}

// isWorktreeDir reports whether dir is a git worktree (not the main repo).
func isWorktreeDir(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return false
	}
	return !info.IsDir() // worktree .git is a file pointing to main repo
}

// worktreeCreate creates a new git worktree with project skillshare config.
func worktreeCreate(args []string) error {
	force := false
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		} else if a == "--help" || a == "-h" {
			fmt.Println("Usage: skillshare worktree create <branch> [--force]")
			fmt.Println()
			fmt.Println("Create a new git worktree on a feature branch for skill development.")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --force, -f  Remove existing worktree directory if it exists")
			fmt.Println("  --help, -h   Show this help")
			return nil
		}
	}
	// Filter out flags for branch name extraction
	var branchArgs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			branchArgs = append(branchArgs, a)
		}
	}
	if len(branchArgs) < 1 {
		return fmt.Errorf("usage: skillshare worktree create <branch>\nRun 'skillshare worktree create --help' for details")
	}
	branch := branchArgs[0]

	repoPath, err := requireRepoRoot()
	if err != nil {
		return err
	}

	wt, err := worktree.Create(repoPath, branch)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Initialize skillshare project config in the worktree (shared-repo flow)
	if err := initWorktreeSkillshare(wt.Path, force); err != nil {
		return fmt.Errorf("failed to initialize skillshare in worktree: %w", err)
	}

	ui.Logo(version)
	ui.Header("Worktree created")
	ui.Success("Branch: %s", wt.Branch)
	ui.Success("Path: %s", wt.Path)
	fmt.Println()
	ui.Info("Next steps:")
	fmt.Printf("  cd %s\n", wt.Path)
	fmt.Printf("  skillshare worktree target add <name> <path>    # Add a development target\n")
	fmt.Printf("  skillshare sync -p                               # Sync existing skills\n")
	return nil
}

// initWorktreeSkillshare scaffolds .skillshare/ in the worktree.
func initWorktreeSkillshare(worktreePath string, force bool) error {
	skillsDir := filepath.Join(worktreePath, ".skillshare", "skills")
	agentsDir := filepath.Join(worktreePath, ".skillshare", "agents")

	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	// Create .gitignore that excludes config.yaml (shared-repo pattern)
	gitignorePath := filepath.Join(worktreePath, ".skillshare", ".gitignore")
	gitignoreContent := "config.yaml\n"
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	// Create minimal project config (no targets yet — developer adds them)
	cfg := &config.ProjectConfig{
		Audit: config.AuditConfig{
			BlockThreshold: "CRITICAL",
		},
	}
	if err := cfg.Save(worktreePath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

// worktreeList lists all worktrees for the current repo.
func worktreeList(args []string) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Println("Usage: skillshare worktree list")
			fmt.Println()
			fmt.Println("List all git worktrees for the current repository.")
			return nil
		}
	}

	repoPath, err := requireRepoRoot()
	if err != nil {
		return err
	}

	wts, err := worktree.List(repoPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(wts) == 0 {
		ui.Info("No worktrees found")
		return nil
	}

	cwd, _ := os.Getwd()
	ui.Header("Worktrees")
	for _, wt := range wts {
		marker := " "
		if wt.Path == cwd || isSameDir(wt.Path, cwd) {
			marker = "*"
		}
		targetCount := countWorktreeTargets(wt.Path)
		targetStr := "-"
		if targetCount >= 0 {
			targetStr = fmt.Sprintf("%d", targetCount)
		}
		fmt.Printf(" %s %-30s %s\n", marker, wt.Branch, wt.Path)
		_ = targetStr // printed in detailed view
		ui.Info("  branch=%s targets=%s commitish=%s%s", wt.Branch, targetStr, wt.Commitish, marker)
	}
	return nil
}

// countWorktreeTargets reads the worktree's config.yaml and returns the target count.
func countWorktreeTargets(wtPath string) int {
	cfg, err := config.LoadProject(wtPath)
	if err != nil {
		return -1
	}
	return len(cfg.Targets)
}

// isSameDir checks if two paths refer to the same directory (resolving symlinks).
func isSameDir(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		return false
	}
	return ra == rb
}

// worktreeSelect prints the absolute path of a worktree for the given branch.
func worktreeSelect(args []string) error {
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: skillshare worktree select <branch>")
		fmt.Println()
		fmt.Println("Print the absolute path of a worktree for use with cd.")
		fmt.Println("Example: cd $(skillshare worktree select feat/my-skill)")
		return nil
	}
	branch := args[0]

	repoPath, err := requireRepoRoot()
	if err != nil {
		return err
	}

	wts, err := worktree.List(repoPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	for _, wt := range wts {
		if wt.Branch == branch {
			fmt.Print(wt.Path)
			return nil
		}
	}
	return fmt.Errorf("worktree for branch %q not found", branch)
}

// worktreeRemove removes a worktree by branch name.
func worktreeRemove(args []string) error {
	force := false
	for _, a := range args {
		switch a {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			fmt.Println("Usage: skillshare worktree remove <branch> [--force]")
			fmt.Println()
			fmt.Println("Remove a git worktree and prune its branch.")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --force, -f  Remove even if the worktree has uncommitted changes")
			fmt.Println("  --help, -h   Show this help")
			return nil
		}
	}
	var branchArgs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			branchArgs = append(branchArgs, a)
		}
	}
	if len(branchArgs) < 1 {
		return fmt.Errorf("usage: skillshare worktree remove <branch>\nRun 'skillshare worktree remove --help' for details")
	}
	branch := branchArgs[0]

	repoPath, err := requireRepoRoot()
	if err != nil {
		return err
	}

	if err := worktree.Remove(repoPath, branch, force); err != nil {
		if !force && strings.Contains(err.Error(), "modified") {
			return fmt.Errorf("worktree has uncommitted changes. Use --force to remove anyway: %w", err)
		}
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	ui.Success("Removed worktree for branch %q", branch)
	return nil
}

func printWorktreeHelp() {
	fmt.Println("Usage: skillshare worktree <subcommand> [options]")
	fmt.Println()
	fmt.Println("Manage git worktrees for skill development.")
	fmt.Println()
	fmt.Println("SUBCOMMANDS")
	fmt.Println("  create <branch>           Create a new worktree on a feature branch")
	fmt.Println("  list                      List all worktrees")
	fmt.Println("  select <branch>           Print the worktree path for the branch")
	fmt.Println("  remove <branch> [--force] Remove a worktree")
	fmt.Println()
	fmt.Println("Run 'skillshare worktree <subcommand> --help' for more details.")
}
