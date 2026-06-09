# Docker Runtime

Task 20.0 adds the Docker runtime foundation.

DeonClaw can load and validate `runtime.yaml`, produce a dry-run Docker command plan, and execute a simple explicit command inside Docker. Codex and OpenCode still run through the existing local runtime. No worker is executed inside Docker yet.

## Commands

Validate runtime config:

~~~bash
deonctl runtime validate --config configs/examples/runtime.yaml
~~~

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
- Codex and OpenCode workers still execute locally

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

Secrets should come from a future controlled secret injection layer, not from raw host mounts. Task 20.0 deliberately does not mount secrets by default.

## Runtime Boundary

The Docker runtime exists before fallback execution and MCP execution because it is the isolation layer those later features will need.

Current state:

- runtime schema implemented
- runtime validation implemented
- Docker command planning implemented
- simple Docker command execution implemented
- Docker validation command execution implemented
- worker execution remains local

Not implemented yet:

- running Codex or OpenCode inside Docker
- automatic fallback execution
- MCP manager
- LanceDB or memory index
- UI/dashboard
