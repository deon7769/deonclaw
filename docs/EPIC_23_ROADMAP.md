# Epic 23 — Stable Proactive Multi-Agent Runtime

## Purpose

Epic 23 moves DeonClaw from a manually invoked, strongly governed execution harness into a stable personal multi-agent runtime.

Tasks 22.0–22.68 established the contract, policy, approval, hashing, anti-leak, fixture, and release-gate foundation around memory retrieval, prompt materialization, provider dispatch, and future real execution. Epic 23 deliberately changes the center of gravity:

- from additional metadata-only gates to operational vertical slices;
- from one-shot workers to persistent agents;
- from manual invocation to scheduled and event-driven work;
- from passive logs to actionable insight and learning;
- from tool-specific skills to a portable skill registry;
- from unmetered runs to atomic cost and budget control;
- from OpenClaw dependency to a reversible migration path;
- from terminal-only operations to generic Git, browser, and action contracts.

The project remains a personal control plane. It is not a fork of Codex, OpenCode, OpenClaw, Hermes, or Paperclip.

## Current baseline

At the start of Epic 23, DeonClaw already provides:

- real Codex CLI and OpenCode worker execution;
- isolated Git worktrees and path policy;
- structured tasks, runs, events, artifacts, and approvals;
- memory proposal/apply/restore governance;
- MCP registry and governed read-only execution foundations;
- memory indexing and controlled LanceDB smoke paths;
- governed retrieval-context materialization and injection contracts;
- a complete provider-dispatch design and preimplementation gate;
- CI and end-to-end fixtures for the provider-call chain.

As of **23.19.1 on `main`**, the operational foundation includes persistent agents and sessions, proactive runtime (`daemon run-once`, schedules, wakeups, heartbeat dry-run), work queue with leases, usage/pricing ledger, atomic budget reservations, and budgeted fake dispatch with insight-review queueing.

The principal missing capabilities are:

- closed learning loop through dispatch (reviewer fixture → report → proposal → skill apply → session → effectiveness);
- real worker dispatch behind lease, budget, and session gates;
- long-running `deonclawd` with automatic schedule → work → dispatch;
- OpenClaw migration shadow and cutover;
- active retrieval and automatic MCP/tool orchestration;
- generic Git/browser actions and an operator UI/API.

## Core model

The following concepts must remain separate:

| Concept | Responsibility |
|---|---|
| `Agent` | Persistent identity, role, goals, memory scope, skill policy, budget, and schedules |
| `Worker` | Execution backend such as Codex CLI or OpenCode |
| `ModelProfile` | Provider/model/auth selection used by a worker |
| `Session` | Durable agent context and skill snapshot across runs |
| `WorkItem` | Assignable unit of work with priority, dependencies, and acceptance criteria |
| `Run` | One concrete attempt to execute a work item |
| `Schedule` | Cron, interval, one-shot, webhook, or heartbeat trigger |
| `Skill` | Versioned portable procedural knowledge loaded into sessions |
| `Insight` | Evidence-backed conclusion produced from completed work |
| `LearningProposal` | Governed memory, skill, rule, eval, workflow, or documentation change |

A worker is not an agent. Codex and OpenCode are execution runtimes attached to persistent DeonClaw agents.

## Target architecture

```mermaid
flowchart TD
    CLI[deonctl / API / future UI] --> D[deond]
    D --> S[Scheduler and Wakeup Queue]
    D --> AM[Agent Manager]
    D --> PE[Policy and Approval Engine]
    S --> WQ[Work Queue]
    AM --> WQ
    WQ --> L[Lease and Budget Reservation]
    L --> RD[Runtime Dispatcher]
    RD --> C[Codex Adapter]
    RD --> O[OpenCode Adapter]
    C --> ES[Run Event Stream]
    O --> ES
    ES --> CA[Cost Accounting]
    ES --> OE[Outcome Evaluator]
    OE --> IR[Insight Report]
    IR --> LP[Learning Proposal]
    LP --> AP[Approval / Apply / Rollback]
    AP --> MR[Memory and Skill Registry]
    MR --> AM
```

## Implementation order

The order is dependency-driven. A later epic may be researched earlier, but production activation must follow this sequence.

