Status: completed

## Parent

`.scratch/worktree-development-workflow/PRD.md`

## What to build

Create a new `internal/worktree/` package that wraps `git worktree` operations in a clean Go interface. This is a deep module: three functions, no dependency on other skillshare internals, testable in isolation with real temp git repos.

The package provides:

- `Create(branch string, repoPath string) (*Worktree, error)` — runs `git worktree add` to create a new worktree on a feature branch, returns worktree metadata.
- `List(repoPath string) ([]Worktree, error)` — runs `git worktree list` and parses the output into structured `Worktree` structs (branch name, path, commitish).
- `Remove(branch string, force bool) error` — runs `git worktree remove` (with `--force` if requested). Refuses to remove if the worktree has uncommitted changes unless force is true.

`Worktree` struct:

```go
type Worktree struct {
    Branch    string
    Path      string
    Commitish string
}
```

All functions must handle edge cases gracefully: the worktree path not existing, git not installed, the repo not being a git repo.

## Acceptance criteria

- [ ] `Create` creates a new git worktree on a new branch, returns the correct Worktree with branch/path/commitish populated
- [ ] `Create` returns an error when the worktree path already exists or the repo is not a git repo
- [ ] `List` returns all worktrees for a repo, including the main worktree, with correct metadata
- [ ] `Remove` deletes a worktree and prunes its branch metadata
- [ ] `Remove` without `force` returns an error when the worktree has uncommitted changes
- [ ] `Remove` with `force` succeeds even with uncommitted changes
- [ ] Unit tests use a real git repo in a temp directory (no mocking of `git` CLI)

## Blocked by

None — can start immediately.
