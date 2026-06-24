# DeonClaw

Personal agent orchestration and harness control plane.

DeonClaw coordinates external coding agents, memory domains, workspaces, policies, runs, logs, artifacts and approvals.

It is not a fork of Codex, OpenCode, OpenClaw, Hermes or Paperclip.

## Core idea

Agents do work.

DeonClaw controls:

- task scope
- workspace isolation
- memory boundaries
- domain routing
- worker execution
- policy validation
- event capture
- artifacts
- approvals

## MVP focus

The first MVP focuses on:

- Go CLI/core
- structured tasks
- Codex CLI and OpenCode workers
- JSONL, mixed, text and empty stdout capture
- isolated workspaces
- memory/domain policy
- path validation
- memory proposal/apply/restore workflow
- run observability through execution traces and reports

## Epic 23 direction

Tasks 22.0–22.68 established the contract, governance, approval, hash, anti-leak, fixture and provider-dispatch design foundation.

Epic 23 shifts DeonClaw from manual contract-heavy execution toward a stable personal multi-agent runtime:

- [Epic 23 Roadmap](docs/EPIC_23_ROADMAP.md) — full implementation map and dependency graph
- [Insight and Learning Loop](docs/INSIGHT_LEARNING_LOOP.md) — Cursor/Hermes-inspired self-evaluation after runs, commits and corrections
- [Universal Skill Registry](docs/UNIVERSAL_SKILLS.md) — AgentSkills-compatible `SKILL.md` installation, provenance, permissions and session snapshots
- [Persistent Agents and Sessions](docs/PERSISTENT_AGENTS.md) — agent identities, roles, lifecycle, sessions, inbox and delegation
- [Proactive Runtime](docs/PROACTIVE_RUNTIME.md) — `deond`, cron, heartbeat, hooks, wakeup queue and recovery
- [Budgets, Costs and Leases](docs/BUDGETS_AND_COSTS.md) — work queue, execution leases, usage events, reservations and hard stops
- [OpenClaw Migration Plan](docs/OPENCLAW_MIGRATION.md) — inspect, plan, passive import, shadow mode, cutover and rollback
- [Rich Runtime Actions](docs/RICH_RUNTIME_ACTIONS.md) — stable Git, browser, review and worker-session actions independent of any one agent UI

The implementation priority is to prove vertical slices, not to add more metadata-only gates.

## Memory direction

`mysecondbrain` is the general memory base.

`escalasoft_brain` is an isolated domain memory base.

Indexes such as LanceDB may be used to improve retrieval and mapping, but they are not the source of truth.

Markdown + Git remain canonical.

## First workers

1. Codex CLI
2. OpenCode
3. Claude Code, later
4. OpenClaw adapter, later

Kimi is `inspiration_only`. It is not an operational worker and must not appear in `workers.yaml`, doctor output, smoke runs, or fallback lists.

## Status

Functional MVP in active development.

Implemented:

