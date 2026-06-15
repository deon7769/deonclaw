# MCP Registry

Task 21.0 adds the MCP registry foundation. Task 21.1 adds operational diagnostics, risk reporting, and Docker launch planning. Task 21.2 adds a controlled fake/test stdio smoke with transcript artifacts. Task 21.2.1 enables the same fake/test smoke through the Docker runtime.

The registry records MCP server definitions so DeonClaw can validate and inspect them before any future execution layer exists. It does not start real MCP servers, call real MCP tools, connect MCP to Codex or OpenCode, or participate in fallback execution.

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

It never calls a tool. `tools/list` may return an empty list. Environment values are never printed.

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

Run the built-in fake server directly:

~~~bash
deonctl mcp fake-server
~~~

`mcp fake-server` is a minimal JSON-RPC stdio server for smoke tests only. It responds to `initialize`, `tools/list`, `shutdown`, and `exit`.

## Boundary

Docker runtime comes before MCP execution. The current registry is a static validation, inventory, diagnostic, risk, planning, and fake/test-smoke layer only.

Not implemented in Task 21.0, Task 21.1, Task 21.2, or Task 21.2.1:

- real MCP server execution
- real MCP tool calls
- MCP integration with Codex or OpenCode
- fallback execution
- LanceDB or memory index
- UI/dashboard

Kimi remains inspiration for UX/MCP/ACP ideas only. It is not a DeonClaw worker, not an MCP server entry, and not part of operational worker config, doctor output, smoke runs, or fallback lists.
