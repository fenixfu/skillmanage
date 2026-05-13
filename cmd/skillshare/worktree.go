package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	case "merge":
		return worktreeMerge(subargs)
	case "target":
		return worktreeTarget(subargs)
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

// requireMainRepoRoot resolves the main repo root from a worktree path.
// Reads the worktree's .git file to find the main repo location.
func requireMainRepoRoot(wtPath string) (string, error) {
	gitFile := filepath.Join(wtPath, ".git")
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", fmt.Errorf("failed to read .git file in worktree: %w", err)
	}
	// .git file format: "gitdir: /path/to/main/.git/worktrees/name\n"
	content := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(content, prefix) {
		return "", fmt.Errorf("unexpected .git file format")
	}
	gitDir := strings.TrimPrefix(content, prefix)
	// gitDir points to .../main/.git/worktrees/name
	// parent of .git is the main repo: worktrees/name -> .git -> main
	dotGit := filepath.Dir(filepath.Dir(filepath.Dir(gitDir)))
	return dotGit, nil
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

// worktreeMerge merges a feature worktree branch back to the main branch.
func worktreeMerge(args []string) error {
	force := false
	for _, a := range args {
		switch a {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			fmt.Println("Usage: skillshare worktree merge <branch> [--force]")
			fmt.Println()
			fmt.Println("Merge a feature worktree branch into the main branch,")
			fmt.Println("then remove the worktree and prune the feature branch.")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --force, -f  Skip safety checks (uncommitted changes, broken symlinks)")
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
		return fmt.Errorf("usage: skillshare worktree merge <branch>\nRun 'skillshare worktree merge --help' for details")
	}
	branch := branchArgs[0]

	// First find the worktree — we need its path to resolve main repo
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	// Use the current directory if it's a worktree, otherwise look up from main repo
	var wtPath string
	var repoPath string
	if isWorktreeDir(cwd) {
		wtPath = cwd
		repoPath, err = requireMainRepoRoot(wtPath)
		if err != nil {
			return fmt.Errorf("cannot find main repo from worktree: %w", err)
		}
	} else {
		repoPath, err = requireRepoRoot()
		if err != nil {
			return err
		}
	}

	// Verify worktree exists
	wts, err := worktree.List(repoPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}
	var wt *worktree.Worktree
	for i := range wts {
		if wts[i].Branch == branch {
			wt = &wts[i]
			break
		}
	}
	if wt == nil {
		return fmt.Errorf("worktree for branch %q not found", branch)
	}
	wtPath = wt.Path // use listed path (more reliable)

	// Safety check: uncommitted changes in worktree
	dirty, err := worktreeHasChanges(wt.Path)
	if err != nil {
		return fmt.Errorf("failed to check worktree status: %w", err)
	}
	if dirty && !force {
		return fmt.Errorf("worktree %q has uncommitted changes.\nCommit them first, or use --force to discard.\nPath: %s", branch, wt.Path)
	}

	// Safety check: broken target symlinks
	broken, err := checkBrokenTargetSymlinks(wt.Path)
	if err != nil {
		return fmt.Errorf("failed to check target symlinks: %w", err)
	}
	if len(broken) > 0 && !force {
		msg := fmt.Sprintf("worktree has %d broken target symlink(s):\n", len(broken))
		for _, b := range broken {
			msg += fmt.Sprintf("  %s -> %s\n", b.name, b.target)
		}
		msg += "Fix them or use --force to proceed anyway."
		return fmt.Errorf("%s", msg)
	}

	if dirty && force {
		ui.Warning("Worktree has uncommitted changes (--force: proceeding anyway)")
	}
	if len(broken) > 0 && force {
		ui.Warning("%d broken symlink(s) detected (--force: proceeding anyway)", len(broken))
	}

	prePreview, prePreviewErr := runProjectSyncDryRunPreview(repoPath)
	reportPreMergeSyncPreview(prePreview, prePreviewErr)

	// Switch to main branch in main repo
	ui.Header(fmt.Sprintf("Merging %s", branch))
	mainBranch := detectMainBranch(repoPath)
	if err := gitCheckout(repoPath, mainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", mainBranch, err)
	}

	// Merge
	if err := gitMerge(repoPath, branch); err != nil {
		// Conflict — abort and leave worktree intact
		_ = gitMergeAbort(repoPath)
		return fmt.Errorf("merge conflict — aborted.\nWorktree preserved at: %s\nResolve manually and try again.", wt.Path)
	}

	ui.Success("Merged %s into %s", branch, mainBranch)

	// Remove worktree
	if err := worktree.Remove(repoPath, branch, true); err != nil {
		ui.Warning("Merge succeeded but failed to remove worktree: %v", err)
		ui.Info("Worktree path: %s", wt.Path)
	} else {
		ui.Success("Removed worktree for branch %q", branch)
	}

	postPreview, postPreviewErr := runProjectSyncDryRunPreview(repoPath)
	reportPostMergeSyncPreview(prePreview, prePreviewErr, postPreview, postPreviewErr)

	// Next steps
	fmt.Println()
	ui.Header("Next steps")
	ui.Info("Push to remote:   git push")
	ui.Info("Distribute skills: skillshare sync -p")
	return nil
}

// worktreeHasChanges reports whether the worktree has uncommitted changes.
func worktreeHasChanges(wtPath string) (bool, error) {
	cmd := exec.Command("git", "-C", wtPath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// brokenSymlink represents a broken target symlink.
type brokenSymlink struct {
	name   string
	target string
}

// checkBrokenTargetSymlinks scans the worktree's target configs for broken symlinks.
func checkBrokenTargetSymlinks(wtPath string) ([]brokenSymlink, error) {
	cfg, err := config.LoadProject(wtPath)
	if err != nil {
		return nil, err
	}

	var broken []brokenSymlink
	for _, t := range cfg.Targets {
		sc := t.SkillsConfig()
		if sc.Path == "" {
			continue
		}
		entries, err := os.ReadDir(sc.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.Name()[0] == '.' {
				continue
			}
			entryPath := filepath.Join(sc.Path, entry.Name())
			info, err := os.Lstat(entryPath)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if _, err := os.Stat(entryPath); os.IsNotExist(err) {
					target, _ := os.Readlink(entryPath)
					broken = append(broken, brokenSymlink{name: entry.Name(), target: target})
				}
			}
		}
	}
	return broken, nil
}

// detectMainBranch returns the main branch name (main or master).
func detectMainBranch(repoPath string) string {
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch)
		if err := cmd.Run(); err == nil {
			return branch
		}
	}
	return "main" // fallback
}

