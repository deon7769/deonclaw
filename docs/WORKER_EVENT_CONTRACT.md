# Worker Event Contract

This document records the current worker contract used by DeonClaw runners.

Workers are external command adapters. They do not own task policy, memory policy, workspace lifecycle, persistence, or approvals. A worker receives a scoped RunSpec, invokes its external tool, and returns a RunResult with events, artifacts, stderr, and optional metadata.

## Core Types

### RunSpec

RunSpec is the input passed from the runner to a worker:

~~~go
type RunSpec struct {
    Task      *tasks.Task
    Workspace string
    Prompt    string
}
~~~

- Task is the validated task definition.
- Workspace is the isolated workspace path prepared by the runner.
- Prompt is optional. When a context pack is configured, the shared runner builds a prompt containing the task goal and context pack.

### WorkerEvent

WorkerEvent is the normalized event envelope stored in run artifacts and SQLite:

~~~go
type WorkerEvent struct {
    Type      string
    Worker    string
    Command   []string
    Workspace string
    Sandbox   string
    Payload   json.RawMessage
}
~~~

- Type should identify the event kind. For JSON stdout lines that include a type field, workers may use that value.
- Worker is the worker adapter name, such as codex or opencode.
- Command is the external command shape used by the worker when relevant.
- Workspace is the workspace used by the command.
- Sandbox is the effective sandbox or policy mode when relevant.
- Payload must be valid JSON when present.

### RunResult

RunResult is the worker output returned to the shared runner:

~~~go
type RunResult struct {
    Worker    string
    Command   []string
    Workspace string
    Sandbox   string
    Events    []WorkerEvent
    Artifacts []artifacts.Artifact
    Stderr    string
    Metadata  map[string]string
}
~~~

- Events are normalized worker events.
- Artifacts are worker-supplied artifacts. The shared runner writes its own standard artifacts around them.
- Stderr is the raw stderr string when available.
- Metadata carries worker-specific facts used by summaries and future diagnostics.

## Shared Runner Artifacts

The shared runner writes the standard run artifact set:

- stdout.jsonl or stdout.log
- stderr.log
- events.jsonl
- summary.md
- diff.patch
- changed-files.json
- validation.log
- validation.json
- execution-trace.json
- artifact-manifest.json
- context-pack.md, when --domains is used
- memory-proposal-lint.json, when --memory-policy produces a lint artifact

Worker artifacts with names owned by the CLI are not duplicated. Additional worker artifacts are preserved with unique names.

SQLite stores artifact metadata only. The filesystem remains the artifact body store.

## Execution Trace

Every worker run that reaches artifact writing includes `execution-trace.json`.

The trace is an audit artifact for the runner lifecycle. It records:

- run_id, task_id, worker, and final status
- model_profile, provider, model, and model_arg when present
- command_display with the prompt masked as `<prompt>`
- prompt_sha256 instead of raw prompt text
- context_pack_sha256 when `--domains` built a context pack
- memory_policy_sha256 when `--memory-policy` was configured and readable
- env_requirements by name, requirement, and state only
- started_at, finished_at, and duration_ms
- stdout_format, parsed_events, and parse_warnings
- validation_status, policy_status, changed_paths_count, cleanup_action, and cleanup_reason
- timeline entries for task load/validation, profile and env checks, context pack build, workspace preparation, worker start/finish, validation, diff, path policy, artifact writing, and workspace cleanup

The trace must not contain raw prompts or environment variable values. Environment variable names can appear because they are part of the requirement contract; values must not.

## Runs Report

`deonctl runs report --store <path>` reads persisted run rows from SQLite and uses `execution-trace.json` when available to aggregate duration, parsed events, parse warnings, validation status, changed path counts, and model profile.

Runs that predate `execution-trace.json` remain reportable as `unknown/legacy`. Legacy rows still contribute run status and worker counts, but trace-derived metrics stay unknown.

`deonctl runs report --store <path> --by model_profile --worker opencode --status succeeded --since 2026-06-03` adds model profile grouping and optional read-only filters. Runs without `execution-trace.json` are grouped as `unknown/legacy`; runs with a readable trace but an empty `model_profile` are grouped as `no_model_profile`.

## Stdout And Stderr

Workers must preserve raw stdout as a run artifact and preserve stderr as stderr.log.

For OpenCode:

- JSONL stdout is written as stdout.jsonl.
- Mixed, text, and empty stdout are written as stdout.log.
- stderr.log is always returned as a worker artifact, even when stderr is empty.
- Invalid JSON in stdout does not fail the run by itself.
- Large event payloads may be truncated in events, but the raw stdout artifact remains complete.

For Codex:

- Codex is expected to emit JSONL from codex exec --json.
- Invalid Codex JSONL remains a parse error in the Codex adapter path.
- Raw stdout and stderr are still preserved as artifacts by the shared runner.

## OpenCode Stdout Formats

OpenCode classifies stdout into one of four formats:

| Format | Meaning | Raw artifact | Event behavior |
| --- | --- | --- | --- |
| jsonl | Every non-empty stdout line is valid JSON | stdout.jsonl | Each JSON line becomes a WorkerEvent |
| mixed | Stdout contains at least one valid JSON line and at least one non-JSON line | stdout.log | Valid JSON lines become events; a parse warning event is added |
| text | Stdout has content but no valid JSON event lines | stdout.log | A worker.stdout.log event is generated |
| empty | Stdout has no non-empty lines | stdout.log | No stdout event is generated |

OpenCode JSON event lines use the embedded type field as the normalized event type when present. Otherwise DeonClaw uses worker.stdout.json.

Mixed stdout produces a worker.stdout.parse_warning event. This is diagnostic evidence, not a run failure.

## Payload Limits

OpenCode event payloads are bounded.

If a JSON stdout line is too large for an event payload, the event payload is replaced with a safe truncated payload of type worker.stdout.json.truncated.

If text stdout is too large, the worker.stdout.log event stores a truncated preview with:

- truncated: true
- original_bytes

The raw stdout artifact is not truncated by this event payload limit.

## OpenCode Metadata

OpenCode adds worker metadata to RunResult.Metadata:

- opencode.stdout_format
- opencode.parsed_events
- opencode.parse_warnings
- model_profile, when the task selected one
- provider, when the selected profile defines it
- model, when the selected profile defines it
- model_arg, when the selected profile defines it

The shared summary renders these as:

- Model profile: profile name
- Provider: provider id
- Model: model id
- Model arg: OpenCode --model argument
- OpenCode stdout format: jsonl|mixed|text|empty
- OpenCode parsed events: N
- OpenCode parse warnings: N

When `model_arg` is set for an OpenCode model profile, the command is:

~~~bash
opencode run --dir <workspace> --format json --model <model_arg> "<prompt>"
~~~

The prompt remains masked as `<prompt>` in dry-run output, RunResult.Command, summaries, and artifacts.

## Run Failure Rules

Worker command failure still fails the run.

OpenCode stdout parse problems do not fail the run by themselves. They are represented as artifacts, events, metadata, and summary lines.

Policy failure, validation failure, workspace preparation failure, persistence failure, and shared runner errors remain runner-level failures.