- Codex CLI worker dry-run and run
- OpenCode worker dry-run and run
- OpenCode real smoke validated on the VPS
- isolated Git worktree runs
- path policy and diff artifacts
- validation commands
- artifact metadata/list/prune
- domain config
- context packs
- `workers.yaml` command/provider/model/env metadata
- `model_profiles`
- `task.model_profile`
- OpenCode `--model` through selected `model_profile`
- `model_strategy` schema and validation
- `model_strategy` dry-run planning
- controlled `model_strategy.preferred[0]` selection for OpenCode real runs
- `workers smoke --dry-run` with `model_strategy` planning
- `execution-trace.json`
- `runs report`
- `runs report --by model_profile` and filters
- Docker runtime config validation, docker-plan, docker-exec, and Docker validation commands
- OpenCode Docker worker runtime supported experimental
- OpenCode Docker Z.AI smoke runtime
- MCP registry config validation, list and plan foundation
- MCP fake/test smoke, fake read-only tool-smoke, real read-only discovery, policy-gated real read-only call-smoke, explicit MCP call proposal approval workflow with stale-hash checks and execution bundle, passive MCP context attachments for worker runs, worker-generated MCP proposal lint/preflight artifacts without tool execution, and run-scoped MCP proposal review queue (`mcp proposals list/show/export`) without automatic execution
- fallback policy schema validation only; no fallback execution or retries
- memory proposal/lint/dry-run/approval/preflight/backup/materialize/apply/restore workflow
- real memory apply for create/append/update/archive
- real restore execution
- memory index foundation (`memory index validate/plan/build/doctor/report`) with auditable chunks, without runner retrieval or LanceDB writes
- embedding policy dry-run and deterministic fake vector smoke (`memory embedding validate/doctor/plan/build-fake/report`), without real provider APIs or LanceDB writes
- LanceDB write plan-only (`memory lancedb validate/plan`) over embedding artifacts, without LanceDB import or database writes
- LanceDB fake-write artifact smoke (`memory lancedb fake-write`), without LanceDB SDK or real database creation
- LanceDB real local write smoke (`memory lancedb write-smoke`) via Python adapter, without retrieval or runner integration
- LanceDB structural readback doctor/report (`memory lancedb doctor/report`) over write-smoke databases, without search or runner integration
- LanceDB controlled vector search smoke (`memory lancedb search-smoke`) with explicit query vector or chunk_id, without natural-language retrieval, provider APIs, or runner integration
- LanceDB search-smoke result QA/report (`memory lancedb search-report`) over search artifacts, without new search or runner integration
- passive LanceDB retrieval context attachment in runner (`retrieval_context` on tasks), without runner search or chunk text injection
- retrieval context inspect and runs retrieval-report for passive attachment auditing, without active search
- governed retrieval context chunk text materialize (`retrieval context materialize`), without runner auto-injection
- retrieval context materialized-report for chunk text artifact QA, without runner auto-injection
- retrieval context audit bundle (`retrieval context bundle`), without runner text injection
- retrieval context approval workflow (`retrieval context approval`), without runner text injection
- retrieval context injection plan (`retrieval context injection-plan`), without runner execution
- retrieval context governance report (`retrieval context governance-report`), without runner injection
- retrieval context governance e2e fixture (`configs/examples/retrieval-context-fixture/`)
- retrieval governance release checklist (`docs/RETRIEVAL_GOVERNANCE_CHECKLIST.md`)
- retrieval runner injection design ADR (`docs/ADR_RETRIEVAL_RUNNER_INJECTION.md`)
- retrieval context injection policy schema (`retrieval context injection-policy`), no execution
- retrieval injection policy e2e fixture (`configs/examples/retrieval-context-fixture/retrieval-injection-policy.yaml`), validate/plan only
- runner injection approval artifact (`retrieval context injection-approval`), no execution
- runner injection execution plan (`retrieval context injection-execution-plan`), no worker execution
- materialized prompt section preview (`retrieval context prompt-preview`), no worker execution
- prompt preview report/QA (`retrieval context prompt-preview-report`), no worker execution
- injection governance release bundle (`retrieval context injection-governance-bundle`), no worker execution
- task schema declaration for materialized injection (`retrieval_context.materialized_injection`), no runner execution
- materialized injection task declaration report (`task materialized-injection-report`), no runner execution
- runner materialized injection preflight (`worker codex materialized-injection-preflight`), no runner execution
- runner materialized injection dry-run prompt artifact (`worker codex materialized-injection-dry-run`), no worker execution
- runner materialized injection dry-run report (`worker codex materialized-injection-dry-run-report`), no worker execution
- runner materialized injection readiness report (`worker codex materialized-injection-readiness-report`), no worker execution
- runner materialized injection execution gate (`worker codex materialized-injection-execution-gate`), no worker execution
- materialized prompt assembly dry-run (`worker codex materialized-prompt-assembly-dry-run`), no worker execution
- materialized prompt assembly report (`worker codex materialized-prompt-assembly-report`), no worker execution
- materialized injection runtime config schema (`worker codex materialized-injection-runtime validate`), no worker execution
- materialized injection run planner (`worker codex materialized-injection-run-plan`), no worker execution
- materialized injection execution enablement policy (`worker codex materialized-injection-execution-enable validate`), no worker execution
- materialized injection provider run-plan (`worker codex materialized-injection-provider-run-plan`), no provider call
- materialized provider dispatch validate (`worker codex materialized-provider-dispatch validate`), no provider call
- materialized provider payload dry-run/report (`worker codex materialized-provider-payload-dry-run`, `materialized-provider-payload-report`), no provider call
- materialized provider call gate/readiness report (`worker codex materialized-provider-call-gate`, `materialized-provider-call-readiness-report`), no provider call
- provider call approval (`worker codex provider-call-approval new/approve/inspect`), no provider call
- provider call execution bundle (`worker codex provider-call-execution-bundle`), no provider call
- provider call chain continuity audit (`worker codex provider-call-chain-audit`), no provider call
- provider call chain fixture smoke / CI guard (`make provider-call-chain-smoke`), no provider call
- provider call executor skeleton (`worker codex provider-call-executor config validate`, `validate`, `plan`, `dry-run`, `dry-run-report`, `preflight`), no provider call
- provider call executor dispatch approval (`worker codex provider-call-executor-dispatch-approval new/approve/inspect`), no provider call
- provider transport plan (`worker codex provider-transport-plan`), blocked, no provider call
- provider executor release bundle / gate (`worker codex provider-executor-release-bundle`, `release-gate`), no provider call
- provider execution simulation layer (`provider-request-envelope`, `provider-adapter-registry`, `provider-adapter-plan`, `provider-response-fixture`, `provider-execution-simulation-bundle`, `provider-execution-simulation-report`), no provider call
- provider activation readiness layer (`provider-credential-policy`, `provider-real-call-proposal`, `provider-response-change-proposal`, `provider-activation-readiness-audit`), no provider call
- provider activation control plane (`provider-activation-policy`, `provider-activation-approval`, `provider-activation-rehearsal`, `provider-activation-release-package`, `provider-activation-release-gate`), no provider call
- provider activation hardening (`provider-activation-operator-review-bundle`, `provider-activation-kill-switch`, `provider-activation-final-audit`, `provider-activation-ci-report`), no provider call
- provider real dispatch design package (`provider-secret-read-proposal`, `provider-real-transport-implementation-plan`, `provider-real-dispatch-design`, `provider-real-activation-design-review-package`), no provider call
- provider real dispatch external approval package (`provider-real-dispatch-external-approval`, `provider-real-dispatch-runbook`, `provider-real-dispatch-risk-register`, `provider-real-dispatch-preimplementation-gate`), no provider call
- provider activation policy config (`configs/examples/provider-activation-policy.yaml`), no provider call
- provider credential policy config (`configs/examples/provider-credential-policy.yaml`), no provider call
- provider adapters config (`configs/examples/provider-adapters.yaml`), no provider call
- provider call executor policy config (`configs/examples/provider-call-executor.yaml`), no provider call
- insight trigger policy validate, evidence bundle build from runs, and trigger evaluate (`insights policy validate`, `insights evidence build`, `insights trigger evaluate`), no evaluator or learning proposals yet
- insight reviewer dry-run/evaluate and insight report materialization (`insights evaluate dry-run`, `insights evaluate`, `insights report`), no live Codex/OpenCode reviewer execution or learning proposals yet
- learning proposal materialize/list/show from insight reports and reviewer responses (`insights proposals materialize`, `insights proposals list`, `insights proposals show`)
- learning proposal approval, apply dry-run/preview execute, effectiveness record/report, and timeline (`insights proposals approve`, `insights proposals apply dry-run`, `insights proposals apply`, `insights effectiveness record`, `insights effectiveness report`, `insights timeline`), documentation-like types only for preview apply; skill/memory/policy types blocked
- universal skill registry foundation (`skills policy validate`, `skills inspect`, `skills import`, `skills install`, `skills verify`, `skills list`, `skills show`, `skills enable`, `skills disable`, `skills snapshot`, `skills materialize`), local install only; git import plan-only; no worker auto-loading yet
- persistent agents and sessions foundation (`agents validate`, `agents sync`, `agents list`, `agents show`, `agents pause`, `agents resume`, `agents terminate`, `agents sessions create/list/show`, `agents assign`, `agents inbox list/accept/defer`, `agents delegate propose`), SQLite-backed; no automatic run dispatch yet
- proactive runtime foundation (`daemon status`, `daemon doctor`, `daemon start`, `daemon stop`, `daemon run-once`, `schedules validate`, `schedules sync`, `schedules list`, `schedules due`, `heartbeat validate`, `heartbeat dry-run`, `hooks validate`, `hooks plan`), SQLite schema v3; persisted `NextDueAt` due semantics, catch-up/max lateness, atomic wakeup claims; fixture smoke in CI (`make proactive-runtime-smoke`); no `deonclawd` or worker dispatch yet
- skill approval gate (`skills approve`) and snapshot/registry validation before materialize; delegation path subset and manager privilege guard
- work queue and execution leases (`deonctl work list/show/claim/release/recover/doctor`); schema v4; no budget enforcement or worker dispatch yet

