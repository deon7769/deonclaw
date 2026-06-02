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

## Environment Requirements

`env` entries declare required environment variables by name:

~~~yaml
workers:
  opencode:
    command: opencode
    env:
      ZAI_API_KEY: required
~~~

`required` validates presence only. If the variable exists in the process environment, DeonClaw reports it as `set_masked`; it does not inspect, print, or validate the value.

If a required variable is absent, DeonClaw reports it as `missing`.

Doctor output includes requirement state per worker:

~~~text
worker opencode: command=opencode status=implemented available=false env_required_ok=false
worker opencode env ZAI_API_KEY: requirement=required state=missing
~~~

JSON doctor output includes the same data in `env_requirements` with:

- `name`
- `requirement`
- `state`: `set_masked` or `missing`

Run behavior:

- `worker codex run` fails before worker execution when a configured `required` env is missing.
- `worker opencode run` fails before worker execution when a configured `required` env is missing.
- dry-run commands do not fail for missing required env; they print a warning so command planning remains usable.

Secrets must come from the process environment or an external secret manager. Never put actual secret values in `workers.yaml`.

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

## Runtime Boundaries

Provider and model are diagnostic config only in the current task. They are not injected into worker command arguments and they do not trigger a real provider call.

Environment requirements are validation metadata only. DeonClaw does not inject env values into worker command arguments.

OpenCode still runs through the existing command contract:

~~~bash
opencode run --cwd <workspace> -
~~~

## Kimi Inspiration

Kimi is `inspiration_only`.

Do not add Kimi to operational `workers.yaml` examples yet. DeonClaw does not provide a Kimi worker adapter, doctor entry, dry-run command, run command, or fallback command.

Kimi can be revisited only through a dedicated future task that defines the worker contract first.
