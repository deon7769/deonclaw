# Docker Runtime

Task 20.0 adds the Docker runtime foundation.

DeonClaw can load and validate `runtime.yaml`, produce a dry-run Docker command plan, execute a simple explicit command inside Docker, run Docker-backed validation commands, run a fake/test MCP smoke, and run an OpenCode Docker worker runtime path marked `supported_experimental`. Codex still runs through the existing local worker runtime.

The OpenCode Docker Z.AI smoke has been validated, but production use still requires caution because the current OpenCode Docker prompt contract uses a prompt argument placeholder. The recorded command keeps `<prompt>`, but the real process args receive the raw prompt. A future wrapper may move this to stdin or another hardened transport.

## Commands

Validate runtime config:

~~~bash
deonctl runtime validate --config configs/examples/runtime.yaml
~~~

`configs/examples/runtime.yaml` is the safe base runtime example. It keeps `network: none` and does not require provider API key passthrough. External-provider smoke tests should use a dedicated runtime such as `configs/examples/runtime-opencode-zai-smoke.yaml`.

Plan the future Docker command without executing it:

~~~bash
deonctl runtime docker-plan --config configs/examples/runtime.yaml --workspace .
deonctl runtime docker-plan --config configs/examples/runtime.yaml --workspace . --output-format json
~~~

`docker-plan` prints a `docker run ...` command shape only. It does not start a container and it does not dispatch workers.

Execute a simple command inside the configured Docker runtime:

~~~bash
deonctl runtime docker-exec --config configs/examples/runtime.yaml --workspace . -- echo hello
~~~

`docker-exec` runs `docker run` with the validated runtime plan and appends the command after the image. It captures and streams stdout/stderr directly and returns the container exit code.

`docker-exec` is intentionally narrow:

- the command must appear after `--`
- empty commands are rejected
- there is no implicit shell
- DeonClaw does not add `sh -c`
- workers are not dispatched through this command

Run task validation commands inside Docker while keeping the worker local:

~~~bash
deonctl worker opencode run examples/tasks/opencode.yaml \
  --store deonclaw.db \
  --artifacts-dir artifacts \
  --runtime-config configs/examples/runtime.yaml \
  --validation-runtime docker
~~~

Task YAML can also opt into Docker validation:

~~~yaml
validation:
  runtime: docker
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
      timeout_seconds: 300
~~~

When `validation.runtime` is `docker`, DeonClaw uses the same validated Docker runtime plan as `docker-exec`, appends each validation command after the image, captures stdout/stderr/exit code into the existing validation artifacts, and respects each command's `timeout_seconds`.

Validation Docker execution has the same boundaries as `docker-exec`:

- local validation remains the default
- commands are executed directly, without implicit shell and without `sh -c`
- invalid runtime config or dangerous mounts fail before Docker is executed
- missing configured passthrough env fails before Docker is executed
- Codex and OpenCode workers still execute locally

Validation audit fields:

- `summary.md` records `Validation runtime: local|docker`
- `execution-trace.json` records `validation_runtime`
- `execution-trace.json` records `runtime_config_sha256` when `--runtime-config` is used
- the raw `runtime.yaml` content is not copied into trace or summary
- `validation.log` and `validation.json` keep per-command runtime fields

Worker runtime scaffold:

~~~bash
deonctl worker codex run task.yaml \
  --store deonclaw.db \
  --artifacts-dir artifacts \
  --worker-runtime local \
  --runtime-config configs/examples/runtime.yaml
~~~

`--worker-runtime local|docker` is part of the execution contract. Docker worker runtime started as a fake/test scaffold in Task 20.4. OpenCode Docker worker runtime is now supported experimental for the validated smoke path. Codex Docker worker runtime remains blocked and not implemented.

MCP fake/test smoke through Docker:

~~~bash
deonctl mcp smoke \
  --config configs/examples/mcp-fake.yaml \
  --server fake-stdio \
  --artifacts-dir artifacts/mcp-smoke \
  --runtime docker \
  --runtime-config configs/examples/runtime.yaml \
  --workspace .
~~~

This executes Docker only for a registry entry marked `test_only: true`, `protocol: stdio`, `enabled: false`, and read-only capability. It sends `initialize`, `tools/list`, `shutdown`, and `exit`; it does not call MCP tools and does not start real MCP servers. Server env passthrough must be present before Docker starts, and values are never written to artifacts.

Prompt delivery is worker-specific. A Docker worker plan must declare one of:

- `prompt_delivery: stdin`: the runner writes the prompt to process stdin.
- `prompt_delivery: arg_placeholder` with `placeholder: <prompt>`: the runner replaces the placeholder only in the real process args.

