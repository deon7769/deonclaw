# ADR — Retrieval runner materialized context injection (design record)

Task 22.14 — design decision record only. No runner, task schema, or execution changes.

## Status

Accepted (design frozen; **not implemented**).

## 1. Context

### Pipeline today (Tasks 22.8–22.13)

DeonClaw has a governed retrieval-context pipeline separate from canonical memory writes:

~~~text
retrieval-context (runner metadata attachment)
  -> materialize
  -> materialized-report
  -> bundle
  -> approval request
  -> approval
  -> injection-plan
  -> governance-report
  -> fixture e2e + release checklist
~~~

| Task | Capability |
|------|------------|
| 22.8 | Passive `retrieval_context` on tasks; runner validates search-report metadata only |
| 22.8.1 | `retrieval context inspect`, `runs retrieval-report` |
| 22.9 | `retrieval context materialize` from memory-index chunks JSONL |
| 22.9.1 | `materialized-report`, materialize path hardening |
| 22.10 | `retrieval context bundle` (hashes, no text) |
| 22.11 | Approval workflow (`manual_review_only`) |
| 22.12 | `injection-plan` (plan-only, `can_inject_now: false`) |
| 22.12.1 | `governance-report` (chain audit) |
| 22.12.2 | `configs/examples/retrieval-context-fixture/` + e2e tests |
| 22.13 | [RETRIEVAL_GOVERNANCE_CHECKLIST.md](RETRIEVAL_GOVERNANCE_CHECKLIST.md) |
| 22.15 | `retrieval context injection-policy` validate/plan (schema only, no execution) |
| 22.16 | `retrieval context injection-approval` new/approve/inspect (runner injection approval artifact, no execution) |
| 22.17 | `retrieval context injection-execution-plan` (execution plan only, no worker execution) |
| 22.18 | `retrieval context prompt-preview` (materialized prompt section preview, no worker execution) |
| 22.18.1 | `retrieval context prompt-preview-report` (prompt preview QA, no worker execution) |
| 22.18.2 | `retrieval context injection-governance-bundle` (injection governance release bundle, no worker execution) |
| 22.19 | Task YAML `retrieval_context.materialized_injection` declaration (schema validation only, no runner execution) |

### What already exists

- **Metadata-only runner attachment** — workers receive `Retrieved context metadata only` (chunk IDs, distances, hashes). No chunk text in prompts.
- **Materialized text as governed artifact only** — `text_excerpt` lives in materialized JSON/MD produced by explicit `materialize` + `--confirm-include-chunk-text`. Not auto-loaded by the runner.
- **Governance chain** — approval, injection-plan, and governance-report bind SHA256s across artifacts and enforce `can_inject_now: false` and `runner_injection_allowed: false`.

See [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md) for CLI details.

## 2. Current decision

- **Runner injection of materialized context is not implemented** and must not be implied by existing task YAML or CLI defaults.
- **`can_inject_now` must remain `false`** in injection-plan and governance-report until a future task explicitly enables execution.
- **`runner_injection_allowed` must remain `false`** on materialized-context approval artifacts (`manual_review_only`) until a dedicated injection-approval artifact authorizes injection with `runner_injection_allowed: true`.
- Any future injection work requires a **new ADR amendment or successor task**; this record does not authorize shipping injection by itself.

## 3. Future options (not chosen yet)

| Option | Description | Trade-offs |
|--------|-------------|------------|
| **A. Manual review only** | Keep today’s model: materialized artifacts for human/agent review outside the runner prompt. | Safest; no prompt bloat; no new runner surface. |
| **B. Task schema opt-in** | Extend task YAML (e.g. `retrieval_context.materialized_injection`) referencing approved artifacts and caps. | Declarative per task; visible in task files; needs schema + validation. |
| **C. Run-time CLI flag** | `deonctl worker … run` with e.g. `--confirm-inject-materialized-context` + artifact paths. | Explicit per run; not persisted in task; good for one-off experiments. |
| **D. Separate policy file** | YAML policy listing allowed materialized artifacts, caps, and approval requirements. | Reusable across tasks; another config surface to audit. |

No option is selected for implementation in Task 22.14. Option **B** and **C** are the most likely combination for a first execution task if injection is approved later.

## 4. Minimum requirements for any future injection

All of the following would be **mandatory** before runner execution could include materialized text:

