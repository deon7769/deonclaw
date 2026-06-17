# MCP Registry

Task 21.0 adds the MCP registry foundation. Task 21.1 adds operational diagnostics, risk reporting, and Docker launch planning. Task 21.2 adds a controlled fake/test stdio smoke with transcript artifacts. Task 21.2.1 enables the same fake/test smoke through the Docker runtime. Task 21.3 adds a fake read-only tool-call smoke plus a minimal tool policy scaffold. Task 21.4 adds a real read-only discovery smoke that lists tools only under a discovery policy. Task 21.5 adds a policy-gated real read-only call smoke with exactly one tool call. Task 21.6 adds an explicit proposal, preflight, approval, and execute workflow for read-only MCP tool calls. Task 21.6.1 hardens approval hashes and writes an auditable execution bundle. Task 21.7 allows worker runs to attach already-audited MCP artifacts as passive context only. Task 21.8 allows workers to emit MCP tool call proposals as artifacts for runner-side lint/preflight only, with no execution. Task 21.9 adds a run-scoped MCP proposal review queue for human review only, with no automatic execution.

The registry records MCP server definitions so DeonClaw can validate and inspect them before any future worker integration exists. It can run fake/test smoke, fake/test tool smoke, policy-gated real read-only discovery, one policy-gated real read-only call smoke, the explicit proposal workflow for that same read-only call path, passive MCP context attachments in runner prompts, runner-side validation of worker-generated MCP proposal artifacts, and a run-scoped proposal review queue for human triage. The proposal workflow records hashes for arguments, policy, optional MCP config, optional runtime config, proposal, approval, and execution bundle. MCP context attachments are passive and already audited. Worker-generated proposals are suggestions only. The review queue helps humans inspect proposals produced by runs; it does not execute tools, approve proposals, or start MCP servers. Approval remains manual through `mcp proposal approve`, and execute remains a separate command through `mcp proposal execute --confirm-execute`. DeonClaw still does not give Codex or OpenCode permission to call MCP tools, does not automatically dispatch tools for agents, and does not participate in fallback execution.

## Config

Example config:

~~~bash
configs/examples/mcp.yaml
~~~

Shape:

~~~yaml
mcp:
  servers:
    filesystem-readonly:
      command: npx
      args:
        - "@modelcontextprotocol/server-filesystem"
        - "."
      enabled: false
      test_only: false
      protocol: stdio
      trust: local
      capabilities:
        - read
      env:
        passthrough: []
~~~

Supported trust values:

- `local`
- `external`

Supported capabilities:

- `read`
- `write`
- `exec`

`write` and `exec` are dangerous. During the registry foundation phase, servers with `write` or `exec` must remain disabled. Disabled entries produce strong warnings so they can be reviewed intentionally without being executable.

Supported protocol values:

- empty, for registry entries that are not smoke-testable yet
- `stdio`, for controlled stdio planning and fake/test smoke

`test_only: true` marks entries that may be used by `mcp smoke`. The smoke path still requires `enabled: false`, `protocol: stdio`, local trust, and read-only capability.

Safe fake smoke example:

~~~bash
configs/examples/mcp-fake.yaml
~~~

Safe fake tool policy example:

~~~bash
configs/examples/mcp-tool-policy.yaml
~~~

Safe discovery policy example:

~~~bash
configs/examples/mcp-discovery-policy.yaml
~~~

Safe real call policy template:

~~~bash
configs/examples/mcp-call-policy.yaml
~~~

Safe fake call policy example:

~~~bash
configs/examples/mcp-call-policy-fake.yaml
~~~

## Secrets

Secrets are referenced by environment variable name only:

~~~yaml
env:
  passthrough:
    - GITHUB_TOKEN
~~~

Inline values such as `GITHUB_TOKEN=value` are rejected. CLI output and JSON plans show env names only and must not print environment values.

## CLI

Validate a registry:

~~~bash
deonctl mcp validate --config configs/examples/mcp.yaml
~~~

`validate` checks registry schema and policy only. It rejects malformed commands, unsupported trust values, unsupported capabilities, inline env values, invalid env names, and enabled servers that declare `write` or `exec`.

List configured servers:

~~~bash
deonctl mcp list --config configs/examples/mcp.yaml
~~~

`list` is inventory only. It shows configured server names, commands, enabled state, trust and capabilities.