Not implemented yet:

- `deonclawd` long-running daemon process and automatic worker dispatch from wakeups
- Work queue, execution leases, budgets, usage events, and hard stops
- OpenClaw migration inspect/plan/apply/shadow/cutover/rollback
- Rich Git/browser/action contracts and future UI/API surfaces
- real fallback execution
- Codex Docker worker runtime
- MCP execution/manager for automatic real external MCP tool orchestration
- memory index retrieval/LanceDB natural-language search or active search in runner
- UI/dashboard

## Quick command map

```bash
deonctl doctor
deonctl workers doctor --worker opencode --workers-config configs/examples/workers.yaml --profiles
deonctl workers smoke --worker opencode --task examples/tasks/opencode-smoke.yaml --store deonclaw.db --artifacts-dir artifacts --workers-config configs/examples/workers.yaml --dry-run
deonctl runs report --store deonclaw.db --by model_profile
deonctl worker opencode run examples/tasks/opencode-smoke.yaml --store deonclaw.db --artifacts-dir artifacts --workers-config configs/examples/workers.yaml
deonctl worker opencode run examples/tasks/opencode-smoke.yaml --store deonclaw.db --artifacts-dir artifacts --workers-config configs/examples/workers.yaml --domains configs/examples/domains.yaml --memory-policy configs/examples/memory-policy.yaml
```