1. **Injection-approval artifact** with `runner_injection_allowed: true` and `allowed_use: runner_injection_policy_only`, bound to governance-report/policy/bundle/materialized hashes (Task 22.16). Materialized-context approval (`manual_review_only`) does not authorize runner injection.
2. **Explicit confirm flag** — e.g. `--confirm-inject-materialized-context` on run or materialize-equivalent gate; no silent opt-in.
3. **Hard cap on injected characters** — enforce `max_total_chars` / `max_chars_per_chunk` at injection time (stricter than materialize limits if needed).
4. **`governance-report` status `ok`** (or `warning` with documented acceptance) immediately before run dispatch.
5. **Injection-plan reviewed** — plan artifact present; `reason` and `required_future_flag` satisfied for the execution path.
6. **Execution trace fields** — at minimum `materialized_context_sha256` (and related attachment flags) on `execution-trace.json`.
7. **Clearly marked prompt section** — e.g. `## Governed materialized retrieval context` distinct from metadata-only section; workers must not confuse with canonical memory.
8. **No active search** in runner — injection reads pre-materialized artifacts only.
9. **No provider API** calls in the injection path.
10. **No source Markdown reads** — text only from validated materialized artifact + chunks JSONL lineage already audited.

## 5. Risks

| Risk | Mitigation direction |
|------|---------------------|
| **Prompt bloat** | Hard char caps; omit or truncate with auditable warnings; keep metadata-only default. |
| **Stale chunks** | Hash chain from retrieval-context → materialized → approval; reject stale artifacts at run time. |
| **Source-of-truth confusion** | Mark prompt section; never write injected text to canonical memory; index remains derived. |
| **Sensitive text leakage** | Same leak rules as materialize; governance-report and bundle must not echo `text_excerpt`; trace metadata only. |
| **Accidental bypass of memory governance** | Injection ≠ memory apply; no shortcut around proposal/approval/backup for canonical writes. |

## 6. Non-goals

Future injection design and implementation must **not** include:

- active retrieval or LanceDB search inside the runner
- natural-language query embedding or search
- automatic memory writes from injected context
- MCP tool execution or MCP-driven injection
- UI/dashboard for injection control (CLI/artifacts first)

## 7. Next implementation candidate

**Task 22.15 (implemented):** *Runner injection policy schema only* — `configs/examples/retrieval-injection-policy.yaml` plus `deonctl retrieval context injection-policy validate/plan`. **Still no runner execution**, no prompt changes, no task schema wiring.

**Task 22.16 (implemented):** *Runner injection approval artifact* — `deonctl retrieval context injection-approval new/approve/inspect` creates a dedicated approval artifact with `runner_injection_allowed: true` after governance-report and injection-policy validation. **Still no runner execution**, no prompt changes, no task schema wiring.

**Task 22.17 (implemented):** *Runner injection execution-plan* — `deonctl retrieval context injection-execution-plan` binds injection-policy, governance-report, injection-approval, and materialized artifacts into a plan-only execution artifact with `would_execute_runner: false` and `execution_supported_now: false`. **Still no worker execution**, no prompt changes, no task schema wiring.

**Task 22.18 (implemented):** *Materialized prompt section preview* — `deonctl retrieval context prompt-preview` renders a dry-run markdown preview and manifest from a valid execution-plan and materialized artifact. **Still no worker execution**, no runner prompt injection, no task schema wiring.

**Task 22.18.1 (implemented):** *Prompt preview report/QA* — `deonctl retrieval context prompt-preview-report` validates preview markdown and manifest metadata without printing chunk text. **Still no worker execution**, no runner prompt injection, no task schema wiring.

**Task 22.18.2 (implemented):** *Injection governance release bundle* — `deonctl retrieval context injection-governance-bundle` consolidates the full injection governance chain into a metadata-only release bundle JSON and summary. **Still no worker execution**, no runner prompt injection, no task schema wiring.

**Task 22.19 (implemented):** *Task schema declaration for materialized injection* — task YAML `retrieval_context.materialized_injection` declares future governed injection with governance bundle binding and schema validation only. **Still no runner execution**, no prompt changes, no worker materialized text injection.

**Task 22.20+ (proposal):** runner injection execution behind explicit confirm flag at worker dispatch time.

Until a future execution task ships, the codebase remains at metadata-only runner attachment plus governed offline artifacts and plan-only injection policy.

## References

- [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md)
- [RETRIEVAL_GOVERNANCE_CHECKLIST.md](RETRIEVAL_GOVERNANCE_CHECKLIST.md)
- [MEMORY_INDEX.md](MEMORY_INDEX.md) — Task 22.x table
- [configs/examples/retrieval-context-fixture/README.md](../configs/examples/retrieval-context-fixture/README.md)

## Boundary

Task 22.14 is documentation only. Memory apply/restore, runner code, task schema, LanceDB search, and provider integrations are unchanged.