Plan a server command without executing it:

~~~bash
deonctl mcp plan --config configs/examples/mcp.yaml --server filesystem-readonly
deonctl mcp plan --config configs/examples/mcp.yaml --server github-readonly --output-format json
~~~

`mcp plan` prints command, args, trust, capabilities, enabled state, and env names. It does not execute the command.

Diagnose MCP registry entries:

~~~bash
deonctl mcp doctor --config configs/examples/mcp.yaml
deonctl mcp doctor --config configs/examples/mcp.yaml --output-format json
~~~

`doctor` loads and validates the registry, then reports each server:

- name
- enabled
- command
- command availability by checking only the first command with `exec.LookPath`
- trust
- capabilities
- risk level
- env requirements by name and state only
- warnings

Command availability is a diagnostic. It never starts an MCP server and never calls tools. Environment states are `set_masked` or `missing`; values are never printed.

Risk levels:

- `low`: local read-only entries, especially disabled registry entries
- `medium`: external trust entries
- `high`: entries declaring `write` or `exec`

Generate an aggregate risk report:

~~~bash
deonctl mcp risk --config configs/examples/mcp.yaml
deonctl mcp risk --config configs/examples/mcp.yaml --output-format json
~~~

`risk` reports total, enabled, disabled, external, write-capability, exec-capability, missing-env, and risk-level counts. It is read-only diagnostics.

Plan an MCP server launch inside the Docker runtime without executing anything:

~~~bash
deonctl mcp docker-plan \
  --config configs/examples/mcp.yaml \
  --server filesystem-readonly \
  --runtime-config configs/examples/runtime.yaml \
  --workspace . \
  --output-format json
~~~

`mcp docker-plan` loads and validates both `mcp.yaml` and `runtime.yaml`, uses the Docker runtime planner, appends the MCP server command and args after the runtime image, and prints a command plan only. The output explicitly says it is not executed.

MCP env passthrough names are added to the Docker command as `-e NAME`. Missing MCP env values produce a warning in the plan instead of failing, because Task 21.1 is planning only. Future execution should fail before launching when required env is missing. Runtime env passthrough keeps the existing Docker runtime policy.

`mcp docker-plan` does not execute Docker, does not start an MCP server, does not call MCP tools, and does not connect MCP to Codex or OpenCode.

Run a controlled fake/test stdio smoke:

~~~bash
deonctl mcp smoke \
  --config configs/examples/mcp-fake.yaml \
  --server fake-stdio \
  --artifacts-dir artifacts/mcp-smoke \
  --timeout-seconds 5
~~~

Run the same fake/test smoke through Docker:

~~~bash
deonctl mcp smoke \
  --config configs/examples/mcp-fake.yaml \
  --server fake-stdio \
  --artifacts-dir artifacts/mcp-smoke \
  --runtime docker \
  --runtime-config configs/examples/runtime.yaml \
  --workspace .
~~~

`mcp smoke` is not real MCP execution. It only accepts registry entries that are explicitly marked for tests:

- `test_only: true`
- `protocol: stdio`
- `enabled: false`
- no `write` or `exec` capability

The smoke starts the configured fake/test process, either locally or through the validated Docker runtime, and sends only:

- `initialize`
- `tools/list`
- `shutdown`
- `exit`

It never calls a tool. The built-in fake server lists `deonclaw.fake.echo`, but plain `mcp smoke` stops after `tools/list`. Environment values are never printed.

For `--runtime docker`, DeonClaw loads and validates `runtime.yaml`, uses the Docker runtime planner, appends the fake/test server command and args after the Docker image, and executes Docker directly without `sh -c`. Runtime mount and env policy are preserved. MCP server env passthrough is name-only; if a required MCP env passthrough name is absent from the DeonClaw process environment, smoke fails before Docker starts.

`mcp docker-plan` and `mcp smoke --runtime docker` are different:

- `mcp docker-plan` prints a Docker command plan and never executes it.
- `mcp smoke --runtime docker` executes Docker, but only for a `test_only: true` fake/test stdio server and still does not call MCP tools.

Artifacts:

- `mcp-smoke-summary.md`
- `mcp-transcript.jsonl`
- `mcp-stdout.log`
- `mcp-stderr.log`
- `mcp-smoke-result.json`

The transcript is JSONL. Each line records:

