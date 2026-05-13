Status: completed

## Parent

`.scratch/worktree-development-workflow/PRD.md`

## What to build

`skillshare worktree merge <branch> [--force]` — wraps the full merge-and-cleanup flow for a completed feature worktree.

The command:

1. Verifies the worktree exists (via `worktree.List`).
2. Checks the worktree's git status. If dirty (uncommitted changes), refuses unless `--force` is passed.
3. Checks that all target symlinks are valid (no broken links pointing to non-existent source paths). Warns if broken links are found; refuses unless `--force`.
4. Checks out the main branch in the main repo.
5. Runs `git merge <branch>`.
6. On merge success, runs `worktree.Remove` to delete the worktree and prune the branch.
7. Prints next steps: push (`git push`), sync (`skillshare sync -p` from main repo).

If the merge conflicts, abort and leave the worktree intact so the user can resolve manually. Print the worktree path for context.

## Acceptance criteria

- [ ] `skillshare worktree merge feat/foo` on a clean, fully-committed worktree merges to main and removes the worktree
- [ ] Refuses to merge if the worktree has uncommitted changes (without `--force`)
- [ ] Refuses to merge if broken target symlinks are detected (without `--force`)
- [ ] `--force` overrides both the dirty-check and broken-symlink guards
- [ ] On merge conflict, the merge is aborted, worktree is preserved, and a clear message is printed
- [ ] After successful merge, prints `git push` and `skillshare sync -p` as next steps
- [ ] Running outside a git repo prints a clear error
- [ ] Integration test: create worktree → target add → sync → create skill → collect → commit → merge → verify main branch has the skill → verify worktree removed

## Blocked by

- `04-enhanced-collect-symlink-back`
