Status: completed

## Parent

`.scratch/worktree-development-workflow/PRD.md`

## What to build

Subcommands under `skillshare worktree target` for managing development targets within a worktree's `.skillshare/config.yaml`.

- `skillshare worktree target add <name> <path>` — appends a target entry to the worktree's `config.yaml`. The target uses default merge mode. Handles `~` expansion. Validates that the target name is a known built-in target (e.g., `claude`, `cursor`, `universal`) or provides a clear error.

- `skillshare worktree target list` — reads the worktree's config and lists targets with name, path, and mode.

- `skillshare worktree target remove <name>` — removes a target entry by name from the worktree's config.

Must operate from inside a git worktree (not the main repo). Detect this by checking if `.git` is a file (worktree marker) vs a directory.

## Acceptance criteria

- [ ] `skillshare worktree target add claude ~/DEV/project/.claude/skills` adds the target to the worktree's config
- [ ] `skillshare worktree target list` shows all targets with name and path
- [ ] `skillshare worktree target remove claude` removes the target from config
- [ ] Running outside a worktree prints a clear error directing to `skillshare worktree create`
- [ ] `~` in paths is expanded to the user's home directory
- [ ] Integration test: create worktree → add target → list → remove target → verify config.yaml

## Blocked by

- `02-worktree-lifecycle-cli`
