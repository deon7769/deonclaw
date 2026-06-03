# OpenCode Z.ai / GLM VPS Smoke

This runbook verifies the DeonClaw OpenCode worker path with Z.ai / GLM configuration on the VPS.

It does not implement a provider in DeonClaw. It only checks command configuration, required environment state, dry-run planning, and a real OpenCode worker run when the VPS has OpenCode and `ZAI_API_KEY` configured.

Run from the repository root:

~~~bash
cd /opt/projetos/deonclaw
~~~

The examples below assume `deonctl` is available on `PATH`. From a source checkout, use `go run ./cmd/deonctl` in place of `deonctl` if needed.

## Pre-checks

Run the general doctor with the worker config:

~~~bash
deonctl doctor --workers-config configs/examples/workers.yaml
~~~

Check OpenCode specifically:

~~~bash
deonctl workers doctor --worker opencode --workers-config configs/examples/workers.yaml
~~~

Check secret visibility without printing values:

~~~bash
deonctl config env
~~~

Expected env behavior:

- `ZAI_API_KEY` is reported as `set(masked)` by `deonctl config env` when present.
- `workers doctor` reports `env_required_ok=true` for OpenCode when `ZAI_API_KEY` is present.
- Missing required env appears as `state=missing` and run commands fail before invoking the worker.

## Secret Source

`ZAI_API_KEY` must come from the process environment or a secret manager. Do not store real secrets in `workers.yaml`, task files, docs, artifacts, or commits.

Temporary shell example:

~~~bash
export ZAI_API_KEY=replace-with-secret-from-secret-manager
~~~

After exporting, re-run:

~~~bash
deonctl workers doctor --worker opencode --workers-config configs/examples/workers.yaml
deonctl config env
~~~

Do not paste real secret values into logs or commits.

## Worker Config Shape

The example config lives at `configs/examples/workers.yaml`:

~~~yaml
workers:
  opencode:
    command: opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
~~~

`provider`, `model`, and `env` are configuration and diagnostic metadata for this smoke. DeonClaw does not inject secrets into the command and does not call a Z.ai provider directly.

## Validate The Task

Validate the smoke task before running a worker:

~~~bash
deonctl task validate examples/tasks/opencode-smoke.yaml
~~~

## Dry-run

Plan the OpenCode command without executing the worker:

~~~bash
deonctl worker opencode dry-run examples/tasks/opencode-smoke.yaml --workers-config configs/examples/workers.yaml
~~~

The parity smoke command runs the same checks in one path: task validation, worker doctor, required-env validation, and OpenCode dry-run.

~~~bash
deonctl workers smoke \
  --worker opencode \
  --task examples/tasks/opencode-smoke.yaml \
  --store deonclaw.db \
  --artifacts-dir artifacts \
  --workers-config configs/examples/workers.yaml \
  --dry-run
~~~

Expected dry-run behavior:

- prints the workspace
- prints the policy
- prints a command shaped like `opencode run --dir . --format json <prompt>`
- if `ZAI_API_KEY` is missing, prints a warning but does not fail
- `workers smoke --dry-run` does not print required env names in the warning

## Run

Run the full worker harness:

~~~bash
deonctl worker opencode run examples/tasks/opencode-smoke.yaml \
  --store deonclaw.db \
  --artifacts-dir artifacts \
  --domains configs/examples/domains.yaml \
  --memory-policy configs/examples/memory-policy.yaml \
  --workers-config configs/examples/workers.yaml
~~~

The parity smoke command can run the full harness with the same flags:

~~~bash
deonctl workers smoke \
  --worker opencode \
  --task examples/tasks/opencode-smoke.yaml \
  --store deonclaw.db \
  --artifacts-dir artifacts \
  --domains configs/examples/domains.yaml \
  --memory-policy configs/examples/memory-policy.yaml \
  --workers-config configs/examples/workers.yaml
~~~

This command requires:

- `opencode` installed and available on `PATH`, or `workers.yaml` pointing at the executable
- `ZAI_API_KEY` present in the environment
- a clean source workspace, because the shared runner refuses dirty baselines

If required env is missing, `workers smoke` fails before invoking OpenCode and prints a final summary with `env_required_ok: false` and `status: env_missing`.

The smoke summary includes:

- `worker`
- `env_required_ok`
- `command`
- `run_id`, when a harness run started
- `artifacts_dir`, when a harness run produced artifacts
- `status`

DeonClaw uses the current non-interactive OpenCode CLI contract:

~~~bash
opencode run --dir <workspace> --format json "<prompt>"
~~~

Smoke output reports the prompt argument as `<prompt>` so task text is not echoed in DeonClaw output.

## Expected Artifacts

A run creates an artifact directory under `artifacts/<run-id>/`.

Expected files include:

- `summary.md`
- `stdout.log` or `stdout.jsonl`
- `stderr.log`
- `events.jsonl`
- `diff.patch`
- `changed-files.json`
- `artifact-manifest.json`

Depending on the configured flags and worker output, runs may also include:

- `context-pack.md`
- `memory-proposal-lint.json`
- `validation.log`
- `validation.json`

## Success Criteria

The smoke is acceptable when:

- `deonctl workers doctor --worker opencode --workers-config configs/examples/workers.yaml` shows `env_required_ok=true`.
- `deonctl config env` masks `ZAI_API_KEY`; the secret value never appears in stdout, stderr, artifacts, or commits.
- dry-run prints the planned OpenCode command.
- the full run reaches a terminal state and preserves the standard artifact set.
- a successful run reports `succeeded`; an OpenCode/provider failure reports `failed` but still preserves stdout/stderr/events/summary artifacts from the harness.
- workspace cleanup follows run status: succeeded workspaces are removed, failed or policy-failed workspaces are kept for inspection.

If the run fails before worker execution because `ZAI_API_KEY` is missing, fix the environment first and re-run the smoke.
