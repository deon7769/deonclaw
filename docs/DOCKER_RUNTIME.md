# Docker Runtime

Task 20.0 adds the Docker runtime foundation only.

DeonClaw can now load and validate `runtime.yaml` and produce a dry-run Docker command plan. Codex and OpenCode still run through the existing local runtime in this task. No worker is executed inside Docker yet.

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
- worker execution remains local

Not implemented in Task 20.0:

- running Codex or OpenCode inside Docker
- automatic fallback execution
- MCP manager
- LanceDB or memory index
- UI/dashboard
