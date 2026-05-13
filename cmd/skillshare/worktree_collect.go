package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// worktreeCollect implements `skillshare worktree collect <target> [flags]`.
func worktreeCollect(args []string) error {
	dryRun := false
	force := false
	var nameOverride string
	var targetName string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--dry-run" || a == "-n":
			dryRun = true
		case a == "--force" || a == "-f":
			force = true
		case a == "--name":
			if i+1 >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			i++
			nameOverride = strings.TrimSpace(args[i])
			if nameOverride == "" {
				return fmt.Errorf("--name requires a non-empty value")
			}
		case a == "--help" || a == "-h":
			printWorktreeCollectHelp()
			return nil
		default:
			if targetName == "" && !strings.HasPrefix(a, "-") {
				targetName = a
			}
		}
	}

	if targetName == "" {
		return fmt.Errorf("usage: skillshare worktree collect <target>\nRun 'skillshare worktree collect --help' for details")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	// Must be in a worktree
	if !isWorktreeDir(cwd) {
		return fmt.Errorf("not in a git worktree (run 'skillshare worktree create <branch>' first)")
	}

	runtime, err := loadProjectRuntime(cwd)
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Find the named target
	target, ok := runtime.targets[targetName]
	if !ok {
		return fmt.Errorf("target %q not found in worktree config", targetName)
	}
	sc := target.SkillsConfig()
	mode := sync.EffectiveMode(sc.Mode)

	// Discover local skills in the target
	ui.Header("Worktree Collect")
	spinner := ui.StartSpinner(fmt.Sprintf("Scanning %s for local skills...", targetName))
	localSkills, err := sync.FindLocalSkills(sc.Path, runtime.sourcePath, mode)
	if err != nil {
		spinner.Fail("Scan failed")
		return fmt.Errorf("failed to scan for local skills: %w", err)
	}

	if len(localSkills) == 0 {
		spinner.Success("No local skills found")
		return nil
	}

	// Apply name override for single-skill collect
	if nameOverride != "" {
		if len(localSkills) != 1 {
			spinner.Stop()
			return fmt.Errorf("--name can only be used when exactly one local skill is found (found %d)", len(localSkills))
		}
		localSkills[0].Name = nameOverride
	}

	spinner.Success(fmt.Sprintf("Found %d local skill(s)", len(localSkills)))

	// List what will be collected
	for _, s := range localSkills {
		ui.Info("  %s (%s)", s.Name, filepath.Base(s.Path))
	}

	if dryRun {
		ui.Info("Dry run - no changes made")
		ui.Info("Run 'skillshare worktree collect %s' to collect these skills", targetName)
		return nil
	}

	// Confirm unless forced
	if !force {
		fmt.Println()
		fmt.Print("Collect these skills to the worktree? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Pull skills with LinkBack
	result, err := sync.PullSkills(localSkills, runtime.sourcePath, sync.PullOptions{
		DryRun:   false,
		Force:    force,
		LinkBack: true,
	})
	if err != nil {
		return fmt.Errorf("failed to collect skills: %w", err)
	}

	// Report results
	ui.Header("Collect Results")
	for _, name := range result.Pulled {
		skillPath := filepath.Join(runtime.sourcePath, name)
		ui.StepDone(name, fmt.Sprintf("collected to %s", skillPath))
	}
	for _, name := range result.Skipped {
		ui.StepSkip(name, "already exists in source, use --force to overwrite")
	}
	for name, e := range result.Failed {
		ui.StepFail(name, e.Error())
	}

	if len(result.Pulled) > 0 {
		fmt.Println()
		ui.Info("Skills collected. Edit them in the worktree and commit when ready.")
		ui.Info("Run 'skillshare worktree merge <branch>' when development is complete.")
	}

	return nil
}

func printWorktreeCollectHelp() {
	fmt.Println("Usage: skillshare worktree collect <target> [options]")
	fmt.Println()
	fmt.Println("Collect local skills from a target into the worktree's source directory.")
	fmt.Println("Local skills (non-symlink directories in the target) are copied to")
	fmt.Println(".skillshare/skills/ and replaced with symlinks back to the source.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --dry-run, -n  Preview changes without applying")
	fmt.Println("  --force, -f    Skip confirmation and overwrite existing skills")
	fmt.Println("  --name <name>  Override the skill name (single-skill collect only)")
	fmt.Println("  --help, -h     Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  skillshare worktree collect claude")
	fmt.Println("  skillshare worktree collect claude --dry-run")
	fmt.Println("  skillshare worktree collect claude --name my-new-skill")
}
