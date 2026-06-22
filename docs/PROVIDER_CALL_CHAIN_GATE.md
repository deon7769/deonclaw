# Provider call chain gate (Tasks 22.37–22.64)

This document defines the **governance gates before any real provider executor dispatch** (Task 22.64+).

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

Task 22.45 adds **request envelope dry-run/report** — metadata-only future request shape; hash references only, no payload text.

Task 22.46 adds **adapter registry / capability plan** — schema-only `provider-adapters.yaml`; codex adapter blocked for future use.

Task 22.47 adds **response fixture generate/inspect** — deterministic fake response schema; `response_source: fixture`, no provider receive.

Task 22.48 adds **execution simulation bundle/report** — consolidates envelope + adapter plan + fixture response; `execution_result_available: false`.

Task 22.49 adds **credential policy validate/plan** — declares allowed env var names without reading secret values; `credential_check_enabled: false`.

Task 22.50 adds **real-call proposal new/inspect** — formal future-call proposal; `would_call_provider_if_enabled: true`, `provider_call_allowed_now: false`.

Task 22.51 adds **response change proposal/report** — fixture-only response-to-change contract; no diff, no workspace modification.

Task 22.52 adds **activation readiness audit/report** — final audit before any future controlled activation; `activation_allowed_now: false`.

Task 22.53 adds **activation policy validate/plan** — schema-only `provider-activation-policy.yaml`; all execution flags blocked.

Task 22.54 adds **activation approval** new/approve/inspect — manual operator authorization for future activation only (`activation_authorized_for_future: true`, `activation_allowed_now: false`).

Task 22.55 adds **activation rehearsal** / **rehearsal-report** — metadata-only sequence validation with `future_steps`; no dispatch.

Task 22.56 adds **activation release package** / **release-gate** — consolidates activation control plane; `activation_gate_ready: true`, `real_activation_supported_now: false`.

Task 22.57 adds **activation chain hardening** — unified anti-leak helpers for metadata JSON/stdout; fixture README path fixes for policy config when run inside the fixture directory.

Task 22.58 adds **operator review bundle/report** — metadata-only human review package with critical hashes and blocked execution checklist.

Task 22.59 adds **activation kill-switch validate/plan** — `provider-activation-kill-switch.yaml` with `global_disabled: true` and all `block_*: true`.

Task 22.60 adds **activation final audit / CI report** — last activation gate before real dispatch design; `final_audit_ready: true`, `kill_switch_active: true`, `operator_review_required: true`.

Task 22.61 adds **provider secret-read proposal** new/inspect — declares future allowed env var names only; `secret_read_allowed_now: false`, `secret_values_read: false`.

Task 22.62 adds **real transport implementation plan/report** — future transport contract metadata; `BlockedProviderTransport` remains active; no SDK, no network, no `Deliver`.

Task 22.63 adds **real dispatch command design/report** — future `provider-real-dispatch execute` command metadata; `execute_subcommand_registered: false`.