- `direction`: `request` or `response`
- `method`
- `id`, when present
- `timestamp`
- `payload`

Run a controlled fake/read-only tool smoke:

~~~bash
deonctl mcp tool-smoke \
  --config configs/examples/mcp-fake.yaml \
  --server fake-stdio \
  --tool deonclaw.fake.echo \
  --arguments '{"text":"hello"}' \
  --artifacts-dir artifacts/mcp-tool-smoke \
  --policy configs/examples/mcp-tool-policy.yaml
~~~

Run the same fake tool smoke through Docker:

~~~bash
deonctl mcp tool-smoke \
  --config configs/examples/mcp-fake.yaml \
  --server fake-stdio \
  --tool deonclaw.fake.echo \
  --arguments '{"text":"hello"}' \
  --artifacts-dir artifacts/mcp-tool-smoke \
  --runtime docker \
  --runtime-config configs/examples/runtime.yaml \
  --workspace . \
  --policy configs/examples/mcp-tool-policy.yaml
~~~

`mcp tool-smoke` is still not real MCP tool execution. It accepts only fake/test stdio registry entries that satisfy the same smoke restrictions, then performs:

- `initialize`
- `tools/list`
- exactly one `tools/call`
- `shutdown`
- `exit`

The only built-in fake tool is:

- `deonclaw.fake.echo`: read-only test echo for MCP smoke only

The fake tool has a minimal object input schema with required `text`. The response content echoes the provided text. Unknown tools return a JSON-RPC error.

Tool policy scaffold:

~~~yaml
mcp_tool_policy:
  allow_test_only: true
  max_tool_calls: 1
  allowed_servers:
    - fake-stdio
  allowed_tools:
    - deonclaw.fake.echo
  allowed_capabilities:
    - read
~~~

If `--policy` is omitted, DeonClaw uses a safe default policy: test-only servers, one tool call, read-only capability, and the fake echo tool only. Explicit policy files must keep `allow_test_only: true`, `max_tool_calls >= 1`, and must not allow `write` or `exec`.

Arguments must be valid JSON objects and are capped at 64 KiB. Environment values are never printed. Docker tool-smoke follows the same Docker runtime path as smoke: direct `docker` execution, no `sh -c`, server env name-only passthrough, and failure before Docker starts when required MCP env passthrough is missing.

Tool-smoke artifacts:

- `mcp-tool-smoke-summary.md`
- `mcp-tool-transcript.jsonl`
- `mcp-tool-stdout.log`
- `mcp-tool-stderr.log`
- `mcp-tool-result.json`

Run a real read-only discovery smoke:

~~~bash
deonctl mcp discover \
  --config configs/examples/mcp.yaml \
  --server filesystem-readonly \
  --artifacts-dir artifacts/mcp-discovery \
  --runtime docker \
  --runtime-config configs/examples/runtime.yaml \
  --workspace . \
  --policy configs/examples/mcp-discovery-policy.yaml
~~~

`mcp discover` is a read-only discovery path. It may start a real MCP server only to perform:

- `initialize`
- `tools/list`
- `shutdown`
- `exit`

It never sends `tools/call`. Real tool calls remain unimplemented. Worker integration with Codex/OpenCode also remains unimplemented.

For a non-test server (`test_only: false`), discovery requires:

- discovery policy with `allow_real_readonly: true`
- `max_tool_calls: 0`
- server allowlisted in `allowed_servers`
- read-only capabilities only
- `enabled: false`
- `protocol: stdio`
- `--runtime docker` when `require_docker_for_real: true`

Servers with `write` or `exec` capability are always refused. Required MCP env passthrough must be present before execution starts. Docker discovery uses the same validated Docker runtime planner, appends the server command after the image, passes env names only, preserves mount policy, avoids secret mounts, and does not use `sh -c`.

Discovery policy scaffold:

~~~yaml
mcp_discovery_policy:
  allow_real_readonly: true
  max_tool_calls: 0
  allowed_servers:
    - filesystem-readonly
  allowed_capabilities:
    - read
  require_docker_for_real: true
~~~

Discovery artifacts:

- `mcp-discovery-summary.md`
- `mcp-discovery-transcript.jsonl`
- `mcp-discovery-stdout.log`
- `mcp-discovery-stderr.log`
- `mcp-discovery-result.json`
- `mcp-tools-list.json`