func gitCheckout(repoPath, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "checkout", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", err, string(out))
	}
	return nil
}

func gitMerge(repoPath, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "merge", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", err, string(out))
	}
	return nil
}

func gitMergeAbort(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "merge", "--abort")
	cmd.Run() // best-effort
	return nil
}

func runProjectSyncDryRunPreview(root string) (*syncJSONOutput, error) {
	if !projectConfigExists(root) {
		return nil, nil
	}

	stats, results, iStats, ctxCost, err := cmdSyncProject(root, true, false, true, true)

	var totals syncModeStats
	details := make([]syncJSONTargetDetail, 0, len(results))
	for _, r := range results {
		totals.linked += r.stats.linked
		totals.local += r.stats.local
		totals.updated += r.stats.updated
		totals.pruned += r.stats.pruned
		details = append(details, syncJSONTargetDetail{
			Name:    r.name,
			Mode:    r.mode,
			Linked:  r.stats.linked,
			Local:   r.stats.local,
			Updated: r.stats.updated,
			Pruned:  r.stats.pruned,
			Error:   r.errMsg,
		})
	}

	ignoredSkills := []string{}
	if iStats != nil && len(iStats.IgnoredSkills) > 0 {
		ignoredSkills = iStats.IgnoredSkills
	}

	return &syncJSONOutput{
		Targets:       len(results),
		Linked:        totals.linked,
		Local:         totals.local,
		Updated:       totals.updated,
		Pruned:        totals.pruned,
		IgnoredCount:  len(ignoredSkills),
		IgnoredSkills: ignoredSkills,
		DryRun:        stats.DryRun,
		Details:       details,
		ContextCost:   ctxCost,
	}, err
}

