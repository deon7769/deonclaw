# Provider call chain gate (Tasks 22.37–22.44)

This document defines the **governance gates before any real provider executor dispatch** (Task 22.44+).

## Purpose

Tasks 22.29–22.36 build a metadata-only provider-call chain over materialized injection artifacts. No step calls Codex/OpenCode, opens network connections, dispatches workers, or sends payloads to providers.

Task 22.37 adds a **fixture smoke / CI guard** that executes the documented chain end-to-end in isolated temp dirs and asserts the final audit passes with execution flags still blocked.

Task 22.38 adds a **provider executor skeleton** that consumes the execution bundle, chain audit, and approval, re-validates hashes and policy flags, and returns `blocked_reason: implementation_not_enabled`.

Task 22.39 adds the **executor policy config** (`provider-call-executor.yaml`), CLI validation, and integration into the skeleton validate/plan path.

Task 22.40 adds the **executor dry-run contract** (`dry-run` + `dry-run-report`) with metadata-only artifacts; no payload markdown reads and no transport dispatch.

Task 22.41 adds **executor preflight** — reconciles executor-config, dry-run, dry-run-report, execution-bundle, and approval hashes without reading payload markdown.

Task 22.42 adds **dispatch approval** — separate from payload approval; authorizes future dispatch only (`dispatch_authorized_for_future: true`, `dispatch_allowed_now: false`).

Task 22.43 adds **blocked transport plan** — metadata-only plan using `BlockedProviderTransport`; no SDK, no network, no `Deliver`.

Task 22.44 adds **release bundle / activation gate** — consolidates artifacts 22.38–22.43; `activation_allowed_now: false`.

## Chain boundary (must stay false)

| Flag | Meaning |
|------|---------|
| `provider_call` | No provider API / CLI dispatch |
| `network_call` | No outbound network |
| `worker_execution` | No worker run |
| `sent_to_provider` | Payload not transmitted |
| `prompt_injection_real_runner` | Normal runner prompt unchanged |
| `transport_called` | No transport `Deliver` invocation |
| `execution_supported_now` | No real executor activation |

## Last gate artifacts

Before any future real provider dispatch:

1. **`materialized-provider-dispatch.yaml`** — dispatch contract (`enabled: false`, `allow_provider_call: false`, `allow_network: false`)
2. **`materialized-injection-provider-run-plan.json`** — provider run-plan ready, `sent_to_provider: false`
3. **`materialized-provider-payload-dry-run.json`** + **`materialized-provider-payload.md`** — dry-run payload only; markdown may contain `text_excerpt`; JSON must not
4. **`materialized-provider-payload-report.json`** — hash QA, `provider_payload_validated: true`
5. **`materialized-provider-call-gate.json`** — `provider_call_gate_ready: true`, `provider_call_allowed_now: false`
6. **`materialized-provider-call-readiness-report.json`** — `provider_call_readiness_ready: true`
7. **`provider-call-approval.json`** — explicit `--confirm-provider-payload-sha256`
8. **`provider-call-execution-bundle.json`** — final metadata consolidation
9. **`provider-call-chain-audit`** — continuity / reconciliation gate
10. **`provider-call-executor.yaml`** — executor policy (`enabled: false`, all `allow_*: false`)
11. **`provider-call-executor validate/plan`** — skeleton executor re-validation (no dispatch)
12. **`provider-call-executor dry-run`** + **`dry-run-report`** — metadata-only dry-run contract (no transport)
13. **`provider-call-executor preflight`** — hash reconciliation gate (`executor_preflight_ready: true`)
14. **`provider-call-executor-dispatch-approval`** — future dispatch authorization (separate from payload approval)
15. **`provider-transport-plan`** — blocked transport metadata plan (`transport_enabled: false`)
16. **`provider-executor-release-bundle`** + **`release-gate`** — final activation gate (`activation_allowed_now: false`)

The **execution bundle**, **chain audit**, **executor policy**, **executor validate/plan**, **executor dry-run**, **preflight**, **dispatch approval**, **transport plan**, and **release gate** are the last gates. A future real dispatch must still require `--confirm-provider-executor-dispatch`.

