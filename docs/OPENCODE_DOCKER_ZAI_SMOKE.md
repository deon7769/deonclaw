# OpenCode Docker Z.AI Smoke

This runbook records the real OpenCode Docker smoke against Z.AI. It keeps the default runtime example safe while providing a separate external-model smoke runtime.

The safe base runtime is `configs/examples/runtime.yaml`. It uses `network: none`, read-only memory mounts, and a workspace mount scoped to `/workspace`.

The external Z.AI smoke runtime is `configs/examples/runtime-opencode-zai-smoke.yaml`. It intentionally uses `network: default` because OpenCode must reach the Z.AI API from inside the container.

## Secret Source

`ZAI_API_KEY` must come from the environment or a secret manager. Never commit the key value in config, docs, task files, artifacts, logs, or shell history.

VPS example:

~~~bash
source /home/davi/.openclaw/secrets/deonclaw-zai-test.env
~~~

The secret file should be readable only by the owner:

~~~bash
chmod 600 /home/davi/.openclaw/secrets/deonclaw-zai-test.env
~~~

The Docker runtime passes only the name:

~~~text
-e ZAI_API_KEY
~~~

It must never record `ZAI_API_KEY=<value>`.

## Runtime Validate

Validate the safe runtime:

~~~bash
deonctl runtime validate --config configs/examples/runtime.yaml
~~~

Validate the Z.AI smoke runtime:

~~~bash
deonctl runtime validate --config configs/examples/runtime-opencode-zai-smoke.yaml
~~~

Expected warning for the Z.AI smoke runtime:

~~~text
warning: runtime.docker.network default allows container network access
~~~

If `ZAI_API_KEY` is not set in the shell, validation may also warn that the passthrough env is missing. That is expected during static validation; the real worker run requires the key to be set before Docker starts.

## Run

Run from the repository root on the VPS:

~~~bash
cd /opt/projetos/deonclaw
source /home/davi/.openclaw/secrets/deonclaw-zai-test.env
ARTIFACTS_DIR=/tmp/deonclaw-docker-smoke-zai-$(date +%Y%m%d-%H%M%S)

deonctl worker opencode run examples/tasks/opencode-smoke.yaml \
  --store /tmp/deonclaw-docker-smoke.db \
  --artifacts-dir "$ARTIFACTS_DIR" \
  --workers-config configs/examples/workers.yaml \
  --runtime-config configs/examples/runtime-opencode-zai-smoke.yaml \
  --worker-runtime docker
~~~

The smoke runtime is specific to an external provider call. Do not replace the safe base runtime with `network: default`.

## Expected Artifacts

The run directory should include:

- `summary.md`
- `stdout.jsonl`
- `events.jsonl`
- `stderr.log`
- `execution-trace.json`
- `diff.patch`
- `changed-files.json`

Other standard artifacts such as `artifact-manifest.json` may also be present.

## Success Criteria

The smoke is successful when:

- `summary.md` reports `Status: succeeded`.
- `summary.md` reports `Worker runtime: docker`.
- validation/runtime metadata is recorded when validation is configured.
- `stdout.jsonl` and `events.jsonl` capture OpenCode events.
- `diff.patch` is empty for this read-only smoke task.
- `changed-files.json` is empty for this read-only smoke task.
- workspace cleanup reports `removed`.
- leak scan finds no key value and no prompt literal in `summary.md`, `stdout.jsonl`, `events.jsonl`, `stderr.log`, `execution-trace.json`, `diff.patch`, or `changed-files.json`.

## Validated Smoke

A real smoke was validated on 2026-06-10 with:

- OpenCode `1.15.13`
- model `glm-5.1`
- endpoint `https://api.z.ai/api/coding/paas/v4`

The validation recorded endpoint/model configuration only. It did not record the key value.