`mcp-tools-list.json` contains metadata from the `tools/list` response, including `tool_count`, tool names, tool descriptions, and input schema metadata with a schema size cap/truncation marker for large schemas.

Run one real read-only tool call smoke:

~~~bash
deonctl mcp call-smoke \
  --config configs/examples/mcp.yaml \
  --server filesystem-readonly \
  --tool <tool-name> \
  --arguments '{"key":"value"}' \
  --artifacts-dir artifacts/mcp-call-smoke \
  --runtime docker \
  --runtime-config configs/examples/runtime.yaml \
  --workspace . \
  --policy configs/examples/mcp-call-policy.yaml
~~~

Use `mcp discover` first, then replace the real call policy template's `allowed_tools` placeholder with the explicit read-only tool name returned by that server. The fake echo tool policy is separate and lives in `configs/examples/mcp-call-policy-fake.yaml`.

`mcp call-smoke` is a policy-gated read-only call smoke. It may start a real MCP server only to perform:

- `initialize`
- `tools/list`
- exactly one `tools/call`
- `shutdown`
- `exit`

It first validates that the requested tool is listed by `tools/list`. It does not connect MCP to Codex/OpenCode, does not automatically execute tools for an agent, and does not participate in fallback execution.

For a non-test server (`test_only: false`), call-smoke requires:

- call policy with `allow_real_readonly: true`
- `max_tool_calls: 1`
- server allowlisted in `allowed_servers`
- tool allowlisted in `allowed_tools`
- read-only capabilities only
- `enabled: false`
- `protocol: stdio`
- `--runtime docker` when `require_docker_for_real: true`

Servers with `write` or `exec` capability are always refused. Required MCP env passthrough must be present before execution starts. Docker call-smoke uses the same validated Docker runtime planner, appends the server command after the image, passes env names only, preserves mount policy, avoids secret mounts, and does not use `sh -c`.

Call policy scaffold:

~~~yaml
mcp_call_policy:
  allow_real_readonly: true
  require_docker_for_real: true
  max_tool_calls: 1
  allowed_servers:
    - filesystem-readonly
  allowed_tools:
    - "<replace-with-read-only-tool-name>"
  allowed_capabilities:
    - read
  max_arguments_bytes: 65536
  max_response_bytes: 1048576
~~~

The real call policy template intentionally does not suggest `deonclaw.fake.echo` for `filesystem-readonly`. Fake echo call-smoke examples use `configs/examples/mcp-call-policy-fake.yaml` with `fake-stdio`.

Arguments must be valid JSON objects and fit within `max_arguments_bytes`. Response artifacts are capped by `max_response_bytes`; oversized responses are written with a preview plus truncation metadata instead of storing the full response. Env passthrough values are redacted from call-smoke artifacts, then each call-smoke artifact is scanned for unredacted passthrough values before it is written.

Call-smoke artifacts:

- `mcp-call-smoke-summary.md`
- `mcp-call-transcript.jsonl`
- `mcp-call-result.json`
- `mcp-call-stdout.log`
- `mcp-call-stderr.log`
- `mcp-call-response.json`
- `mcp-call-execution-bundle.json` when invoked through `mcp proposal execute`

The call summary records `tool_call_count` and `response_truncated` explicitly.

Run the recommended proposal workflow for one read-only tool call:

~~~bash
deonctl mcp proposal new \
  --server filesystem-readonly \
  --tool <tool-name> \
	  --arguments '{"key":"value"}' \
	  --reason "Manual read-only smoke" \
	  --policy configs/examples/mcp-call-policy.yaml \
	  --config configs/examples/mcp.yaml \
	  --runtime docker \
	  --runtime-config configs/examples/runtime.yaml \
	  --workspace . \
	  --output artifacts/mcp-call-proposal.json

deonctl mcp proposal inspect \
  --proposal artifacts/mcp-call-proposal.json

deonctl mcp proposal lint \
  --proposal artifacts/mcp-call-proposal.json \
  --config configs/examples/mcp.yaml \
  --policy configs/examples/mcp-call-policy.yaml

deonctl mcp proposal preflight \
  --proposal artifacts/mcp-call-proposal.json \
  --config configs/examples/mcp.yaml \
  --policy configs/examples/mcp-call-policy.yaml \
  --output artifacts/mcp-call-preflight.json

