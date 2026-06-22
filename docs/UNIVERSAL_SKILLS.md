# Universal Skill Registry

## Purpose

This document defines the DeonClaw skill system. The goal is to install and use skills from Codex, OpenCode, OpenClaw, Hermes, local folders, Git repositories, and future registries without binding DeonClaw to any single agent runtime.

The canonical portable format is AgentSkills-compatible `SKILL.md`. DeonClaw may add sidecar metadata for provenance, permissions, and runtime policy, but the skill body should remain usable by other agents whenever possible.

## Design goals

- Support `SKILL.md` as the portable skill unit.
- Install skills from local paths, Git repositories, archives, and future registries.
- Preserve provenance, version, source ref, and content hashes.
- Support per-agent and per-session skill allowlists.
- Build immutable session snapshots for Codex, OpenCode, and future workers.
- Keep third-party skills untrusted until scanned, reviewed, and approved.
- Allow agent-generated skill proposals, but keep promotion governed.
- Support rollback and deterministic rehydration.

## Non-goals

- No silent auto-update of installed skills.
- No direct skill writes from workers to active skill roots.
- No execution of skill scripts during install.
- No registry hosting in the first implementation.
- No proprietary required format that prevents use by Codex/OpenCode/OpenClaw/Hermes.

## Portable skill layout

Recommended directory:

```text
my-skill/
  SKILL.md
  scripts/
  references/
  assets/
  examples/
  deonclaw.skill.yaml
```

Only `SKILL.md` is required. Extra folders are optional.

Minimum `SKILL.md`:

```markdown
---
name: github-review
description: Review GitHub pull requests with source-backed comments and checklist output
license: MIT
compatibility: agentskills
---

## When to use

Use this when reviewing a pull request or summarizing changed files.

## Procedure

1. Inspect metadata.
2. Inspect changed files.
3. Identify correctness, safety, and test gaps.
4. Produce actionable comments.
```

## DeonClaw sidecar

`deonclaw.skill.yaml` is optional and records DeonClaw-specific runtime metadata:

```yaml
deonclaw:
  source:
    type: git
    repository: owner/repo
    ref: v1.2.0
    resolved_commit: abc123
    sha256: "..."
  trust:
    level: untrusted
    reviewed_by: null
    reviewed_at: null
  permissions:
    shell: ask
    network: deny
    browser: allow
    filesystem: workspace-read
    secrets: deny
  compatible_workers:
    - codex
    - opencode
  lifecycle:
    state: active
    channel: stable
    auto_update: false
```

Sidecars must not be required by other runtimes. They are DeonClaw policy overlays.

## Skill sources

Supported source types:

| Source | Example | Notes |
|---|---|---|
| local | `./skills/foo` | Reads staged local directory |
| git | `git:owner/repo@ref` | Ref must resolve to commit; hash recorded |
| archive | `./foo.zip` | Disabled by default until archive policy exists |
| managed | `~/.deonclaw/skills/foo` | Installed local registry |
| workspace | `.agents/skills/foo` | Project/session scoped |
| imported-openclaw | OpenClaw skill roots | Migration path |
| imported-hermes | Hermes AgentSkills roots | Migration path |
| codex-home | `$CODEX_HOME/skills` | Imported, not directly owned |
| opencode | `.opencode/skills` | Imported or mirrored |

## Discovery and precedence

DeonClaw should use explicit installation rather than silently walking many roots at runtime. Discovery is allowed for inspect/plan commands.

Precedence for session materialization:

1. session override snapshot;
2. agent allowlist pinned revisions;
3. workspace installed skills;
4. managed shared skills;
5. imported external skill revisions;
6. bundled examples.

When duplicate skill names exist, the highest precedence source wins only after provenance and policy checks pass.

## Skill lifecycle

States:

```text
discovered
staged
verified
pending_approval
active
deprecated
archived
rejected
quarantined
```

State transitions:

```text
discovered -> staged -> verified -> pending_approval -> active
active -> deprecated -> archived
staged/verified -> rejected
any unsafe state -> quarantined
```

No unreviewed third-party skill should become active automatically.

## Installation pipeline

```text
source reference
  ↓
stage into temp directory
  ↓
format detection
  ↓
SKILL.md parse and schema validate
  ↓
path/symlink containment
  ↓
static security scan
  ↓
capability extraction
  ↓
compatibility report
  ↓
approval request
  ↓
install into managed registry
  ↓
record lockfile and provenance
  ↓
make available to agent/session snapshots
```

## CLI commands

Initial commands:

```bash
deonctl skills inspect ./path/to/skill
deonctl skills inspect git:owner/repo@ref
deonctl skills import ./path/to/skill --output <skill-import-plan.json>
deonctl skills install ./path/to/skill --as <name>
deonctl skills install git:owner/repo@ref --as <name>
deonctl skills verify <name>
deonctl skills list
deonctl skills show <name>
deonctl skills enable <name> --agent <agent-id>
deonctl skills disable <name> --agent <agent-id>
deonctl skills snapshot --agent <agent-id> --session <session-id> --output <snapshot.json>
deonctl skills materialize --snapshot <snapshot.json> --workspace <path>
deonctl skills update-plan <name>
deonctl skills update <name> --confirm-sha256 <sha256>
deonctl skills rollback <name> --to-revision <revision>
```

