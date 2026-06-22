# Rich Runtime Actions and Worker Adapters

## Purpose

This document defines how DeonClaw will expose stable, runtime-neutral actions for Git, browser, review, and worker sessions. The goal is to gain useful Codex/OpenCode-style ergonomics without coupling the DeonClaw control plane to any one product UI or CLI implementation.

The action layer turns capabilities such as stage, revert, commit, browser preview, annotation, cancel, resume, and review into governed, auditable contracts.

## Design goals

- Treat Codex, OpenCode, browser tooling, Git, and future runtimes as adapters behind stable actions.
- Provide action metadata that can be rendered as CLI commands now and UI buttons later.
- Support dry-run/plan/report for mutating actions.
- Enforce workspace, budget, approval, and skill policies before actions run.
- Capture action events and artifacts consistently.
- Support worker session management: start, stream, cancel, resume, summarize, hand off.
- Avoid product-specific coupling while preserving useful features from Codex App, OpenCode, and browser-enabled workflows.

## Non-goals

- No full GUI in this epic.
- No unrestricted Git push or PR creation.
- No browser automation with secrets by default.
- No direct provider dispatch bypassing the existing gates.
- No hidden action execution outside events and policy checks.

## Action model

```json
{
  "action_id": "git.stage.file",
  "label": "Stage file",
  "category": "git",
  "risk": "medium",
  "capability": "git_stage",
  "input_schema": {
    "path": "string"
  },
  "requires_approval": false,
  "dry_run_supported": true,
  "workers_supported": ["host-git"],
  "artifacts": ["git-action-report.json"]
}
```

Action lifecycle:

```text
discovered
planned
approved
running
succeeded
failed
cancelled
blocked
```

## Core action groups

### Git actions

```text
git.status
git.diff
git.diff.file
git.diff.hunk
git.stage.file
git.stage.hunk
git.unstage.file
git.unstage.hunk
git.revert.file
git.revert.hunk
git.commit
git.push
git.open_pr
git.branch.create
git.branch.switch
git.worktree.create
git.worktree.prune
```

Mutating Git actions require:

- workspace containment;
- dirty baseline awareness;
- action plan artifact;
- approval for destructive actions such as revert, push, or PR creation depending on policy;
- run/agent attribution.

### Browser actions

```text
browser.open
browser.capture
browser.inspect
browser.annotate
browser.click_dry_run
browser.form_fill_dry_run
browser.run_check
browser.close
```

Browser actions initially should be preview-oriented:

- no credential entry;
- no persistent cookies unless configured;
- no form submit without approval;
- screenshot/DOM artifacts with redaction policy;
- annotations reference coordinates/selectors and screenshots.

### Worker session actions

```text
worker.session.start
worker.session.stream
worker.session.cancel
worker.session.resume
worker.session.compact
worker.session.summarize
worker.session.hand_off
worker.session.snapshot
```

These wrap Codex/OpenCode sessions and future adapters.

### Review actions

```text
review.diff
review.file
review.hunk
review.comment
review.apply_suggestion
review.resolve
review.request_changes
```

Review actions should work over local worktrees first and GitHub PRs later.

### Generic operator actions

```text
action.list
action.show
action.plan
action.run
action.report
action.approve
action.cancel
```

## Git action contracts

Example `git.stage.file` plan:

```json
{
  "status": "ok",
  "action": "git.stage.file",
  "workspace_path": "...",
  "path": "cmd/deonctl/main.go",
  "path_allowed": true,
  "would_modify_index": true,
  "working_tree_modified": false,
  "approval_required": false,
  "action_allowed_now": true
}
```

Example `git.commit` plan:

```json
{
  "status": "ok",
  "action": "git.commit",
  "staged_files": ["docs/EPIC_23_ROADMAP.md"],
  "message": "docs: add Epic 23 roadmap",
  "approval_required": true,
  "would_create_commit": true,
  "push_allowed_now": false
}
```

## Browser action contracts

Example `browser.capture` result:

```json
{
  "status": "ok",
  "action": "browser.capture",
  "url": "http://localhost:3000",
  "screenshot_path": "artifacts/browser/capture.png",
  "dom_summary_path": "artifacts/browser/dom-summary.json",
  "network_call": true,
  "credential_entry": false,
  "form_submitted": false,
  "approval_required": false
}
```

Browser network is a real network action. It must be gated separately from provider/network dispatch and must be clear in policy.

## Adapter capabilities

Adapters declare capabilities rather than actions owning product-specific behavior.

```json
{
  "adapter": "codex-cli",
  "capabilities": [
    "worker.session.start",
    "worker.session.cancel",
    "worker.session.stream",
    "git.diff.read"
  ]
}
```

