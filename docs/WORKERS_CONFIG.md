# Workers Config

DeonClaw loads worker command settings from `workers.yaml` when commands receive `--workers-config`.

The current config is command-first. Provider metadata can be recorded for diagnostics and future routing, but the OpenCode run contract does not consume provider or model fields yet.

## Fields

Each entry under `workers` may define:

- `command`: executable name or path used by the worker adapter.
- `provider`: optional provider identifier for diagnostics and future routing.
- `model`: optional model identifier for diagnostics and future routing.
- `env`: optional map of environment variable requirements.

Example:

~~~yaml
workers:
  opencode:
    command: opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
~~~

## Z.ai / GLM Example

For OpenCode with Z.ai / GLM, record the provider and model in config:

~~~yaml
workers:
  opencode:
    command: opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
~~~

`ZAI_API_KEY` must come from the process environment or a separate secret manager. Do not put real secret values in `workers.yaml`.

`deonctl config env` masks sensitive values including `ZAI_API_KEY`.

## Compatibility

Simple command-only config remains valid:

~~~yaml
workers:
  codex:
    command: codex
  opencode:
    command: opencode
~~~

If a command is missing, DeonClaw keeps the built-in fallback:

- `codex` -> `codex`
- `opencode` -> `opencode`
- `kimi` -> `kimi`

## Runtime Boundaries

Provider and model are diagnostic config only in the current task. They are not injected into worker command arguments and they do not trigger a real provider call.

OpenCode still runs through the existing command contract:

~~~bash
opencode run --cwd <workspace> -
~~~

## Kimi

Kimi remains `future_worker`.

`workers.yaml` may contain a Kimi command for doctor diagnostics, but DeonClaw does not provide a Kimi worker adapter, dry-run command, or run command yet.
