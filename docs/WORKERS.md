# Workers

## Worker philosophy

Workers are external harnesses controlled by DeonClaw.

DeonClaw does not need to replace them.

DeonClaw should:

- select the worker
- build scoped context
- create the workspace
- invoke the worker
- capture events
- validate policy
- store artifacts

## Initial worker order

### 1. Codex CLI

Use first because it supports non-interactive execution and JSONL event output.

Initial mode:

```bash
codex exec --json --sandbox workspace-write --cd <workspace> -
```

### 2. OpenCode

Use for provider-agnostic execution and Z.ai/GLM experiments.

Initial mode:

```bash
opencode run --dir <workspace> "<task>"
```

### 3. Claude Code

Use later for complex coding and refactoring.

### 4. OpenClaw adapter

Use later for gateway/canal/legacy tasks.

## Worker interface

```go
type Worker interface {
    Name() string
    Capabilities() WorkerCapabilities
    Run(ctx context.Context, spec RunSpec) (<-chan WorkerEvent, error)
    Cancel(ctx context.Context, runID string) error
}
```

## Worker event types

Internal event types:

- run.started
- run.completed
- run.failed
- worker.stdout
- worker.stderr
- worker.message
- worker.command.started
- worker.command.completed
- worker.file.changed
- worker.tool.called
- policy.failed
- artifact.created

## Auth strategy

MVP auth strategy is harness-managed.

DeonClaw should not copy OAuth tokens.

DeonClaw should not persist `~/.codex/auth.json`.

DeonClaw may pass environment variables only when explicitly configured.
