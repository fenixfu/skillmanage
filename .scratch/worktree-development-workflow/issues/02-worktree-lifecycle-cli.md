Status: ready-for-agent

## Parent

`.scratch/worktree-development-workflow/PRD.md`

## What to build

The `skillshare worktree` lifecycle subcommands: `create`, `list`, `select`, `remove`. These are CLI entry points that call `internal/worktree/` and wire up skillshare project initialization for the worktree.

- `skillshare worktree create <branch>` — runs `worktree.Create`, then inside the new worktree runs the equivalent of `skillshare init -p --config local` to scaffold `.skillshare/config.yaml` with `.gitignore` excluding it. Prints the worktree path on success.

- `skillshare worktree list` — runs `worktree.List`, renders a table with columns: BRANCH, PATH, TARGETS (count from config.yaml, or "-" if not initialized). Highlights the current worktree.

- `skillshare worktree select <branch>` — prints the absolute path of the worktree for the given branch, for use with `cd $(skillshare worktree select feat/x)`.

- `skillshare worktree remove <branch> [--force]` — confirms the worktree exists, warns if unmerged changes present (without `--force`), runs `worktree.Remove`.

All commands operate from the skill-repo root (detected via `git rev-parse --show-toplevel`). Graceful error messages if not in a git repo.

## Acceptance criteria

- [ ] `skillshare worktree create feat/foo` creates a worktree at `<repo>_feat_foo/` on branch `feat/foo`, with `.skillshare/config.yaml` initialized and gitignored
- [ ] `skillshare worktree list` shows all worktrees in a formatted table
- [ ] `skillshare worktree select feat/foo` prints the absolute path
- [ ] `skillshare worktree remove feat/foo` deletes the worktree and prunes the branch
- [ ] `skillshare worktree remove feat/foo` without `--force` on a dirty worktree prints a warning and refuses
- [ ] Running outside a git repo prints a clear error message
- [ ] Integration test: create → list → select → remove a worktree in a temp git repo

## Blocked by

- `01-internal-worktree-package`