deonctl mcp proposal approve \
  --proposal artifacts/mcp-call-proposal.json \
  --policy configs/examples/mcp-call-policy.yaml \
  --decision approved \
  --reason "Preflight passed and call is read-only" \
	  --output artifacts/mcp-call-approval.json \
	  --confirm-read-only

deonctl mcp approval inspect \
  --approval artifacts/mcp-call-approval.json

deonctl mcp proposal execute \
  --proposal artifacts/mcp-call-proposal.json \
  --approval artifacts/mcp-call-approval.json \
  --config configs/examples/mcp.yaml \
  --policy configs/examples/mcp-call-policy.yaml \
  --artifacts-dir artifacts/mcp-call-smoke \
  --confirm-execute
	~~~

`mcp proposal inspect` and `mcp approval inspect` show status and hashes without printing raw arguments. Use `--output-format json` when a machine-readable sanitized view is needed.

`mcp proposal execute` reuses the same hardened `mcp call-smoke` execution path. It still sends exactly one `tools/call`, keeps Docker required for real servers when policy says so, checks the approval decision, verifies the proposal arguments hash, verifies the current policy hash, verifies the current MCP config and runtime config hashes when the approval recorded them, requires `confirm_read_only`, reruns preflight, and refuses rejected or stale approvals before any MCP process is started.

`mcp-call-execution-bundle.json` ties together the proposal id, proposal hash, approval hash, policy hash, config hash, runtime config hash, preflight status, server, tool, arguments hash, runtime, call status, timestamps, `response_truncated`, `tool_calls`, and the generated call-smoke artifact paths. It is the audit envelope for the approved manual call.

The workflow is the recommended checkpoint and prerequisite before future runner integration. There is still no agent, Codex worker, OpenCode worker, or fallback path that can call MCP tools automatically.

Attach audited MCP context to a worker task passively:

~~~yaml
mcp_context:
  attachments:
    - name: filesystem-discovery
      kind: discovery
      path: artifacts/mcp-discovery/mcp-tools-list.json
    - name: filesystem-call
      kind: call
      path: artifacts/mcp-call-smoke/mcp-call-execution-bundle.json
~~~

This is not MCP worker integration. The runner validates the listed files before worker execution and writes a summarized `mcp-context.md` artifact. The prompt receives only:

- attachment name, kind, path and hash
- discovery `tool_count` and `tool_names`
- call bundle server, tool, status, preflight status, `tool_calls`, `response_truncated`, and artifact paths

For `kind: discovery`, the attachment must be a valid `mcp-tools-list.json` with `tool_count` and `tool_names`. For `kind: call`, the recommended attachment is `mcp-call-execution-bundle.json`, not the raw transcript or raw response. The bundle must include proposal/approval/policy hashes, `preflight_status: passed`, `status: succeeded`, and `tool_calls: 1`.

The runner does not include raw tool responses, raw transcripts, or large payloads by default. It also passes a sanitized task copy to the worker so MCP attachment paths are not exposed through `RunSpec.Task`. `execution-trace.json` records `mcp_context_sha256` when MCP context is attached, and `summary.md` records the attachment count.

Allow a worker to suggest an MCP tool call without executing it:

~~~yaml
mcp_proposal_policy:
  config: configs/examples/mcp.yaml
  policy: configs/examples/mcp-call-policy.yaml
  runtime_config: configs/examples/runtime.yaml
  require_preflight: true
~~~

Workers may emit this artifact:

~~~text
mcp-tool-call-proposal.json
~~~

The runner validates the proposal schema and `arguments_sha256` after the worker exits. If `mcp_proposal_policy` is configured, the runner loads the MCP config, call policy, and optional runtime config, then runs the same `internal/mcpapproval` lint/preflight checks used by the manual proposal workflow. It writes:

- `mcp-tool-call-proposal-lint.json`
- `mcp-tool-call-preflight.json`, when policy/config are available

If `mcp_proposal_policy` is absent, a structurally valid proposal is preserved and the lint artifact records `skipped_policy` as a warning. If `require_preflight: true` and preflight fails, the run fails with `MCP proposal preflight failed`. If `require_preflight: false`, a failed preflight is recorded but the runner still does not execute the tool.