func reportPreMergeSyncPreview(preview *syncJSONOutput, previewErr error) {
	if previewErr != nil {
		ui.Warning("Could not preview project sync state before merge: %v", previewErr)
		return
	}
	if preview == nil {
		return
	}

	localTargets := previewTargetsMatching(preview, func(detail syncJSONTargetDetail) bool {
		return detail.Local > 0
	})
	if len(localTargets) > 0 {
		ui.Warning("Some targets already have unmanaged local content before merge.")
		ui.Info("Save or reconcile those changes before running a full 'skillshare sync -p'.")
		ui.Info("Affected targets: %s", strings.Join(localTargets, ", "))
		return
	}

	driftTargets := previewTargetsMatching(preview, previewHasDrift)
	if len(driftTargets) > 0 {
		ui.Warning("Some targets are already out of sync before merge.")
		ui.Info("Affected targets: %s", strings.Join(driftTargets, ", "))
		return
	}

	ui.Info("Pre-merge 'skillshare sync -p --dry-run --json' preview is clean.")
}

func reportPostMergeSyncPreview(prePreview *syncJSONOutput, preErr error, postPreview *syncJSONOutput, postErr error) {
	if postErr != nil {
		ui.Warning("Could not preview project sync state after merge: %v", postErr)
		return
	}
	if postPreview == nil {
		return
	}

	changedTargets := diffPreviewTargets(prePreview, postPreview)
	if len(changedTargets) == 1 && preErr == nil {
		target := changedTargets[0]
		preDetail, _ := previewTargetDetail(prePreview, target)
		postDetail, _ := previewTargetDetail(postPreview, target)
		if !previewHasDrift(preDetail) && postDetail.Local == 0 && previewHasDrift(postDetail) {
			ui.Warning("Target %q now needs sync repair after merge.", target)
			ui.Info("Run 'skillshare sync -p' now to repair the merged skill link.")
			return
		}
	}

	if len(changedTargets) > 0 {
		ui.Warning("Project sync preview changed after merge.")
		ui.Info("Affected targets: %s", strings.Join(changedTargets, ", "))
		ui.Info("Run 'skillshare sync -p --dry-run --json' to inspect drift before a full sync.")
		return
	}

	driftTargets := previewTargetsMatching(postPreview, previewHasDrift)
	if len(driftTargets) > 0 {
		ui.Warning("Some targets remain out of sync after merge.")
		ui.Info("Affected targets: %s", strings.Join(driftTargets, ", "))
		ui.Info("Run 'skillshare sync -p --dry-run --json' to inspect drift before a full sync.")
	}
}

func previewTargetsMatching(preview *syncJSONOutput, match func(syncJSONTargetDetail) bool) []string {
	if preview == nil {
		return nil
	}
	var targets []string
	for _, detail := range preview.Details {
		if match(detail) {
			targets = append(targets, detail.Name)
		}
	}
	sort.Strings(targets)
	return targets
}

func previewHasDrift(detail syncJSONTargetDetail) bool {
	return detail.Linked > 0 || detail.Local > 0 || detail.Updated > 0 || detail.Pruned > 0 || detail.Error != ""
}

func diffPreviewTargets(pre, post *syncJSONOutput) []string {
	postMap := make(map[string]syncJSONTargetDetail)
	for _, detail := range post.Details {
		postMap[detail.Name] = detail
	}

	seen := make(map[string]struct{})
	var changed []string
	if pre != nil {
		for _, detail := range pre.Details {
			seen[detail.Name] = struct{}{}
			postDetail, ok := postMap[detail.Name]
			if !ok || detail != postDetail {
				changed = append(changed, detail.Name)
			}
		}
	}
	for _, detail := range post.Details {
		if _, ok := seen[detail.Name]; ok {
			continue
		}
		changed = append(changed, detail.Name)
	}

	sort.Strings(changed)
	return changed
}

func previewTargetDetail(preview *syncJSONOutput, target string) (syncJSONTargetDetail, bool) {
	if preview == nil {
		return syncJSONTargetDetail{}, false
	}
	for _, detail := range preview.Details {
		if detail.Name == target {
			return detail, true
		}
	}
	return syncJSONTargetDetail{}, false
}

// worktreeTarget dispatches `skillshare worktree target <add|list|remove>`.
func worktreeTarget(args []string) error {
	if len(args) < 1 {
		printWorktreeTargetHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "help", "--help", "-h":
		printWorktreeTargetHelp()
		return nil
	case "add":
		return worktreeTargetAdd(subargs)
	case "list":
		return worktreeTargetList(subargs)
	case "remove", "rm":
		return worktreeTargetRemove(subargs)
	default:
		return fmt.Errorf("unknown target subcommand: %s\nRun 'skillshare worktree target --help'", subcmd)
	}
}

func worktreeTargetAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: skillshare worktree target add <name> <path>\nRun 'skillshare worktree target add --help' for details")
	}
	targetName := args[0]
	targetPath := args[1]

	// Expand ~
	if strings.HasPrefix(targetPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot expand ~: %w", err)
		}
		targetPath = filepath.Join(home, targetPath[1:])
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	if !isWorktreeDir(cwd) {
		return fmt.Errorf("not in a git worktree (run 'skillshare worktree create <branch>' first)")
	}

	cfg, err := config.LoadProject(cwd)
	if err != nil {
		return fmt.Errorf("failed to load worktree config: %w", err)
	}

	// Check for duplicate name
	for _, t := range cfg.Targets {
		if t.Name == targetName {
			return fmt.Errorf("target %q already exists in worktree config", targetName)
		}
	}

	// Add target with merge mode
	cfg.Targets = append(cfg.Targets, config.ProjectTargetEntry{
		Name: targetName,
		Skills: &config.ResourceTargetConfig{
			Path: targetPath,
			Mode: "merge",
		},
	})

	if err := cfg.Save(cwd); err != nil {
		return fmt.Errorf("failed to save worktree config: %w", err)
	}

	ui.Success("Added target %q -> %s", targetName, targetPath)
	ui.Info("Run 'skillshare sync -p' to symlink skills to this target")
	return nil
}

func worktreeTargetList(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	if !isWorktreeDir(cwd) {
		return fmt.Errorf("not in a git worktree (run 'skillshare worktree create <branch>' first)")
	}

	cfg, err := config.LoadProject(cwd)
	if err != nil {
		return fmt.Errorf("failed to load worktree config: %w", err)
	}

	if len(cfg.Targets) == 0 {
		ui.Info("No targets configured.")
		ui.Info("Add one: skillshare worktree target add <name> <path>")
		return nil
	}

	ui.Header("Worktree Targets")
	for _, t := range cfg.Targets {
		sc := t.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = "merge"
		}
		ui.Info("  %-20s %-10s %s", t.Name, mode, sc.Path)
	}
	return nil
}

func worktreeTargetRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: skillshare worktree target remove <name>\nRun 'skillshare worktree target remove --help' for details")
	}
	targetName := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	if !isWorktreeDir(cwd) {
		return fmt.Errorf("not in a git worktree (run 'skillshare worktree create <branch>' first)")
	}

	cfg, err := config.LoadProject(cwd)
	if err != nil {
		return fmt.Errorf("failed to load worktree config: %w", err)
	}

	found := false
	var newTargets []config.ProjectTargetEntry
	for _, t := range cfg.Targets {
		if t.Name == targetName {
			found = true
		} else {
			newTargets = append(newTargets, t)
		}
	}
	if !found {
		return fmt.Errorf("target %q not found in worktree config", targetName)
	}

	cfg.Targets = newTargets
	if err := cfg.Save(cwd); err != nil {
		return fmt.Errorf("failed to save worktree config: %w", err)
	}

	ui.Success("Removed target %q", targetName)
	return nil
}

func printWorktreeTargetHelp() {
	fmt.Println("Usage: skillshare worktree target <subcommand> [options]")
	fmt.Println()
	fmt.Println("Manage development targets in a worktree's .skillshare/config.yaml.")
	fmt.Println()
	fmt.Println("SUBCOMMANDS")
	fmt.Println("  add <name> <path>     Add a development target")
	fmt.Println("  list                  List configured targets")
	fmt.Println("  remove <name>         Remove a target")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  skillshare worktree target add claude ~/DEV/my-project/.claude/skills")
	fmt.Println("  skillshare worktree target list")
	fmt.Println("  skillshare worktree target remove claude")
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
	fmt.Println("  merge <branch> [--force]  Merge feature branch to main")
	fmt.Println("  target <subcommand>       Manage targets in the worktree")
	fmt.Println("  collect <target>          Collect local skills from a target")
	fmt.Println()
	fmt.Println("Run 'skillshare worktree <subcommand> --help' for more details.")
}
