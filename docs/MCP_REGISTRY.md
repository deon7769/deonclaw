# MCP Registry

Task 21.0 adds the MCP registry foundation. Task 21.1 adds operational diagnostics, risk reporting, and Docker launch planning. Task 21.2 adds a controlled fake/test stdio smoke with transcript artifacts. Task 21.2.1 enables the same fake/test smoke through the Docker runtime. Task 21.3 adds a fake read-only tool-call smoke plus a minimal tool policy scaffold. Task 21.4 adds a real read-only discovery smoke that lists tools only under a discovery policy. Task 21.5 adds a policy-gated real read-only call smoke with exactly one tool call.

The registry records MCP server definitions so DeonClaw can validate and inspect them before any future execution layer exists. It can run fake/test smoke, fake/test tool smoke, policy-gated real read-only discovery, and one policy-gated real read-only call smoke. It does not connect MCP to Codex or OpenCode, does not automatically dispatch tools for agents, and does not participate in fallback execution.

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

Safe call policy example:

~~~bash
configs/examples/mcp-call-policy.yaml
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
    - deonclaw.fake.echo
  allowed_capabilities:
    - read
  max_arguments_bytes: 65536
  max_response_bytes: 1048576
~~~

Arguments must be valid JSON objects and fit within `max_arguments_bytes`. Response artifacts are capped by `max_response_bytes`; oversized responses are written with a preview plus truncation metadata instead of storing the full response. Env passthrough values are redacted from call-smoke artifacts.

Call-smoke artifacts:

- `mcp-call-smoke-summary.md`
- `mcp-call-transcript.jsonl`
- `mcp-call-result.json`
- `mcp-call-stdout.log`
- `mcp-call-stderr.log`
- `mcp-call-response.json`

Run the built-in fake server directly:

~~~bash
deonctl mcp fake-server
~~~

`mcp fake-server` is a minimal JSON-RPC stdio server for smoke tests only. It responds to `initialize`, `tools/list`, `tools/call` for `deonclaw.fake.echo`, `shutdown`, and `exit`.

## Boundary

Docker runtime comes before MCP execution. The current registry is a static validation, inventory, diagnostic, risk, planning, fake/test smoke, fake read-only tool-smoke, real read-only discovery, and one-call real read-only call-smoke layer only.

Implemented smoke boundaries:

- `mcp smoke`: fake initialize/tools-list only
- `mcp tool-smoke`: fake read-only allowlisted `tools/call`
- `mcp discover`: real read-only `tools/list` only
- `mcp call-smoke`: one real read-only allowlisted `tools/call`

Not implemented through Task 21.5:

- MCP integration with Codex or OpenCode
- automatic MCP tool dispatch by workers or agents
- fallback execution
- LanceDB or memory index
- UI/dashboard

Kimi remains inspiration for UX/MCP/ACP ideas only. It is not a DeonClaw worker, not an MCP server entry, and not part of operational worker config, doctor output, smoke runs, or fallback lists.