```json
{
  "adapter": "opencode",
  "capabilities": [
    "worker.session.start",
    "worker.session.cancel",
    "skills.snapshot.load",
    "tool.events.stream"
  ]
}
```

```json
{
  "adapter": "host-git",
  "capabilities": [
    "git.status",
    "git.diff",
    "git.stage.file",
    "git.commit"
  ]
}
```

## Policy model

```yaml
action_policy:
  defaults:
    git:
      read: allow
      stage: ask
      revert: ask
      commit: ask
      push: deny
      open_pr: ask
    browser:
      open: ask
      capture: allow
      submit: deny
    worker_session:
      start: allow
      cancel: ask
      resume: ask
  per_agent:
    reviewer:
      git:
        read: allow
        stage: deny
        commit: deny
```

## CLI commands

```bash
deonctl actions list --workspace <path>
deonctl actions show git.stage.file
deonctl actions plan git.stage.file --workspace <path> --path <file>
deonctl actions run git.stage.file --workspace <path> --path <file>
deonctl actions report --action-run <action-run-id>

deonctl git status --workspace <path>
deonctl git diff --workspace <path>
deonctl git stage --workspace <path> --path <file>
deonctl git commit --workspace <path> --message "..." --approval <approval.json>

deonctl browser capture --url <url> --output-dir artifacts/browser
deonctl browser annotate --capture <capture.json> --selector <selector> --comment "..."

deonctl worker session list --agent <agent-id>
deonctl worker session cancel <session-id>
deonctl worker session summarize <session-id>
```

## Store additions

```sql
action_definitions(id, category, capability, schema_json, risk, created_at)
action_runs(id, action_id, agent_id, run_id, workspace_path, status, input_json, result_json, created_at, finished_at)
action_approvals(id, action_run_id, approval_path, sha256, approved_at)
browser_captures(id, action_run_id, url, screenshot_path, dom_summary_path, created_at)
git_action_reports(id, action_run_id, workspace_path, report_path, sha256, created_at)
worker_sessions(id, agent_id, worker, status, workspace_path, metadata_json, created_at, updated_at)
```

## Event types

```text
action.planned
action.approved
action.started
action.completed
action.failed
action.blocked
git.action.completed
browser.capture.created
worker.session.started
worker.session.cancelled
worker.session.resumed
```

## Integration with skills

Skills can request actions by capability, not product-specific commands.

Example skill guidance:

```text
When reviewing a local diff, use `git.diff` and `review.diff` capabilities. Do not run `git push` unless an operator-approved action plan exists.
```

The skill registry can expose action capabilities as part of the session snapshot.

## Integration with learning

Action outcomes create evidence for insights:

- rejected revert action;
- repeated browser check failure;
- commit created after specific validation pattern;
- review comment that produced a fix;
- hunk revert after bad agent edit.

The insight loop can propose:

- better skill instructions;
- new eval cases;
- safer action defaults;
- budget/cadence changes;
- workspace policy changes.

## Codex/OpenCode adapter direction

### Codex

Track and adapt to useful Codex capabilities such as app/IDE workflows, Git review operations, browser previews, and SDK/App Server protocols. DeonClaw should not depend on Codex UI. It should expose equivalent contracts and use Codex as a worker/reviewer when helpful.

### OpenCode

Use OpenCode's skill and session capabilities through materialized `.agents/skills` snapshots and worker events. Prefer generic DeonClaw actions over OpenCode-specific commands in core state.

## Tests

Minimum tests:

- action definition validate;
- policy denies unsafe action;
- git status read-only succeeds;
- git stage plan requires allowed path;
- git commit requires approval;
- git push denied by default;
- browser capture plan emits network boundary;
- browser submit denied by default;
- action run records event and artifact;
- worker session cancel creates event;
- skill snapshot references allowed capabilities;
- insight evidence can include action refs.

## Implementation sequence

### 23.24 — Action registry and Git read actions

- action schema;
- action policy validate;
- Git status/diff actions;
- read-only e2e.

### 23.25 — Git mutating actions with approval

- stage/unstage/revert/commit plan/run/report;
- approval required by policy;
- action artifacts and events.

### 23.26 — Browser preview and annotation

- capture artifacts;
- selector/coordinate comments;
- no credential/form submit;
- policy gates.

### 23.27 — Worker session actions

- session list/start/cancel/summarize contracts;
- Codex/OpenCode adapter capability reports;
- learning integration.

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` status;
- `docs/EPIC_23_ROADMAP.md`;
- `docs/UNIVERSAL_SKILLS.md` for skill capability exposure;
- `docs/INSIGHT_LEARNING_LOOP.md` for action evidence;
- examples under `configs/examples/action-policy.yaml`;
- CLI usage documentation.