### 23A — Insight and Learning Kernel (`23.0–23.3`)

Build the first complete learning vertical slice:

1. evidence bundle and trigger policy;
2. Codex/OpenCode reflection evaluator;
3. insight and learning-proposal lifecycle;
4. approval, apply, reuse, and effectiveness measurement.

Detailed design: [INSIGHT_LEARNING_LOOP.md](INSIGHT_LEARNING_LOOP.md).

### 23B — Universal Skill Registry (`23.4–23.7`)

Adopt AgentSkills-compatible `SKILL.md` as the portable format. Add discovery, import, verification, provenance, versioning, per-agent permissions, immutable session snapshots, and adapters for Codex, OpenCode, OpenClaw, and Hermes.

Detailed design: [UNIVERSAL_SKILLS.md](UNIVERSAL_SKILLS.md).

### 23C — Persistent Agents and Sessions (`23.8–23.11`)

Introduce persistent agent identity, roles, capabilities, lifecycle, goals, sessions, inboxes, delegation, and worker/model bindings.

Detailed design: [PERSISTENT_AGENTS.md](PERSISTENT_AGENTS.md).

### 23D — Proactive Runtime (`23.12–23.15`)

Introduce `deond`, durable schedules, wakeups, cron, heartbeats, active hours, event hooks, concurrency policies, idempotency, timeout/cancellation, and orphan recovery.

Detailed design: [PROACTIVE_RUNTIME.md](PROACTIVE_RUNTIME.md).

