# PRD: Worktree-based skill development workflow

## Problem Statement

skillshare is purpose-built for one-directional skill distribution: a central source syncs skills outward to many targets. The centralized-skills-repo recipe handles distribution cleanly, but a developer who iterates on skills across multiple projects hits a wall: there is no safe, first-class workflow for developing a skill *in the context of a real project* and then promoting it back to the shared skill repository.

The existing `skillshare collect` command partially addresses this — it finds local skills in a target and copies them to source — but it does not create the reverse symlink (that requires a separate `sync --force`), and it operates indiscriminately against the main branch, bypassing feature-branch review. The result is a workflow gap: distribution is smooth, but the development loop (create → test in project → promote → merge) requires manual git operations and discipline, with no tooling guardrails.

The user maintains a dedicated skill-repo (per the centralized-skills-repo recipe) and wants to fork skillshare to add a git-worktree-based development workflow that makes bidirectional skill iteration safe, explicit, and CLI-driven.

## Solution

Introduce a `skillshare worktree` subcommand family that wraps git-worktree operations for skill development. A worktree is a linked working directory tied to a feature branch. Within a worktree, skills can be developed against a real project target, then collected (pulled to source + symlinked back), and finally merged to the main branch.

The core interaction loop:

1. **Create**: `skillshare worktree create <branch>` — creates a git worktree on a new feature branch, with its own `.skillshare/config.yaml`
2. **Target**: `skillshare worktree target add <name> <path>` — registers a development project as a target
3. **Sync**: `skillshare sync -p` (from worktree) — symlinks existing skills into the project
4. **Develop**: create/edit skills directly in the project's agent directory (e.g., `.claude/skills/`)
5. **Collect**: `skillshare worktree collect <target>` — discovers local skills in the target, copies them to the worktree's `.skillshare/skills/`, and *replaces* the target's local copy with a symlink back to source
6. **Merge**: `skillshare worktree merge <branch>` — merges the feature branch back to the main branch, cleans up the worktree

For release/distribution, a separate target with `mode: copy` is added to the main branch's config pointing at the release branch directory — no mode-switching on existing targets.

## User Stories

### Worktree lifecycle

1. As a skill developer, I want to run `skillshare worktree create feat/my-skill` from my skill-repo root, so that a new git worktree is created on a feature branch with its own config ready for development.

2. As a skill developer, I want `skillshare worktree list` to show all active worktrees with their branch names, target count, and last-modified dates, so that I can see what's in flight.

3. As a skill developer, I want `skillshare worktree select <branch>` to print the worktree path, so that I can quickly switch my terminal to that worktree.

4. As a skill developer, I want `skillshare worktree remove <branch>` to safely delete a worktree after confirming no unmerged changes, so that stale worktrees don't accumulate.

### Target management in worktrees

5. As a skill developer, I want `skillshare worktree target add claude ~/DEV/my-project/.claude/skills` to register a development project as a target in the worktree's config, so that sync knows where to symlink skills.

6. As a skill developer, I want `skillshare worktree target list` to show all targets registered in the current worktree, so that I can verify my development setup.

7. As a skill developer, I want `skillshare worktree target remove <name>` to unregister a target from the worktree, so that I can clean up after a project is no longer relevant.

### Skill development (collect with symlink-back)

8. As a skill developer, I want to create a new skill directory directly in my project's agent directory (e.g., `.claude/skills/my-new-skill/SKILL.md`), edit it there with my AI coding agent, and have the edits persist through the symlink once collected, so that I can develop skills in their real usage context.

9. As a skill developer, I want `skillshare worktree collect claude` to find local (non-symlink) skills in the target, copy each one to the worktree's `.skillshare/skills/` with its flat name, then *replace the target's local directory with a symlink* pointing to the new source location, so that the skill is both promoted to managed and stays editable from the project.

10. As a skill developer, I want `skillshare worktree collect claude --dry-run` to preview which skills would be collected without making changes, so that I can verify before acting.

11. As a skill developer, I want `skillshare worktree collect claude --name <override>` to specify a custom flat name for the collected skill when the auto-detected name is wrong, so that I can follow naming conventions.

### Safety and guardrails

12. As a skill developer, I want `skillshare collect` (the bare command without the worktree subcommand) to refuse to run outside a worktree context, displaying an error that directs me to use `skillshare worktree create` first or `skillshare install` for remote sources, so that I never accidentally bypass the feature-branch workflow.

13. As a skill developer, I want `skillshare worktree collect` to refuse to overwrite a skill that already exists in the worktree's source without `--force`, so that I don't accidentally clobber work in progress.

14. As a skill developer, I want `skillshare worktree merge` to check that all collected skills have been committed and that the target symlinks are valid before allowing the merge, so that broken references don't leak into the main branch.

15. As a skill developer, I want `skillshare worktree merge` to warn if the feature branch has uncommitted changes and offer to abort, so that I don't lose work.

16. As a skill developer, I want `skillshare worktree merge` to refuse to merge if the worktree has uncommitted changes, unless `--force` is passed, so that I don't lose work by default.

### Release workflow

17. As a skill maintainer, I want to add a separate target to the main branch's config with `mode: copy` pointing at my project's release-branch agent directory, then run `skillshare sync -p`, so that the release branch contains actual skill files (not symlinks) that downstream users can consume without access to my skill-repo.

### General

18. As a skill developer running from a worktree, I want `skillshare sync -p` to work exactly as it does from the main branch (discovering skills from `.skillshare/skills/` and symlinking them to configured targets), so that existing sync behavior is unchanged and predictable.

