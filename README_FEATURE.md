# Worktree-Based Skill Development Workflow

> Fork of [skillshare](https://github.com/runkids/skillshare) that adds a first-class Git worktree workflow for developing skills inside real project targets and merging them back into a shared skills repository.

## Overview

Upstream `skillshare` is optimized for one-way distribution from a central skill source to one or more targets. That works well for publishing, but it leaves a gap for day-to-day development:

- a developer wants to create or iterate on a skill inside a real project
- the project should use the in-progress skill immediately
- the resulting skill should be promoted back into the shared skills repository through a feature branch

This branch adds a `skillshare worktree` command family to support that loop directly:

1. create a feature worktree
2. attach one or more development targets to that worktree
3. sync existing skills into the project
4. build or edit a skill inside the project
5. collect the skill back into the worktree source and link the project to it
6. merge the worktree branch back to the main repo with merge-time guardrails

## Repository Shape

This workflow assumes a dedicated skills repository using project-mode layout:

```text
skills-repo/
  .skillshare/
    config.yaml
    skills/
      my-skill/
        SKILL.md
```

The main repo owns the shared source of truth. Each worktree gets its own local `.skillshare/config.yaml` for development targets.

## Build

```bash
cd <fork-repo>
go build -o ./bin/skillshare ./cmd/skillshare
sudo cp ./bin/skillshare /usr/local/bin/skillshare
```

## Standard Distribution Flow

The normal centralized distribution flow remains unchanged:

```bash
# in the main skills repo
skillshare sync -p
skillshare push
skillshare pull
```

If you want a self-contained release target instead of symlinks, configure that target with `mode: copy`.

```yaml
targets:
  - name: claude
    skills:
      path: ~/DEV/project/.claude/skills
      mode: merge
  - name: claude-release
    skills:
      path: ~/DEV/project-release/.claude/skills
      mode: copy
```

## Worktree Development Flow

### Main repo setup

Create the project first and register it in the main repo if you want post-merge repair syncs to be meaningful.

```yaml
# skills-repo/.skillshare/config.yaml
targets:
  - name: claude
    skills:
      path: ~/DEV/my-project/.claude/skills
      mode: merge
```

### Core loop

```bash
# 1. create a feature worktree
skillshare worktree create feat/my-skill

# 2. enter the worktree
cd "$(skillshare worktree select feat/my-skill)"

# 3. add the development target to the worktree-local config
skillshare worktree target add claude ~/DEV/my-project/.claude/skills

# 4. sync existing repo skills into the project
skillshare sync -p

# 5. create or edit a skill inside the project target
#    e.g. ~/DEV/my-project/.claude/skills/my-skill/SKILL.md

# 6. collect local project skills into the worktree source
skillshare worktree collect claude

# 7. commit from the worktree
git add -A
git commit -m "add my-skill"

# 8. merge back to the main repo
skillshare worktree merge feat/my-skill

# 9. push and distribute from the main repo
git push
skillshare sync -p
```

## How the Worktree Is Initialized

`skillshare worktree create <branch>` creates a Git worktree on a new branch and scaffolds a worktree-local `.skillshare/` directory:

```text
.skillshare/
  .gitignore
  config.yaml
  agents/
  skills/
```

The generated `.skillshare/.gitignore` contains:

```gitignore
config.yaml
```

The generated `.skillshare/config.yaml` is minimal and target-free:

```yaml
audit:
  block_threshold: CRITICAL
```

Targets are intentionally not copied from the main repo. Each worktree declares its own development targets locally.

## Command Reference

### `skillshare worktree create <branch>`

Creates a new feature worktree and initializes worktree-local Skillshare project config.

```bash
skillshare worktree create feat/my-skill
skillshare worktree create feat/my-skill --force
```

- Worktree path: sibling directory named `<repo>_<branch-slug>`
- Branch slugs replace `/` with `_` in the directory name
- `--force` allows replacing an existing worktree directory during setup

### `skillshare worktree list`

Lists worktrees for the current repository.

```bash
skillshare worktree list
```

### `skillshare worktree select <branch>`

Prints the worktree path for a branch, intended for shell composition.

```bash
cd "$(skillshare worktree select feat/my-skill)"
```

### `skillshare worktree remove <branch>`

Removes a worktree and prunes the branch.

```bash
skillshare worktree remove feat/my-skill
skillshare worktree remove feat/my-skill --force
```

Without `--force`, removal is blocked if the worktree has uncommitted changes.

### `skillshare worktree target add <name> <path>`

Adds a target to the worktree-local config.

```bash
skillshare worktree target add claude ~/DEV/my-project/.claude/skills
```

- Must be run inside a worktree
- `~` is expanded
- targets default to `mode: merge`

### `skillshare worktree target list`

Lists targets configured in the current worktree.

```bash
skillshare worktree target list
```

### `skillshare worktree target remove <name>`

Removes a target from the current worktree config.

```bash
skillshare worktree target remove claude
```

### `skillshare worktree collect <target>`

Collects local project skills from the target into the worktree source at `.skillshare/skills/`, then replaces the target directories with symlinks back into the worktree source.

```bash
skillshare worktree collect claude
skillshare worktree collect claude --dry-run
skillshare worktree collect claude --force
skillshare worktree collect claude --name my-skill
```

Behavior:

- must be run inside a worktree
- only local non-symlink skill directories are collected
- `--dry-run` previews the collect set
- `--force` skips confirmation and allows overwrite of existing source skills
- `--name` can rename the collected skill when exactly one local skill is found

### `skillshare worktree merge <branch>`

Merges the feature branch back to the main repo and removes the worktree.

```bash
skillshare worktree merge feat/my-skill
skillshare worktree merge feat/my-skill --force
```

Without `--force`, merge is blocked if:

- the worktree has uncommitted changes
- configured target directories contain broken symlinks

If a Git merge conflict occurs, the merge is aborted and the worktree is preserved for manual resolution.

## Merge-Time Sync Guardrails

`skillshare worktree merge` includes non-destructive project-sync preview checks around the Git merge.

### Before merge

If the main repo already has `.skillshare/config.yaml`, the command runs the equivalent of:

```bash
skillshare sync -p --dry-run --json
```

It uses that preview to warn when:

- a target already has unmanaged local content
- a target is already out of sync before the merge

A clean preview does not block the merge.

### After merge

After a successful merge and worktree removal, the command runs a second project sync dry-run preview from the main repo and compares the before/after target states.

If only one target newly needs repair, the command prints a focused recommendation to run:

```bash
skillshare sync -p
```

If multiple targets changed, or if a target was already dirty before merge, the command prints a broader warning and recommends inspecting drift before a full sync.

These guardrails are advisory only. `worktree merge` never performs a real project sync automatically.

## Safety Model

- bare `skillshare collect` is blocked outside the intended worktree flow
- `skillshare worktree collect` links collected skills back into the target immediately
- `skillshare worktree merge` performs safety checks before merging unless `--force` is used
- worktree-local targets stay local to the worktree config and are not promoted into the main repo automatically

## Practical Notes

- To keep a target healthy after merge, configure that same target in the main repo as well as in the worktree.
- If the main repo does not have project config, merge preview checks are skipped.
- `worktree merge` detects `main` first and falls back to `master`.
- The source path for collected skills is `.skillshare/skills/`, so the main repo should follow that layout for the smoothest workflow.