**Status:** **merged and operational on `main`** (PR #8). Foundation `5224005` plus hardening `23.15.1–23.15.4`:

| Slice | Scope |
|-------|--------|
| 23.15.1 | Persisted `NextDueAt`, materialize only `due_at <= now`, catch-up/max lateness |
| 23.15.2 | Atomic SQLite wakeup claims (`UPDATE … WHERE status='queued'`) |
| 23.15.3 | Skill approval before active, snapshot hash/revision validation, path containment, delegation path subset + privilege guard |
| 23.15.4 | `make proactive-runtime-smoke` in CI, docs/AGENTS reconciliation |

Long-running `deonclawd` process and automatic dispatch from wakeups remain deferred to `23.28–23.31`.

### 23E — Work Queue, Costs, and Budgets (`23.16–23.19.1`)

Add atomic checkout, leases, usage normalization, cost events, budget reservations, warning thresholds, hard stops, and budget-aware routing.

Detailed design: [BUDGETS_AND_COSTS.md](BUDGETS_AND_COSTS.md), [BUDGETED_DISPATCH.md](BUDGETED_DISPATCH.md).

**Status:** **merged and operational on `main`** (PR #9) — 23.16.1 queue hardening (inbox→queued, task snapshots, lease renew, transactional release/recover, read-only doctor), 23.17 usage/pricing ledger, 23.18 atomic budget reservations, 23.19 budgeted fake dispatch + insight review queue, 23.19.1 dispatch hardening (auto-claim lease, mandatory budget policy, atomic usage+budget commit, skill snapshot materialization). CI: `make work-queue-smoke`, `make budgeted-dispatch-smoke`. `ModeReal` remains blocked in CI while `23.24–23.27` hardens manual real dispatch.

| Task | State |
|------|--------|
| 23A insight kernel | foundation → **integrated** (reviewer queue, manual proposal CLI) |
| 23B skill registry | foundation → **integrated** (dispatch skill auto-loading) |
| 23C persistent agents | **integrated** (queue dispatch linkage) |
| 23D proactive runtime | **merged** (`run-once` foundation; long-running deferred) |
| 23E budgeted dispatch | **merged and operational** (fixture smokes green on `main`) |

### Post-23E execution order (reconciled)

Sprint numbers below supersede the original 23F/23G labels in [OPENCLAW_MIGRATION.md](OPENCLAW_MIGRATION.md) and [RICH_RUNTIME_ACTIONS.md](RICH_RUNTIME_ACTIONS.md) for **execution sequencing**. Design docs keep their original section numbering.

#### 23.20–23.23 — Closed Learning Loop Operationalization

Transform `insight_review` into a reusable fake E2E path: reviewer response fixture in the queue, automatic `InsightReport` and `LearningProposal` materialization, operator approval, governed apply preview/result artifacts, a planned session-refresh snapshot, and automatic effectiveness records.

**Status:** **23.20-23.23 implemented in the budgeted-dispatch fixture path** — fake `insight_review` dispatch can load an explicit reviewer response fixture through `work dispatch-once --insight-policy ... --reviewer-response ...` and materialize `insight-report.json` plus `learning-proposals.json` artifacts linked to the parent evidence bundle. When `--learning-approval-decision ... --learning-approval-reason ...` are supplied, it also writes governed `learning-approval-<proposal>.json` artifacts bound to each proposal. When `--learning-confirm-apply` is supplied with that approval, it writes `learning-apply-preview-<proposal>.md`, `learning-apply-result-<proposal>.json`, `learning-effectiveness-<proposal>.json`, `learning-session-refresh.json`, and `learning-session-snapshot-<session>.json` artifacts without mutating canonical learning targets. Real skill registry apply remains gated behind later integration.

Detailed design: [INSIGHT_LEARNING_LOOP.md](INSIGHT_LEARNING_LOOP.md).

**Ready when:** run → evidence → `insight_review` → proposal → approve → apply preview/result → planned session refresh → effectiveness recorded.

#### 23.24–23.27 — Real Codex/OpenCode Dispatch Adapter

Wire `dispatch.ModeReal` to existing Codex/OpenCode runners behind lease, budget, task snapshot, and skill snapshot gates. Require `--confirm-worker-dispatch`; block real mode in CI. Preserve path policy, artifacts, validation commands, memory policy, lease renewal on long runs, usage capture (with estimate fallback), and basic cancellation/timeout.

**Status:** **23.24-23.26 implemented** — `ModeReal` no longer stops before operational gates. Without `--confirm-worker-dispatch`, dispatch returns a blocked `real_dispatch_requires_confirmation` result without starting a worker. In `CI`/`GITHUB_ACTIONS`, it returns `real_dispatch_blocked_in_ci` even with confirmation. Outside CI, manual dispatch reaches Codex/OpenCode worker adapters only after lease, budget reservation, task snapshot validation, and skill snapshot materialization. Worker events and artifacts are persisted under the active dispatch run instead of creating a competing run. Real dispatch now renews the active lease while the worker runs, rejects expired explicit leases, revalidates lease ownership before budget/usage commit, and supports `--timeout-seconds` plus `--lease-ttl-seconds` for controlled manual runs. Fixture smokes remain fake-only; `23.27` remains for final real-run hardening before `deonclawd`.

#### 23.28–23.31 — `deonclawd` Long-Running Runtime

Real long-running process, configurable loop interval, graceful shutdown, robust PID/state, internal `run-once`, recovery loop, schedule → work → dispatch automation, operational heartbeat, pause/resume, logs and doctor, dry-run/shadow mode.

Detailed design: [PROACTIVE_RUNTIME.md](PROACTIVE_RUNTIME.md).

#### 23.32–23.35 — OpenClaw Migration Inspect/Plan/Shadow

Inventory, skills import plan, heartbeat/cron migration plan, memory/instructions migration proposal, shadow mode without assuming execution, OpenClaw vs DeonClaw behavior diff.

Detailed design: [OPENCLAW_MIGRATION.md](OPENCLAW_MIGRATION.md) (original 23F scope).

#### 23.36–23.39 — OpenClaw Cutover/Rollback

Explicit cutover and rollback only after shadow mode is stable.

#### 23.40–23.43 — Rich Runtime Actions

`deonctl actions list/show/plan/run`; governed Git and browser actions; review actions; action artifacts; UI-ready metadata.

Detailed design: [RICH_RUNTIME_ACTIONS.md](RICH_RUNTIME_ACTIONS.md) (original 23G scope).

## Dependency graph

```text
23A Insight Kernel
  └── feeds learning proposals into 23B skills and memory

23B Skill Registry
  └── required for immutable session skill snapshots

23C Persistent Agents
  ├── required by heartbeat ownership
  ├── required by budgets per agent
  └── required by OpenClaw agent migration

23D Proactive Runtime
  ├── requires persistent agents and sessions
  └── provides schedules and wakeups for migration shadow mode

23E Queue and Budgets
  ├── merged on main (23.16–23.19.1)
  └── required before real dispatch and unattended execution

23.20–23.23 Closed Learning Loop
  └── fake E2E before real reviewer workers; report/proposal/approval/apply-preview/effectiveness/session-refresh artifacts done

23.24–23.27 Real Dispatch Adapter
  ├── requires 23E + closed learning loop fixture path
  └── 23.24-23.25 manual confirmed adapter + worker output persistence done; lease renewal/cancellation still next

23.28–23.31 deonclawd
  ├── requires real dispatch adapter
  └── schedule → work → dispatch automation

23.32+ OpenClaw Migration
  ├── passive inventory may begin earlier
  └── execution cutover requires 23C–23E and stable deonclawd

23.40+ Rich Actions
  ├── can begin with read-only Git/browser actions
  └── mutating actions require queue, budget, and approval enforcement
```

## Required vertical slices

Each epic must close at least one real end-to-end path. Schema-only completion is not sufficient.

### Learning slice

```text
OpenCode changes code
→ validations and events are captured
→ Codex reviews evidence
→ insight is created
→ skill proposal is approved
→ skill is installed
→ a later Codex/OpenCode session loads it
→ an eval records whether the behavior improved
```

### Proactivity slice

```text
schedule is stored
→ deond restarts
→ wakeup remains durable
→ assigned agent receives the work once
→ lease and budget are reserved
→ worker runs
→ result and cost are recorded
→ heartbeat reports only when action is needed
```

### Migration slice

```text
OpenClaw inventory
→ deterministic migration plan
→ passive skill and memory import
→ shadow heartbeat comparison
→ explicit cutover
→ rollback restores OpenClaw ownership
```

## Stability principles

1. **Pinned dependencies and skills.** No silent updates.
2. **Git and Markdown remain canonical** for user-controlled memory and skills.
3. **SQLite owns operational state** such as queues, schedules, sessions, leases, and costs.
4. **Every mutation is attributable** to an agent, operator, run, approval, and evidence set.
5. **Automatic evaluation is allowed; automatic application is not the default.**
6. **No hidden chain-of-thought storage.** Store evidence, observations, conclusions, and proposals only.
7. **Fail closed** on missing policy, stale hashes, budget exhaustion, unknown skills, or broken provenance.
8. **Shadow before cutover.** Migrations and proactive behaviors must support dry-run and comparison modes.
9. **Adapters are replaceable.** Codex, OpenCode, browser, channels, and future runtimes do not own core state.
10. **Operational progress over gate proliferation.** New gates must protect a real capability, not substitute for it.

## Cross-cutting artifacts

Every Epic 23 feature should produce or update:

- structured events;
- a machine-readable result artifact;
- a human-readable report where useful;
- source hashes and provenance;
- negative tests for boundary violations;
- an end-to-end fixture;
- a doctor/report command;
- documentation and migration notes;
- a CI target or extension to an existing smoke target.

## Definition of Done for every sprint

A sprint is complete only when all applicable items are satisfied:

- implementation and narrow interfaces;
- CLI wiring and helpful missing-flag errors;
- SQLite migration or explicit reason no migration is needed;
- unit tests including negative cases;
- library e2e and CLI e2e;
- fixture/example config;
- documentation updated in the same sprint;
- `gofmt -w .`;
- `git diff --check`;
- `go test ./...`;
- relevant smoke target passes;
- feature branch, push, draft PR, CI review;
- no secrets, memory vault contents, or generated credentials committed.

## Documentation ownership

When an implementation changes an epic contract, update:

- this roadmap if sequencing or scope changed;
- the corresponding detailed epic document;
- `README.md` status and links;
- `AGENTS.md` if agent workflow or repository rules changed;
- relevant ADR/checklist documents;
- fixture README and smoke script when the executable path changed.

## Deferred work

The following are intentionally after the operational foundation:

- full dashboard and mobile surfaces;
- multi-company isolation;
- unrestricted autonomous skill application;
- unrestricted provider/network dispatch;
- automatic MCP mutation without review;
- model fine-tuning or reinforcement learning;
- public skill marketplace hosting.

These may be designed early but must not delay the first stable personal runtime.