## Final audit expectations

`deonctl worker codex provider-call-chain-audit` must return:

```json
{
  "status": "ok",
  "chain_continuity_ready": true,
  "producer_commands_active": true,
  "loaders_reconciled": true,
  "provider_call": false,
  "network_call": false,
  "worker_execution": false,
  "sent_to_provider": false
}
```

## Executor skeleton expectations (Task 22.38–22.39)

`deonctl worker codex provider-call-executor validate` must return:

```json
{
  "status": "ok",
  "executor_policy_validated": true,
  "executor_config_validated": true,
  "execution_supported_now": false,
  "provider_call_authorized_for_future": true,
  "chain_continuity_ready": true,
  "provider_call_allowed_now": false,
  "sent_to_provider": false,
  "provider_call": false,
  "network_call": false,
  "worker_execution": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-call-executor config validate` must return `status: ok` with `enabled: false` and all `allow_*: false`.

`deonctl worker codex provider-call-executor dry-run` must return:

```json
{
  "status": "ok",
  "executor_dry_run_ready": true,
  "transport_called": false,
  "contains_text": false,
  "preview_only": true,
  "provider_call": false,
  "network_call": false,
  "worker_execution": false,
  "sent_to_provider": false,
  "blocked_reason": "implementation_not_enabled"
}
```

Audit, executor, dry-run, preflight, dispatch approval, transport plan, and release JSON/stdout must not contain `text_excerpt` or payload content.

## Executor preflight expectations (Task 22.41)

`deonctl worker codex provider-call-executor preflight` must return:

```json
{
  "status": "ok",
  "executor_preflight_ready": true,
  "execution_allowed_now": false,
  "transport_called": false,
  "provider_call": false,
  "network_call": false,
  "worker_execution": false,
  "sent_to_provider": false,
  "prompt_injection_real_runner": false,
  "required_future_confirm_flag": "--confirm-provider-executor-dispatch",
  "blocked_reason": "implementation_not_enabled"
}
```

## Dispatch approval expectations (Task 22.42)

`deonctl worker codex provider-call-executor-dispatch-approval approve` must return:

```json
{
  "dispatch_authorized_for_future": true,
  "dispatch_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "sent_to_provider": false
}
```

## Transport plan expectations (Task 22.43)

`deonctl worker codex provider-transport-plan` must return:

```json
{
  "status": "ok",
  "transport_plan_ready": true,
  "transport_enabled": false,
  "transport_called": false,
  "provider_call": false,
  "network_call": false,
  "sent_to_provider": false,
  "blocked_reason": "implementation_not_enabled"
}
```

## Release gate expectations (Task 22.44)

`deonctl worker codex provider-executor-release-gate` must return:

```json
{
  "release_bundle_ready": true,
  "activation_gate_ready": true,
  "activation_allowed_now": false,
  "execution_supported_now": false,
  "provider_call": false,
  "network_call": false,
  "worker_execution": false,
  "transport_called": false,
  "sent_to_provider": false
}
```

## CI smoke

```bash
bash scripts/provider-call-chain-fixture-smoke.sh
```

Or via Make:

```bash
make provider-call-chain-smoke
```

This runs:

- `TestProviderCallChainFixtureSmokeE2E` — library chain 29–49 equivalent with hash coherence and anti-leak assertions
- `TestProviderCallChainFixtureCLISmokeE2E` — same chain via `deonctl` CLI (steps 31–49 + audit + executor sprint)

GitHub Actions also runs `make provider-call-chain-smoke` explicitly before `go test ./...`.

## Manual fixture

See [configs/examples/retrieval-context-fixture/README.md](../configs/examples/retrieval-context-fixture/README.md) steps 29–50.

## What 22.45+ may do (proposal)

- Enable executor policy with explicit operator approval and confirm flag
- Invoke transport behind policy gates
- Still must not bypass approval, bundle, audit, or executor validate chain

## References

- [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md) — Tasks 22.29–22.39 command reference
- [RETRIEVAL_GOVERNANCE_CHECKLIST.md](RETRIEVAL_GOVERNANCE_CHECKLIST.md) — release checklist
- [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md) — injection design ADR
