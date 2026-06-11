# MCP Registry

Task 21.0 adds the MCP registry foundation.

The registry records MCP server definitions so DeonClaw can validate and inspect them before any future execution layer exists. It does not start MCP servers, call MCP tools, connect MCP to Codex or OpenCode, or participate in fallback execution.

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

List configured servers:

~~~bash
deonctl mcp list --config configs/examples/mcp.yaml
~~~

Plan a server command without executing it:

~~~bash
deonctl mcp plan --config configs/examples/mcp.yaml --server filesystem-readonly
deonctl mcp plan --config configs/examples/mcp.yaml --server github-readonly --output-format json
~~~

`mcp plan` prints command, args, trust, capabilities, enabled state, and env names. It does not execute the command.

## Boundary

Docker runtime comes before MCP execution. The current registry is a static planning and validation layer only.

Not implemented in Task 21.0:

- MCP server execution
- MCP tool calls
- MCP integration with Codex or OpenCode
- fallback execution
- LanceDB or memory index
- UI/dashboard

Kimi remains inspiration for UX/MCP/ACP ideas only. It is not a DeonClaw worker, not an MCP server entry, and not part of operational worker config, doctor output, smoke runs, or fallback lists.