Workers must not emit approval artifacts such as `mcp-tool-call-approval.json`; the runner refuses the run if they do. The summary records only proposal status and hashes, never raw arguments. `execution-trace.json` records `mcp_tool_proposal_status` and `mcp_tool_proposal_sha256` when present.

Review worker-generated proposals from persisted runs:

~~~bash
deonctl mcp proposals list --store deonclaw.db
deonctl mcp proposals list --store deonclaw.db --status preflight_passed --worker codex --output-format json
deonctl mcp proposals show --store deonclaw.db --run <run-id>
deonctl mcp proposals export --store deonclaw.db --run <run-id> --output artifacts/review/proposal.json
~~~

`mcp proposals list` reads SQLite run and artifact metadata, finds `mcp-tool-call-proposal.json`, `mcp-tool-call-proposal-lint.json`, and `mcp-tool-call-preflight.json`, and reports run id, task id, worker, proposal status, `ready_for_approval`, proposal hash, proposal id, server, tool, preflight status, and proposal path. It never prints raw arguments or environment values.

`mcp proposals show` loads the proposal plus lint/preflight artifacts for one run and prints a sanitized summary with `proposal_path`, `lint_path`, `preflight_path`, `proposal_status`, `proposal_sha256`, `arguments_sha256`, `ready_for_approval`, violations, warnings, and preflight failures. When the proposal is parseable and was not refused, show may also print suggested manual commands for `mcp proposal approve` and `mcp proposal execute`. Suggested execute commands always include an explicit `--config` value or `<mcp.yaml>` placeholder because execute requires MCP config. Docker runtime config remains on the proposal JSON (`runtime_config_path`) because execute reads it from the proposal file rather than a separate CLI flag. Raw arguments are omitted in text and JSON output.

`ready_for_approval` is `true` only when:

- `proposal_status == preflight_passed`
- no worker approval artifact was refused
- the proposal parses successfully
- `server` and `tool` are present

`mcp proposals export` copies the run proposal artifact to a chosen path without modifying, approving, or executing it. After export it prints `exported_proposal_sha256` and a `next_step_hint` for manual preflight, approval, or execute.

If a worker emitted an MCP approval artifact, list/show mark the entry as `refused`.

Preflight for exported proposals still uses the existing manual command:

~~~bash
deonctl mcp proposal preflight \
  --proposal artifacts/review/proposal.json \
  --config configs/examples/mcp.yaml \
  --policy configs/examples/mcp-call-policy.yaml \
  --output artifacts/review/preflight.json
~~~

No `mcp proposals` command starts an MCP server, calls MCP tools, approves proposals, or executes proposals.

Run the built-in fake server directly:

~~~bash
deonctl mcp fake-server
~~~

`mcp fake-server` is a minimal JSON-RPC stdio server for smoke tests only. It responds to `initialize`, `tools/list`, `tools/call` for `deonclaw.fake.echo`, `shutdown`, and `exit`.

## Boundary

Docker runtime comes before MCP execution. The current registry is a static validation, inventory, diagnostic, risk, planning, fake/test smoke, fake read-only tool-smoke, real read-only discovery, one-call real read-only call-smoke, approval-gated proposal workflow, passive context attachment layer, and worker proposal lint/preflight layer only.

Implemented smoke boundaries:

- `mcp smoke`: fake initialize/tools-list only
- `mcp tool-smoke`: fake read-only allowlisted `tools/call`
- `mcp discover`: real read-only `tools/list` only
- `mcp call-smoke`: one manual real read-only allowlisted `tools/call`
- `mcp proposal execute`: one approved real read-only allowlisted `tools/call` through the same call-smoke path
- `task.mcp_context`: passive summaries of audited discovery/call artifacts only
- `task.mcp_proposal_policy`: runner-side lint/preflight for worker-generated `mcp-tool-call-proposal.json` only
- `mcp proposals list/show/export`: run-scoped human review queue over persisted proposal artifacts only

Not implemented through Task 21.9:

- MCP integration with Codex or OpenCode
- automatic MCP tool dispatch by workers or agents
- automatic approval or proposal execution
- MCP server startup from runner proposal checks
- fallback execution
- LanceDB or memory index
- UI/dashboard

Kimi remains inspiration for UX/MCP/ACP ideas only. It is not a DeonClaw worker, not an MCP server entry, and not part of operational worker config, doctor output, smoke runs, or fallback lists.
