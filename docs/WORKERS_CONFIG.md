# Workers Config

DeonClaw loads worker command settings from `workers.yaml` when commands receive `--workers-config`.

The current config is command-first. Provider metadata can be recorded for diagnostics and future routing. When an OpenCode task selects a model profile with `model_arg`, DeonClaw passes that value as `--model <model_arg>` to OpenCode.

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

model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
      - general
~~~

## Model Profiles

`model_profiles` define reusable model metadata and env requirements for a worker. A task can opt into a profile with `model_profile`.

Example:

~~~yaml
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
      - general
~~~

Task example:

~~~yaml
worker: opencode
model_profile: opencode-zai-glm-5-1
~~~

Resolver rules:

- If `model_profile` is empty, DeonClaw keeps the existing worker behavior.
- If `model_profile` is set, the profile must exist in `workers.yaml`.
- `model_profiles.<name>.worker` must match the task `worker`.
- Profile `env` requirements are validated together with worker-level `env` requirements.
- For OpenCode, `model_arg` becomes `--model <model_arg>` in dry-run and run commands.
- If `model_arg` is empty, DeonClaw keeps the existing worker command.

Doctor can list profiles when requested:

~~~bash
deonctl workers doctor --worker opencode --workers-config configs/examples/workers.yaml --profiles
~~~

Profile entries include provider, model, model_arg, tags, and env requirement states. Secrets are never printed.

Worker runs that reach artifact writing also record selected profile metadata in `execution-trace.json`: model_profile, provider, model, model_arg, and env requirement states. The trace stores requirement names and states only; it never stores environment values.

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

model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
      - general
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
- Worker runs also fail before execution when the selected task `model_profile` has a missing `required` env.
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

Model profiles are diagnostic and validation metadata plus OpenCode model selection. DeonClaw resolves them, checks worker/profile compatibility, validates required env presence, and passes `model_arg` as `--model` only for OpenCode.

OpenCode with a selected model profile that defines `model_arg` runs through:

~~~bash
opencode run --dir <workspace> --format json --model <model_arg> "<prompt>"
~~~

Dry-runs, summaries, and artifacts keep the prompt masked as `<prompt>`.

Environment requirements are validation metadata only. DeonClaw does not inject env values into worker command arguments.

The execution trace records env requirement state as `set_masked` or `missing`, but never records the actual environment value.

OpenCode runs through the current non-interactive OpenCode CLI contract:

~~~bash
opencode run --dir <workspace> --format json "<prompt>"
~~~

DeonClaw reports the prompt argument as `<prompt>` in dry-runs, summaries, and artifacts so task text is not echoed in DeonClaw output.

## Future Inspiration

Kimi is `inspiration_only`.

Kimi was used as inspiration for UX, MCP, and ACP ideas. It is not a DeonClaw worker.

Kimi does not participate in operational `workers.yaml`, worker doctor, worker smoke, dry-run, run, or fallback command behavior.

Keep Kimi references in this future-inspiration section only. Kimi can be revisited only through a dedicated future task that defines the worker contract first.