The recorded command stays masked with `<prompt>` in `RunResult.Command`, `summary.md`, and `execution-trace.json`. The raw prompt is represented by `prompt_sha256` only. OpenCode Docker currently uses `arg_placeholder`, matching the OpenCode prompt-as-argument contract. This keeps artifacts masked but is still a production caveat because the real process receives the raw prompt as an argument; the prompt transport can move to stdin or a wrapper later.

For Docker worker runtime, the prepared host workspace is mounted dynamically into the container workdir, usually `/workspace`, as `rw`. Worker command planning receives the container workspace path, so OpenCode smoke commands use `--dir /workspace` even though the prepared workspace lives on the host. Existing memory mounts from `runtime.yaml` remain configured separately and should stay `ro`.

Worker runtime audit fields:

- `summary.md` records `Worker runtime: local|docker`
- `execution-trace.json` records `worker_runtime`
- `execution-trace.json` records `runtime_config_sha256` when Docker worker runtime uses `--runtime-config`

## Config Shape

~~~yaml
runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    memory_limit: 2g
    cpus: "2"
    mounts:
      - source: .
        target: /workspace
        mode: rw
      - source: mysecondbrain
        target: /memory/mysecondbrain
        mode: ro
      - source: escalasoft_brain
        target: /memory/escalasoft_brain
        mode: ro
~~~

Supported modes:

- `local`: existing runtime behavior.
- `docker`: validates Docker runtime settings and enables command planning.

Supported Docker networks:

- `none`: default safe mode.
- `default`: valid but emits a warning because it gives the container network access.

Supported mount modes:

- `rw`: read-write, intended for the isolated code workspace only.
- `ro`: read-only, intended for memory/domain mounts by default.

## Env Passthrough Policy

Docker runtime env passthrough is an explicit allowlist by variable name.

Example:

~~~yaml
runtime:
  mode: docker
  docker:
    network: default
    env:
      passthrough:
        - ZAI_API_KEY
~~~

Use provider env passthrough in purpose-specific configs. The OpenCode Z.AI Docker smoke uses `configs/examples/runtime-opencode-zai-smoke.yaml` and requires `network: default` so the container can reach the external model endpoint.

Rules:

- only env names are configured and recorded
- values come from the DeonClaw process environment or its secret manager
- `docker-plan`, `docker-exec`, and Docker validation pass env as `-e NAME`
- inline values such as `NAME=value` are rejected
- names must contain only `A-Z`, `0-9`, and `_`
- empty names, names with spaces, `=`, `-`, `/`, or `.` are rejected
- `runtime validate` warns when an allowlisted env is missing
- `docker-exec` and Docker validation fail before Docker starts when an allowlisted env is missing or empty
- env values are never written to summary, trace, validation artifacts, or plan output

Use env passthrough for API keys that must reach a container. Do not mount secret files or secret directories into Docker.

## Mount Policy

Docker runtime mounts are intentionally conservative.

Allowed pattern:

- code workspace mounted `rw` into `/workspace`
- memory sources such as `mysecondbrain` mounted `ro`
- isolated domain sources such as `escalasoft_brain` mounted `ro`
- secrets are never passed through raw mounts

Blocked sources:

- `/`
- `/home` and anything under `/home`
- `/root` and anything under `/root`
- `~` and anything under `~`
- `~/.ssh`
- `.env` files
- any path component named `secrets`
- `/var/run/docker.sock`

Blocked targets:

- relative paths
- `/`
- `/root` and anything under `/root`
- `/etc` and anything under `/etc`
- `/var/run/docker.sock`

Secrets should come from controlled env passthrough or a future secret injection layer, not from raw host mounts. Docker runtime does not mount secrets by default.

## Runtime Boundary

The Docker runtime exists before fallback execution and MCP execution because it is the isolation layer those later features will need.

Current state:

- runtime schema implemented
- runtime validation implemented
- Docker command planning implemented
- simple Docker command execution implemented
- Docker validation command execution implemented
- controlled env passthrough by allowlisted name implemented
- Docker worker execution scaffold implemented for fake/test workers
- MCP Docker fake/test smoke implemented for `test_only` stdio servers
- OpenCode Docker worker runtime supported experimental with prompt placeholder masking
- OpenCode Docker Z.AI smoke validated with `configs/examples/runtime-opencode-zai-smoke.yaml`
- Codex worker execution remains local

Not implemented yet:

- running Codex inside Docker
- production-ready hardened OpenCode Docker prompt transport
- automatic fallback execution
- MCP manager
- LanceDB or memory index
- UI/dashboard