Task 22.64 adds **real activation design review package/gate** — consolidates 22.61–22.63; `real_activation_design_gate_ready: true`, `real_dispatch_supported_now: false`.

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
17. **`provider-request-envelope`** — future request metadata envelope (`provider_request_ready: true`)
18. **`provider-adapter-registry`** + **`provider-adapter-plan`** — blocked adapter capability plan
19. **`provider-response-fixture`** — deterministic fake response schema
20. **`provider-execution-simulation-bundle`** + **`report`** — metadata-only outcome simulation (`execution_result_available: false`)
21. **`provider-credential-policy`** validate/plan — env var name policy only (`secret_values_read: false`)
22. **`provider-real-call-proposal`** new/inspect — future real-call proposal (`provider_call_allowed_now: false`)
23. **`provider-response-change-proposal`** + **report** — fixture response-to-change metadata (`workspace_modified: false`)
24. **`provider-activation-readiness-audit`** + **report** — activation readiness gate (`activation_allowed_now: false`)
25. **`provider-activation-policy`** validate/plan — activation policy schema (`enabled: false`, all `allow_*: false`)
26. **`provider-activation-approval`** new/approve/inspect — future activation authorization with explicit confirm hashes
27. **`provider-activation-rehearsal`** + **rehearsal-report** — metadata-only activation sequence rehearsal (`future_steps` only)
28. **`provider-activation-release-package`** + **release-gate** — activation release consolidation (`activation_allowed_now: false`)
29. **`provider-activation-operator-review-bundle`** + **report** — human operator review metadata (`operator_approved_now: false`)
30. **`provider-activation-kill-switch`** validate/plan — global kill-switch manifest (`global_disabled: true`, all `block_*: true`)
31. **`provider-activation-final-audit`** + **ci-report** — final activation audit (`kill_switch_active: true`, `ci_observability_ready: true`)
32. **`provider-secret-read-proposal`** new/inspect — future secret-read env var names only (`secret_values_read: false`)
33. **`provider-real-transport-implementation-plan`** + **report** — future transport contract (`transport_enabled: false`, `BlockedProviderTransport` active)
34. **`provider-real-dispatch-design`** + **report** — future dispatch command design (`execute_subcommand_registered: false`)
35. **`provider-real-activation-design-review-package`** + **gate** — real dispatch design review (`real_dispatch_supported_now: false`)

The full chain through **real activation design review gate** is the last gate before any real provider dispatch execution. Kill-switch must remain active; operator review remains required; no secret reads, no provider call, no transport, no network.

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

## Simulation layer expectations (Tasks 22.45–22.48)

`deonctl worker codex provider-request-envelope dry-run` must return:

```json
{
  "status": "ok",
  "provider_request_ready": true,
  "request_contains_text": false,
  "sent_to_provider": false,
  "provider_call": false,
  "network_call": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-adapter-plan` must return:

```json
{
  "adapter_registry_validated": true,
  "provider": "codex",
  "adapter_available_for_future": true,
  "adapter_enabled_now": false,
  "transport_enabled": false,
  "network_call": false,
  "provider_call": false
}
```

`deonctl worker codex provider-execution-simulation-report` must return:

```json
{
  "simulation_bundle_ready": true,
  "simulated_response_ready": true,
  "execution_result_available": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "sent_to_provider": false,
  "received_from_provider": false,
  "worker_execution": false
}
```

Audit, executor, simulation, activation, and fixture JSON/stdout must not contain `text_excerpt` or payload content.

## Activation readiness expectations (Tasks 22.49–22.52)

`deonctl worker codex provider-credential-policy validate` must return:

```json
{
  "credential_policy_validated": true,
  "credential_check_enabled": false,
  "secret_values_read": false,
  "provider_call": false,
  "network_call": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-real-call-proposal inspect` must return:

```json
{
  "real_call_proposal_ready": true,
  "would_call_provider_if_enabled": true,
  "provider_call_allowed_now": false,
  "transport_called": false,
  "sent_to_provider": false,
  "secret_values_read": false
}
```

`deonctl worker codex provider-response-change-proposal-report` must return:

```json
{
  "change_proposal_ready": true,
  "response_source": "fixture",
  "workspace_modified": false,
  "diff_applied": false,
  "commit_created": false,
  "pr_created": false,
  "worker_execution": false
}
```

`deonctl worker codex provider-activation-readiness-report` must return:

```json
{
  "activation_readiness_ready": true,
  "real_provider_call_supported_now": false,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "secret_values_read": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

## Activation control plane expectations (Tasks 22.53–22.56)

`deonctl worker codex provider-activation-policy validate` must return:

```json
{
  "activation_policy_validated": true,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "transport_called": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-activation-approval inspect` must return:

```json
{
  "approved": true,
  "activation_authorized_for_future": true,
  "activation_allowed_now": false,
  "provider_call_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "sent_to_provider": false,
  "secret_values_read": false,
  "workspace_modified": false,
  "allowed_use": "provider_activation_policy_only"
}
```

`deonctl worker codex provider-activation-rehearsal-report` must return:

```json
{
  "activation_rehearsal_ready": true,
  "activation_sequence_validated": true,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "transport_called": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-activation-release-gate` must return:

```json
{
  "activation_gate_ready": true,
  "activation_release_package_ready": true,
  "real_activation_supported_now": false,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "transport_called": false,
  "sent_to_provider": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

## Activation hardening expectations (Tasks 22.57–22.60)

`deonctl worker codex provider-activation-operator-review-bundle` must return:

```json
{
  "operator_review_bundle_ready": true,
  "operator_review_required": true,
  "operator_approved_now": false,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "transport_called": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-activation-kill-switch validate` must return:

```json
{
  "kill_switch_validated": true,
  "global_disabled": true,
  "provider_call_blocked": true,
  "network_blocked": true,
  "secret_read_blocked": true,
  "transport_blocked": true,
  "workspace_write_blocked": true,
  "activation_allowed_now": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-activation-final-audit` must return:

```json
{
  "final_audit_ready": true,
  "kill_switch_active": true,
  "operator_review_required": true,
  "real_activation_supported_now": false,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "transport_called": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-activation-ci-report` must return:

```json
{
  "final_audit_ready": true,
  "ci_observability_ready": true,
  "kill_switch_active": true,
  "operator_review_required": true,
  "real_activation_supported_now": false,
  "activation_allowed_now": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "transport_called": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

## Real dispatch design expectations (Tasks 22.61–22.64)

`deonctl worker codex provider-secret-read-proposal new` must return:

```json
{
  "secret_read_proposal_ready": true,
  "secret_read_allowed_now": false,
  "secret_values_read": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "activation_allowed_now": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-real-transport-implementation-plan` must return:

```json
{
  "real_transport_implementation_plan_ready": true,
  "real_transport_available_now": false,
  "transport_enabled": false,
  "transport_called": false,
  "provider_call": false,
  "network_call": false,
  "secret_values_read": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-real-dispatch-design` must return:

```json
{
  "real_dispatch_design_ready": true,
  "real_dispatch_command_available": false,
  "execute_subcommand_registered": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "secret_values_read": false,
  "sent_to_provider": false,
  "blocked_reason": "implementation_not_enabled"
}
```

`deonctl worker codex provider-real-activation-design-review-gate` must return:

```json
{
  "real_activation_design_review_ready": true,
  "real_activation_design_gate_ready": true,
  "real_dispatch_supported_now": false,
  "activation_allowed_now": false,
  "secret_values_read": false,
  "provider_call": false,
  "network_call": false,
  "transport_called": false,
  "sent_to_provider": false,
  "received_from_provider": false,
  "workspace_modified": false,
  "blocked_reason": "implementation_not_enabled"
}
```

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

- `TestProviderCallChainFixtureSmokeE2E` — library chain 29–58 equivalent with hash coherence and anti-leak assertions
- `TestProviderCallChainFixtureCLISmokeE2E` — same chain via `deonctl` CLI (steps 31–58 + audit + executor + simulation)

GitHub Actions also runs `make provider-call-chain-smoke` explicitly before `go test ./...`.

## Manual fixture

See [configs/examples/retrieval-context-fixture/README.md](../configs/examples/retrieval-context-fixture/README.md) steps 29–58.

## What 22.49+ may do (proposal)

- Enable executor policy with explicit operator approval and confirm flag
- Invoke transport behind policy gates
- Still must not bypass approval, bundle, audit, or executor validate chain

## References

- [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md) — Tasks 22.29–22.39 command reference
- [RETRIEVAL_GOVERNANCE_CHECKLIST.md](RETRIEVAL_GOVERNANCE_CHECKLIST.md) — release checklist
- [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md) — injection design ADR