## Registry layout

Suggested managed layout:

```text
~/.deonclaw/skills/
  registry.json
  lock.json
  skills/
    github-review/
      revisions/
        2026-06-22T10-00-00Z-abc123/
          SKILL.md
          deonclaw.skill.yaml
          scripts/
          references/
      current -> revisions/...
```

Workspace materialization:

```text
<workspace>/.agents/skills/<name>/SKILL.md
<workspace>/.agents/skills/<name>/scripts/...
```

Session materialization should be disposable and reproducible from `skills snapshot`.

## Session snapshot

A session receives a frozen skill snapshot. Updates during the session should not mutate the active snapshot.

```json
{
  "session_id": "ses_...",
  "agent_id": "backend-engineer",
  "created_at": "...",
  "skills": [
    {
      "name": "github-review",
      "revision": "2026-06-22T10-00-00Z-abc123",
      "sha256": "...",
      "source": "managed",
      "materialized_path": ".agents/skills/github-review/SKILL.md",
      "permissions": {
        "shell": "ask",
        "network": "deny"
      }
    }
  ]
}
```

## Skill loading policy

Policy fields:

```yaml
skill_policy:
  default_visibility: deny
  allow_sources:
    - managed
    - workspace
  deny_sources:
    - archive
  require_approval_for:
    - git
    - archive
    - skill_patch
  permissions:
    network: deny
    shell: ask
    secrets: deny
  agent_allowlists:
    backend-engineer:
      - github-review
      - go-testing
    reviewer:
      - pr-review
```

## Compatibility adapters

### Codex

Materialize AgentSkills-compatible directories to `.agents/skills/<name>/SKILL.md` or the Codex-supported skill root. Keep scripts and assets inside the skill directory. Do not copy unapproved skills.

### OpenCode

Materialize to `.agents/skills/<name>/SKILL.md` by default. OpenCode can discover `.opencode/skills`, `.claude/skills`, and `.agents/skills`, but DeonClaw should prefer `.agents/skills` for portability.

### OpenClaw

Inventory OpenClaw skills from workspace, `.agents/skills`, managed `~/.openclaw/skills`, bundled/plugin roots, and ClawHub metadata. Import into DeonClaw with provenance and do not enable until the migration plan is approved.

### Hermes

Import AgentSkills-compatible directories and preserve scripts/references. Hermes memory/skill write approval semantics should map into DeonClaw proposals rather than direct application.

## Security scanning

Initial scan checks:

- `SKILL.md` present;
- frontmatter parseable;
- skill name safe;
- no path traversal in references;
- symlink containment;
- scripts listed but not executed;
- suspicious secret exfiltration patterns;
- hidden Unicode/invisible control characters;
- network/tool permission declarations;
- large binary files flagged;
- license metadata captured;
- origin and resolved commit recorded.

Untrusted skills must fail closed if scanning cannot complete.

## Agent-generated skill proposals

Agents may propose skills through the insight loop, but they do not write active skills directly.

Flow:

```text
InsightReport
  ↓
LearningProposal(type=skill_create or skill_patch)
  ↓
SkillProposal artifact
  ↓
review / diff / security scan
  ↓
approval
  ↓
install as new revision
  ↓
next session snapshot may include it
```

Skill proposal fields:

```json
{
  "proposal_id": "lp_...",
  "skill_name": "go-cli-handlers",
  "proposal_type": "skill_patch",
  "target_revision": "...",
  "evidence_bundle_sha256": "...",
  "summary": "Require shared CLI path validation helper.",
  "patch_path": "artifacts/skill.patch",
  "validation_plan": ["go test ./internal/retrievalcontext/..."],
  "approval_required": true
}
```

## Tests

Minimum tests:

- parse valid `SKILL.md`;
- reject missing `SKILL.md`;
- reject invalid name;
- reject path traversal and symlink escapes;
- inspect local skill;
- inspect git source plan without executing scripts;
- install with provenance and lockfile;
- update-plan detects revision diff;
- rollback restores prior revision;
- per-agent allowlist hides denied skill;
- session snapshot is immutable;
- materialization creates `.agents/skills/<name>/SKILL.md`;
- untrusted script permissions default to ask/deny;
- skill proposal requires approval before install.

## Implementation packages

Suggested packages:

```text
internal/skills/
  contract.go
  parser.go
  registry.go
  installer.go
  provenance.go
  scanner.go
  policy.go
  snapshot.go
  materialize.go
  proposals.go
  adapters/
    codex.go
    opencode.go
    openclaw.go
    hermes.go
```

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` with implemented skill registry features;
- `docs/EPIC_23_ROADMAP.md` if scope changes;
- `docs/INSIGHT_LEARNING_LOOP.md` if skill proposals change;
- `docs/OPENCLAW_MIGRATION.md` for skill import behavior;
- examples under `configs/examples/skills/`;
- `AGENTS.md` if repository workflow for skills changes.