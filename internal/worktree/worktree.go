package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree holds metadata for a single git worktree.
type Worktree struct {
	Branch    string // branch name, e.g. "main", "feat/my-skill"
	Path      string // absolute path to worktree directory
	Commitish string // short HEAD hash, e.g. "abc1234"
}

// List returns all worktrees for the repository at repoPath.
func List(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(string(out)), nil
}

// Remove deletes the worktree for the given branch name.
// Set force to true to remove even if the worktree has uncommitted changes.
func Remove(repoPath, branch string, force bool) error {
	wts, err := List(repoPath)
	if err != nil {
		return err
	}
	var wtPath string
	for _, wt := range wts {
		if wt.Branch == branch {
			wtPath = wt.Path
			break
		}
	}
	if wtPath == "" {
		return fmt.Errorf("worktree: branch %q not found", branch)
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wtPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, string(out))
	}
	return nil
}

// Create creates a new git worktree on a new feature branch.
// The worktree directory is a sibling of repoPath named "<repoName>_<branchSlug>".
func Create(repoPath, branch string) (*Worktree, error) {
	repoName := filepath.Base(repoPath)
	branchSlug := strings.ReplaceAll(branch, "/", "_")
	wtPath := filepath.Join(filepath.Dir(repoPath), repoName+"_"+branchSlug)

	// Remove existing directory if it exists (from a failed prior attempt)
	if _, err := os.Stat(wtPath); err == nil {
		if err := os.RemoveAll(wtPath); err != nil {
			return nil, fmt.Errorf("create worktree: failed to remove existing path %s: %w", wtPath, err)
		}
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree add: %w\n%s", err, string(out))
	}

	// Look up the created worktree in the list
	wts, err := List(repoPath)
	if err != nil {
		return nil, err
	}
	for _, wt := range wts {
		if wt.Branch == branch {
			return &wt, nil
		}
	}
	return nil, fmt.Errorf("worktree: created worktree for branch %q not found after add", branch)
}

// parsePorcelain parses git worktree list --porcelain output.
func parsePorcelain(output string) []Worktree {
	var worktrees []Worktree
	var current *Worktree

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "worktree":
			current = &Worktree{Path: value}
		case "HEAD":
			if current != nil {
				current.Commitish = value
			}
		case "branch":
			if current != nil {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		}
	}
	if current != nil {
		worktrees = append(worktrees, *current)
	}
	return worktrees
}
