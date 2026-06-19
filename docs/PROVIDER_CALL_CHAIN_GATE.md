# Provider call chain gate (Task 22.37)

This document defines the **last governance gate before a future provider executor skeleton** (Task 22.38+).

## Purpose

Tasks 22.29–22.36 build a metadata-only provider-call chain over materialized injection artifacts. No step calls Codex/OpenCode, opens network connections, dispatches workers, or sends payloads to providers.

Task 22.37 adds a **fixture smoke / CI guard** that executes the documented chain end-to-end in isolated temp dirs and asserts the final audit passes with execution flags still blocked.

## Chain boundary (must stay false)

| Flag | Meaning |
|------|---------|
| `provider_call` | No provider API / CLI dispatch |
| `network_call` | No outbound network |
| `worker_execution` | No worker run |
| `sent_to_provider` | Payload not transmitted |
| `prompt_injection_real_runner` | Normal runner prompt unchanged |

## Last gate artifacts

Before any future provider executor:

1. **`materialized-provider-dispatch.yaml`** — dispatch contract (`enabled: false`, `allow_provider_call: false`, `allow_network: false`)
2. **`materialized-injection-provider-run-plan.json`** — provider run-plan ready, `sent_to_provider: false`
3. **`materialized-provider-payload-dry-run.json`** + **`materialized-provider-payload.md`** — dry-run payload only; markdown may contain `text_excerpt`; JSON must not
4. **`materialized-provider-payload-report.json`** — hash QA, `provider_payload_validated: true`
5. **`materialized-provider-call-gate.json`** — `provider_call_gate_ready: true`, `provider_call_allowed_now: false`
6. **`materialized-provider-call-readiness-report.json`** — `provider_call_readiness_ready: true`
7. **`provider-call-approval.json`** — explicit `--confirm-provider-payload-sha256`
8. **`provider-call-execution-bundle.json`** — final metadata consolidation
9. **`provider-call-chain-audit`** — continuity / reconciliation gate

The **execution bundle** plus **chain audit** are the last gates. A future executor skeleton (22.38+) must consume the bundle, re-validate hashes, and still require an explicit confirm flag at dispatch time.

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

Audit JSON/stdout must not contain `text_excerpt` or payload content.

## CI smoke

```bash
bash scripts/provider-call-chain-fixture-smoke.sh
```

Or via Make:

```bash
make provider-call-chain-smoke
```

This runs:

- `TestProviderCallChainFixtureSmokeE2E` — library chain 29–38 equivalent with hash coherence and anti-leak assertions
- `TestProviderCallChainFixtureCLISmokeE2E` — same chain via `deonctl` CLI (steps 31–38 + audit)

## Manual fixture

See [configs/examples/retrieval-context-fixture/README.md](../configs/examples/retrieval-context-fixture/README.md) steps 29–38.

## What 22.38+ may do (proposal)

- Read `provider-call-execution-bundle.json` and dispatch config
- Re-run hash / policy validation
- Require explicit confirm flag at executor dispatch time
- Still must not bypass approval, bundle, or audit chain

## References

- [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md) — Tasks 22.29–22.36 command reference
- [RETRIEVAL_GOVERNANCE_CHECKLIST.md](RETRIEVAL_GOVERNANCE_CHECKLIST.md) — release checklist
- [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md) — injection design ADR
