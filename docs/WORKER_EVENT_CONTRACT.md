# Worker Event Contract

This document records the current worker contract used by DeonClaw runners.

Workers are external command adapters. They do not own task policy, memory policy, workspace lifecycle, persistence, or approvals. A worker receives a scoped RunSpec, invokes its external tool, and returns a RunResult with events, artifacts, stderr, and optional metadata.

Task 20.x Docker runtime work keeps production worker execution conservative. Docker runtime support covers config validation, `docker-plan`, `runtime docker-exec`, optional Docker-backed validation commands, allowlisted env passthrough by name, a Docker worker execution scaffold for fake/test workers, and OpenCode Docker worker runtime marked `supported_experimental`. Codex still runs through the local worker adapter in this contract, and Codex Docker remains blocked.

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
    PromptDelivery string
    PromptPlaceholder string
    Payload   json.RawMessage
}
~~~

- Type should identify the event kind. For JSON stdout lines that include a type field, workers may use that value.
- Worker is the worker adapter name, such as codex or opencode.
- Command is the external command shape used by the worker when relevant.
- Workspace is the workspace used by the command.
- Sandbox is the effective sandbox or policy mode when relevant.
- PromptDelivery is optional and currently used by the Docker worker scaffold. Supported values are `stdin` and `arg_placeholder`; empty means `stdin`.
- PromptPlaceholder is the argument marker to replace when PromptDelivery is `arg_placeholder`; the default is `<prompt>`.
- Payload must be valid JSON when present.

For Docker worker execution, the prompt delivery contract is explicit:

- `stdin`: the runner passes the prompt through stdin. Fake/test workers use this path.
- `arg_placeholder`: the runner replaces the placeholder only in the process args used for execution. The stored command, summary and trace keep `<prompt>`.

OpenCode plans a prompt argument. The Docker runtime path uses `arg_placeholder` to pass the raw prompt only to the process args while preserving `<prompt>` in the recorded command, summary, and trace. This is supported experimental, not production hardening; a future wrapper may switch OpenCode prompt delivery to stdin.

For Docker worker runtime, RunSpec.Workspace passed to the worker dry-run is the container workspace path from `runtime.docker.workdir`, usually `/workspace`. The runner mounts the prepared host workspace to that target as `rw` and keeps memory mounts from `runtime.yaml` separate. This prevents workers from planning commands with host-only workspace paths.

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
- mcp-context.md, when task.mcp_context attachments are configured
- mcp-tool-call-proposal-lint.json, when a worker emits mcp-tool-call-proposal.json
- mcp-tool-call-preflight.json, when a worker proposal is checked with task.mcp_proposal_policy
- memory-proposal-lint.json, when --memory-policy produces a lint artifact

Worker artifacts with names owned by the CLI are not duplicated. Additional worker artifacts are preserved with unique names.

SQLite stores artifact metadata only. The filesystem remains the artifact body store.

## Execution Trace

Every worker run that reaches artifact writing includes `execution-trace.json`.

The trace is an audit artifact for the runner lifecycle. It records:

- run_id, task_id, worker, and final status
- model_profile, provider, model, and model_arg when present
- model_strategy and selected_model_profile when a real OpenCode run selects `model_strategy.preferred[0]`
- command_display with the prompt masked as `<prompt>`
- prompt_sha256 instead of raw prompt text
- context_pack_sha256 when `--domains` built a context pack
- mcp_context_sha256 when the task attached audited MCP context
- mcp_tool_proposal_status for worker-generated MCP tool call proposals
- mcp_tool_proposal_sha256 when a worker emitted `mcp-tool-call-proposal.json`
- memory_policy_sha256 when `--memory-policy` was configured and readable
- worker_runtime as `local` or `docker`
- validation_runtime as `local` or `docker`
- runtime_config_sha256 when `--runtime-config` was configured and readable
- env_requirements by name, requirement, and state only
- started_at, finished_at, and duration_ms
- stdout_format, parsed_events, and parse_warnings
- validation_status, policy_status, changed_paths_count, cleanup_action, and cleanup_reason
- timeline entries for task load/validation, profile and env checks, context pack build, workspace preparation, worker start/finish, validation, diff, path policy, artifact writing, and workspace cleanup

The trace must not contain raw prompts, raw runtime config content, or environment variable values. Environment variable names can appear because they are part of the requirement contract and Docker env passthrough allowlist; values must not.

Task `mcp_context.attachments` is passive context only. The shared runner validates referenced discovery/call artifacts before worker execution, writes `mcp-context.md`, appends a summarized `# MCP Context Attachments` section to the prompt, and records `mcp_context_sha256`. Workers do not receive permission to start MCP servers or call MCP tools, and the runner passes a sanitized task copy to workers without MCP attachment paths. Raw MCP transcripts and raw tool responses are not included in prompts by default.

Workers may emit `mcp-tool-call-proposal.json` as a suggestion artifact only. After the worker exits, the shared runner validates the proposal schema and `arguments_sha256`, then writes `mcp-tool-call-proposal-lint.json`. If task `mcp_proposal_policy` is configured, the runner loads the named MCP config, call policy, and optional runtime config and runs preflight through `internal/mcpapproval`, writing `mcp-tool-call-preflight.json`. The runner never starts an MCP server, never calls MCP tools, never creates approval artifacts, and never executes the proposal. The runner passes a sanitized task copy to workers without `mcp_proposal_policy`.

Workers must not emit MCP approval artifacts such as `mcp-tool-call-approval.json`. A worker-supplied approval artifact fails the run because workers cannot self-approve MCP calls. Summaries and traces record proposal status and hashes only; raw proposal arguments remain only in the worker proposal artifact.

Task `model_strategy` is controlled selection metadata in the current OpenCode contract. Dry-run resolves it, selects `preferred[0]` as `planned_model_profile`, and can show an OpenCode `--model` command when that planned profile defines `model_arg`. Real OpenCode runs select `preferred[0]` as `selected_model_profile`, record `model_strategy: selected`, reuse `model_profile` for run report grouping, add `--model <model_arg>` when defined, and validate env requirements from the selected profile before worker execution.

`model_strategy.fallback_policy` is schema and policy metadata only. It defines whether future fallback is enabled, the allowed attempt count, allowed retry reasons, and reasons that must never retry. The default is disabled. `policy_failed` and `memory_policy_failed` never trigger automatic fallback. Fallback profiles must resolve to the same worker, and fallback profile env requirements can be checked before any future fallback attempt. Current real runs do not switch workers, retry, or execute fallbacks, so no fallback attempt event is emitted yet.

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
- model_strategy=selected, when a real run selected from model_strategy
- selected_model_profile, when a real run selected from model_strategy
- provider, when the selected profile defines it
- model, when the selected profile defines it
- model_arg, when the selected profile defines it

OpenCode does not emit fallback metadata today because Task 18.17 is schema/policy only. When fallback execution is added later, any attempt metadata must preserve the current guarantees: no raw prompt text, no environment values, no automatic worker switching, and no retry for policy failures.

The shared summary renders these as:

- Model profile: profile name
- Model strategy: selected
- Selected model profile: selected profile name
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
