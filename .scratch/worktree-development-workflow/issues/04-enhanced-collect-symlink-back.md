Status: ready-for-agent

## Parent

`.scratch/worktree-development-workflow/PRD.md`

## What to build

Two changes that together create the safe "promote local skill to managed" flow:

### A. Symlink-back in PullSkill

Add a `LinkBack bool` field to `sync.PullOptions`. When true, after `PullSkill` copies a local skill from target to source, it removes the target's local directory and replaces it with a symlink to the newly created source directory. Uses the existing `CreateSymlink` function (which already handles relative vs absolute symlinks via `shouldUseRelative`).

The copy → symlink replacement must be atomic in effect: the skill should never be absent from the target (the copy completes first, then the symlink replaces it).

### B. Collect gate (worktree-only)

Modify `cmdCollect` so that the bare `skillshare collect` command checks whether the current directory is inside a git worktree (`.git` is a file, not a directory). If not in a worktree, refuse with an error message directing the user to `skillshare worktree create` or `skillshare install` for remote sources.

### C. Worktree collect subcommand

`skillshare worktree collect <target> [--dry-run] [--force] [--name <override>]` — finds local skills in the named target, presents them for confirmation (unless `--force`), then runs `PullSkills` with `LinkBack: true`. The `--name` flag overrides the auto-detected name for a single-skill collect.

## Acceptance criteria

- [ ] `PullSkill` with `LinkBack: true` copies the local skill to source, then replaces the target local directory with a symlink to source
- [ ] `PullSkill` with `LinkBack: true` and `DryRun: true` previews both the copy and symlink creation without making changes
- [ ] The symlink format (relative vs absolute) respects the existing `shouldUseRelative` logic
- [ ] `skillshare collect claude` outside a worktree prints a clear error and exits
- [ ] `skillshare worktree collect claude` discovers local skills, confirms, copies to source, and replaces target with symlinks
- [ ] `skillshare worktree collect claude --dry-run` previews without making changes
- [ ] `skillshare worktree collect claude --force` skips confirmation and overwrites existing source skills
- [ ] `skillshare worktree collect claude --name my-custom-name` overrides the skill name for single-skill collect
- [ ] Unit test for PullSkill LinkBack path (temp dirs, real symlinks)
- [ ] Integration test: create worktree → target add → sync → manually create local skill in target → collect → verify symlink in target → verify skill in source

## Blocked by

- `01-internal-worktree-package`