19. As a user who does NOT use the worktree workflow, I want the existing `skillshare` commands (sync, install, update, collect, push, pull) to continue working unchanged, so that the fork does not break the standard distribution flow.

## Implementation Decisions

### Fork scope: minimal surgical changes

The fork targets the smallest possible diff from upstream skillshare. No core abstractions (sync modes, config schema, discovery walk) are redesigned. New functionality is added in a new `worktree` subcommand tree and a small enhancement to the collect path.

### New module: `internal/worktree/`

A new package encapsulating git-worktree operations. It wraps `git worktree` commands with structured outputs and safety checks.

```
// Create creates a new git worktree on a feature branch.
Create(branch string, repoPath string) (*Worktree, error)

// List returns all worktrees for the repo.
List(repoPath string) ([]Worktree, error)

// Remove deletes a worktree, with a safety check for unmerged changes.
Remove(branch string, force bool) error

// Worktree holds metadata for a single worktree.
type Worktree struct {
    Branch    string
    Path      string
    Commitish string
}
```

### Enhanced collect: symlink-back

The `internal/sync/pull.go` `PullSkill` function gains an optional symlink-back step. After copying a local skill from target to source, if a new `PullOptions.LinkBack` field is true, the target's local directory is removed and replaced with a symlink to the newly created source directory.

This respects the existing relative-symlink logic (`shouldUseRelative`) already used by `SyncTargetMerge`.

### Worktree config

A worktree is a standard git worktree with a `.skillshare/config.yaml` in it. No new config format. The worktree's config is initialized with sensible defaults (merge mode, single development target). The `project_runtime.go` sourcePath remains `<root>/.skillshare/skills/` — no change needed because the worktree IS the project root.

### Collect gate: worktree-only

The existing `cmdCollect` checks whether the current directory is inside a git worktree (by inspecting `.git` — in a worktree it's a file containing the path to the main repo). If not in a worktree, `skillshare collect` prints an error and exits. The new `skillshare worktree collect` path bypasses this gate.

### Worktree merge

`skillshare worktree merge <branch>`:
1. Verifies worktree exists and has no dirty git state (unless `--force`)
2. Checks out the main branch in the main repo
3. Runs `git merge <branch>`
4. On success, removes the worktree
5. Prints next steps (push, sync to distribute)

### Release target pattern

No code changes. The user adds a separate target entry in the main branch's config:

```yaml
targets:
  - name: claude
    skills:
      path: ~/DEV/project/.claude/skills
  - name: claude-release
    skills:
      path: ~/DEV/project-release/.claude/skills
      mode: copy
```

### Skill naming convention

Skills created during development use single-level flat names (e.g., `my-feature-skill/`) directly in `.skillshare/skills/`. The existing flat-naming transformation (`/` → `__`) only applies to nested paths from tracked repos — it is not triggered for manually created single-level skill directories.

### Command tree

```
skillshare worktree
├── list                          # List all worktrees
├── create <branch>               # Create worktree + feature branch
├── select <branch>               # Print worktree path
├── remove <branch> [--force]     # Remove worktree
├── target
│   ├── add <name> <path>         # Add development target
│   ├── list                      # List worktree targets
│   └── remove <name>             # Remove target
├── collect <target> [--dry-run] [--force] [--name]  # Collect local skills
└── merge <branch> [--force]      # Merge feature branch to main
```

## Testing Decisions

### What makes a good test

Tests verify externally observable behavior: given a set of files on disk and a command invocation, the resulting filesystem state and stdout/stderr match expectations. Tests use the existing `internal/testutil/` runner pattern and create real directories and symlinks in temporary paths. No mocking of the filesystem.

### Modules to test

- **`internal/worktree/`** — unit tests for Create, List, Remove using a real git repo in a temp directory
- **`internal/sync/pull.go` (symlink-back)** — extend existing pull tests to cover the LinkBack option: verify the target local directory is replaced with a symlink pointing to the correct source path
- **`cmd/skillshare/worktree_*.go`** — integration tests using the existing test runner pattern (see `ai_docs/tests/` for prior art), covering the full worktree lifecycle: create → target add → sync → create local skill → collect → verify symlink → merge

### Prior art

The repo has extensive integration test runbooks in `ai_docs/tests/` covering sync, install, collect, symlink behavior, and target naming. The test utilities in `internal/testutil/` provide git repo setup helpers, temp directory management, and CLI invocation.

## Out of Scope

- **Remote worktree operations**: Worktrees are local-only. No pushing/pulling worktree state to remotes.
- **Multi-developer worktree collaboration**: A worktree belongs to a single developer on a single machine.
- **Changing the global-mode workflow**: The fork only adds worktree support to project mode (`-p`). Global-mode `~/.config/skillshare/` behavior is untouched.
- **GUI/TUI for worktree management**: CLI only.
- **Rebase-based merge**: Only `git merge` is supported for the merge step.
- **Automated sync on merge**: After merge, the user manually runs sync to distribute updated skills.
- **Backward compatibility with upstream skillshare configs**: The fork's worktree commands and config additions may diverge from upstream; no guarantee of config compatibility.

## Further Notes

- The user's skill-repo is expected to follow the centralized-skills-repo recipe: a standalone git repo with `.skillshare/skills/` containing skill directories, and `.skillshare/config.yaml` tracking targets.
- The worktree's `.skillshare/config.yaml` should be gitignored (matching the `--config local` pattern) so that each developer's target paths don't leak into the shared repo.
- CI for the fork should run the existing test suite plus new worktree tests. Worktree tests require a real git repo and may need `git config user.email/user.name` set in the test environment.
- The `collect` gate (worktree-only) is a policy decision encoded in the CLI, not a technical limitation. Power users who understand the risks can still copy skills manually.
