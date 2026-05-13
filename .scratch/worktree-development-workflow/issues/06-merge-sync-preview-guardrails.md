Status: completed

## Parent

`.scratch/worktree-development-workflow/PRD.md`

## What to build

Extend `skillshare worktree merge <branch>` with sync preview guardrails instead of requiring the user to infer target repair needs manually after merge.

The command should use `skillshare sync -p --dry-run --json` from the main repo in two places:

1. Before the merge, to warn if main-repo targets are already out of sync.
2. After a successful merge, to preview whether the merge changed target sync state and whether the user should run `skillshare sync -p` immediately.

The UX should stay non-destructive: no automatic real sync, only preview plus guidance.

Decision rules:

- If the pre-merge preview shows target drift, warn before merging.
- If any target reports local unmanaged content, warn more strongly that the target was already dirty and that a later full sync may affect more than the merged skill.
- After merge, compare the post-merge dry-run JSON with the pre-merge dry-run JSON.
- If the post-merge preview introduces new repair-like drift for only one target, print a strong recommendation to run `skillshare sync -p` now to repair the merged skill link.
- If multiple targets changed, or if the target already had broader drift before merge, print a broader warning instead of an overconfident repair message.

This slice depends on the current `sync --dry-run --json` output shape rather than changing the sync JSON schema. Target-level comparisons are sufficient; skill-level naming is out of scope here.

## Acceptance criteria

- [x] `skillshare worktree merge <branch>` runs a pre-merge `skillshare sync -p --dry-run --json` preview from the main repo before performing the merge
- [x] If the pre-merge preview shows unmanaged local target content, the merge output warns the user before continuing
- [x] A clean pre-merge preview does not block the merge
- [x] After a successful merge, `skillshare worktree merge <branch>` runs a second `skillshare sync -p --dry-run --json` preview from the main repo
- [x] The command compares pre- and post-merge preview results per target and prints a focused recommendation when only one target newly needs repair
- [x] If multiple targets change, or if targets were already dirty before merge, the command prints a broader warning rather than a narrow “just repair this merged skill” message
- [x] The command does not run a real `skillshare sync -p` automatically
- [x] If the dry-run preview command fails, the merge output reports that previewing sync state failed without silently hiding the problem
- [x] Integration test covers: main repo target configured before worktree creation, collect in mock project, merge, post-merge preview detects new target repair need
- [x] Integration test covers: pre-existing unrelated local target drift produces a stronger pre-merge warning

## Blocked by

- `.scratch/worktree-development-workflow/issues/05-worktree-merge.md`
