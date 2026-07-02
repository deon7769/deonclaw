package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	artifactspkg "github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/config"
	configenvpkg "github.com/deon7769/deonclaw/internal/configenv"
	"github.com/deon7769/deonclaw/internal/contextpack"
	doctorpkg "github.com/deon7769/deonclaw/internal/doctor"
	"github.com/deon7769/deonclaw/internal/domains"
	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/mcpconfig"
	"github.com/deon7769/deonclaw/internal/mcpproposalqueue"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/memory"
	"github.com/deon7769/deonclaw/internal/memoryindex"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/runner"
	"github.com/deon7769/deonclaw/internal/runreport"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workerconfig"
	"github.com/deon7769/deonclaw/internal/workers"
	"github.com/deon7769/deonclaw/internal/workers/codex"
	"github.com/deon7769/deonclaw/internal/workers/opencode"
)

const usage = `deonctl - DeonClaw control CLI

Usage:
  deonctl version
  deonctl doctor [--output-format text|json] [--store <path>] [--artifacts-dir <path>] [--workers-config <path>] [--profiles]
  deonctl workers doctor [--worker codex|opencode] [--output-format text|json] [--store <path>] [--artifacts-dir <path>] [--workers-config <path>] [--profiles]
  deonctl workers smoke --worker opencode --task <task.yaml> --store <path> --artifacts-dir <path> --workers-config <path> [--domains <domains.yaml>] [--memory-policy <policy.yaml>] [--dry-run]
  deonctl config env [--output-format text|json]
  deonctl task validate <path>
  deonctl task materialized-injection-report --task <path> [--output-format text|json]
  deonctl domains validate --config <path>
  deonctl domains list --config <path>
  deonctl context build --task <task.yaml> --domains <domains.yaml> --output <path>
  deonctl runtime validate --config <runtime.yaml>
  deonctl runtime docker-plan --config <runtime.yaml> --workspace <path> [--output-format text|json]
  deonctl runtime docker-exec --config <runtime.yaml> --workspace <path> -- <command> [args...]
  deonctl mcp validate --config <mcp.yaml>
  deonctl mcp list --config <mcp.yaml>
  deonctl mcp plan --config <mcp.yaml> --server <name> [--output-format text|json]
  deonctl mcp doctor --config <mcp.yaml> [--output-format text|json]
  deonctl mcp risk --config <mcp.yaml> [--output-format text|json]
  deonctl mcp docker-plan --config <mcp.yaml> --server <name> --runtime-config <runtime.yaml> --workspace <path> [--output-format text|json]
  deonctl mcp fake-server
  deonctl mcp smoke --config <mcp.yaml> --server <name> --artifacts-dir <dir> [--timeout-seconds 5] [--runtime local|docker] [--runtime-config <runtime.yaml>] [--workspace <path>]
  deonctl mcp tool-smoke --config <mcp.yaml> --server <name> --tool <name> --arguments <json> --artifacts-dir <dir> [--timeout-seconds 5] [--runtime local|docker] [--runtime-config <runtime.yaml>] [--workspace <path>] [--policy <policy.yaml>]
  deonctl mcp discover --config <mcp.yaml> --server <name> --artifacts-dir <dir> --policy <policy.yaml> [--timeout-seconds 5] [--runtime local|docker] [--runtime-config <runtime.yaml>] [--workspace <path>]
  deonctl mcp call-smoke --config <mcp.yaml> --server <name> --tool <name> --arguments <json> --artifacts-dir <dir> --policy <policy.yaml> [--timeout-seconds 5] [--runtime docker] [--runtime-config <runtime.yaml>] [--workspace <path>]
  deonctl mcp proposal new --server <name> --tool <tool> --arguments <json> --reason <text> --policy <policy.yaml> [--config <mcp.yaml>] --runtime docker --runtime-config <runtime.yaml> --workspace <path> --output <proposal.json>
  deonctl mcp proposal inspect --proposal <proposal.json> [--output-format text|json]
  deonctl mcp proposal lint --proposal <proposal.json> --config <mcp.yaml> --policy <policy.yaml>
  deonctl mcp proposal preflight --proposal <proposal.json> --config <mcp.yaml> --policy <policy.yaml> --output <preflight.json>
  deonctl mcp proposal approve --proposal <proposal.json> --policy <policy.yaml> --decision approved|rejected --reason <text> --output <approval.json> --confirm-read-only
  deonctl mcp proposal execute --proposal <proposal.json> --approval <approval.json> --config <mcp.yaml> --policy <policy.yaml> --artifacts-dir <dir> --confirm-execute
  deonctl mcp approval inspect --approval <approval.json> [--output-format text|json]
  deonctl mcp proposals list --store <deonclaw.db> [--status valid|invalid|preflight_passed|preflight_failed|refused] [--worker codex|opencode] [--task <task-id>] [--output-format text|json]
  deonctl mcp proposals show --store <deonclaw.db> --run <run-id> [--output-format text|json]
  deonctl mcp proposals export --store <deonclaw.db> --run <run-id> --output <proposal.json>
  deonctl memory proposal new --run <run-id> --task <task-id> --domain <domain> --target <path> --operation <operation> --reason <text> --output <path>
  deonctl memory proposal lint --proposal <path> --policy <path>
  deonctl memory proposal apply --proposal <path> --policy <path> --dry-run [--output <path>]
  deonctl memory proposal approve --proposal <path> --policy <path> --reviewer <name> --decision approved|rejected --reason <text> --output <path>
  deonctl memory proposal apply-preflight --proposal <path> --approval <path> --policy <path> [--output <path>]
  deonctl memory proposal backup-plan --proposal <path> --approval <path> --policy <path> --output <path>
  deonctl memory proposal backup-materialize --backup-plan <path> --output <path>
  deonctl memory proposal restore --backup-plan <path> --backup-result <path> --dry-run --output <path>
  deonctl memory proposal restore-execute --backup-plan <path> --backup-result <path> --restore-preview <path> --output <path> --confirm-restore
  deonctl memory proposal apply-execute --proposal <path> --approval <path> --policy <path> --backup-plan <path> --backup-result <path> --output <path> --confirm-apply
  deonctl memory index validate --config <memory-index.yaml>
  deonctl memory index plan --config <memory-index.yaml> [--output-format text|json]
  deonctl memory index build --config <memory-index.yaml> --artifacts-dir <dir>
  deonctl memory index doctor --config <memory-index.yaml> [--output-format text|json]
  deonctl memory index report --manifest <memory-index-manifest.json> --chunks <memory-index-chunks.jsonl> [--output-format text|json]
  deonctl memory embedding validate --policy <embedding-policy.yaml>
  deonctl memory embedding doctor --policy <embedding-policy.yaml> [--output-format text|json]
  deonctl memory embedding plan --policy <embedding-policy.yaml> [--output-format text|json]
  deonctl memory embedding build-fake --policy <embedding-policy-fake.yaml> --artifacts-dir <dir> --confirm-fake-vectors
  deonctl memory embedding report --manifest <memory-embedding-manifest.json> --vectors <memory-index-vectors.jsonl> [--chunks <memory-index-chunks.jsonl>] [--output-format text|json]
  deonctl memory lancedb validate --policy <lancedb-policy.yaml>
  deonctl memory lancedb plan --policy <lancedb-policy.yaml> [--output-format text|json]
  deonctl memory lancedb fake-write --policy <lancedb-policy-fake.yaml> --artifacts-dir <dir> --confirm-fake-write
  deonctl memory lancedb write-smoke --policy <lancedb-policy-write-smoke.yaml> --artifacts-dir <dir> --confirm-lancedb-write [--allow-overwrite-smoke]
  deonctl memory lancedb doctor --policy <lancedb-policy-write-smoke.yaml> [--output-format text|json]
  deonctl memory lancedb report --manifest <lancedb-write-smoke-manifest.json> --policy <lancedb-policy-write-smoke.yaml> [--output-format text|json]
  deonctl memory lancedb search-smoke --policy <lancedb-policy-write-smoke.yaml> (--query-vector <json-array>|--query-chunk-id <chunk-id>) --top-k <n> --artifacts-dir <dir> --confirm-search-smoke
  deonctl memory lancedb search-report --result <lancedb-search-smoke-result.json> --policy <lancedb-policy-write-smoke.yaml> [--output-format text|json]
  deonctl insights policy validate --config <insight-policy.yaml>
  deonctl insights evidence build --store <path> --run <run-id> --output <evidence-bundle.json> [--trigger <trigger>] [--policy-ref <path>]
  deonctl insights trigger evaluate --config <insight-policy.yaml> --evidence <evidence-bundle.json> [--output-format text|json]
  deonctl insights evaluate dry-run --evidence <evidence-bundle.json> --reviewer codex|opencode --prompt-output <prompt.txt> --plan-output <evaluation-plan.json>
  deonctl insights evaluate --evidence <evidence-bundle.json> --reviewer codex|opencode --response <reviewer-response.json> --output <insight-report.json>
  deonctl insights proposals materialize --insight <insight-report.json> --response <reviewer-response.json> --output <learning-proposals.json> [--config <insight-policy.yaml>]
  deonctl insights proposals list --bundle <learning-proposals.json> [--output-format text|json]
  deonctl insights proposals show --bundle <learning-proposals.json> --proposal <proposal-id> [--output-format text|json]
  deonctl insights proposals approve --bundle <learning-proposals.json> --proposal <proposal-id> --reviewer codex|opencode --decision approved|rejected --reason <text> --output <approval.json> [--bundle-output <learning-proposals.json>]
  deonctl insights proposals apply dry-run --bundle <learning-proposals.json> --proposal <proposal-id> [--approval <approval.json>] --output <apply-dry-run.json>
  deonctl insights proposals apply --bundle <learning-proposals.json> --proposal <proposal-id> --approval <approval.json> --preview-output <preview.md> --output <apply-result.json> --confirm-apply [--bundle-output <learning-proposals.json>]
  deonctl insights effectiveness record --bundle <learning-proposals.json> --proposal <proposal-id> --run <run-id> --metric validation_pass_rate|repeated_failure_count|time_to_fix_ms --value <float> --output <effectiveness.json> [--effectiveness <effectiveness.json>]
  deonctl insights effectiveness report --effectiveness <effectiveness.json> [--output-format text|json]
  deonctl insights timeline --bundle <learning-proposals.json> [--effectiveness <effectiveness.json>] [--output-format text|json]
  deonctl insights report --insight <insight-report.json> [--output-format text|json]
  deonctl skills policy validate --config <skill-policy.yaml>
  deonctl skills inspect <path|git:owner/repo@ref> [--output-format text|json]
  deonctl skills import <path|git:owner/repo@ref> --output <skill-import-plan.json> [--policy <skill-policy.yaml>]
  deonctl skills install <path> --registry-root <dir> [--as <name>] [--policy <skill-policy.yaml>] [--output <install-result.json>]
  deonctl skills approve <name> --registry-root <dir>
  deonctl skills verify <name> --registry-root <dir> [--output-format text|json]
  deonctl skills list --registry-root <dir> [--output-format text|json]
  deonctl skills show <name> --registry-root <dir> [--output-format text|json]
  deonctl skills enable <name> --agent <agent-id> --registry-root <dir>
  deonctl skills disable <name> --agent <agent-id> --registry-root <dir>
  deonctl skills snapshot --agent <agent-id> --session <session-id> --registry-root <dir> --output <snapshot.json> [--policy <skill-policy.yaml>]
  deonctl skills materialize --snapshot <snapshot.json> --workspace <path> --registry-root <dir> [--output <materialize-result.json>]
  deonctl agents validate --config <agents.yaml> [--workers-config <workers.yaml>]
  deonctl agents sync --config <agents.yaml> --store <deonclaw.db> [--workers-config <workers.yaml>]
  deonctl agents list --store <deonclaw.db>
  deonctl agents show <agent-id> --store <deonclaw.db>
  deonctl agents pause <agent-id> --store <deonclaw.db> [--actor <name>]
  deonctl agents resume <agent-id> --store <deonclaw.db> [--actor <name>]
  deonctl agents terminate <agent-id> --store <deonclaw.db> --approval <termination-approval.json>
  deonctl agents sessions create --agent <agent-id> --store <deonclaw.db> [--kind main|heartbeat|scratch] [--session <session-id>] [--workspace <path>] [--skill-snapshot <snapshot.json>]
  deonctl agents sessions list --agent <agent-id> --store <deonclaw.db>
  deonctl agents sessions show <session-id> --store <deonclaw.db> [--output-format text|json]
  deonctl agents assign --agent <agent-id> --task <task.yaml> --store <deonclaw.db> [--created-by <name>]
  deonctl agents inbox list --agent <agent-id> --store <deonclaw.db>
  deonctl agents inbox accept <item-id> --store <deonclaw.db>
  deonctl agents inbox defer <item-id> --store <deonclaw.db>
  deonctl agents delegate propose --parent-agent <agent-id> --child-agent <agent-id> --parent-work-item <work-item-id> --task <task.yaml> --reason <text> --output <delegation-proposal.json> --store <deonclaw.db>
  deonctl daemon status --store <deonclaw.db>
  deonctl daemon doctor --store <deonclaw.db>
  deonctl daemon start --store <deonclaw.db>
  deonctl daemon stop --store <deonclaw.db>
  deonctl daemon run-once --store <deonclaw.db> [--heartbeat-config <heartbeat.yaml>] [--hooks-config <hooks.yaml>]
  deonctl schedules validate --config <schedules.yaml>
  deonctl schedules sync --config <schedules.yaml> --store <deonclaw.db>
  deonctl schedules list --store <deonclaw.db>
  deonctl schedules due --store <deonclaw.db>
  deonctl heartbeat validate --config <heartbeat.yaml>
  deonctl heartbeat dry-run --agent <agent-id> --policy <policy-id> --config <heartbeat.yaml> [--agent-busy]
  deonctl hooks validate --config <hooks.yaml>
  deonctl hooks plan --config <hooks.yaml> --event <event-name>
  deonctl work list --store <deonclaw.db>
  deonctl work show <work-item-id> --store <deonclaw.db>
  deonctl work claim --store <deonclaw.db> --agent <agent-id> [--work-item <id>] [--ttl-seconds <n>]
  deonctl work release --store <deonclaw.db> --lease <lease-id> [--reason <text>] [--requeue]
  deonctl work leases list --store <deonclaw.db>
  deonctl work leases renew --store <deonclaw.db> --lease <lease-id> [--ttl-seconds <n>]
  deonctl work recover --store <deonclaw.db>
  deonctl work doctor --store <deonclaw.db>
  deonctl work dispatch-once --store <deonclaw.db> --work-item <id> [--lease <lease-id>] --artifacts-dir <dir> --registry-root <dir> --skill-policy <skill-policy.yaml> [--insight-policy <insight-policy.yaml> --reviewer-response <reviewer-response.json>] [--learning-approval-decision approved|rejected --learning-approval-reason <text> [--learning-approval-reviewer codex|opencode] [--learning-confirm-apply]] [--mode fake|real] [--confirm-worker-dispatch] [--timeout-seconds <seconds>] [--lease-ttl-seconds <seconds>]
  deonctl pricing validate --config <model-prices.yaml>
  deonctl pricing sync --config <model-prices.yaml> --store <deonclaw.db>
  deonctl pricing list --store <deonclaw.db>
  deonctl pricing show <model-profile-id> --store <deonclaw.db>
  deonctl usage record --store <deonclaw.db> --input <usage.json>
  deonctl usage show <usage-event-id> --store <deonclaw.db>
  deonctl usage report --store <deonclaw.db> --by agent|worker|model_profile [--since <RFC3339|YYYY-MM-DD>]
  deonctl budgets validate --config <budgets.yaml>
  deonctl budgets sync --config <budgets.yaml> --store <deonclaw.db>
  deonctl budgets status --store <deonclaw.db> [--agent <agent-id>]
  deonctl budgets plan --config <budgets.yaml> [--agent <agent-id>]
  deonctl budgets reserve --store <deonclaw.db> --work-item <id> --policy <policy-id> [--estimate-usd <n>]
  deonctl budgets commit --store <deonclaw.db> --reservation <id> --usage <usage.json>
  deonctl budgets release --store <deonclaw.db> --reservation <id> [--reason <text>]
  deonctl budgets report --store <deonclaw.db> [--agent <agent-id>]
  deonctl budgets override new --policy <policy-id> --output <approval.json> [--agent <id>] [--work-item <id>] [--reviewer <name>] [--reason <text>] [--requested-microusd <n>]
  deonctl budgets override approve --store <deonclaw.db> --input <approval.json>
  deonctl budgets override inspect <approval-id> --store <deonclaw.db>
  deonctl work-templates validate --config <work-templates.yaml>
  deonctl work-templates sync --config <work-templates.yaml> --store <deonclaw.db>
  deonctl work-templates list --store <deonclaw.db>
  deonctl runs retrieval-report --store <path> [--run <run-id>] [--output-format text|json]
  deonctl retrieval context inspect --artifact <retrieval-context.json> [--output-format text|json]
  deonctl retrieval context materialize --retrieval-context <retrieval-context.json> --chunks <memory-index-chunks.jsonl> --output <materialized.json> --summary <materialized.md> --max-chars-per-chunk <n> --max-total-chars <n> --confirm-include-chunk-text
  deonctl retrieval context materialized-report --artifact <retrieval-context-materialized.json> [--output-format text|json]
  deonctl retrieval context bundle --retrieval-context <retrieval-context.json> --materialized <retrieval-context-materialized.json> --output <retrieval-context-bundle.json> --summary <retrieval-context-bundle.md> [--output-format text|json]
  deonctl retrieval context approval new --bundle <retrieval-context-bundle.json> --output <retrieval-context-approval-request.json>
  deonctl retrieval context approval approve --request <retrieval-context-approval-request.json> --output <retrieval-context-approval.json> --confirm-approve-materialized-context
  deonctl retrieval context approval inspect --approval <retrieval-context-approval.json> [--output-format text|json]
  deonctl retrieval context injection-plan --approval <retrieval-context-approval.json> --request <retrieval-context-approval-request.json> --bundle <retrieval-context-bundle.json> --materialized <retrieval-context-materialized.json> --output <retrieval-context-injection-plan.json> --summary <retrieval-context-injection-plan.md>
  deonctl retrieval context injection-policy validate --policy <retrieval-injection-policy.yaml>
  deonctl retrieval context injection-policy plan --policy <retrieval-injection-policy.yaml> [--output-format text|json]
  deonctl retrieval context injection-approval new --governance-report <retrieval-context-governance-report.json> --policy <retrieval-injection-policy.yaml> --output <retrieval-context-injection-approval-request.json>
  deonctl retrieval context injection-approval approve --request <retrieval-context-injection-approval-request.json> --output <retrieval-context-injection-approval.json> --confirm-allow-runner-injection
  deonctl retrieval context injection-approval inspect --approval <retrieval-context-injection-approval.json> [--request <retrieval-context-injection-approval-request.json>] [--output-format text|json]
  deonctl retrieval context injection-execution-plan --policy <retrieval-injection-policy.yaml> --governance-report <retrieval-context-governance-report.json> --injection-approval <retrieval-context-injection-approval.json> --injection-approval-request <retrieval-context-injection-approval-request.json> --materialized <retrieval-context-materialized.json> --output <retrieval-context-injection-execution-plan.json> --summary <retrieval-context-injection-execution-plan.md>
  deonctl retrieval context prompt-preview --execution-plan <retrieval-context-injection-execution-plan.json> --policy <retrieval-injection-policy.yaml> --materialized <retrieval-context-materialized.json> --output <retrieval-context-prompt-preview.md> --manifest <retrieval-context-prompt-preview.json> --confirm-render-materialized-context
  deonctl retrieval context prompt-preview-report --preview <retrieval-context-prompt-preview.md> --manifest <retrieval-context-prompt-preview.json> --execution-plan <retrieval-context-injection-execution-plan.json> [--output-format text|json]
  deonctl retrieval context injection-governance-bundle --governance-report <retrieval-context-governance-report.json> --policy <retrieval-injection-policy.yaml> --injection-approval-request <retrieval-context-injection-approval-request.json> --injection-approval <retrieval-context-injection-approval.json> --execution-plan <retrieval-context-injection-execution-plan.json> --prompt-preview-manifest <retrieval-context-prompt-preview.json> --prompt-preview-report <retrieval-context-prompt-preview-report.json> --output <retrieval-context-injection-governance-bundle.json> --summary <retrieval-context-injection-governance-bundle.md>
  deonctl retrieval context governance-report --retrieval-context <retrieval-context.json> --materialized <retrieval-context-materialized.json> --bundle <retrieval-context-bundle.json> --request <retrieval-context-approval-request.json> --approval <retrieval-context-approval.json> --injection-plan <retrieval-context-injection-plan.json> [--output-format text|json]
  deonctl worker codex dry-run <task-path> [--workers-config <path>]
  deonctl worker codex materialized-injection-preflight --task <path> --prompt-preview-report <retrieval-context-prompt-preview-report.json> --output <materialized-injection-preflight.json> [--output-format text|json]
  deonctl worker codex materialized-injection-dry-run --task <path> --preflight <materialized-injection-preflight.json> --prompt-preview <retrieval-context-prompt-preview.md> --output <materialized-injection-dry-run.json> --prompt-output <materialized-injection-prompt-section.md> --confirm-inject-materialized-context [--output-format text|json]
  deonctl worker codex materialized-injection-dry-run-report --dry-run <materialized-injection-dry-run.json> --prompt-output <materialized-injection-prompt-section.md> [--output-format text|json]
  deonctl worker codex materialized-injection-readiness-report --preflight <materialized-injection-preflight.json> --dry-run <materialized-injection-dry-run.json> --dry-run-report <materialized-injection-dry-run-report.json> [--output-format text|json]
  deonctl worker codex materialized-injection-execution-gate --task <path> --readiness-report <materialized-injection-readiness-report.json> --prompt-output <materialized-injection-prompt-section.md> --output <materialized-injection-execution-gate.json> --confirm-inject-materialized-context [--output-format text|json]
  deonctl worker codex materialized-prompt-assembly-dry-run --execution-gate <materialized-injection-execution-gate.json> --base-prompt-fixture <base-worker-prompt-fixture.md> --prompt-output <materialized-injection-prompt-section.md> --output <materialized-prompt-assembly-dry-run.json> --assembled-output <materialized-prompt-assembly.md> --confirm-inject-materialized-context [--output-format text|json]
  deonctl worker codex materialized-prompt-assembly-report --assembly-dry-run <materialized-prompt-assembly-dry-run.json> --assembled-output <materialized-prompt-assembly.md> [--output-format text|json]
  deonctl worker codex materialized-injection-runtime validate --config <materialized-injection-runtime.yaml> [--output-format text|json]
  deonctl worker codex materialized-injection-run-plan --task <path> --runtime-config <materialized-injection-runtime.yaml> --execution-gate <materialized-injection-execution-gate.json> --assembly-report <materialized-prompt-assembly-report.json> --assembled-output <materialized-prompt-assembly.md> --output <materialized-injection-run-plan.json> --confirm-inject-materialized-context [--output-format text|json]
  deonctl worker codex materialized-injection-execution-enable validate --config <materialized-injection-execution-enable.yaml> [--output-format text|json]
  deonctl worker codex materialized-injection-provider-run-plan --task <path> --enable-config <materialized-injection-execution-enable.yaml> --execution-gate <materialized-injection-execution-gate.json> --assembly-report <materialized-prompt-assembly-report.json> --assembled-output <materialized-prompt-assembly.md> --output <materialized-injection-provider-run-plan.json> --confirm-inject-materialized-context [--output-format text|json]
  deonctl worker codex materialized-provider-dispatch validate --config <materialized-provider-dispatch.yaml> [--output-format text|json]
  deonctl worker codex materialized-provider-payload-dry-run --dispatch-config <materialized-provider-dispatch.yaml> --provider-run-plan <materialized-injection-provider-run-plan.json> --assembled-output <materialized-prompt-assembly.md> --output <materialized-provider-payload-dry-run.json> --payload-output <materialized-provider-payload.md> --confirm-inject-materialized-context [--output-format text|json]
  deonctl worker codex materialized-provider-payload-report --payload-dry-run <materialized-provider-payload-dry-run.json> --payload-output <materialized-provider-payload.md> [--output-format text|json]
  deonctl worker codex materialized-provider-call-gate --dispatch-config <materialized-provider-dispatch.yaml> --payload-report <materialized-provider-payload-report.json> --payload-output <materialized-provider-payload.md> --confirm-inject-materialized-context --output <materialized-provider-call-gate.json> [--output-format text|json]
  deonctl worker codex materialized-provider-call-readiness-report --provider-call-gate <materialized-provider-call-gate.json> --payload-report <materialized-provider-payload-report.json> [--output-format text|json]
  deonctl worker codex provider-call-approval new --readiness-report <provider-call-readiness-report.json> --provider-call-gate <provider-call-gate.json> --payload-report <payload-report.json> --output <provider-call-approval-request.json>
  deonctl worker codex provider-call-approval approve --request <provider-call-approval-request.json> --output <provider-call-approval.json> --confirm-provider-payload-sha256 <sha256>
  deonctl worker codex provider-call-approval inspect --approval <provider-call-approval.json> [--request <provider-call-approval-request.json>] [--output-format text|json]
  deonctl worker codex provider-call-execution-bundle --dispatch-config <materialized-provider-dispatch.yaml> --payload-report <payload-report.json> --provider-call-gate <provider-call-gate.json> --readiness-report <provider-call-readiness-report.json> --approval <provider-call-approval.json> --output <provider-call-execution-bundle.json> [--output-format text|json]
  deonctl worker codex provider-call-chain-audit --dispatch-config <materialized-provider-dispatch.yaml> --provider-run-plan <materialized-injection-provider-run-plan.json> --payload-dry-run <materialized-provider-payload-dry-run.json> --payload-output <materialized-provider-payload.md> --payload-report <materialized-provider-payload-report.json> --provider-call-gate <materialized-provider-call-gate.json> --readiness-report <provider-call-readiness-report.json> --approval-request <provider-call-approval-request.json> --approval <provider-call-approval.json> --execution-bundle <provider-call-execution-bundle.json> [--output-format text|json]
  deonctl worker codex provider-call-executor config validate --config <provider-call-executor.yaml> [--output-format text|json]
  deonctl worker codex provider-call-executor validate --executor-config <provider-call-executor.yaml> --dispatch-config <materialized-provider-dispatch.yaml> --provider-run-plan <materialized-injection-provider-run-plan.json> --payload-dry-run <materialized-provider-payload-dry-run.json> --payload-output <materialized-provider-payload.md> --payload-report <materialized-provider-payload-report.json> --provider-call-gate <materialized-provider-call-gate.json> --readiness-report <provider-call-readiness-report.json> --approval-request <provider-call-approval-request.json> --approval <provider-call-approval.json> --execution-bundle <provider-call-execution-bundle.json> [--output-format text|json]
  deonctl worker codex provider-call-executor plan --executor-config <provider-call-executor.yaml> --dispatch-config <materialized-provider-dispatch.yaml> --provider-run-plan <materialized-injection-provider-run-plan.json> --payload-dry-run <materialized-provider-payload-dry-run.json> --payload-output <materialized-provider-payload.md> --payload-report <materialized-provider-payload-report.json> --provider-call-gate <materialized-provider-call-gate.json> --readiness-report <provider-call-readiness-report.json> --approval-request <provider-call-approval-request.json> --approval <provider-call-approval.json> --execution-bundle <provider-call-execution-bundle.json> --output <provider-call-executor-plan.json> [--output-format text|json]
  deonctl worker codex provider-call-executor dry-run --executor-config <provider-call-executor.yaml> --dispatch-config <materialized-provider-dispatch.yaml> --provider-run-plan <materialized-injection-provider-run-plan.json> --payload-dry-run <materialized-provider-payload-dry-run.json> --payload-report <materialized-provider-payload-report.json> --provider-call-gate <materialized-provider-call-gate.json> --readiness-report <provider-call-readiness-report.json> --approval-request <provider-call-approval-request.json> --approval <provider-call-approval.json> --execution-bundle <provider-call-execution-bundle.json> --output <provider-call-executor-dry-run.json> [--output-format text|json]
  deonctl worker codex provider-call-executor dry-run-report --dry-run <provider-call-executor-dry-run.json> --executor-config <provider-call-executor.yaml> [--output-format text|json]
  deonctl worker codex provider-call-executor preflight --executor-config <provider-call-executor.yaml> --dry-run <provider-call-executor-dry-run.json> --dry-run-report <provider-call-executor-dry-run-report.json> --execution-bundle <provider-call-execution-bundle.json> --approval <provider-call-approval.json> --output <provider-call-executor-preflight.json> [--output-format text|json]
  deonctl worker codex provider-call-executor-dispatch-approval new --executor-config <provider-call-executor.yaml> --dry-run <provider-call-executor-dry-run.json> --dry-run-report <provider-call-executor-dry-run-report.json> --preflight <provider-call-executor-preflight.json> --execution-bundle <provider-call-execution-bundle.json> --approval <provider-call-approval.json> --output <provider-call-executor-dispatch-approval-request.json>
  deonctl worker codex provider-call-executor-dispatch-approval approve --request <provider-call-executor-dispatch-approval-request.json> --output <provider-call-executor-dispatch-approval.json> --confirm-executor-config-sha256 <sha256> --confirm-execution-bundle-sha256 <sha256> --confirm-provider-payload-sha256 <sha256>
  deonctl worker codex provider-call-executor-dispatch-approval inspect --approval <provider-call-executor-dispatch-approval.json> [--request <provider-call-executor-dispatch-approval-request.json>] [--output-format text|json]
  deonctl worker codex provider-transport-plan --executor-config <provider-call-executor.yaml> --executor-preflight <provider-call-executor-preflight.json> --dispatch-approval <provider-call-executor-dispatch-approval.json> --output <provider-transport-plan.json> [--output-format text|json]
  deonctl worker codex provider-executor-release-bundle --executor-config <provider-call-executor.yaml> --execution-bundle <provider-call-execution-bundle.json> --dry-run <provider-call-executor-dry-run.json> --dry-run-report <provider-call-executor-dry-run-report.json> --executor-preflight <provider-call-executor-preflight.json> --dispatch-approval <provider-call-executor-dispatch-approval.json> --transport-plan <provider-transport-plan.json> --output <provider-executor-release-bundle.json> [--output-format text|json]
  deonctl worker codex provider-executor-release-gate --executor-config <provider-call-executor.yaml> --execution-bundle <provider-call-execution-bundle.json> --dry-run <provider-call-executor-dry-run.json> --dry-run-report <provider-call-executor-dry-run-report.json> --executor-preflight <provider-call-executor-preflight.json> --dispatch-approval <provider-call-executor-dispatch-approval.json> --transport-plan <provider-transport-plan.json> --release-bundle <provider-executor-release-bundle.json> [--output-format text|json]
  deonctl worker codex provider-request-envelope dry-run --executor-config <provider-call-executor.yaml> --execution-bundle <provider-call-execution-bundle.json> --executor-preflight <provider-call-executor-preflight.json> --dispatch-approval <provider-call-executor-dispatch-approval.json> --transport-plan <provider-transport-plan.json> --release-bundle <provider-executor-release-bundle.json> --release-gate <provider-executor-release-gate.json> --output <provider-request-envelope.json> [--output-format text|json]
  deonctl worker codex provider-request-envelope report --request-envelope <provider-request-envelope.json> --executor-config <provider-call-executor.yaml> --execution-bundle <provider-call-execution-bundle.json> --executor-preflight <provider-call-executor-preflight.json> --dispatch-approval <provider-call-executor-dispatch-approval.json> --transport-plan <provider-transport-plan.json> --release-bundle <provider-executor-release-bundle.json> --release-gate <provider-executor-release-gate.json> [--output-format text|json]
  deonctl worker codex provider-adapter-registry validate --config <provider-adapters.yaml> [--output-format text|json]
  deonctl worker codex provider-adapter-plan --config <provider-adapters.yaml> --request-envelope <provider-request-envelope.json> --output <provider-adapter-plan.json> [--output-format text|json]
  deonctl worker codex provider-response-fixture generate --request-envelope <provider-request-envelope.json> --adapter-plan <provider-adapter-plan.json> --output <provider-response-fixture.json> [--output-format text|json]
  deonctl worker codex provider-response-fixture inspect --fixture <provider-response-fixture.json> [--request-envelope <provider-request-envelope.json>] [--adapter-plan <provider-adapter-plan.json>] [--output-format text|json]
  deonctl worker codex provider-execution-simulation-bundle --request-envelope <provider-request-envelope.json> --adapter-plan <provider-adapter-plan.json> --response-fixture <provider-response-fixture.json> --release-gate <provider-executor-release-gate.json> --executor-config <provider-call-executor.yaml> --output <provider-execution-simulation-bundle.json> [--output-format text|json]
  deonctl worker codex provider-execution-simulation-report --simulation-bundle <provider-execution-simulation-bundle.json> --request-envelope <provider-request-envelope.json> --adapter-plan <provider-adapter-plan.json> --response-fixture <provider-response-fixture.json> --release-gate <provider-executor-release-gate.json> --executor-config <provider-call-executor.yaml> [--output-format text|json]
  deonctl worker codex provider-credential-policy validate --config <provider-credential-policy.yaml> [--output-format text|json]
  deonctl worker codex provider-credential-policy plan --config <provider-credential-policy.yaml> --output <provider-credential-policy-plan.json> [--output-format text|json]
  deonctl worker codex provider-real-call-proposal new --execution-simulation-report <provider-execution-simulation-report.json> --request-envelope <provider-request-envelope.json> --adapter-plan <provider-adapter-plan.json> --credential-policy-plan <provider-credential-policy-plan.json> --release-gate <provider-executor-release-gate.json> --output <provider-real-call-proposal.json>
  deonctl worker codex provider-real-call-proposal inspect --proposal <provider-real-call-proposal.json> [--execution-simulation-report <...>] [--request-envelope <...>] [--adapter-plan <...>] [--credential-policy-plan <...>] [--release-gate <...>] [--output-format text|json]
  deonctl worker codex provider-response-change-proposal --response-fixture <provider-response-fixture.json> --execution-simulation-report <provider-execution-simulation-report.json> --real-call-proposal <provider-real-call-proposal.json> --output <provider-response-change-proposal.json> [--output-format text|json]
  deonctl worker codex provider-response-change-proposal-report --change-proposal <provider-response-change-proposal.json> --response-fixture <provider-response-fixture.json> --execution-simulation-report <provider-execution-simulation-report.json> --real-call-proposal <provider-real-call-proposal.json> [--output-format text|json]
  deonctl worker codex provider-activation-readiness-audit --credential-policy-plan <provider-credential-policy-plan.json> --real-call-proposal <provider-real-call-proposal.json> --change-proposal <provider-response-change-proposal.json> --execution-simulation-report <provider-execution-simulation-report.json> --release-gate <provider-executor-release-gate.json> [--output-format text|json]
  deonctl worker codex provider-activation-readiness-report --credential-policy-plan <provider-credential-policy-plan.json> --real-call-proposal <provider-real-call-proposal.json> --change-proposal <provider-response-change-proposal.json> --execution-simulation-report <provider-execution-simulation-report.json> --release-gate <provider-executor-release-gate.json> [--output-format text|json]
  deonctl worker codex provider-activation-policy validate --config <provider-activation-policy.yaml> [--output-format text|json]
  deonctl worker codex provider-activation-policy plan --config <provider-activation-policy.yaml> --output <provider-activation-policy-plan.json> [--output-format text|json]
  deonctl worker codex provider-activation-approval new --activation-policy-plan <provider-activation-policy-plan.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --credential-policy-plan <provider-credential-policy-plan.json> --execution-simulation-report <provider-execution-simulation-report.json> --output <provider-activation-approval-request.json>
  deonctl worker codex provider-activation-approval approve --request <provider-activation-approval-request.json> --output <provider-activation-approval.json> --confirm-activation-policy-sha256 <sha256> --confirm-readiness-audit-sha256 <sha256> --confirm-real-call-proposal-sha256 <sha256> --confirm-provider-payload-sha256 <sha256>
  deonctl worker codex provider-activation-approval inspect --approval <provider-activation-approval.json> [--request <provider-activation-approval-request.json>] [--output-format text|json]
  deonctl worker codex provider-activation-rehearsal --activation-policy-plan <provider-activation-policy-plan.json> --activation-approval <provider-activation-approval.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --credential-policy-plan <provider-credential-policy-plan.json> --execution-simulation-report <provider-execution-simulation-report.json> --output <provider-activation-rehearsal.json> [--output-format text|json]
  deonctl worker codex provider-activation-rehearsal-report --rehearsal <provider-activation-rehearsal.json> --activation-policy-plan <provider-activation-policy-plan.json> --activation-approval <provider-activation-approval.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --credential-policy-plan <provider-credential-policy-plan.json> --execution-simulation-report <provider-execution-simulation-report.json> [--output-format text|json]
  deonctl worker codex provider-activation-release-package --activation-policy-plan <provider-activation-policy-plan.json> --activation-approval <provider-activation-approval.json> --activation-rehearsal <provider-activation-rehearsal.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --execution-simulation-report <provider-execution-simulation-report.json> --credential-policy-plan <provider-credential-policy-plan.json> --response-change-proposal-report <provider-response-change-proposal-report.json> --output <provider-activation-release-package.json> [--output-format text|json]
  deonctl worker codex provider-activation-release-gate --activation-policy-plan <provider-activation-policy-plan.json> --activation-approval <provider-activation-approval.json> --activation-rehearsal <provider-activation-rehearsal.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --execution-simulation-report <provider-execution-simulation-report.json> --credential-policy-plan <provider-credential-policy-plan.json> --response-change-proposal-report <provider-response-change-proposal-report.json> --activation-release-package <provider-activation-release-package.json> [--output-format text|json]
  deonctl worker codex provider-activation-operator-review-bundle --activation-release-package <provider-activation-release-package.json> [--activation-release-gate <provider-activation-release-gate.json>] --activation-policy-plan <provider-activation-policy-plan.json> --activation-approval <provider-activation-approval.json> --activation-rehearsal <provider-activation-rehearsal.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --execution-simulation-report <provider-execution-simulation-report.json> --credential-policy-plan <provider-credential-policy-plan.json> --response-change-proposal-report <provider-response-change-proposal-report.json> --output <provider-activation-operator-review-bundle.json> [--output-format text|json]
  deonctl worker codex provider-activation-operator-review-report --operator-review-bundle <provider-activation-operator-review-bundle.json> [--activation-release-package <provider-activation-release-package.json>] [--activation-release-gate <provider-activation-release-gate.json>] --activation-policy-plan <provider-activation-policy-plan.json> --activation-approval <provider-activation-approval.json> --activation-rehearsal <provider-activation-rehearsal.json> --activation-readiness-audit <provider-activation-readiness-audit.json> --real-call-proposal <provider-real-call-proposal.json> --execution-simulation-report <provider-execution-simulation-report.json> --credential-policy-plan <provider-credential-policy-plan.json> --response-change-proposal-report <provider-response-change-proposal-report.json> [--output-format text|json]
  deonctl worker codex provider-activation-kill-switch validate --config <provider-activation-kill-switch.yaml> [--output-format text|json]
  deonctl worker codex provider-activation-kill-switch plan --config <provider-activation-kill-switch.yaml> --output <provider-activation-kill-switch-plan.json> [--output-format text|json]
  deonctl worker codex provider-activation-final-audit --activation-release-package <provider-activation-release-package.json> --activation-release-gate <provider-activation-release-gate.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> [--activation-policy-plan <provider-activation-policy-plan.json>] [--activation-readiness-audit <provider-activation-readiness-audit.json>] [--real-call-proposal <provider-real-call-proposal.json>] [--credential-policy-plan <provider-credential-policy-plan.json>] [--response-change-proposal-report <provider-response-change-proposal-report.json>] [--activation-approval <provider-activation-approval.json>] [--activation-rehearsal <provider-activation-rehearsal.json>] [--execution-simulation-report <provider-execution-simulation-report.json>] [--output-format text|json]
  deonctl worker codex provider-activation-ci-report --activation-release-package <provider-activation-release-package.json> --activation-release-gate <provider-activation-release-gate.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> [--activation-policy-plan <provider-activation-policy-plan.json>] [--activation-readiness-audit <provider-activation-readiness-audit.json>] [--real-call-proposal <provider-real-call-proposal.json>] [--credential-policy-plan <provider-credential-policy-plan.json>] [--response-change-proposal-report <provider-response-change-proposal-report.json>] [--activation-approval <provider-activation-approval.json>] [--activation-rehearsal <provider-activation-rehearsal.json>] [--execution-simulation-report <provider-execution-simulation-report.json>] [--output-format text|json]
  deonctl worker codex provider-secret-read-proposal new --credential-policy-plan <provider-credential-policy-plan.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --activation-release-gate <provider-activation-release-gate.json> --output <provider-secret-read-proposal.json> [--output-format text|json]
  deonctl worker codex provider-secret-read-proposal inspect --proposal <provider-secret-read-proposal.json> --credential-policy-plan <provider-credential-policy-plan.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --activation-release-gate <provider-activation-release-gate.json> [--output-format text|json]
  deonctl worker codex provider-real-transport-implementation-plan --secret-read-proposal <provider-secret-read-proposal.json> --provider-adapter-plan <provider-adapter-plan.json> --activation-final-audit <provider-activation-final-audit.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --output <provider-real-transport-implementation-plan.json> [--output-format text|json]
  deonctl worker codex provider-real-transport-implementation-report --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --secret-read-proposal <provider-secret-read-proposal.json> --provider-adapter-plan <provider-adapter-plan.json> --activation-final-audit <provider-activation-final-audit.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> [--output-format text|json]
  deonctl worker codex provider-real-dispatch-design --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --secret-read-proposal <provider-secret-read-proposal.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --provider-request-envelope <provider-request-envelope.json> --provider-real-call-proposal <provider-real-call-proposal.json> --output <provider-real-dispatch-design.json> [--output-format text|json]
  deonctl worker codex provider-real-dispatch-design-report --real-dispatch-design <provider-real-dispatch-design.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --secret-read-proposal <provider-secret-read-proposal.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --provider-request-envelope <provider-request-envelope.json> --provider-real-call-proposal <provider-real-call-proposal.json> [--output-format text|json]
  deonctl worker codex provider-real-activation-design-review-package --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --real-dispatch-design <provider-real-dispatch-design.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> --output <provider-real-activation-design-review-package.json> [--output-format text|json]
  deonctl worker codex provider-real-activation-design-review-gate --design-review-package <provider-real-activation-design-review-package.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --real-dispatch-design <provider-real-dispatch-design.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> [--output-format text|json]
  deonctl worker codex provider-real-dispatch-external-approval new --design-review-package <provider-real-activation-design-review-package.json> --design-review-gate <provider-real-activation-design-review-gate.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-dispatch-design <provider-real-dispatch-design.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> --output <provider-real-dispatch-external-approval-request.json>
  deonctl worker codex provider-real-dispatch-external-approval approve --request <provider-real-dispatch-external-approval-request.json> --output <provider-real-dispatch-external-approval.json> --confirm-design-review-package-sha256 <sha256> --confirm-design-review-gate-sha256 <sha256> --confirm-secret-read-proposal-sha256 <sha256> --confirm-real-dispatch-design-sha256 <sha256> --confirm-provider-payload-sha256 <sha256>
  deonctl worker codex provider-real-dispatch-external-approval inspect --approval <provider-real-dispatch-external-approval.json> [--request <provider-real-dispatch-external-approval-request.json>] [--design-review-gate <provider-real-activation-design-review-gate.json>] [--output-format text|json]
  deonctl worker codex provider-real-dispatch-runbook generate --external-approval <provider-real-dispatch-external-approval.json> --external-approval-request <provider-real-dispatch-external-approval-request.json> --design-review-gate <provider-real-activation-design-review-gate.json> --real-dispatch-design <provider-real-dispatch-design.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --output <provider-real-dispatch-runbook.json>
  deonctl worker codex provider-real-dispatch-runbook report --runbook <provider-real-dispatch-runbook.json> --external-approval <provider-real-dispatch-external-approval.json> --external-approval-request <provider-real-dispatch-external-approval-request.json> --design-review-gate <provider-real-activation-design-review-gate.json> --real-dispatch-design <provider-real-dispatch-design.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> [--output-format text|json]
  deonctl worker codex provider-real-dispatch-risk-register --runbook <provider-real-dispatch-runbook.json> --external-approval <provider-real-dispatch-external-approval.json> --design-review-gate <provider-real-activation-design-review-gate.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --real-dispatch-design <provider-real-dispatch-design.json> --activation-final-audit <provider-activation-final-audit.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --output <provider-real-dispatch-risk-register.json>
  deonctl worker codex provider-real-dispatch-risk-report --risk-register <provider-real-dispatch-risk-register.json> --runbook <provider-real-dispatch-runbook.json> --external-approval <provider-real-dispatch-external-approval.json> --design-review-gate <provider-real-activation-design-review-gate.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --real-dispatch-design <provider-real-dispatch-design.json> --activation-final-audit <provider-activation-final-audit.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> [--output-format text|json]
  deonctl worker codex provider-real-dispatch-preimplementation-gate --external-approval <provider-real-dispatch-external-approval.json> --runbook <provider-real-dispatch-runbook.json> --risk-register <provider-real-dispatch-risk-register.json> --design-review-gate <provider-real-activation-design-review-gate.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --real-dispatch-design <provider-real-dispatch-design.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> [--output-format text|json]
  deonctl worker codex provider-real-dispatch-preimplementation-report --external-approval <provider-real-dispatch-external-approval.json> --runbook <provider-real-dispatch-runbook.json> --risk-register <provider-real-dispatch-risk-register.json> --design-review-gate <provider-real-activation-design-review-gate.json> --secret-read-proposal <provider-secret-read-proposal.json> --real-transport-implementation-plan <provider-real-transport-implementation-plan.json> --real-dispatch-design <provider-real-dispatch-design.json> --activation-final-audit <provider-activation-final-audit.json> --activation-ci-report <provider-activation-ci-report.json> --kill-switch-plan <provider-activation-kill-switch-plan.json> --operator-review-bundle <provider-activation-operator-review-bundle.json> [--output-format text|json]
  deonctl worker codex run <task-path> --store <path> --artifacts-dir <path> [--domains <domains.yaml>] [--memory-policy <policy.yaml>] [--workers-config <path>] [--runtime-config <runtime.yaml>] [--validation-runtime local|docker] [--worker-runtime local|docker]
  deonctl worker opencode dry-run <task-path> [--workers-config <path>]
  deonctl worker opencode run <task-path> --store <path> --artifacts-dir <path> [--domains <domains.yaml>] [--memory-policy <policy.yaml>] [--workers-config <path>] [--runtime-config <runtime.yaml>] [--validation-runtime local|docker] [--worker-runtime local|docker]
  deonctl artifacts list --store <path> [--run <run-id>] [--status <status>]
  deonctl artifacts prune --store <path> --artifacts-dir <path> --older-than <duration> [--dry-run]
`

var codexWorkerFactory = func() workers.Worker {
	return codex.New()
}

var opencodeWorkerFactory = func() workers.Worker {
	return opencode.New()
}

var memoryExecuteApply = memory.ExecuteApply

var runIDFactory = func() string {
	return "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
}

var gitDiffRunner = captureGitDiff

var gitSnapshotRunner = git.TakeSnapshot

type workspacePreparer = runner.WorkspacePreparer

type changedFile = runner.ChangedFile

var workspaceManagerFactory = func() workspacePreparer {
	return runtime.NewWorkspaceManager()
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "deonclaw %s\n", config.Version)
		return 0
	case "doctor":
		opts, err := parseDoctorOptions(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return runDoctor(opts, stdout, stderr)
	case "workers":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "doctor":
			opts, err := parseDoctorOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runDoctor(opts, stdout, stderr)
		case "smoke":
			opts, err := parseWorkersSmokeOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runWorkersSmoke(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "config":
		if len(args) < 2 || args[1] != "env" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		opts, err := parseConfigEnvOptions(args[2:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return runConfigEnv(opts, stdout, stderr)
	case "task":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			if len(args) != 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runTaskValidate(args[2], stdout, stderr)
		case "materialized-injection-report":
			opts, err := parseTaskMaterializedInjectionReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runTaskMaterializedInjectionReport(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "domains":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		opts, err := parseDomainsOptions(args[2:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			return runDomainsValidate(opts, stdout, stderr)
		case "list":
			return runDomainsList(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "context":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "build":
			opts, err := parseContextBuildOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runContextBuild(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "runtime":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			opts, err := parseRuntimeValidateOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runRuntimeValidate(opts, stdout, stderr)
		case "docker-plan":
			opts, err := parseRuntimeDockerPlanOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runRuntimeDockerPlan(opts, stdout, stderr)
		case "docker-exec":
			opts, err := parseRuntimeDockerExecOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runRuntimeDockerExec(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "mcp":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			opts, err := parseMCPConfigOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPValidate(opts, stdout, stderr)
		case "list":
			opts, err := parseMCPConfigOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPList(opts, stdout, stderr)
		case "plan":
			opts, err := parseMCPPlanOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPPlan(opts, stdout, stderr)
		case "doctor":
			opts, err := parseMCPReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPDoctor(opts, stdout, stderr)
		case "risk":
			opts, err := parseMCPReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPRisk(opts, stdout, stderr)
		case "docker-plan":
			opts, err := parseMCPDockerPlanOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPDockerPlan(opts, stdout, stderr)
		case "fake-server":
			if len(args) != 2 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPFakeServer(os.Stdin, stdout, stderr)
		case "smoke":
			opts, err := parseMCPSmokeOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPSmoke(opts, stdout, stderr)
		case "tool-smoke":
			opts, err := parseMCPToolSmokeOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPToolSmoke(opts, stdout, stderr)
		case "discover":
			opts, err := parseMCPDiscoverOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPDiscover(opts, stdout, stderr)
		case "call-smoke":
			opts, err := parseMCPCallSmokeOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMCPCallSmoke(opts, stdout, stderr)
		case "proposal":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "new":
				opts, err := parseMCPProposalNewOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalNew(opts, stdout, stderr)
			case "inspect":
				opts, err := parseMCPProposalInspectOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalInspect(opts, stdout, stderr)
			case "lint":
				opts, err := parseMCPProposalLintOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalLint(opts, stdout, stderr)
			case "preflight":
				opts, err := parseMCPProposalPreflightOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalPreflight(opts, stdout, stderr)
			case "approve":
				opts, err := parseMCPProposalApproveOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalApprove(opts, stdout, stderr)
			case "execute":
				opts, err := parseMCPProposalExecuteOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalExecute(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "proposals":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "list":
				opts, err := parseMCPProposalsListOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalsList(opts, stdout, stderr)
			case "show":
				opts, err := parseMCPProposalsShowOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalsShow(opts, stdout, stderr)
			case "export":
				opts, err := parseMCPProposalsExportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPProposalsExport(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "approval":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "inspect":
				opts, err := parseMCPApprovalInspectOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMCPApprovalInspect(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "memory":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "index":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "validate":
				opts, err := parseMemoryIndexConfigOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryIndexValidate(opts, stdout, stderr)
			case "plan":
				opts, err := parseMemoryIndexPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryIndexPlan(opts, stdout, stderr)
			case "build":
				opts, err := parseMemoryIndexBuildOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryIndexBuild(opts, stdout, stderr)
			case "doctor":
				opts, err := parseMemoryIndexPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryIndexDoctor(opts, stdout, stderr)
			case "report":
				opts, err := parseMemoryIndexReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryIndexReport(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "embedding":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "validate":
				opts, err := parseMemoryEmbeddingPolicyOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryEmbeddingValidate(opts, stdout, stderr)
			case "doctor":
				opts, err := parseMemoryEmbeddingDoctorOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryEmbeddingDoctor(opts, stdout, stderr)
			case "plan":
				opts, err := parseMemoryEmbeddingDoctorOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryEmbeddingPlan(opts, stdout, stderr)
			case "build-fake":
				opts, err := parseMemoryEmbeddingBuildFakeOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryEmbeddingBuildFake(opts, stdout, stderr)
			case "report":
				opts, err := parseMemoryEmbeddingReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryEmbeddingReport(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "lancedb":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "validate":
				opts, err := parseMemoryLanceDBPolicyOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBValidate(opts, stdout, stderr)
			case "plan":
				opts, err := parseMemoryLanceDBPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBPlan(opts, stdout, stderr)
			case "fake-write":
				opts, err := parseMemoryLanceDBFakeWriteOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBFakeWrite(opts, stdout, stderr)
			case "write-smoke":
				opts, err := parseMemoryLanceDBWriteSmokeOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBWriteSmoke(opts, stdout, stderr)
			case "doctor":
				opts, err := parseMemoryLanceDBDoctorOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBDoctor(opts, stdout, stderr)
			case "report":
				opts, err := parseMemoryLanceDBReadbackReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBReadbackReport(opts, stdout, stderr)
			case "search-smoke":
				opts, err := parseMemoryLanceDBSearchSmokeOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBSearchSmoke(opts, stdout, stderr)
			case "search-report":
				opts, err := parseMemoryLanceDBSearchReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryLanceDBSearchReport(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "proposal":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "new":
				opts, err := parseMemoryProposalNewOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalNew(opts, stdout, stderr)
			case "lint":
				opts, err := parseMemoryProposalLintOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalLint(opts, stdout, stderr)
			case "apply":
				opts, err := parseMemoryProposalApplyOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalApply(opts, stdout, stderr)
			case "approve":
				opts, err := parseMemoryProposalApproveOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalApprove(opts, stdout, stderr)
			case "apply-preflight":
				opts, err := parseMemoryProposalApplyPreflightOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalApplyPreflight(opts, stdout, stderr)
			case "backup-plan":
				opts, err := parseMemoryProposalBackupPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalBackupPlan(opts, stdout, stderr)
			case "backup-materialize":
				opts, err := parseMemoryProposalBackupMaterializeOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalBackupMaterialize(opts, stdout, stderr)
			case "restore":
				opts, err := parseMemoryProposalRestoreOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalRestore(opts, stdout, stderr)
			case "restore-execute":
				opts, err := parseMemoryProposalRestoreExecuteOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalRestoreExecute(opts, stdout, stderr)
			case "apply-execute":
				opts, err := parseMemoryProposalApplyExecuteOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runMemoryProposalApplyExecute(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "worker":
		if len(args) < 4 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "codex":
			switch args[2] {
			case "dry-run":
				opts, err := parseWorkerDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexDryRun(opts, stdout, stderr)
			case "materialized-injection-preflight":
				opts, err := parseCodexMaterializedInjectionPreflightOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionPreflight(opts, stdout, stderr)
			case "materialized-injection-dry-run":
				opts, err := parseCodexMaterializedInjectionDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionDryRun(opts, stdout, stderr)
			case "materialized-injection-dry-run-report":
				opts, err := parseCodexMaterializedInjectionDryRunReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionDryRunReport(opts, stdout, stderr)
			case "materialized-injection-readiness-report":
				opts, err := parseCodexMaterializedInjectionReadinessReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionReadinessReport(opts, stdout, stderr)
			case "materialized-injection-execution-gate":
				opts, err := parseCodexMaterializedInjectionExecutionGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionExecutionGate(opts, stdout, stderr)
			case "materialized-prompt-assembly-dry-run":
				opts, err := parseCodexMaterializedPromptAssemblyDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedPromptAssemblyDryRun(opts, stdout, stderr)
			case "materialized-prompt-assembly-report":
				opts, err := parseCodexMaterializedPromptAssemblyReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedPromptAssemblyReport(opts, stdout, stderr)
			case "materialized-injection-runtime":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexMaterializedInjectionRuntimeValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexMaterializedInjectionRuntimeValidate(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "materialized-injection-run-plan":
				opts, err := parseCodexMaterializedInjectionRunPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionRunPlan(opts, stdout, stderr)
			case "materialized-injection-execution-enable":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexMaterializedInjectionExecutionEnableValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexMaterializedInjectionExecutionEnableValidate(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "materialized-injection-provider-run-plan":
				opts, err := parseCodexMaterializedInjectionProviderRunPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedInjectionProviderRunPlan(opts, stdout, stderr)
			case "materialized-provider-dispatch":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexMaterializedProviderDispatchValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexMaterializedProviderDispatchValidate(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "materialized-provider-payload-dry-run":
				opts, err := parseCodexMaterializedProviderPayloadDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedProviderPayloadDryRun(opts, stdout, stderr)
			case "materialized-provider-payload-report":
				opts, err := parseCodexMaterializedProviderPayloadReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedProviderPayloadReport(opts, stdout, stderr)
			case "materialized-provider-call-gate":
				opts, err := parseCodexMaterializedProviderCallGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedProviderCallGate(opts, stdout, stderr)
			case "materialized-provider-call-readiness-report":
				opts, err := parseCodexMaterializedProviderCallReadinessReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexMaterializedProviderCallReadinessReport(opts, stdout, stderr)
			case "provider-call-approval":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					subOpts, err := parseCodexProviderCallApprovalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallApprovalNew(subOpts, stdout, stderr)
				case "approve":
					subOpts, err := parseCodexProviderCallApprovalApproveOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallApprovalApprove(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderCallApprovalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallApprovalInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-call-execution-bundle":
				opts, err := parseCodexProviderCallExecutionBundleOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderCallExecutionBundle(opts, stdout, stderr)
			case "provider-call-chain-audit":
				opts, err := parseCodexProviderCallChainAuditOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderCallChainAudit(opts, stdout, stderr)
			case "provider-call-executor":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "config":
					if len(args) < 5 {
						fmt.Fprint(stderr, usage)
						return 2
					}
					switch args[4] {
					case "validate":
						subOpts, err := parseCodexProviderCallExecutorConfigValidateOptions(args[5:])
						if err != nil {
							fmt.Fprintf(stderr, "error: %v\n", err)
							fmt.Fprint(stderr, usage)
							return 2
						}
						return runCodexProviderCallExecutorConfigValidate(subOpts, stdout, stderr)
					default:
						fmt.Fprint(stderr, usage)
						return 2
					}
				case "validate":
					subOpts, err := parseCodexProviderCallExecutorOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorValidate(subOpts, stdout, stderr)
				case "plan":
					subOpts, err := parseCodexProviderCallExecutorPlanOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorPlan(subOpts, stdout, stderr)
				case "dry-run":
					subOpts, err := parseCodexProviderCallExecutorDryRunOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorDryRun(subOpts, stdout, stderr)
				case "dry-run-report":
					subOpts, err := parseCodexProviderCallExecutorDryRunReportOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorDryRunReport(subOpts, stdout, stderr)
				case "preflight":
					subOpts, err := parseCodexProviderCallExecutorPreflightOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorPreflight(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-call-executor-dispatch-approval":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					subOpts, err := parseCodexProviderCallExecutorDispatchApprovalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorDispatchApprovalNew(subOpts, stdout, stderr)
				case "approve":
					subOpts, err := parseCodexProviderCallExecutorDispatchApprovalApproveOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorDispatchApprovalApprove(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderCallExecutorDispatchApprovalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCallExecutorDispatchApprovalInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-transport-plan":
				opts, err := parseCodexProviderTransportPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderTransportPlan(opts, stdout, stderr)
			case "provider-executor-release-bundle":
				opts, err := parseCodexProviderExecutorReleaseBundleOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderExecutorReleaseBundle(opts, stdout, stderr)
			case "provider-executor-release-gate":
				opts, err := parseCodexProviderExecutorReleaseGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderExecutorReleaseGate(opts, stdout, stderr)
			case "provider-request-envelope":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "dry-run":
					subOpts, err := parseCodexProviderRequestEnvelopeDryRunOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRequestEnvelopeDryRun(subOpts, stdout, stderr)
				case "report":
					subOpts, err := parseCodexProviderRequestEnvelopeReportOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRequestEnvelopeReport(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-adapter-registry":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexProviderAdapterRegistryValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderAdapterRegistryValidate(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-adapter-plan":
				opts, err := parseCodexProviderAdapterPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderAdapterPlan(opts, stdout, stderr)
			case "provider-response-fixture":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "generate":
					subOpts, err := parseCodexProviderResponseFixtureGenerateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderResponseFixtureGenerate(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderResponseFixtureInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderResponseFixtureInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-execution-simulation-bundle":
				opts, err := parseCodexProviderExecutionSimulationBundleOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderExecutionSimulationBundle(opts, stdout, stderr)
			case "provider-execution-simulation-report":
				opts, err := parseCodexProviderExecutionSimulationReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderExecutionSimulationReport(opts, stdout, stderr)
			case "provider-credential-policy":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexProviderCredentialPolicyValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCredentialPolicyValidate(subOpts, stdout, stderr)
				case "plan":
					subOpts, err := parseCodexProviderCredentialPolicyPlanOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderCredentialPolicyPlan(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-real-call-proposal":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					subOpts, err := parseCodexProviderRealCallProposalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealCallProposalNew(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderRealCallProposalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealCallProposalInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-response-change-proposal":
				opts, err := parseCodexProviderResponseChangeProposalOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderResponseChangeProposal(opts, stdout, stderr)
			case "provider-response-change-proposal-report":
				opts, err := parseCodexProviderResponseChangeProposalReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderResponseChangeProposalReport(opts, stdout, stderr)
			case "provider-activation-readiness-audit":
				opts, err := parseCodexProviderActivationReadinessAuditOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationReadinessAudit(opts, stdout, stderr)
			case "provider-activation-readiness-report":
				opts, err := parseCodexProviderActivationReadinessAuditOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationReadinessReport(opts, stdout, stderr)
			case "provider-activation-policy":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexProviderActivationPolicyValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationPolicyValidate(subOpts, stdout, stderr)
				case "plan":
					subOpts, err := parseCodexProviderActivationPolicyPlanOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationPolicyPlan(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-activation-approval":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					subOpts, err := parseCodexProviderActivationApprovalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationApprovalNew(subOpts, stdout, stderr)
				case "approve":
					subOpts, err := parseCodexProviderActivationApprovalApproveOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationApprovalApprove(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderActivationApprovalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationApprovalInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-activation-rehearsal":
				opts, err := parseCodexProviderActivationRehearsalOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationRehearsal(opts, stdout, stderr)
			case "provider-activation-rehearsal-report":
				opts, err := parseCodexProviderActivationRehearsalReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationRehearsalReport(opts, stdout, stderr)
			case "provider-activation-release-package":
				opts, err := parseCodexProviderActivationReleasePackageOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationReleasePackage(opts, stdout, stderr)
			case "provider-activation-release-gate":
				opts, err := parseCodexProviderActivationReleaseGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationReleaseGate(opts, stdout, stderr)
			case "provider-activation-operator-review-bundle":
				opts, err := parseCodexProviderActivationOperatorReviewBundleOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationOperatorReviewBundle(opts, stdout, stderr)
			case "provider-activation-operator-review-report":
				opts, err := parseCodexProviderActivationOperatorReviewReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationOperatorReviewReport(opts, stdout, stderr)
			case "provider-activation-kill-switch":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					subOpts, err := parseCodexProviderActivationKillSwitchValidateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationKillSwitchValidate(subOpts, stdout, stderr)
				case "plan":
					subOpts, err := parseCodexProviderActivationKillSwitchPlanOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderActivationKillSwitchPlan(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-activation-final-audit":
				opts, err := parseCodexProviderActivationFinalAuditOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationFinalAudit(opts, stdout, stderr)
			case "provider-activation-ci-report":
				opts, err := parseCodexProviderActivationCIReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderActivationCIReport(opts, stdout, stderr)
			case "provider-secret-read-proposal":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					subOpts, err := parseCodexProviderSecretReadProposalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderSecretReadProposalNew(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderSecretReadProposalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderSecretReadProposalInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-real-transport-implementation-plan":
				opts, err := parseCodexProviderRealTransportImplementationPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealTransportImplementationPlan(opts, stdout, stderr)
			case "provider-real-transport-implementation-report":
				opts, err := parseCodexProviderRealTransportImplementationReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealTransportImplementationReport(opts, stdout, stderr)
			case "provider-real-dispatch-design":
				opts, err := parseCodexProviderRealDispatchDesignOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealDispatchDesign(opts, stdout, stderr)
			case "provider-real-dispatch-design-report":
				opts, err := parseCodexProviderRealDispatchDesignReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealDispatchDesignReport(opts, stdout, stderr)
			case "provider-real-activation-design-review-package":
				opts, err := parseCodexProviderRealActivationDesignReviewPackageOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealActivationDesignReviewPackage(opts, stdout, stderr)
			case "provider-real-activation-design-review-gate":
				opts, err := parseCodexProviderRealActivationDesignReviewGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealActivationDesignReviewGate(opts, stdout, stderr)
			case "provider-real-dispatch-external-approval":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					subOpts, err := parseCodexProviderRealDispatchExternalApprovalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealDispatchExternalApprovalNew(subOpts, stdout, stderr)
				case "approve":
					subOpts, err := parseCodexProviderRealDispatchExternalApprovalApproveOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealDispatchExternalApprovalApprove(subOpts, stdout, stderr)
				case "inspect":
					subOpts, err := parseCodexProviderRealDispatchExternalApprovalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealDispatchExternalApprovalInspect(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-real-dispatch-runbook":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "generate":
					subOpts, err := parseCodexProviderRealDispatchRunbookGenerateOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealDispatchRunbookGenerate(subOpts, stdout, stderr)
				case "report":
					subOpts, err := parseCodexProviderRealDispatchRunbookReportOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runCodexProviderRealDispatchRunbookReport(subOpts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "provider-real-dispatch-risk-register":
				opts, err := parseCodexProviderRealDispatchRiskRegisterOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealDispatchRiskRegister(opts, stdout, stderr)
			case "provider-real-dispatch-risk-report":
				opts, err := parseCodexProviderRealDispatchRiskReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealDispatchRiskReport(opts, stdout, stderr)
			case "provider-real-dispatch-preimplementation-gate":
				opts, err := parseCodexProviderRealDispatchPreimplementationGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealDispatchPreimplementationGate(opts, stdout, stderr)
			case "provider-real-dispatch-preimplementation-report":
				opts, err := parseCodexProviderRealDispatchPreimplementationGateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexProviderRealDispatchPreimplementationReport(opts, stdout, stderr)
			case "run":
				opts, err := parseCodexRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexRun(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "opencode":
			switch args[2] {
			case "dry-run":
				opts, err := parseWorkerDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runOpenCodeDryRun(opts, stdout, stderr)
			case "run":
				opts, err := parseOpenCodeRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runOpenCodeRun(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "insights":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "policy":
			if len(args) < 3 || args[2] != "validate" {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseInsightsPolicyValidateOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runInsightsPolicyValidate(opts, stdout, stderr)
		case "evidence":
			if len(args) < 3 || args[2] != "build" {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseInsightsEvidenceBuildOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runInsightsEvidenceBuild(opts, stdout, stderr)
		case "trigger":
			if len(args) < 3 || args[2] != "evaluate" {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseInsightsTriggerEvaluateOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runInsightsTriggerEvaluate(opts, stdout, stderr)
		case "evaluate":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "dry-run":
				opts, err := parseInsightsEvaluateDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsEvaluateDryRun(opts, stdout, stderr)
			default:
				evalArgs := args[2:]
				opts, err := parseInsightsEvaluateOptions(evalArgs)
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsEvaluate(opts, stdout, stderr)
			}
		case "proposals":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "materialize":
				opts, err := parseInsightsProposalsMaterializeOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsProposalsMaterialize(opts, stdout, stderr)
			case "list":
				opts, err := parseInsightsProposalsListOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsProposalsList(opts, stdout, stderr)
			case "show":
				opts, err := parseInsightsProposalsShowOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsProposalsShow(opts, stdout, stderr)
			case "approve":
				opts, err := parseInsightsProposalsApproveOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsProposalsApprove(opts, stdout, stderr)
			case "apply":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "dry-run":
					opts, err := parseInsightsProposalsApplyDryRunOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runInsightsProposalsApplyDryRun(opts, stdout, stderr)
				default:
					opts, err := parseInsightsProposalsApplyOptions(args[3:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runInsightsProposalsApply(opts, stdout, stderr)
				}
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "report":
			opts, err := parseInsightsReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runInsightsReport(opts, stdout, stderr)
		case "effectiveness":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "record":
				opts, err := parseInsightsEffectivenessRecordOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsEffectivenessRecord(opts, stdout, stderr)
			case "report":
				opts, err := parseInsightsEffectivenessReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runInsightsEffectivenessReport(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "timeline":
			opts, err := parseInsightsTimelineOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runInsightsTimeline(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "skills":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "policy":
			if len(args) < 3 || args[2] != "validate" {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseSkillsPolicyValidateOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsPolicyValidate(opts, stdout, stderr)
		case "inspect":
			opts, err := parseSkillsInspectOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsInspect(opts, stdout, stderr)
		case "import":
			opts, err := parseSkillsImportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsImport(opts, stdout, stderr)
		case "install":
			opts, err := parseSkillsInstallOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsInstall(opts, stdout, stderr)
		case "approve":
			opts, err := parseSkillsShowOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsApprove(opts, stdout, stderr)
		case "verify":
			opts, err := parseSkillsVerifyOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsVerify(opts, stdout, stderr)
		case "list":
			opts, err := parseSkillsListOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsList(opts, stdout, stderr)
		case "show":
			opts, err := parseSkillsShowOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsShow(opts, stdout, stderr)
		case "enable":
			opts, err := parseSkillsEnableOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsEnable(opts, stdout, stderr)
		case "disable":
			opts, err := parseSkillsEnableOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsDisable(opts, stdout, stderr)
		case "snapshot":
			opts, err := parseSkillsSnapshotOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsSnapshot(opts, stdout, stderr)
		case "materialize":
			opts, err := parseSkillsMaterializeOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runSkillsMaterialize(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "agents":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			opts, err := parseAgentsValidateOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsValidate(opts, stdout, stderr)
		case "sync":
			opts, err := parseAgentsSyncOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsSync(opts, stdout, stderr)
		case "list":
			opts, err := parseAgentsStoreOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsList(opts, stdout, stderr)
		case "show":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseAgentsStoreOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsShow(args[2], opts, stdout, stderr)
		case "pause":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseAgentsLifecycleOptions(args[2], args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsPause(args[2], opts, stdout, stderr)
		case "resume":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseAgentsLifecycleOptions(args[2], args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsResume(args[2], opts, stdout, stderr)
		case "terminate":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseAgentsLifecycleOptions(args[2], args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsTerminate(args[2], opts, stdout, stderr)
		case "sessions":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "create":
				opts, err := parseAgentsSessionCreateOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runAgentsSessionCreate(opts, stdout, stderr)
			case "list":
				opts, err := parseAgentsInboxListOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				storeOpts := agentsStoreOptions{storePath: opts.storePath}
				return runAgentsSessionsList(opts.agentID, storeOpts, stdout, stderr)
			case "show":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				opts, err := parseAgentsSessionShowOptions(args[3], args[4:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runAgentsSessionShow(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "assign":
			opts, err := parseAgentsAssignOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsAssign(opts, stdout, stderr)
		case "inbox":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "list":
				opts, err := parseAgentsInboxListOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runAgentsInboxList(opts, stdout, stderr)
			case "accept":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				opts, err := parseAgentsInboxItemOptions(args[3], args[4:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runAgentsInboxAccept(args[3], opts, stdout, stderr)
			case "defer":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				opts, err := parseAgentsInboxItemOptions(args[3], args[4:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runAgentsInboxDefer(args[3], opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "delegate":
			if len(args) < 3 || args[2] != "propose" {
				fmt.Fprint(stderr, usage)
				return 2
			}
			opts, err := parseAgentsDelegateProposeOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runAgentsDelegatePropose(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "daemon":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "status", "doctor", "start", "stop":
			storePath, err := parseDaemonStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			switch args[1] {
			case "status":
				return runDaemonStatus(storePath, stdout, stderr)
			case "doctor":
				return runDaemonDoctor(storePath, stdout, stderr)
			case "start":
				return runDaemonStart(storePath, stdout, stderr)
			case "stop":
				return runDaemonStop(storePath, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "run-once":
			opts, err := parseDaemonRunOnceOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runDaemonRunOnce(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "schedules":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			configPath, err := parseConfigPathArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runSchedulesValidate(configPath, stdout, stderr)
		case "sync":
			configPath, storePath, err := parseSchedulesSyncOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runSchedulesSync(configPath, storePath, stdout, stderr)
		case "list":
			storePath, err := parseDaemonStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runSchedulesList(storePath, stdout, stderr)
		case "due":
			storePath, err := parseDaemonStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runSchedulesDue(storePath, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "heartbeat":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			configPath, err := parseConfigPathArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runHeartbeatValidate(configPath, stdout, stderr)
		case "dry-run":
			agentID, policyID, configPath, agentBusy, err := parseHeartbeatDryRunOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runHeartbeatDryRun(agentID, policyID, configPath, agentBusy, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "hooks":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			configPath, err := parseConfigPathArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runHooksValidate(configPath, stdout, stderr)
		case "plan":
			configPath, event, err := parseHooksPlanOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runHooksPlan(configPath, event, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "pricing":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			configPath, err := parseConfigFlag(args[2:], "--config")
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runPricingValidate(configPath, stdout, stderr)
		case "sync":
			storePath, configPath, err := parseStoreConfigFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runPricingSync(storePath, configPath, stdout, stderr)
		case "list":
			storePath, err := parseWorkStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runPricingList(storePath, stdout, stderr)
		case "show":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			storePath, err := parseWorkStoreArg(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runPricingShow(storePath, args[2], stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "usage":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "record":
			storePath, inputPath, err := parseStoreInputFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runUsageRecord(storePath, inputPath, stdout, stderr)
		case "show":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			storePath, err := parseWorkStoreArg(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runUsageShow(storePath, args[2], stdout, stderr)
		case "report":
			opts, err := parseUsageReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runUsageReport(opts.storePath, opts.groupBy, opts.since, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "budgets":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			configPath, err := parseConfigFlag(args[2:], "--config")
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsValidate(configPath, stdout, stderr)
		case "sync":
			storePath, configPath, err := parseStoreConfigFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsSync(storePath, configPath, stdout, stderr)
		case "status":
			storePath, agentID, err := parseStoreAgentFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsStatus(storePath, agentID, stdout, stderr)
		case "plan":
			configPath, agentID, err := parseConfigAgentFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsPlan(configPath, agentID, stdout, stderr)
		case "reserve":
			opts, err := parseBudgetReserveOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsReserve(opts, stdout, stderr)
		case "commit":
			storePath, reservationID, usagePath, err := parseBudgetCommitOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsCommit(storePath, reservationID, usagePath, stdout, stderr)
		case "release":
			storePath, reservationID, reason, err := parseBudgetReleaseOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsRelease(storePath, reservationID, reason, stdout, stderr)
		case "report":
			storePath, agentID, err := parseStoreAgentFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runBudgetsReport(storePath, agentID, stdout, stderr)
		case "override":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "new":
				configPath, outputPath, rest, err := parseOverrideNewOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					return 2
				}
				return runBudgetsOverrideNew(configPath, outputPath, rest, stdout, stderr)
			case "approve":
				storePath, inputPath, err := parseStoreInputFlags(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					return 2
				}
				return runBudgetsOverrideApprove(storePath, inputPath, stdout, stderr)
			case "inspect":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				storePath, err := parseWorkStoreArg(args[4:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					return 2
				}
				return runBudgetsOverrideInspect(storePath, args[3], stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "work-templates":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			configPath, err := parseConfigFlag(args[2:], "--config")
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkTemplatesValidate(configPath, stdout, stderr)
		case "sync":
			storePath, configPath, err := parseStoreConfigFlags(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkTemplatesSync(storePath, configPath, stdout, stderr)
		case "list":
			storePath, err := parseWorkStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkTemplatesList(storePath, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "work":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "list":
			storePath, err := parseWorkStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkList(storePath, stdout, stderr)
		case "show":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			storePath, err := parseWorkStoreArg(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkShow(storePath, args[2], stdout, stderr)
		case "claim":
			opts, err := parseWorkClaimOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkClaim(opts, stdout, stderr)
		case "release":
			opts, err := parseWorkReleaseOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkRelease(opts, stdout, stderr)
		case "recover":
			storePath, err := parseWorkStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkRecover(storePath, stdout, stderr)
		case "doctor":
			storePath, err := parseWorkStoreArg(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkDoctor(storePath, stdout, stderr)
		case "dispatch-once":
			opts, err := parseWorkDispatchOnceOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			return runWorkDispatchOnce(opts, stdout, stderr)
		case "leases":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "list":
				storePath, err := parseWorkStoreArg(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					return 2
				}
				return runWorkLeasesList(storePath, stdout, stderr)
			case "renew":
				opts, err := parseWorkLeaseRenewOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					return 2
				}
				return runWorkLeaseRenew(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "runs":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "report":
			opts, err := parseRunsReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runRunsReport(opts, stdout, stderr)
		case "retrieval-report":
			opts, err := parseRunsRetrievalReportOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runRunsRetrievalReport(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "retrieval":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "context":
			if len(args) < 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			switch args[2] {
			case "inspect":
				opts, err := parseRetrievalContextInspectOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextInspect(opts, stdout, stderr)
			case "materialize":
				opts, err := parseRetrievalContextMaterializeOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextMaterialize(opts, stdout, stderr)
			case "materialized-report":
				opts, err := parseRetrievalContextMaterializedReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextMaterializedReport(opts, stdout, stderr)
			case "bundle":
				opts, err := parseRetrievalContextBundleOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextBundle(opts, stdout, stderr)
			case "approval":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					opts, err := parseRetrievalContextApprovalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextApprovalNew(opts, stdout, stderr)
				case "approve":
					opts, err := parseRetrievalContextApprovalApproveOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextApprovalApprove(opts, stdout, stderr)
				case "inspect":
					opts, err := parseRetrievalContextApprovalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextApprovalInspect(opts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "injection-policy":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "validate":
					opts, err := parseRetrievalContextInjectionPolicyOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextInjectionPolicyValidate(opts, stdout, stderr)
				case "plan":
					opts, err := parseRetrievalContextInjectionPolicyPlanOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextInjectionPolicyPlan(opts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "injection-approval":
				if len(args) < 4 {
					fmt.Fprint(stderr, usage)
					return 2
				}
				switch args[3] {
				case "new":
					opts, err := parseRetrievalContextInjectionApprovalNewOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextInjectionApprovalNew(opts, stdout, stderr)
				case "approve":
					opts, err := parseRetrievalContextInjectionApprovalApproveOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextInjectionApprovalApprove(opts, stdout, stderr)
				case "inspect":
					opts, err := parseRetrievalContextInjectionApprovalInspectOptions(args[4:])
					if err != nil {
						fmt.Fprintf(stderr, "error: %v\n", err)
						fmt.Fprint(stderr, usage)
						return 2
					}
					return runRetrievalContextInjectionApprovalInspect(opts, stdout, stderr)
				default:
					fmt.Fprint(stderr, usage)
					return 2
				}
			case "injection-execution-plan":
				opts, err := parseRetrievalContextInjectionExecutionPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextInjectionExecutionPlan(opts, stdout, stderr)
			case "prompt-preview":
				opts, err := parseRetrievalContextPromptPreviewOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextPromptPreview(opts, stdout, stderr)
			case "prompt-preview-report":
				opts, err := parseRetrievalContextPromptPreviewReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextPromptPreviewReport(opts, stdout, stderr)
			case "injection-governance-bundle":
				opts, err := parseRetrievalContextInjectionGovernanceBundleOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextInjectionGovernanceBundle(opts, stdout, stderr)
			case "injection-plan":
				opts, err := parseRetrievalContextInjectionPlanOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextInjectionPlan(opts, stdout, stderr)
			case "governance-report":
				opts, err := parseRetrievalContextGovernanceReportOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runRetrievalContextGovernanceReport(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "artifacts":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "list":
			opts, err := parseArtifactListOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runArtifactList(opts, stdout, stderr)
		case "prune":
			opts, err := parseArtifactPruneOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runArtifactPrune(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

type contextBuildOptions struct {
	taskPath    string
	domainsPath string
	outputPath  string
}

func parseContextBuildOptions(args []string) (contextBuildOptions, error) {
	var opts contextBuildOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return contextBuildOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return contextBuildOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return contextBuildOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return contextBuildOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return contextBuildOptions{}, fmt.Errorf("missing --task")
	}
	if opts.domainsPath == "" {
		return contextBuildOptions{}, fmt.Errorf("missing --domains")
	}
	if opts.outputPath == "" {
		return contextBuildOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runContextBuild(opts contextBuildOptions, stdout io.Writer, stderr io.Writer) int {
	pack, err := (contextpack.Builder{}).Build(context.Background(), contextpack.BuildOptions{
		TaskPath:    opts.taskPath,
		DomainsPath: opts.domainsPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "context build failed: %v\n", err)
		return 1
	}

	outputDir := filepath.Dir(opts.outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "context build failed: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(opts.outputPath, pack.Markdown(), 0o600); err != nil {
		fmt.Fprintf(stderr, "context build failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "context pack written: %s\n", opts.outputPath)
	if len(pack.Warnings) > 0 {
		fmt.Fprintf(stdout, "warnings: %d\n", len(pack.Warnings))
	}
	return 0
}

type runtimeValidateOptions struct {
	configPath string
}

func parseRuntimeValidateOptions(args []string) (runtimeValidateOptions, error) {
	var opts runtimeValidateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return runtimeValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return runtimeValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return runtimeValidateOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func runRuntimeValidate(opts runtimeValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := runtimeconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "runtime validate failed: %v\n", err)
		return 1
	}
	result, err := runtimeconfig.Validate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "runtime validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "runtime config valid: mode=%s\n", cfg.Runtime.Mode)
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return 0
}

type runtimeDockerPlanOptions struct {
	configPath   string
	workspace    string
	outputFormat string
}

func parseRuntimeDockerPlanOptions(args []string) (runtimeDockerPlanOptions, error) {
	opts := runtimeDockerPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return runtimeDockerPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return runtimeDockerPlanOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return runtimeDockerPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return runtimeDockerPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return runtimeDockerPlanOptions{}, fmt.Errorf("missing --config")
	}
	if opts.workspace == "" {
		return runtimeDockerPlanOptions{}, fmt.Errorf("missing --workspace")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return runtimeDockerPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRuntimeDockerPlan(opts runtimeDockerPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := runtimeconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "runtime docker-plan failed: %v\n", err)
		return 1
	}
	plan, err := runtimeconfig.PlanDocker(cfg, opts.workspace)
	if err != nil {
		fmt.Fprintf(stderr, "runtime docker-plan failed: %v\n", err)
		return 1
	}
	if opts.outputFormat == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			fmt.Fprintf(stderr, "runtime docker-plan output failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "runtime: docker")
	for _, warning := range plan.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	fmt.Fprintf(stdout, "command: %s\n", plan.Display)
	return 0
}

type runtimeDockerExecOptions struct {
	configPath string
	workspace  string
	command    []string
}

func parseRuntimeDockerExecOptions(args []string) (runtimeDockerExecOptions, error) {
	var opts runtimeDockerExecOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return runtimeDockerExecOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return runtimeDockerExecOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--":
			opts.command = append([]string(nil), args[i+1:]...)
			i = len(args)
		default:
			return runtimeDockerExecOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return runtimeDockerExecOptions{}, fmt.Errorf("missing --config")
	}
	if opts.workspace == "" {
		return runtimeDockerExecOptions{}, fmt.Errorf("missing --workspace")
	}
	if len(opts.command) == 0 || strings.TrimSpace(opts.command[0]) == "" {
		return runtimeDockerExecOptions{}, fmt.Errorf("missing command after --")
	}
	return opts, nil
}

func runRuntimeDockerExec(opts runtimeDockerExecOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := runtimeconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "runtime docker-exec failed: %v\n", err)
		return 1
	}
	plan, err := runtimeconfig.PlanDockerExec(cfg, opts.workspace, opts.command)
	if err != nil {
		fmt.Fprintf(stderr, "runtime docker-exec failed: %v\n", err)
		return 1
	}
	if len(plan.Command) == 0 {
		fmt.Fprintln(stderr, "runtime docker-exec failed: empty docker command")
		return 1
	}

	cmd := exec.CommandContext(context.Background(), plan.Command[0], plan.Command[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "runtime docker-exec failed: %v\n", err)
		return 1
	}
	return 0
}

type mcpConfigOptions struct {
	configPath string
}

func parseMCPConfigOptions(args []string) (mcpConfigOptions, error) {
	var opts mcpConfigOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpConfigOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return mcpConfigOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpConfigOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func runMCPValidate(opts mcpConfigOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp validate failed: %v\n", err)
		return 1
	}
	result, err := mcpconfig.Validate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "mcp validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "mcp config valid: servers=%d\n", len(cfg.MCP.Servers))
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return 0
}

func runMCPList(opts mcpConfigOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp list failed: %v\n", err)
		return 1
	}
	servers, err := mcpconfig.ListServers(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "mcp list failed: %v\n", err)
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "name\tcommand\tenabled\ttrust\tcapabilities")
	for _, server := range servers {
		fmt.Fprintf(writer, "%s\t%s\t%t\t%s\t%s\n", server.Name, server.Command, server.Enabled, server.Trust, strings.Join(server.Capabilities, ","))
	}
	_ = writer.Flush()
	return 0
}

type mcpPlanOptions struct {
	configPath   string
	server       string
	outputFormat string
}

func parseMCPPlanOptions(args []string) (mcpPlanOptions, error) {
	opts := mcpPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--server":
			if i+1 >= len(args) {
				return mcpPlanOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpPlanOptions{}, fmt.Errorf("missing --config")
	}
	if opts.server == "" {
		return mcpPlanOptions{}, fmt.Errorf("missing --server")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMCPPlan(opts mcpPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp plan failed: %v\n", err)
		return 1
	}
	plan, err := mcpconfig.PlanServer(cfg, opts.server)
	if err != nil {
		fmt.Fprintf(stderr, "mcp plan failed: %v\n", err)
		return 1
	}
	if opts.outputFormat == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			fmt.Fprintf(stderr, "mcp plan output failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "server: %s\n", plan.Server)
	fmt.Fprintf(stdout, "enabled: %t\n", plan.Enabled)
	fmt.Fprintf(stdout, "trust: %s\n", plan.Trust)
	fmt.Fprintf(stdout, "capabilities: %s\n", strings.Join(plan.Capabilities, ","))
	fmt.Fprintf(stdout, "command: %s\n", plan.Command)
	if len(plan.Args) > 0 {
		fmt.Fprintf(stdout, "args: %s\n", strings.Join(plan.Args, " "))
	} else {
		fmt.Fprintln(stdout, "args:")
	}
	if len(plan.EnvNames) > 0 {
		fmt.Fprintf(stdout, "env: %s\n", strings.Join(plan.EnvNames, ","))
	} else {
		fmt.Fprintln(stdout, "env:")
	}
	for _, warning := range plan.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return 0
}

type mcpReportOptions struct {
	configPath   string
	outputFormat string
}

func parseMCPReportOptions(args []string) (mcpReportOptions, error) {
	opts := mcpReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpReportOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpReportOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMCPDoctor(opts mcpReportOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp doctor failed: %v\n", err)
		return 1
	}
	report, err := mcpconfig.Doctor(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "mcp doctor failed: %v\n", err)
		return 1
	}
	if opts.outputFormat == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "mcp doctor output failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "mcp doctor:")
	for _, server := range report.Servers {
		fmt.Fprintf(stdout, "server %s: command=%s enabled=%t command_available=%v trust=%s risk=%s capabilities=%s env=%s\n",
			server.Name,
			server.Command,
			server.Enabled,
			server.CommandAvailable,
			server.Trust,
			server.RiskLevel,
			strings.Join(server.Capabilities, ","),
			formatMCPEnvRequirements(server.EnvRequirements),
		)
	}
	for _, server := range report.Servers {
		for _, warning := range server.Warnings {
			fmt.Fprintf(stdout, "warning: %s: %s\n", server.Name, warning)
		}
	}
	return 0
}

func runMCPRisk(opts mcpReportOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp risk failed: %v\n", err)
		return 1
	}
	report, err := mcpconfig.Risk(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "mcp risk failed: %v\n", err)
		return 1
	}
	if opts.outputFormat == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "mcp risk output failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "mcp risk:")
	fmt.Fprintf(stdout, "total_servers: %d\n", report.TotalServers)
	fmt.Fprintf(stdout, "enabled_servers: %d\n", report.EnabledServers)
	fmt.Fprintf(stdout, "disabled_servers: %d\n", report.DisabledServers)
	fmt.Fprintf(stdout, "external_servers: %d\n", report.ExternalServers)
	fmt.Fprintf(stdout, "write_capability_servers: %d\n", report.WriteCapabilityServers)
	fmt.Fprintf(stdout, "exec_capability_servers: %d\n", report.ExecCapabilityServers)
	fmt.Fprintf(stdout, "missing_env_count: %d\n", report.MissingEnvCount)
	fmt.Fprintf(stdout, "high_risk_count: %d\n", report.HighRiskCount)
	fmt.Fprintf(stdout, "medium_risk_count: %d\n", report.MediumRiskCount)
	fmt.Fprintf(stdout, "low_risk_count: %d\n", report.LowRiskCount)
	return 0
}

type mcpDockerPlanOptions struct {
	configPath        string
	server            string
	runtimeConfigPath string
	workspace         string
	outputFormat      string
}

func parseMCPDockerPlanOptions(args []string) (mcpDockerPlanOptions, error) {
	opts := mcpDockerPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpDockerPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--server":
			if i+1 >= len(args) {
				return mcpDockerPlanOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return mcpDockerPlanOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return mcpDockerPlanOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpDockerPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpDockerPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpDockerPlanOptions{}, fmt.Errorf("missing --config")
	}
	if opts.server == "" {
		return mcpDockerPlanOptions{}, fmt.Errorf("missing --server")
	}
	if opts.runtimeConfigPath == "" {
		return mcpDockerPlanOptions{}, fmt.Errorf("missing --runtime-config")
	}
	if opts.workspace == "" {
		return mcpDockerPlanOptions{}, fmt.Errorf("missing --workspace")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpDockerPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMCPDockerPlan(opts mcpDockerPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp docker-plan failed: %v\n", err)
		return 1
	}
	runtimeCfg, err := runtimeconfig.Load(opts.runtimeConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp docker-plan failed: %v\n", err)
		return 1
	}
	plan, err := mcpconfig.PlanDockerLaunch(cfg, opts.server, runtimeCfg, opts.workspace)
	if err != nil {
		fmt.Fprintf(stderr, "mcp docker-plan failed: %v\n", err)
		return 1
	}
	if opts.outputFormat == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			fmt.Fprintf(stderr, "mcp docker-plan output failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "mcp docker-plan: plan only, not executed")
	fmt.Fprintf(stdout, "server: %s\n", plan.Server)
	fmt.Fprintf(stdout, "trust: %s\n", plan.Trust)
	fmt.Fprintf(stdout, "capabilities: %s\n", strings.Join(plan.Capabilities, ","))
	if len(plan.EnvNames) > 0 {
		fmt.Fprintf(stdout, "env: %s\n", strings.Join(plan.EnvNames, ","))
	} else {
		fmt.Fprintln(stdout, "env:")
	}
	for _, warning := range plan.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	fmt.Fprintf(stdout, "command: %s\n", plan.Display)
	return 0
}

func formatMCPEnvRequirements(requirements []mcpconfig.EnvRequirement) string {
	if len(requirements) == 0 {
		return ""
	}
	parts := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		parts = append(parts, requirement.Name+"="+requirement.State)
	}
	return strings.Join(parts, ",")
}

func runMCPFakeServer(stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if err := mcpsmoke.RunFakeServer(context.Background(), stdin, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "mcp fake-server failed: %v\n", err)
		return 1
	}
	return 0
}

type mcpSmokeOptions struct {
	configPath        string
	server            string
	artifactsDir      string
	timeoutSeconds    int
	runtime           string
	runtimeConfigPath string
	workspace         string
}

type mcpToolSmokeOptions struct {
	configPath        string
	server            string
	tool              string
	arguments         string
	artifactsDir      string
	timeoutSeconds    int
	runtime           string
	runtimeConfigPath string
	workspace         string
	policyPath        string
}

type mcpDiscoverOptions struct {
	configPath        string
	server            string
	artifactsDir      string
	timeoutSeconds    int
	runtime           string
	runtimeConfigPath string
	workspace         string
	policyPath        string
}

type mcpCallSmokeOptions struct {
	configPath        string
	server            string
	tool              string
	arguments         string
	artifactsDir      string
	timeoutSeconds    int
	runtime           string
	runtimeConfigPath string
	workspace         string
	policyPath        string
}

type mcpProposalNewOptions struct {
	server            string
	tool              string
	arguments         string
	reason            string
	requestedBy       string
	policyPath        string
	configPath        string
	runtime           string
	runtimeConfigPath string
	workspace         string
	outputPath        string
}

type mcpProposalInspectOptions struct {
	proposalPath string
	outputFormat string
}

type mcpProposalLintOptions struct {
	proposalPath string
	configPath   string
	policyPath   string
}

type mcpProposalPreflightOptions struct {
	proposalPath string
	configPath   string
	policyPath   string
	outputPath   string
}

type mcpProposalApproveOptions struct {
	proposalPath    string
	policyPath      string
	decision        string
	reason          string
	approvedBy      string
	outputPath      string
	confirmReadOnly bool
}

type mcpProposalExecuteOptions struct {
	proposalPath   string
	approvalPath   string
	configPath     string
	policyPath     string
	artifactsDir   string
	confirmExecute bool
	timeoutSeconds int
}

type mcpApprovalInspectOptions struct {
	approvalPath string
	outputFormat string
}

func parseMCPSmokeOptions(args []string) (mcpSmokeOptions, error) {
	opts := mcpSmokeOptions{timeoutSeconds: 5, runtime: mcpsmoke.RuntimeLocal}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--server":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--timeout-seconds":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --timeout-seconds")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return mcpSmokeOptions{}, fmt.Errorf("--timeout-seconds must be a positive integer")
			}
			opts.timeoutSeconds = value
			i++
		case "--runtime":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --runtime")
			}
			opts.runtime = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return mcpSmokeOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		default:
			return mcpSmokeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpSmokeOptions{}, fmt.Errorf("missing --config")
	}
	if opts.server == "" {
		return mcpSmokeOptions{}, fmt.Errorf("missing --server")
	}
	if opts.artifactsDir == "" {
		return mcpSmokeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	switch opts.runtime {
	case mcpsmoke.RuntimeLocal, mcpsmoke.RuntimeDocker:
	default:
		return mcpSmokeOptions{}, fmt.Errorf("unsupported runtime %q", opts.runtime)
	}
	return opts, nil
}

func parseMCPToolSmokeOptions(args []string) (mcpToolSmokeOptions, error) {
	opts := mcpToolSmokeOptions{timeoutSeconds: 5, runtime: mcpsmoke.RuntimeLocal}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--server":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--tool":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --tool")
			}
			opts.tool = args[i+1]
			i++
		case "--arguments":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --arguments")
			}
			opts.arguments = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--timeout-seconds":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --timeout-seconds")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return mcpToolSmokeOptions{}, fmt.Errorf("--timeout-seconds must be a positive integer")
			}
			opts.timeoutSeconds = value
			i++
		case "--runtime":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --runtime")
			}
			opts.runtime = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpToolSmokeOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return mcpToolSmokeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpToolSmokeOptions{}, fmt.Errorf("missing --config")
	}
	if opts.server == "" {
		return mcpToolSmokeOptions{}, fmt.Errorf("missing --server")
	}
	if opts.tool == "" {
		return mcpToolSmokeOptions{}, fmt.Errorf("missing --tool")
	}
	if opts.arguments == "" {
		return mcpToolSmokeOptions{}, fmt.Errorf("missing --arguments")
	}
	if opts.artifactsDir == "" {
		return mcpToolSmokeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	switch opts.runtime {
	case mcpsmoke.RuntimeLocal, mcpsmoke.RuntimeDocker:
	default:
		return mcpToolSmokeOptions{}, fmt.Errorf("unsupported runtime %q", opts.runtime)
	}
	return opts, nil
}

func parseMCPDiscoverOptions(args []string) (mcpDiscoverOptions, error) {
	opts := mcpDiscoverOptions{timeoutSeconds: 5, runtime: mcpsmoke.RuntimeLocal}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--server":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--timeout-seconds":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --timeout-seconds")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return mcpDiscoverOptions{}, fmt.Errorf("--timeout-seconds must be a positive integer")
			}
			opts.timeoutSeconds = value
			i++
		case "--runtime":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --runtime")
			}
			opts.runtime = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpDiscoverOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return mcpDiscoverOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpDiscoverOptions{}, fmt.Errorf("missing --config")
	}
	if opts.server == "" {
		return mcpDiscoverOptions{}, fmt.Errorf("missing --server")
	}
	if opts.artifactsDir == "" {
		return mcpDiscoverOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.policyPath == "" {
		return mcpDiscoverOptions{}, fmt.Errorf("missing --policy")
	}
	switch opts.runtime {
	case mcpsmoke.RuntimeLocal, mcpsmoke.RuntimeDocker:
	default:
		return mcpDiscoverOptions{}, fmt.Errorf("unsupported runtime %q", opts.runtime)
	}
	return opts, nil
}

func parseMCPCallSmokeOptions(args []string) (mcpCallSmokeOptions, error) {
	opts := mcpCallSmokeOptions{timeoutSeconds: 5, runtime: mcpsmoke.RuntimeDocker}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--server":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--tool":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --tool")
			}
			opts.tool = args[i+1]
			i++
		case "--arguments":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --arguments")
			}
			opts.arguments = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--timeout-seconds":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --timeout-seconds")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return mcpCallSmokeOptions{}, fmt.Errorf("--timeout-seconds must be a positive integer")
			}
			opts.timeoutSeconds = value
			i++
		case "--runtime":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --runtime")
			}
			opts.runtime = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpCallSmokeOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return mcpCallSmokeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return mcpCallSmokeOptions{}, fmt.Errorf("missing --config")
	}
	if opts.server == "" {
		return mcpCallSmokeOptions{}, fmt.Errorf("missing --server")
	}
	if opts.tool == "" {
		return mcpCallSmokeOptions{}, fmt.Errorf("missing --tool")
	}
	if opts.arguments == "" {
		return mcpCallSmokeOptions{}, fmt.Errorf("missing --arguments")
	}
	if opts.artifactsDir == "" {
		return mcpCallSmokeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.policyPath == "" {
		return mcpCallSmokeOptions{}, fmt.Errorf("missing --policy")
	}
	switch opts.runtime {
	case mcpsmoke.RuntimeLocal, mcpsmoke.RuntimeDocker:
	default:
		return mcpCallSmokeOptions{}, fmt.Errorf("unsupported runtime %q", opts.runtime)
	}
	return opts, nil
}

func runMCPSmoke(opts mcpSmokeOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp smoke failed: %v\n", err)
		return 1
	}
	var runtimeCfg *runtimeconfig.Config
	if opts.runtime == mcpsmoke.RuntimeDocker && opts.runtimeConfigPath != "" {
		loaded, err := runtimeconfig.Load(opts.runtimeConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "mcp smoke failed: %v\n", err)
			return 1
		}
		runtimeCfg = &loaded
	}
	result, err := mcpsmoke.Smoke(context.Background(), mcpsmoke.Options{
		Config:        cfg,
		Server:        opts.server,
		ArtifactsDir:  opts.artifactsDir,
		Timeout:       time.Duration(opts.timeoutSeconds) * time.Second,
		Runtime:       opts.runtime,
		RuntimeConfig: runtimeCfg,
		Workspace:     opts.workspace,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp smoke failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "mcp smoke: fake/test smoke only")
	fmt.Fprintf(stdout, "server: %s\n", result.Server)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "runtime: %s\n", result.Runtime)
	fmt.Fprintf(stdout, "artifacts_dir: %s\n", result.ArtifactsDir)
	fmt.Fprintf(stdout, "transcript: %s\n", result.TranscriptPath)
	return 0
}

func runMCPDiscover(opts mcpDiscoverOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp discover failed: %v\n", err)
		return 1
	}
	var runtimeCfg *runtimeconfig.Config
	if opts.runtime == mcpsmoke.RuntimeDocker && opts.runtimeConfigPath != "" {
		loaded, err := runtimeconfig.Load(opts.runtimeConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "mcp discover failed: %v\n", err)
			return 1
		}
		runtimeCfg = &loaded
	}
	policy, err := mcpsmoke.LoadDiscoveryPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp discover failed: %v\n", err)
		return 1
	}
	result, err := mcpsmoke.Discover(context.Background(), mcpsmoke.DiscoveryOptions{
		Config:        cfg,
		Server:        opts.server,
		ArtifactsDir:  opts.artifactsDir,
		Timeout:       time.Duration(opts.timeoutSeconds) * time.Second,
		Runtime:       opts.runtime,
		RuntimeConfig: runtimeCfg,
		Workspace:     opts.workspace,
		Policy:        &policy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp discover failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "mcp discover: read-only tools/list only")
	fmt.Fprintf(stdout, "server: %s\n", result.Server)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "runtime: %s\n", result.Runtime)
	fmt.Fprintf(stdout, "tool_count: %d\n", result.ToolCount)
	fmt.Fprintf(stdout, "artifacts_dir: %s\n", result.ArtifactsDir)
	fmt.Fprintf(stdout, "transcript: %s\n", result.TranscriptPath)
	fmt.Fprintf(stdout, "tools_list: %s\n", result.ToolsListPath)
	return 0
}

func runMCPCallSmoke(opts mcpCallSmokeOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp call-smoke failed: %v\n", err)
		return 1
	}
	var runtimeCfg *runtimeconfig.Config
	if opts.runtime == mcpsmoke.RuntimeDocker && opts.runtimeConfigPath != "" {
		loaded, err := runtimeconfig.Load(opts.runtimeConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "mcp call-smoke failed: %v\n", err)
			return 1
		}
		runtimeCfg = &loaded
	}
	policy, err := mcpsmoke.LoadCallPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp call-smoke failed: %v\n", err)
		return 1
	}
	result, err := mcpsmoke.CallSmoke(context.Background(), mcpsmoke.CallOptions{
		Config:        cfg,
		Server:        opts.server,
		Tool:          opts.tool,
		Arguments:     []byte(opts.arguments),
		ArtifactsDir:  opts.artifactsDir,
		Timeout:       time.Duration(opts.timeoutSeconds) * time.Second,
		Runtime:       opts.runtime,
		RuntimeConfig: runtimeCfg,
		Workspace:     opts.workspace,
		Policy:        &policy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp call-smoke failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "mcp call-smoke: read-only policy-gated tool call")
	fmt.Fprintf(stdout, "server: %s\n", result.Server)
	fmt.Fprintf(stdout, "tool: %s\n", result.Tool)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "runtime: %s\n", result.Runtime)
	fmt.Fprintf(stdout, "tool_calls: %d\n", result.ToolCalls)
	fmt.Fprintf(stdout, "response_truncated: %t\n", result.ResponseTruncated)
	fmt.Fprintf(stdout, "artifacts_dir: %s\n", result.ArtifactsDir)
	fmt.Fprintf(stdout, "transcript: %s\n", result.TranscriptPath)
	fmt.Fprintf(stdout, "response: %s\n", result.ResponsePath)
	return 0
}

func parseMCPProposalNewOptions(args []string) (mcpProposalNewOptions, error) {
	var opts mcpProposalNewOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --server")
			}
			opts.server = args[i+1]
			i++
		case "--tool":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --tool")
			}
			opts.tool = args[i+1]
			i++
		case "--arguments":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --arguments")
			}
			opts.arguments = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--requested-by":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --requested-by")
			}
			opts.requestedBy = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--config":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--runtime":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --runtime")
			}
			opts.runtime = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return mcpProposalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return mcpProposalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.server == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --server")
	}
	if opts.tool == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --tool")
	}
	if opts.arguments == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --arguments")
	}
	if opts.reason == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.policyPath == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.runtime == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --runtime")
	}
	switch opts.runtime {
	case mcpsmoke.RuntimeLocal, mcpsmoke.RuntimeDocker:
	default:
		return mcpProposalNewOptions{}, fmt.Errorf("unsupported runtime %q", opts.runtime)
	}
	if opts.runtime == mcpsmoke.RuntimeDocker && opts.runtimeConfigPath == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --runtime-config")
	}
	if opts.workspace == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --workspace")
	}
	if opts.outputPath == "" {
		return mcpProposalNewOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseMCPProposalInspectOptions(args []string) (mcpProposalInspectOptions, error) {
	opts := mcpProposalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return mcpProposalInspectOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpProposalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpProposalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return mcpProposalInspectOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpProposalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseMCPProposalLintOptions(args []string) (mcpProposalLintOptions, error) {
	var opts mcpProposalLintOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return mcpProposalLintOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--config":
			if i+1 >= len(args) {
				return mcpProposalLintOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpProposalLintOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return mcpProposalLintOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return mcpProposalLintOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.configPath == "" {
		return mcpProposalLintOptions{}, fmt.Errorf("missing --config")
	}
	if opts.policyPath == "" {
		return mcpProposalLintOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

func parseMCPProposalPreflightOptions(args []string) (mcpProposalPreflightOptions, error) {
	var opts mcpProposalPreflightOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return mcpProposalPreflightOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--config":
			if i+1 >= len(args) {
				return mcpProposalPreflightOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpProposalPreflightOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return mcpProposalPreflightOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return mcpProposalPreflightOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return mcpProposalPreflightOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.configPath == "" {
		return mcpProposalPreflightOptions{}, fmt.Errorf("missing --config")
	}
	if opts.policyPath == "" {
		return mcpProposalPreflightOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputPath == "" {
		return mcpProposalPreflightOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseMCPProposalApproveOptions(args []string) (mcpProposalApproveOptions, error) {
	var opts mcpProposalApproveOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return mcpProposalApproveOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpProposalApproveOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--decision":
			if i+1 >= len(args) {
				return mcpProposalApproveOptions{}, fmt.Errorf("missing value for --decision")
			}
			opts.decision = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return mcpProposalApproveOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--approved-by":
			if i+1 >= len(args) {
				return mcpProposalApproveOptions{}, fmt.Errorf("missing value for --approved-by")
			}
			opts.approvedBy = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return mcpProposalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-read-only":
			opts.confirmReadOnly = true
		default:
			return mcpProposalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return mcpProposalApproveOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return mcpProposalApproveOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.decision == "" {
		return mcpProposalApproveOptions{}, fmt.Errorf("missing --decision")
	}
	if opts.reason == "" {
		return mcpProposalApproveOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return mcpProposalApproveOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmReadOnly {
		return mcpProposalApproveOptions{}, fmt.Errorf("missing --confirm-read-only")
	}
	return opts, nil
}

func parseMCPProposalExecuteOptions(args []string) (mcpProposalExecuteOptions, error) {
	opts := mcpProposalExecuteOptions{timeoutSeconds: 5}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return mcpProposalExecuteOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return mcpProposalExecuteOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--config":
			if i+1 >= len(args) {
				return mcpProposalExecuteOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return mcpProposalExecuteOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return mcpProposalExecuteOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--timeout-seconds":
			if i+1 >= len(args) {
				return mcpProposalExecuteOptions{}, fmt.Errorf("missing value for --timeout-seconds")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return mcpProposalExecuteOptions{}, fmt.Errorf("--timeout-seconds must be a positive integer")
			}
			opts.timeoutSeconds = value
			i++
		case "--confirm-execute":
			opts.confirmExecute = true
		default:
			return mcpProposalExecuteOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return mcpProposalExecuteOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return mcpProposalExecuteOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.configPath == "" {
		return mcpProposalExecuteOptions{}, fmt.Errorf("missing --config")
	}
	if opts.policyPath == "" {
		return mcpProposalExecuteOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.artifactsDir == "" {
		return mcpProposalExecuteOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if !opts.confirmExecute {
		return mcpProposalExecuteOptions{}, fmt.Errorf("missing --confirm-execute")
	}
	return opts, nil
}

func parseMCPApprovalInspectOptions(args []string) (mcpApprovalInspectOptions, error) {
	opts := mcpApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return mcpApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return mcpApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpApprovalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMCPProposalNew(opts mcpProposalNewOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := mcpapproval.NewProposal(mcpapproval.NewProposalOptions{
		Server:            opts.server,
		Tool:              opts.tool,
		Arguments:         []byte(opts.arguments),
		Reason:            opts.reason,
		RequestedBy:       opts.requestedBy,
		PolicyPath:        opts.policyPath,
		ConfigPath:        opts.configPath,
		Runtime:           opts.runtime,
		RuntimeConfigPath: opts.runtimeConfigPath,
		Workspace:         opts.workspace,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal new failed: %v\n", err)
		return 1
	}
	if err := writeMCPProposal(opts.outputPath, proposal); err != nil {
		fmt.Fprintf(stderr, "mcp proposal new failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "mcp proposal written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", proposal.ID)
	fmt.Fprintf(stdout, "arguments_sha256: %s\n", proposal.ArgumentsSHA256)
	return 0
}

type mcpProposalInspectView struct {
	ID                  string `json:"id"`
	CreatedAt           string `json:"created_at"`
	Server              string `json:"server"`
	Tool                string `json:"tool"`
	ArgumentsSHA256     string `json:"arguments_sha256"`
	RequestedBy         string `json:"requested_by"`
	SourceType          string `json:"source_type"`
	DiscoveryArtifact   string `json:"discovery_artifact,omitempty"`
	PolicyPath          string `json:"policy_path"`
	ConfigPath          string `json:"config_path,omitempty"`
	ConfigSHA256        string `json:"config_sha256,omitempty"`
	Runtime             string `json:"runtime"`
	RuntimeConfigPath   string `json:"runtime_config_path,omitempty"`
	RuntimeConfigSHA256 string `json:"runtime_config_sha256,omitempty"`
	Workspace           string `json:"workspace,omitempty"`
	Status              string `json:"status"`
}

type mcpApprovalInspectView struct {
	ProposalID          string `json:"proposal_id"`
	Decision            string `json:"decision"`
	ApprovedAt          string `json:"approved_at"`
	ApprovedBy          string `json:"approved_by"`
	ArgumentsSHA256     string `json:"arguments_sha256"`
	PolicySHA256        string `json:"policy_sha256"`
	ConfigSHA256        string `json:"config_sha256,omitempty"`
	RuntimeConfigSHA256 string `json:"runtime_config_sha256,omitempty"`
	ConfirmReadOnly     bool   `json:"confirm_read_only"`
}

func runMCPProposalInspect(opts mcpProposalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := mcpapproval.LoadProposal(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal inspect failed: %v\n", err)
		return 1
	}
	view := mcpProposalInspectView{
		ID:                  proposal.ID,
		CreatedAt:           proposal.CreatedAt.Format(time.RFC3339Nano),
		Server:              proposal.Server,
		Tool:                proposal.Tool,
		ArgumentsSHA256:     proposal.ArgumentsSHA256,
		RequestedBy:         proposal.RequestedBy,
		SourceType:          proposal.Source.Type,
		DiscoveryArtifact:   proposal.Source.DiscoveryArtifact,
		PolicyPath:          proposal.PolicyPath,
		ConfigPath:          proposal.ConfigPath,
		ConfigSHA256:        proposal.ConfigSHA256,
		Runtime:             proposal.Runtime,
		RuntimeConfigPath:   proposal.RuntimeConfigPath,
		RuntimeConfigSHA256: proposal.RuntimeConfigSHA256,
		Workspace:           proposal.Workspace,
		Status:              proposal.Status,
	}
	if opts.outputFormat == "json" {
		return writeJSONInspect(stdout, stderr, "mcp proposal inspect", view)
	}
	fmt.Fprintf(stdout, "proposal_id: %s\n", view.ID)
	fmt.Fprintf(stdout, "status: %s\n", view.Status)
	fmt.Fprintf(stdout, "server: %s\n", view.Server)
	fmt.Fprintf(stdout, "tool: %s\n", view.Tool)
	fmt.Fprintf(stdout, "arguments_sha256: %s\n", view.ArgumentsSHA256)
	fmt.Fprintf(stdout, "policy_path: %s\n", view.PolicyPath)
	fmt.Fprintf(stdout, "config_path: %s\n", view.ConfigPath)
	fmt.Fprintf(stdout, "config_sha256: %s\n", view.ConfigSHA256)
	fmt.Fprintf(stdout, "runtime: %s\n", view.Runtime)
	fmt.Fprintf(stdout, "runtime_config_path: %s\n", view.RuntimeConfigPath)
	fmt.Fprintf(stdout, "runtime_config_sha256: %s\n", view.RuntimeConfigSHA256)
	fmt.Fprintf(stdout, "requested_by: %s\n", view.RequestedBy)
	fmt.Fprintf(stdout, "source_type: %s\n", view.SourceType)
	return 0
}

func runMCPApprovalInspect(opts mcpApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	approval, err := mcpapproval.LoadApproval(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp approval inspect failed: %v\n", err)
		return 1
	}
	view := mcpApprovalInspectView{
		ProposalID:          approval.ProposalID,
		Decision:            approval.Decision,
		ApprovedAt:          approval.ApprovedAt.Format(time.RFC3339Nano),
		ApprovedBy:          approval.ApprovedBy,
		ArgumentsSHA256:     approval.ArgumentsSHA256,
		PolicySHA256:        approval.PolicySHA256,
		ConfigSHA256:        approval.ConfigSHA256,
		RuntimeConfigSHA256: approval.RuntimeConfigSHA256,
		ConfirmReadOnly:     approval.ConfirmReadOnly,
	}
	if opts.outputFormat == "json" {
		return writeJSONInspect(stdout, stderr, "mcp approval inspect", view)
	}
	fmt.Fprintf(stdout, "proposal_id: %s\n", view.ProposalID)
	fmt.Fprintf(stdout, "decision: %s\n", view.Decision)
	fmt.Fprintf(stdout, "approved_by: %s\n", view.ApprovedBy)
	fmt.Fprintf(stdout, "arguments_sha256: %s\n", view.ArgumentsSHA256)
	fmt.Fprintf(stdout, "policy_sha256: %s\n", view.PolicySHA256)
	fmt.Fprintf(stdout, "config_sha256: %s\n", view.ConfigSHA256)
	fmt.Fprintf(stdout, "runtime_config_sha256: %s\n", view.RuntimeConfigSHA256)
	fmt.Fprintf(stdout, "confirm_read_only: %t\n", view.ConfirmReadOnly)
	return 0
}

func writeJSONInspect(stdout io.Writer, stderr io.Writer, label string, value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return 0
}

func runMCPProposalLint(opts mcpProposalLintOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, cfg, policy, runtimeCfg, err := loadMCPProposalValidationInputs(opts.proposalPath, opts.configPath, opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal lint failed: %v\n", err)
		return 1
	}
	result := mcpapproval.LintProposal(proposal, mcpapproval.ValidationOptions{Config: cfg, Policy: policy, RuntimeConfig: runtimeCfg})
	printMCPProposalLint(stdout, result)
	if result.Status == mcpapproval.PreflightStatusFailed {
		return 1
	}
	return 0
}

func runMCPProposalPreflight(opts mcpProposalPreflightOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, cfg, policy, runtimeCfg, err := loadMCPProposalValidationInputs(opts.proposalPath, opts.configPath, opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal preflight failed: %v\n", err)
		return 1
	}
	preflight := mcpapproval.BuildPreflight(proposal, mcpapproval.ValidationOptions{Config: cfg, Policy: policy, RuntimeConfig: runtimeCfg})
	if err := writeMCPPreflight(opts.outputPath, preflight); err != nil {
		fmt.Fprintf(stderr, "mcp proposal preflight failed: %v\n", err)
		return 1
	}
	printMCPPreflight(stdout, preflight)
	if preflight.Status == mcpapproval.PreflightStatusFailed {
		return 1
	}
	return 0
}

func runMCPProposalApprove(opts mcpProposalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := mcpapproval.LoadProposal(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal approve failed: %v\n", err)
		return 1
	}
	policy, err := mcpsmoke.LoadCallPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal approve failed: %v\n", err)
		return 1
	}
	if err := mcpsmoke.ValidateCallPolicy(policy); err != nil {
		fmt.Fprintf(stderr, "mcp proposal approve failed: %v\n", err)
		return 1
	}
	approval, err := mcpapproval.BuildApproval(proposal, mcpapproval.NewApprovalOptions{
		Decision:        opts.decision,
		ApprovedBy:      opts.approvedBy,
		Reason:          opts.reason,
		PolicyPath:      opts.policyPath,
		ConfirmReadOnly: opts.confirmReadOnly,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal approve failed: %v\n", err)
		return 1
	}
	if err := writeMCPApproval(opts.outputPath, approval); err != nil {
		fmt.Fprintf(stderr, "mcp proposal approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "mcp approval written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", approval.ProposalID)
	fmt.Fprintf(stdout, "decision: %s\n", approval.Decision)
	fmt.Fprintf(stdout, "policy_sha256: %s\n", approval.PolicySHA256)
	return 0
}

func runMCPProposalExecute(opts mcpProposalExecuteOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, cfg, policy, runtimeCfg, err := loadMCPProposalValidationInputs(opts.proposalPath, opts.configPath, opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal execute failed: %v\n", err)
		return 1
	}
	approval, err := mcpapproval.LoadApproval(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal execute failed: %v\n", err)
		return 1
	}
	preflight := mcpapproval.BuildPreflight(proposal, mcpapproval.ValidationOptions{Config: cfg, Policy: policy, RuntimeConfig: runtimeCfg})
	if err := mcpapproval.ValidateExecution(proposal, approval, opts.policyPath, opts.configPath, proposal.RuntimeConfigPath, preflight); err != nil {
		fmt.Fprintf(stderr, "mcp proposal execute failed: %v\n", err)
		return 1
	}
	result, callErr := mcpsmoke.CallSmoke(context.Background(), mcpsmoke.CallOptions{
		Config:        cfg,
		Server:        proposal.Server,
		Tool:          proposal.Tool,
		Arguments:     []byte(proposal.Arguments),
		ArtifactsDir:  opts.artifactsDir,
		Timeout:       time.Duration(opts.timeoutSeconds) * time.Second,
		Runtime:       proposal.Runtime,
		RuntimeConfig: runtimeCfg,
		Workspace:     proposal.Workspace,
		Policy:        &policy,
	})
	bundle, err := mcpapproval.BuildExecutionBundle(proposal, approval, preflight, result, opts.policyPath, opts.configPath, proposal.RuntimeConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposal execute failed: %v\n", err)
		return 1
	}
	bundlePath := filepath.Join(opts.artifactsDir, "mcp-call-execution-bundle.json")
	if err := writeMCPExecutionBundle(bundlePath, bundle); err != nil {
		fmt.Fprintf(stderr, "mcp proposal execute failed: %v\n", err)
		return 1
	}
	if callErr != nil {
		fmt.Fprintf(stderr, "mcp proposal execute failed: %v\n", callErr)
		return 1
	}
	fmt.Fprintln(stdout, "mcp proposal execute: approved read-only call smoke")
	fmt.Fprintf(stdout, "proposal_id: %s\n", proposal.ID)
	fmt.Fprintf(stdout, "server: %s\n", result.Server)
	fmt.Fprintf(stdout, "tool: %s\n", result.Tool)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "tool_calls: %d\n", result.ToolCalls)
	fmt.Fprintf(stdout, "response_truncated: %t\n", result.ResponseTruncated)
	fmt.Fprintf(stdout, "artifacts_dir: %s\n", result.ArtifactsDir)
	fmt.Fprintf(stdout, "transcript: %s\n", result.TranscriptPath)
	fmt.Fprintf(stdout, "response: %s\n", result.ResponsePath)
	fmt.Fprintf(stdout, "execution_bundle: %s\n", bundlePath)
	return 0
}

type mcpProposalsListOptions struct {
	storePath    string
	status       string
	worker       string
	taskID       string
	outputFormat string
}

func parseMCPProposalsListOptions(args []string) (mcpProposalsListOptions, error) {
	opts := mcpProposalsListOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return mcpProposalsListOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--status":
			if i+1 >= len(args) {
				return mcpProposalsListOptions{}, fmt.Errorf("missing value for --status")
			}
			opts.status = args[i+1]
			i++
		case "--worker":
			if i+1 >= len(args) {
				return mcpProposalsListOptions{}, fmt.Errorf("missing value for --worker")
			}
			opts.worker = args[i+1]
			i++
		case "--task":
			if i+1 >= len(args) {
				return mcpProposalsListOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskID = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpProposalsListOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpProposalsListOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return mcpProposalsListOptions{}, fmt.Errorf("missing --store")
	}
	if err := mcpproposalqueue.ValidateListStatus(opts.status); err != nil {
		return mcpProposalsListOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpProposalsListOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

type mcpProposalsShowOptions struct {
	storePath    string
	runID        string
	outputFormat string
}

func parseMCPProposalsShowOptions(args []string) (mcpProposalsShowOptions, error) {
	opts := mcpProposalsShowOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return mcpProposalsShowOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return mcpProposalsShowOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return mcpProposalsShowOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return mcpProposalsShowOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return mcpProposalsShowOptions{}, fmt.Errorf("missing --store")
	}
	if opts.runID == "" {
		return mcpProposalsShowOptions{}, fmt.Errorf("missing --run")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return mcpProposalsShowOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

type mcpProposalsExportOptions struct {
	storePath  string
	runID      string
	outputPath string
}

func parseMCPProposalsExportOptions(args []string) (mcpProposalsExportOptions, error) {
	var opts mcpProposalsExportOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return mcpProposalsExportOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return mcpProposalsExportOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return mcpProposalsExportOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return mcpProposalsExportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return mcpProposalsExportOptions{}, fmt.Errorf("missing --store")
	}
	if opts.runID == "" {
		return mcpProposalsExportOptions{}, fmt.Errorf("missing --run")
	}
	if opts.outputPath == "" {
		return mcpProposalsExportOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMCPProposalsList(opts mcpProposalsListOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := mcpproposalqueue.List(context.Background(), db, mcpproposalqueue.ListOptions{
		Status: opts.status,
		Worker: opts.worker,
		TaskID: opts.taskID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposals list failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = mcpproposalqueue.WriteListJSON(result, stdout)
	default:
		err = mcpproposalqueue.WriteListText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposals list failed: %v\n", err)
		return 1
	}
	return 0
}

func runMCPProposalsShow(opts mcpProposalsShowOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	detail, err := mcpproposalqueue.Show(context.Background(), db, opts.runID)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposals show failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = mcpproposalqueue.WriteShowJSON(detail, stdout)
	default:
		err = mcpproposalqueue.WriteShowText(detail, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposals show failed: %v\n", err)
		return 1
	}
	return 0
}

func runMCPProposalsExport(opts mcpProposalsExportOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := mcpproposalqueue.Export(context.Background(), db, opts.runID, opts.outputPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp proposals export failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "mcp proposal exported: %s\n", result.OutputPath)
	fmt.Fprintf(stdout, "exported_proposal_sha256: %s\n", result.ProposalSHA256)
	fmt.Fprintf(stdout, "next_step_hint: %s\n", result.NextStepHint)
	return 0
}

func loadMCPProposalValidationInputs(proposalPath string, configPath string, policyPath string) (mcpapproval.MCPToolCallProposal, mcpconfig.Config, mcpsmoke.CallPolicy, *runtimeconfig.Config, error) {
	proposal, err := mcpapproval.LoadProposal(proposalPath)
	if err != nil {
		return mcpapproval.MCPToolCallProposal{}, mcpconfig.Config{}, mcpsmoke.CallPolicy{}, nil, err
	}
	cfg, err := mcpconfig.Load(configPath)
	if err != nil {
		return proposal, mcpconfig.Config{}, mcpsmoke.CallPolicy{}, nil, err
	}
	policy, err := mcpsmoke.LoadCallPolicy(policyPath)
	if err != nil {
		return proposal, cfg, mcpsmoke.CallPolicy{}, nil, err
	}
	var runtimeCfg *runtimeconfig.Config
	if proposal.Runtime == mcpsmoke.RuntimeDocker && proposal.RuntimeConfigPath != "" {
		loaded, err := runtimeconfig.Load(proposal.RuntimeConfigPath)
		if err != nil {
			return proposal, cfg, policy, nil, err
		}
		runtimeCfg = &loaded
	}
	return proposal, cfg, policy, runtimeCfg, nil
}

func writeMCPProposal(outputPath string, proposal mcpapproval.MCPToolCallProposal) error {
	data, err := proposal.JSON()
	if err != nil {
		return err
	}
	return writeOutputFile(outputPath, data)
}

func writeMCPApproval(outputPath string, approval mcpapproval.MCPToolCallApproval) error {
	data, err := approval.JSON()
	if err != nil {
		return err
	}
	return writeOutputFile(outputPath, data)
}

func writeMCPPreflight(outputPath string, preflight mcpapproval.MCPToolCallPreflight) error {
	data, err := preflight.JSON()
	if err != nil {
		return err
	}
	return writeOutputFile(outputPath, data)
}

func writeMCPExecutionBundle(outputPath string, bundle mcpapproval.MCPToolCallExecutionBundle) error {
	data, err := bundle.JSON()
	if err != nil {
		return err
	}
	return writeOutputFile(outputPath, data)
}

func writeOutputFile(outputPath string, data []byte) error {
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func printMCPProposalLint(stdout io.Writer, result mcpapproval.LintResult) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "violations: %d\n", len(result.Violations))
	for _, violation := range result.Violations {
		fmt.Fprintf(stdout, "- %s\n", violation)
	}
	fmt.Fprintf(stdout, "warnings: %d\n", len(result.Warnings))
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
}

func printMCPPreflight(stdout io.Writer, preflight mcpapproval.MCPToolCallPreflight) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", preflight.ProposalID)
	fmt.Fprintf(stdout, "status: %s\n", preflight.Status)
	fmt.Fprintf(stdout, "failures: %d\n", len(preflight.Failures))
	for _, failure := range preflight.Failures {
		fmt.Fprintf(stdout, "- %s\n", failure)
	}
	fmt.Fprintf(stdout, "warnings: %d\n", len(preflight.Warnings))
	for _, warning := range preflight.Warnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
}

func runMCPToolSmoke(opts mcpToolSmokeOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := mcpconfig.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcp tool-smoke failed: %v\n", err)
		return 1
	}
	var runtimeCfg *runtimeconfig.Config
	if opts.runtime == mcpsmoke.RuntimeDocker && opts.runtimeConfigPath != "" {
		loaded, err := runtimeconfig.Load(opts.runtimeConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "mcp tool-smoke failed: %v\n", err)
			return 1
		}
		runtimeCfg = &loaded
	}
	var policy *mcpsmoke.ToolPolicy
	if opts.policyPath != "" {
		loaded, err := mcpsmoke.LoadToolPolicy(opts.policyPath)
		if err != nil {
			fmt.Fprintf(stderr, "mcp tool-smoke failed: %v\n", err)
			return 1
		}
		policy = &loaded
	}
	result, err := mcpsmoke.ToolSmoke(context.Background(), mcpsmoke.ToolOptions{
		Config:        cfg,
		Server:        opts.server,
		Tool:          opts.tool,
		Arguments:     []byte(opts.arguments),
		ArtifactsDir:  opts.artifactsDir,
		Timeout:       time.Duration(opts.timeoutSeconds) * time.Second,
		Runtime:       opts.runtime,
		RuntimeConfig: runtimeCfg,
		Workspace:     opts.workspace,
		Policy:        policy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mcp tool-smoke failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "mcp tool-smoke: fake/test tool smoke only")
	fmt.Fprintf(stdout, "server: %s\n", result.Server)
	fmt.Fprintf(stdout, "tool: %s\n", result.Tool)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "runtime: %s\n", result.Runtime)
	fmt.Fprintf(stdout, "artifacts_dir: %s\n", result.ArtifactsDir)
	fmt.Fprintf(stdout, "transcript: %s\n", result.TranscriptPath)
	return 0
}

type memoryIndexConfigOptions struct {
	configPath string
}

func parseMemoryIndexConfigOptions(args []string) (memoryIndexConfigOptions, error) {
	var opts memoryIndexConfigOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return memoryIndexConfigOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return memoryIndexConfigOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return memoryIndexConfigOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

type memoryIndexPlanOptions struct {
	configPath   string
	outputFormat string
}

func parseMemoryIndexPlanOptions(args []string) (memoryIndexPlanOptions, error) {
	opts := memoryIndexPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return memoryIndexPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryIndexPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryIndexPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return memoryIndexPlanOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryIndexPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

type memoryIndexBuildOptions struct {
	configPath   string
	artifactsDir string
}

func parseMemoryIndexBuildOptions(args []string) (memoryIndexBuildOptions, error) {
	var opts memoryIndexBuildOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return memoryIndexBuildOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return memoryIndexBuildOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		default:
			return memoryIndexBuildOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return memoryIndexBuildOptions{}, fmt.Errorf("missing --config")
	}
	if opts.artifactsDir == "" {
		return memoryIndexBuildOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	return opts, nil
}

func runMemoryIndexValidate(opts memoryIndexConfigOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := memoryindex.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory index validate failed: %v\n", err)
		return 1
	}
	if err := memoryindex.Validate(cfg); err != nil {
		fmt.Fprintf(stderr, "memory index validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory index validate: ok")
	return 0
}

func runMemoryIndexPlan(opts memoryIndexPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := memoryindex.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory index plan failed: %v\n", err)
		return 1
	}
	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "memory index plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = memoryindex.WritePlanJSON(plan, stdout)
	default:
		err = memoryindex.WritePlanText(plan, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory index plan failed: %v\n", err)
		return 1
	}
	return 0
}

func runMemoryIndexBuild(opts memoryIndexBuildOptions, stdout io.Writer, stderr io.Writer) int {
	configBytes, err := os.ReadFile(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory index build failed: %v\n", err)
		return 1
	}
	cfg, err := memoryindex.Parse(configBytes)
	if err != nil {
		fmt.Fprintf(stderr, "memory index build failed: %v\n", err)
		return 1
	}
	result, err := memoryindex.Build(cfg, opts.artifactsDir, configBytes)
	if err != nil {
		fmt.Fprintf(stderr, "memory index build failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory index build: ok")
	fmt.Fprintf(stdout, "manifest: %s\n", result.Manifest.ManifestPath)
	fmt.Fprintf(stdout, "chunks: %s\n", result.Manifest.ChunksPath)
	fmt.Fprintf(stdout, "source_count: %d\n", result.Manifest.SourceCount)
	fmt.Fprintf(stdout, "chunk_count: %d\n", result.Manifest.ChunkCount)
	fmt.Fprintf(stdout, "skipped_count: %d\n", result.Manifest.SkippedCount)
	return 0
}

func runMemoryIndexDoctor(opts memoryIndexPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := memoryindex.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory index doctor failed: %v\n", err)
		return 1
	}
	result, err := memoryindex.Doctor(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "memory index doctor failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = memoryindex.WriteDoctorJSON(result, stdout)
	default:
		err = memoryindex.WriteDoctorText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory index doctor failed: %v\n", err)
		return 1
	}
	if result.Status == memoryindex.StatusFailed {
		return 1
	}
	return 0
}

type memoryIndexReportOptions struct {
	manifestPath string
	chunksPath   string
	outputFormat string
}

func parseMemoryIndexReportOptions(args []string) (memoryIndexReportOptions, error) {
	opts := memoryIndexReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			if i+1 >= len(args) {
				return memoryIndexReportOptions{}, fmt.Errorf("missing value for --manifest")
			}
			opts.manifestPath = args[i+1]
			i++
		case "--chunks":
			if i+1 >= len(args) {
				return memoryIndexReportOptions{}, fmt.Errorf("missing value for --chunks")
			}
			opts.chunksPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryIndexReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryIndexReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.manifestPath == "" {
		return memoryIndexReportOptions{}, fmt.Errorf("missing --manifest")
	}
	if opts.chunksPath == "" {
		return memoryIndexReportOptions{}, fmt.Errorf("missing --chunks")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryIndexReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMemoryIndexReport(opts memoryIndexReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := memoryindex.Report(opts.manifestPath, opts.chunksPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory index report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = memoryindex.WriteReportJSON(result, stdout)
	default:
		err = memoryindex.WriteReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory index report failed: %v\n", err)
		return 1
	}
	if result.Status == memoryindex.StatusFailed {
		return 1
	}
	return 0
}

type memoryEmbeddingPolicyOptions struct {
	policyPath string
}

func parseMemoryEmbeddingPolicyOptions(args []string) (memoryEmbeddingPolicyOptions, error) {
	var opts memoryEmbeddingPolicyOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryEmbeddingPolicyOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return memoryEmbeddingPolicyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryEmbeddingPolicyOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

type memoryEmbeddingDoctorOptions struct {
	policyPath   string
	outputFormat string
}

func parseMemoryEmbeddingDoctorOptions(args []string) (memoryEmbeddingDoctorOptions, error) {
	opts := memoryEmbeddingDoctorOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryEmbeddingDoctorOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryEmbeddingDoctorOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryEmbeddingDoctorOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryEmbeddingDoctorOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryEmbeddingDoctorOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMemoryEmbeddingValidate(opts memoryEmbeddingPolicyOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := embeddingpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding validate failed: %v\n", err)
		return 1
	}
	if err := embeddingpolicy.Validate(cfg); err != nil {
		fmt.Fprintf(stderr, "memory embedding validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory embedding validate: ok")
	return 0
}

func runMemoryEmbeddingDoctor(opts memoryEmbeddingDoctorOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := embeddingpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding doctor failed: %v\n", err)
		return 1
	}
	result, err := embeddingpolicy.Doctor(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding doctor failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = embeddingpolicy.WriteDoctorJSON(result, stdout)
	default:
		err = embeddingpolicy.WriteDoctorText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding doctor failed: %v\n", err)
		return 1
	}
	if result.Status == embeddingpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runMemoryEmbeddingPlan(opts memoryEmbeddingDoctorOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := embeddingpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding plan failed: %v\n", err)
		return 1
	}
	result, err := embeddingpolicy.Plan(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = embeddingpolicy.WritePlanJSON(result, stdout)
	default:
		err = embeddingpolicy.WritePlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding plan failed: %v\n", err)
		return 1
	}
	if result.Status == embeddingpolicy.StatusFailed {
		return 1
	}
	return 0
}

type memoryEmbeddingBuildFakeOptions struct {
	policyPath   string
	artifactsDir string
	confirmFake  bool
}

func parseMemoryEmbeddingBuildFakeOptions(args []string) (memoryEmbeddingBuildFakeOptions, error) {
	var opts memoryEmbeddingBuildFakeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryEmbeddingBuildFakeOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return memoryEmbeddingBuildFakeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--confirm-fake-vectors":
			opts.confirmFake = true
		default:
			return memoryEmbeddingBuildFakeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryEmbeddingBuildFakeOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.artifactsDir == "" {
		return memoryEmbeddingBuildFakeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if !opts.confirmFake {
		return memoryEmbeddingBuildFakeOptions{}, fmt.Errorf("missing --confirm-fake-vectors")
	}
	return opts, nil
}

func runMemoryEmbeddingBuildFake(opts memoryEmbeddingBuildFakeOptions, stdout io.Writer, stderr io.Writer) int {
	policyBytes, err := os.ReadFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding build-fake failed: %v\n", err)
		return 1
	}
	cfg, err := embeddingpolicy.Parse(policyBytes)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding build-fake failed: %v\n", err)
		return 1
	}
	result, err := embeddingpolicy.BuildFake(cfg, policyBytes, opts.artifactsDir, opts.confirmFake)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding build-fake failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory embedding build-fake: ok")
	fmt.Fprintf(stdout, "vectors: %s\n", result.Manifest.OutputVectorsPath)
	fmt.Fprintf(stdout, "manifest: %s\n", result.Manifest.OutputManifestPath)
	fmt.Fprintf(stdout, "vector_count: %d\n", result.Manifest.VectorCount)
	fmt.Fprintf(stdout, "dimensions: %d\n", result.Manifest.Dimensions)
	return 0
}

type memoryEmbeddingReportOptions struct {
	manifestPath string
	vectorsPath  string
	chunksPath   string
	outputFormat string
}

func parseMemoryEmbeddingReportOptions(args []string) (memoryEmbeddingReportOptions, error) {
	opts := memoryEmbeddingReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			if i+1 >= len(args) {
				return memoryEmbeddingReportOptions{}, fmt.Errorf("missing value for --manifest")
			}
			opts.manifestPath = args[i+1]
			i++
		case "--vectors":
			if i+1 >= len(args) {
				return memoryEmbeddingReportOptions{}, fmt.Errorf("missing value for --vectors")
			}
			opts.vectorsPath = args[i+1]
			i++
		case "--chunks":
			if i+1 >= len(args) {
				return memoryEmbeddingReportOptions{}, fmt.Errorf("missing value for --chunks")
			}
			opts.chunksPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryEmbeddingReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryEmbeddingReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.manifestPath == "" {
		return memoryEmbeddingReportOptions{}, fmt.Errorf("missing --manifest")
	}
	if opts.vectorsPath == "" {
		return memoryEmbeddingReportOptions{}, fmt.Errorf("missing --vectors")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryEmbeddingReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMemoryEmbeddingReport(opts memoryEmbeddingReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := embeddingpolicy.Report(opts.manifestPath, opts.vectorsPath, opts.chunksPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = embeddingpolicy.WriteVectorReportJSON(result, stdout)
	default:
		err = embeddingpolicy.WriteVectorReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory embedding report failed: %v\n", err)
		return 1
	}
	if result.Status == embeddingpolicy.StatusFailed {
		return 1
	}
	return 0
}

type memoryLanceDBPolicyOptions struct {
	policyPath string
}

type memoryLanceDBPlanOptions struct {
	policyPath   string
	outputFormat string
}

func parseMemoryLanceDBPolicyOptions(args []string) (memoryLanceDBPolicyOptions, error) {
	opts := memoryLanceDBPolicyOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBPolicyOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return memoryLanceDBPolicyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryLanceDBPolicyOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

func parseMemoryLanceDBPlanOptions(args []string) (memoryLanceDBPlanOptions, error) {
	opts := memoryLanceDBPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBPlanOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryLanceDBPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryLanceDBPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryLanceDBPlanOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryLanceDBPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMemoryLanceDBValidate(opts memoryLanceDBPolicyOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := lancedbpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb validate failed: %v\n", err)
		return 1
	}
	if err := lancedbpolicy.Validate(cfg); err != nil {
		fmt.Fprintf(stderr, "memory lancedb validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory lancedb validate: ok")
	return 0
}

func runMemoryLanceDBPlan(opts memoryLanceDBPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := lancedbpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb plan failed: %v\n", err)
		return 1
	}
	result, err := lancedbpolicy.Plan(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = lancedbpolicy.WritePlanJSON(result, stdout)
	default:
		err = lancedbpolicy.WritePlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb plan failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type memoryLanceDBFakeWriteOptions struct {
	policyPath       string
	artifactsDir     string
	confirmFakeWrite bool
}

func parseMemoryLanceDBFakeWriteOptions(args []string) (memoryLanceDBFakeWriteOptions, error) {
	var opts memoryLanceDBFakeWriteOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBFakeWriteOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return memoryLanceDBFakeWriteOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--confirm-fake-write":
			opts.confirmFakeWrite = true
		default:
			return memoryLanceDBFakeWriteOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryLanceDBFakeWriteOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.artifactsDir == "" {
		return memoryLanceDBFakeWriteOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if !opts.confirmFakeWrite {
		return memoryLanceDBFakeWriteOptions{}, fmt.Errorf("missing --confirm-fake-write")
	}
	return opts, nil
}

func runMemoryLanceDBFakeWrite(opts memoryLanceDBFakeWriteOptions, stdout io.Writer, stderr io.Writer) int {
	policyBytes, err := os.ReadFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb fake-write failed: %v\n", err)
		return 1
	}
	cfg, err := lancedbpolicy.Parse(policyBytes)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb fake-write failed: %v\n", err)
		return 1
	}
	result, err := lancedbpolicy.FakeWrite(cfg, policyBytes, opts.artifactsDir, opts.confirmFakeWrite)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb fake-write failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory lancedb fake-write: ok")
	fmt.Fprintf(stdout, "manifest: %s\n", filepath.Join(opts.artifactsDir, "lancedb-fake-write-manifest.json"))
	fmt.Fprintf(stdout, "rows: %s\n", result.Manifest.OutputRowsPath)
	fmt.Fprintf(stdout, "row_count: %d\n", result.Manifest.RowCount)
	fmt.Fprintf(stdout, "dimensions: %d\n", result.Manifest.Dimensions)
	fmt.Fprintf(stdout, "lancedb_written: %t\n", result.Manifest.LanceDBWritten)
	return 0
}

type memoryLanceDBWriteSmokeOptions struct {
	policyPath          string
	artifactsDir        string
	confirmLanceDBWrite bool
	allowOverwriteSmoke bool
}

func parseMemoryLanceDBWriteSmokeOptions(args []string) (memoryLanceDBWriteSmokeOptions, error) {
	var opts memoryLanceDBWriteSmokeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBWriteSmokeOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return memoryLanceDBWriteSmokeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--confirm-lancedb-write":
			opts.confirmLanceDBWrite = true
		case "--allow-overwrite-smoke":
			opts.allowOverwriteSmoke = true
		default:
			return memoryLanceDBWriteSmokeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryLanceDBWriteSmokeOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.artifactsDir == "" {
		return memoryLanceDBWriteSmokeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if !opts.confirmLanceDBWrite {
		return memoryLanceDBWriteSmokeOptions{}, fmt.Errorf("missing --confirm-lancedb-write")
	}
	return opts, nil
}

func runMemoryLanceDBWriteSmoke(opts memoryLanceDBWriteSmokeOptions, stdout io.Writer, stderr io.Writer) int {
	policyBytes, err := os.ReadFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb write-smoke failed: %v\n", err)
		return 1
	}
	cfg, err := lancedbpolicy.Parse(policyBytes)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb write-smoke failed: %v\n", err)
		return 1
	}
	result, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        opts.artifactsDir,
		ConfirmLanceDBWrite: opts.confirmLanceDBWrite,
		AllowOverwriteSmoke: opts.allowOverwriteSmoke,
	})
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb write-smoke failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory lancedb write-smoke: ok")
	fmt.Fprintf(stdout, "manifest: %s\n", filepath.Join(opts.artifactsDir, "lancedb-write-smoke-manifest.json"))
	fmt.Fprintf(stdout, "rows_summary: %s\n", filepath.Join(opts.artifactsDir, "lancedb-write-smoke-rows-summary.json"))
	fmt.Fprintf(stdout, "log: %s\n", result.LogPath)
	fmt.Fprintf(stdout, "database_path: %s\n", result.Manifest.DatabasePath)
	fmt.Fprintf(stdout, "table: %s\n", result.Manifest.Table)
	fmt.Fprintf(stdout, "row_count: %d\n", result.Manifest.RowCount)
	fmt.Fprintf(stdout, "dimensions: %d\n", result.Manifest.Dimensions)
	fmt.Fprintf(stdout, "lancedb_written: %t\n", result.Manifest.LanceDBWritten)
	fmt.Fprintf(stdout, "retrieval_performed: %t\n", result.Manifest.RetrievalPerformed)
	return 0
}

type memoryLanceDBDoctorOptions struct {
	policyPath   string
	outputFormat string
}

type memoryLanceDBReadbackReportOptions struct {
	manifestPath string
	policyPath   string
	outputFormat string
}

func parseMemoryLanceDBDoctorOptions(args []string) (memoryLanceDBDoctorOptions, error) {
	opts := memoryLanceDBDoctorOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBDoctorOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryLanceDBDoctorOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryLanceDBDoctorOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryLanceDBDoctorOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryLanceDBDoctorOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseMemoryLanceDBReadbackReportOptions(args []string) (memoryLanceDBReadbackReportOptions, error) {
	opts := memoryLanceDBReadbackReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			if i+1 >= len(args) {
				return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("missing value for --manifest")
			}
			opts.manifestPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.manifestPath == "" {
		return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("missing --manifest")
	}
	if opts.policyPath == "" {
		return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryLanceDBReadbackReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMemoryLanceDBDoctor(opts memoryLanceDBDoctorOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := lancedbpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb doctor failed: %v\n", err)
		return 1
	}
	result, err := lancedbpolicy.Doctor(cfg, lancedbpolicy.DoctorOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb doctor failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = lancedbpolicy.WriteDoctorJSON(result, stdout)
	default:
		err = lancedbpolicy.WriteDoctorText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb doctor failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type memoryLanceDBSearchSmokeOptions struct {
	policyPath         string
	artifactsDir       string
	queryVectorJSON    string
	queryChunkID       string
	topK               int
	confirmSearchSmoke bool
}

func parseMemoryLanceDBSearchSmokeOptions(args []string) (memoryLanceDBSearchSmokeOptions, error) {
	var opts memoryLanceDBSearchSmokeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--query-vector":
			if i+1 >= len(args) {
				return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing value for --query-vector")
			}
			opts.queryVectorJSON = args[i+1]
			i++
		case "--query-chunk-id":
			if i+1 >= len(args) {
				return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing value for --query-chunk-id")
			}
			opts.queryChunkID = args[i+1]
			i++
		case "--top-k":
			if i+1 >= len(args) {
				return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing value for --top-k")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil {
				return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("invalid --top-k %q", args[i+1])
			}
			opts.topK = value
			i++
		case "--confirm-search-smoke":
			opts.confirmSearchSmoke = true
		default:
			return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.artifactsDir == "" {
		return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if !opts.confirmSearchSmoke {
		return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing --confirm-search-smoke")
	}
	hasVector := strings.TrimSpace(opts.queryVectorJSON) != ""
	hasChunk := strings.TrimSpace(opts.queryChunkID) != ""
	if hasVector == hasChunk {
		return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("exactly one of --query-vector or --query-chunk-id is required")
	}
	if opts.topK <= 0 {
		return memoryLanceDBSearchSmokeOptions{}, fmt.Errorf("missing or invalid --top-k")
	}
	return opts, nil
}

func runMemoryLanceDBSearchSmoke(opts memoryLanceDBSearchSmokeOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := lancedbpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb search-smoke failed: %v\n", err)
		return 1
	}
	searchOpts := lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       opts.artifactsDir,
		ConfirmSearchSmoke: opts.confirmSearchSmoke,
		QueryChunkID:       opts.queryChunkID,
		TopK:               opts.topK,
	}
	if strings.TrimSpace(opts.queryVectorJSON) != "" {
		var queryVector []float64
		if err := json.Unmarshal([]byte(opts.queryVectorJSON), &queryVector); err != nil {
			fmt.Fprintf(stderr, "memory lancedb search-smoke failed: invalid --query-vector json: %v\n", err)
			return 1
		}
		searchOpts.QueryVector = queryVector
	}
	result, err := lancedbpolicy.SearchSmoke(cfg, searchOpts)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb search-smoke failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "memory lancedb search-smoke: ok")
	fmt.Fprintf(stdout, "result: %s\n", result.ResultPath)
	fmt.Fprintf(stdout, "summary: %s\n", result.SummaryPath)
	fmt.Fprintf(stdout, "log: %s\n", result.LogPath)
	fmt.Fprintf(stdout, "query_mode: %s\n", result.Summary.QueryMode)
	fmt.Fprintf(stdout, "top_k: %d\n", result.Summary.TopK)
	fmt.Fprintf(stdout, "result_count: %d\n", result.Summary.ResultCount)
	fmt.Fprintf(stdout, "retrieval_performed: %t\n", result.Summary.RetrievalPerformed)
	fmt.Fprintf(stdout, "runner_integration: %t\n", result.Summary.RunnerIntegration)
	return 0
}

type memoryLanceDBSearchReportOptions struct {
	resultPath   string
	policyPath   string
	outputFormat string
}

func parseMemoryLanceDBSearchReportOptions(args []string) (memoryLanceDBSearchReportOptions, error) {
	opts := memoryLanceDBSearchReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--result":
			if i+1 >= len(args) {
				return memoryLanceDBSearchReportOptions{}, fmt.Errorf("missing value for --result")
			}
			opts.resultPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryLanceDBSearchReportOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return memoryLanceDBSearchReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return memoryLanceDBSearchReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.resultPath == "" {
		return memoryLanceDBSearchReportOptions{}, fmt.Errorf("missing --result")
	}
	if opts.policyPath == "" {
		return memoryLanceDBSearchReportOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return memoryLanceDBSearchReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runMemoryLanceDBSearchReport(opts memoryLanceDBSearchReportOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := lancedbpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb search-report failed: %v\n", err)
		return 1
	}
	result, err := lancedbpolicy.SearchReport(opts.resultPath, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb search-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = lancedbpolicy.WriteSearchReportJSON(result, stdout)
	default:
		err = lancedbpolicy.WriteSearchReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb search-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runMemoryLanceDBReadbackReport(opts memoryLanceDBReadbackReportOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := lancedbpolicy.Load(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb report failed: %v\n", err)
		return 1
	}
	result, err := lancedbpolicy.ReadbackReport(opts.manifestPath, cfg, lancedbpolicy.ReadbackReportOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = lancedbpolicy.WriteReadbackReportJSON(result, stdout)
	default:
		err = lancedbpolicy.WriteReadbackReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory lancedb report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type memoryProposalNewOptions struct {
	runID      string
	taskID     string
	domain     string
	targetPath string
	operation  string
	reason     string
	outputPath string
}

func parseMemoryProposalNewOptions(args []string) (memoryProposalNewOptions, error) {
	var opts memoryProposalNewOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--task":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskID = args[i+1]
			i++
		case "--domain":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --domain")
			}
			opts.domain = args[i+1]
			i++
		case "--target":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --target")
			}
			opts.targetPath = args[i+1]
			i++
		case "--operation":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --operation")
			}
			opts.operation = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.runID == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --run")
	}
	if opts.taskID == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --task")
	}
	if opts.domain == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --domain")
	}
	if opts.targetPath == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --target")
	}
	if opts.operation == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --operation")
	}
	if opts.reason == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalNew(opts memoryProposalNewOptions, stdout io.Writer, stderr io.Writer) int {
	proposal := memory.NewProposal(memory.NewProposalOptions{
		RunID:      opts.runID,
		TaskID:     opts.taskID,
		Domain:     opts.domain,
		TargetPath: opts.targetPath,
		Operation:  memory.MemoryOperation(opts.operation),
		Reason:     opts.reason,
		Evidence: []memory.MemoryEvidence{
			{Type: "run", RunID: opts.runID},
		},
	})

	var data []byte
	var err error
	if strings.EqualFold(filepath.Ext(opts.outputPath), ".md") {
		data, err = proposal.Markdown()
	} else {
		data, err = proposal.JSON()
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal failed: %v\n", err)
		return 1
	}

	outputDir := filepath.Dir(opts.outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "memory proposal failed: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(opts.outputPath, data, 0o600); err != nil {
		fmt.Fprintf(stderr, "memory proposal failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "memory proposal written: %s\n", opts.outputPath)
	return 0
}

type memoryProposalLintOptions struct {
	proposalPath string
	policyPath   string
}

func parseMemoryProposalLintOptions(args []string) (memoryProposalLintOptions, error) {
	var opts memoryProposalLintOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalLintOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalLintOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return memoryProposalLintOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalLintOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return memoryProposalLintOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

func runMemoryProposalLint(opts memoryProposalLintOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal lint failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal lint failed: %v\n", err)
		return 1
	}

	result := memory.LintProposal(proposal, policy)
	printMemoryProposalLintResult(stdout, result)
	if result.Status == memory.LintStatusFailed {
		return 1
	}
	return 0
}

func printMemoryProposalLintResult(stdout io.Writer, result memory.LintResult) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "violations: %d\n", len(result.Violations))
	for _, violation := range result.Violations {
		fmt.Fprintf(stdout, "- %s\n", violation)
	}
	fmt.Fprintf(stdout, "warnings: %d\n", len(result.Warnings))
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
}

type memoryProposalApproveOptions struct {
	proposalPath string
	policyPath   string
	reviewer     string
	decision     string
	reason       string
	outputPath   string
}

func parseMemoryProposalApproveOptions(args []string) (memoryProposalApproveOptions, error) {
	var opts memoryProposalApproveOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--reviewer":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --reviewer")
			}
			opts.reviewer = args[i+1]
			i++
		case "--decision":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --decision")
			}
			opts.decision = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.reviewer == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --reviewer")
	}
	if opts.decision == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --decision")
	}
	if opts.reason == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalApprove(opts memoryProposalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}
	if outputConflictsWithApprovalTarget(opts.outputPath, proposal) {
		fmt.Fprintf(stderr, "memory proposal approve failed: refusing to write approval artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, policy) {
		fmt.Fprintf(stderr, "memory proposal approve failed: refusing to write approval artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	approval, err := memory.BuildApproval(proposal, policy, memory.NewApprovalOptions{
		Reviewer: opts.reviewer,
		Decision: memory.ApprovalDecision(opts.decision),
		Reason:   opts.reason,
	})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}
	if err := writeMemoryApproval(opts.outputPath, approval); err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "memory approval written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", approval.ProposalID)
	fmt.Fprintf(stdout, "decision: %s\n", approval.Decision)
	fmt.Fprintf(stdout, "lint_status: %s\n", approval.LintStatus)
	fmt.Fprintf(stdout, "apply_status: %s\n", approval.ApplyStatus)
	return 0
}

func outputConflictsWithApprovalTarget(outputPath string, proposal memory.MemoryProposal) bool {
	if sameCleanPath(outputPath, proposal.TargetPath) {
		return true
	}
	for _, patch := range proposal.Patches {
		targetPath := strings.TrimSpace(patch.TargetPath)
		if targetPath == "" {
			targetPath = proposal.TargetPath
		}
		if sameCleanPath(outputPath, targetPath) {
			return true
		}
	}
	return false
}

type memoryProposalApplyPreflightOptions struct {
	proposalPath string
	approvalPath string
	policyPath   string
	outputPath   string
}

func parseMemoryProposalApplyPreflightOptions(args []string) (memoryProposalApplyPreflightOptions, error) {
	var opts memoryProposalApplyPreflightOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalApplyPreflightOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.policyPath == "" {
		return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

func runMemoryProposalApplyPreflight(opts memoryProposalApplyPreflightOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
		return 1
	}
	approval, err := memory.LoadApprovalFromFile(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
		return 1
	}

	preflight := memory.BuildApplyPreflight(proposal, approval, policy, memory.NewApplyPreflightOptions{})
	printMemoryProposalApplyPreflight(stdout, preflight)
	if opts.outputPath != "" {
		if outputConflictsWithApprovalTarget(opts.outputPath, proposal) {
			fmt.Fprintf(stderr, "memory proposal apply-preflight failed: refusing to write preflight artifact to target_path %q\n", opts.outputPath)
			return 1
		}
		if outputTouchesMemoryDomain(opts.outputPath, policy) {
			fmt.Fprintf(stderr, "memory proposal apply-preflight failed: refusing to write preflight artifact inside memory domain %q\n", opts.outputPath)
			return 1
		}
		if err := writeApplyPreflight(opts.outputPath, preflight); err != nil {
			fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
			return 1
		}
	}
	if preflight.Status == memory.ApplyPreflightStatusFailed {
		return 1
	}
	return 0
}

func printMemoryProposalApplyPreflight(stdout io.Writer, preflight memory.ApplyPreflight) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", preflight.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", preflight.ApprovalID)
	fmt.Fprintf(stdout, "status: %s\n", preflight.Status)
	fmt.Fprintf(stdout, "failures: %d\n", len(preflight.Failures))
	for _, failure := range preflight.Failures {
		fmt.Fprintf(stdout, "- %s\n", failure)
	}
	fmt.Fprintf(stdout, "lint_status: %s\n", preflight.LintStatus)
	fmt.Fprintf(stdout, "apply_status: %s\n", preflight.ApplyStatus)
	fmt.Fprintf(stdout, "patch_count: %d\n", preflight.PatchCount)
	fmt.Fprintf(stdout, "patch violations: %d\n", len(preflight.PatchViolations))
	for _, violation := range preflight.PatchViolations {
		fmt.Fprintf(stdout, "- patch[%d] %s %s: %s\n", violation.PatchIndex, violation.Operation, violation.TargetPath, violation.Violation)
	}
}

func writeApplyPreflight(outputPath string, preflight memory.ApplyPreflight) error {
	data, err := preflight.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

type memoryProposalBackupPlanOptions struct {
	proposalPath string
	approvalPath string
	policyPath   string
	outputPath   string
}

func parseMemoryProposalBackupPlanOptions(args []string) (memoryProposalBackupPlanOptions, error) {
	var opts memoryProposalBackupPlanOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalBackupPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.policyPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalBackupPlan(opts memoryProposalBackupPlanOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	approval, err := memory.LoadApprovalFromFile(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	if outputConflictsWithApprovalTarget(opts.outputPath, proposal) {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: refusing to write backup plan artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, policy) {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: refusing to write backup plan artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	plan, err := memory.BuildBackupPlan(proposal, approval, policy, memory.NewBackupPlanOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	if err := writeBackupPlan(opts.outputPath, plan); err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "backup plan written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", plan.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", plan.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(plan.Items))
	return 0
}

func writeBackupPlan(outputPath string, plan memory.BackupPlan) error {
	data, err := plan.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

type memoryProposalBackupMaterializeOptions struct {
	backupPlanPath string
	outputPath     string
}

func parseMemoryProposalBackupMaterializeOptions(args []string) (memoryProposalBackupMaterializeOptions, error) {
	var opts memoryProposalBackupMaterializeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.backupPlanPath == "" {
		return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.outputPath == "" {
		return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalBackupMaterialize(opts memoryProposalBackupMaterializeOptions, stdout io.Writer, stderr io.Writer) int {
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: %v\n", err)
		return 1
	}
	if outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, nil) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	result, err := memory.MaterializeBackup(plan, memory.NewBackupMaterializeOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: %v\n", err)
		return 1
	}
	if err := writeBackupResult(opts.outputPath, result); err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "backup result written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", result.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(result.Items))
	return 0
}

type memoryProposalRestoreOptions struct {
	backupPlanPath   string
	backupResultPath string
	dryRun           bool
	outputPath       string
}

func parseMemoryProposalRestoreOptions(args []string) (memoryProposalRestoreOptions, error) {
	var opts memoryProposalRestoreOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalRestoreOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--backup-result":
			if i+1 >= len(args) {
				return memoryProposalRestoreOptions{}, fmt.Errorf("missing value for --backup-result")
			}
			opts.backupResultPath = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalRestoreOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalRestoreOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.backupPlanPath == "" {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.backupResultPath == "" {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --backup-result")
	}
	if !opts.dryRun {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --dry-run")
	}
	if opts.outputPath == "" {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalRestore(opts memoryProposalRestoreOptions, stdout io.Writer, stderr io.Writer) int {
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}
	result, err := memory.LoadBackupResultFromFile(opts.backupResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}
	if outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, nil) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	preview, err := memory.BuildRestorePreview(plan, result, memory.NewRestorePreviewOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}
	if err := writeRestorePreview(opts.outputPath, preview); err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "restore preview written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", preview.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", preview.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(preview.Items))
	return 0
}

type memoryProposalRestoreExecuteOptions struct {
	backupPlanPath     string
	backupResultPath   string
	restorePreviewPath string
	outputPath         string
	confirmRestore     bool
}

func parseMemoryProposalRestoreExecuteOptions(args []string) (memoryProposalRestoreExecuteOptions, error) {
	var opts memoryProposalRestoreExecuteOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--backup-result":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --backup-result")
			}
			opts.backupResultPath = args[i+1]
			i++
		case "--restore-preview":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --restore-preview")
			}
			opts.restorePreviewPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-restore":
			opts.confirmRestore = true
		default:
			return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.backupPlanPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.backupResultPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --backup-result")
	}
	if opts.restorePreviewPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --restore-preview")
	}
	if opts.outputPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmRestore {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --confirm-restore")
	}
	return opts, nil
}

func runMemoryProposalRestoreExecute(opts memoryProposalRestoreExecuteOptions, stdout io.Writer, stderr io.Writer) int {
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}
	backupResult, err := memory.LoadBackupResultFromFile(opts.backupResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}
	restorePreview, err := memory.LoadRestorePreviewFromFile(opts.restorePreviewPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}

	if outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, nil) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	result, err := memory.ExecuteRestore(plan, backupResult, restorePreview, memory.NewRestoreExecuteOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}
	if err := writeRestoreResult(opts.outputPath, result); err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "restore result written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", result.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(result.Items))
	return 0
}

type memoryProposalApplyExecuteOptions struct {
	proposalPath     string
	approvalPath     string
	policyPath       string
	backupPlanPath   string
	backupResultPath string
	outputPath       string
	confirmApply     bool
}

func parseMemoryProposalApplyExecuteOptions(args []string) (memoryProposalApplyExecuteOptions, error) {
	var opts memoryProposalApplyExecuteOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--backup-result":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --backup-result")
			}
			opts.backupResultPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-apply":
			opts.confirmApply = true
		default:
			return memoryProposalApplyExecuteOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.policyPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.backupPlanPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.backupResultPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --backup-result")
	}
	if opts.outputPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmApply {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --confirm-apply")
	}
	return opts, nil
}

func runMemoryProposalApplyExecute(opts memoryProposalApplyExecuteOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	approval, err := memory.LoadApprovalFromFile(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	backupResult, err := memory.LoadBackupResultFromFile(opts.backupResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}

	if outputConflictsWithApprovalTarget(opts.outputPath, proposal) || outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, policy) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	result, err := memoryExecuteApply(proposal, approval, policy, plan, backupResult, memory.NewApplyExecuteOptions{})
	if err != nil {
		var applyErr *memory.ApplyExecutionError
		if errors.As(err, &applyErr) && applyErr.Result.Status == memory.ApplyResultStatusPartialFailed {
			if writeErr := writeApplyResult(opts.outputPath, applyErr.Result); writeErr != nil {
				fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", writeErr)
				return 1
			}
		}
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	if err := writeApplyResult(opts.outputPath, result); err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "apply result written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", result.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(result.Items))
	return 0
}

func outputConflictsWithBackupPlanTarget(outputPath string, plan memory.BackupPlan) bool {
	for _, item := range plan.Items {
		if sameCleanPath(outputPath, item.TargetPath) {
			return true
		}
	}
	return false
}

func outputConflictsWithBackupPlanBackupPath(outputPath string, plan memory.BackupPlan) bool {
	for _, item := range plan.Items {
		if sameCleanPath(outputPath, item.BackupPath) {
			return true
		}
	}
	return false
}

func outputTouchesBackupRoot(outputPath string, plan memory.BackupPlan) bool {
	if strings.TrimSpace(plan.BackupRoot) == "" {
		return false
	}
	return cleanPathWithinRoot(outputPath, plan.BackupRoot)
}

func writeBackupResult(outputPath string, result memory.BackupResult) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeRestorePreview(outputPath string, preview memory.RestorePreview) error {
	data, err := preview.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeRestoreResult(outputPath string, result memory.RestoreResult) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeApplyResult(outputPath string, result memory.ApplyResult) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

type memoryProposalApplyOptions struct {
	proposalPath string
	policyPath   string
	dryRun       bool
	outputPath   string
}

func parseMemoryProposalApplyOptions(args []string) (memoryProposalApplyOptions, error) {
	var opts memoryProposalApplyOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApplyOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApplyOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApplyOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalApplyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApplyOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return memoryProposalApplyOptions{}, fmt.Errorf("missing --policy")
	}
	if !opts.dryRun {
		return memoryProposalApplyOptions{}, fmt.Errorf("missing --dry-run")
	}
	return opts, nil
}

func runMemoryProposalApply(opts memoryProposalApplyOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply failed: %v\n", err)
		return 1
	}

	preview, previewErr := memory.BuildApplyDryRunPreview(proposal, policy)
	printMemoryProposalApplyPreview(stdout, preview)
	if opts.outputPath != "" {
		if outputConflictsWithApplyTarget(opts.outputPath, preview) {
			fmt.Fprintf(stderr, "memory proposal apply failed: refusing to write apply preview to target_path %q\n", opts.outputPath)
			return 1
		}
		if err := writeApplyPreview(opts.outputPath, preview); err != nil {
			fmt.Fprintf(stderr, "memory proposal apply failed: %v\n", err)
			return 1
		}
	}
	if previewErr != nil {
		return 1
	}
	return 0
}

func outputConflictsWithApplyTarget(outputPath string, preview memory.ApplyPreview) bool {
	if sameCleanPath(outputPath, preview.TargetPath) {
		return true
	}
	for _, action := range preview.Actions {
		if sameCleanPath(outputPath, action.TargetPath) {
			return true
		}
	}
	return false
}

func outputTouchesMemoryDomain(outputPath string, policy *memory.MemoryPolicy) bool {
	roots := []string{
		"/vault/mysecondbrain",
		"vault/mysecondbrain",
		"mysecondbrain",
		"/domains/escalasoft_brain",
		"domains/escalasoft_brain",
		"escalasoft_brain",
	}
	if policy != nil {
		for _, domainPolicy := range policy.IsolatedDomains {
			if strings.TrimSpace(domainPolicy.Root) == "" {
				continue
			}
			roots = append(roots, domainPolicy.Root)
			roots = append(roots, strings.TrimPrefix(domainPolicy.Root, "/"))
			roots = append(roots, filepath.Base(domainPolicy.Root))
		}
	}
	for _, root := range roots {
		if cleanPathWithinRoot(outputPath, root) {
			return true
		}
	}
	return false
}

func cleanPathWithinRoot(value string, root string) bool {
	value = cleanSlashPath(value)
	root = cleanSlashPath(root)
	if value == "" || root == "" {
		return false
	}
	value = strings.Trim(value, "/")
	root = strings.Trim(root, "/")
	if value == root || strings.HasPrefix(value, root+"/") {
		return true
	}
	return strings.Contains(value, "/"+root+"/")
}

func cleanSlashPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." {
		return ""
	}
	return value
}

func sameCleanPath(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return filepath.ToSlash(filepath.Clean(left)) == filepath.ToSlash(filepath.Clean(right))
}

func printMemoryProposalApplyPreview(stdout io.Writer, preview memory.ApplyPreview) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", preview.ProposalID)
	fmt.Fprintf(stdout, "domain: %s\n", preview.Domain)
	fmt.Fprintf(stdout, "target_path: %s\n", preview.TargetPath)
	fmt.Fprintf(stdout, "operation: %s\n", preview.Operation)
	fmt.Fprintf(stdout, "status: %s\n", preview.Status)

	fmt.Fprintf(stdout, "lint warnings: %d\n", len(preview.LintWarnings))
	for _, warning := range preview.LintWarnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
	fmt.Fprintf(stdout, "lint violations: %d\n", len(preview.LintViolations))
	for _, violation := range preview.LintViolations {
		fmt.Fprintf(stdout, "- %s\n", violation)
	}

	fmt.Fprintf(stdout, "patch_count: %d\n", preview.PatchCount)
	fmt.Fprintf(stdout, "patch warnings: %d\n", len(preview.PatchWarnings))
	for _, warning := range preview.PatchWarnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
	fmt.Fprintf(stdout, "patch violations: %d\n", len(preview.PatchViolations))
	for _, violation := range preview.PatchViolations {
		fmt.Fprintf(stdout, "- patch[%d] %s %s: %s\n", violation.PatchIndex, violation.Operation, violation.TargetPath, violation.Violation)
	}
	if len(preview.Actions) == 0 {
		return
	}

	fmt.Fprintln(stdout, "actions:")
	for _, action := range preview.Actions {
		fmt.Fprintf(stdout, "- %s: %s\n", action.Description, action.TargetPath)
		if action.Warning != "" {
			fmt.Fprintf(stdout, "  warning: %s\n", action.Warning)
		}
		for _, violation := range action.Violations {
			fmt.Fprintf(stdout, "  violation: %s\n", violation)
		}
		if action.Content != "" {
			fmt.Fprintln(stdout, "  content:")
			writeIndentedText(stdout, action.Content, "    ")
		}
	}
}

func writeApplyPreview(outputPath string, preview memory.ApplyPreview) error {
	data, err := preview.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeMemoryApproval(outputPath string, approval memory.MemoryApproval) error {
	data, err := approval.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return err
	}
	return nil
}

func writeIndentedText(output io.Writer, value string, prefix string) {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		fmt.Fprintf(output, "%s%s\n", prefix, line)
	}
}

func runTaskValidate(path string, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	materialized, err := retrievalcontext.ValidateTaskMaterializedInjection(task.RetrievalContext.MaterializedInjection)
	if err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "task %s: valid\n", task.ID)
	if materialized.Declared {
		fmt.Fprintf(stdout, "materialized_injection_declared: %t\n", materialized.Declared)
		fmt.Fprintf(stdout, "materialized_injection_enabled: %t\n", materialized.Enabled)
		fmt.Fprintf(stdout, "materialized_injection_supported_now: %t\n", materialized.SupportedNow)
		if materialized.GovernanceBundleSHA256 != "" {
			fmt.Fprintf(stdout, "governance_bundle_sha256: %s\n", materialized.GovernanceBundleSHA256)
		}
	}
	return 0
}

type taskMaterializedInjectionReportOptions struct {
	taskPath     string
	outputFormat string
}

func parseTaskMaterializedInjectionReportOptions(args []string) (taskMaterializedInjectionReportOptions, error) {
	opts := taskMaterializedInjectionReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return taskMaterializedInjectionReportOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return taskMaterializedInjectionReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return taskMaterializedInjectionReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return taskMaterializedInjectionReportOptions{}, fmt.Errorf("missing --task")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return taskMaterializedInjectionReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runTaskMaterializedInjectionReport(opts taskMaterializedInjectionReportOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(opts.taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "task materialized-injection-report failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		fmt.Fprintf(stderr, "task materialized-injection-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionTaskReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionTaskReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "task materialized-injection-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type domainsOptions struct {
	configPath string
}

func parseDomainsOptions(args []string) (domainsOptions, error) {
	var opts domainsOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return domainsOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return domainsOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return domainsOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func loadAndValidateDomains(path string) (*domains.DomainsConfig, error) {
	config, err := domains.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := domains.Validate(config); err != nil {
		return nil, err
	}
	return config, nil
}

func runDomainsValidate(opts domainsOptions, stdout io.Writer, stderr io.Writer) int {
	config, err := loadAndValidateDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "domains validation failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "domains config valid: %d domains\n", len(config.Domains))
	return 0
}

func runDomainsList(opts domainsOptions, stdout io.Writer, stderr io.Writer) int {
	config, err := loadAndValidateDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "domains validation failed: %v\n", err)
		return 1
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "name\ttype\troot\tdefault\tisolated\tdefault_agent")
	for _, domain := range config.List() {
		fmt.Fprintf(table, "%s\t%s\t%s\t%t\t%t\t%s\n",
			domain.Name,
			domain.Type,
			domain.Root,
			domain.Default,
			domain.IsIsolated(),
			domain.DefaultAgent,
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "domains list failed: %v\n", err)
		return 1
	}
	return 0
}

type doctorOptions struct {
	outputFormat      string
	worker            string
	includeProfiles   bool
	workersConfigPath string
	storePath         string
	artifactsDir      string
}

func parseDoctorOptions(args []string) (doctorOptions, error) {
	opts := doctorOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output-format":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		case "--worker":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --worker")
			}
			opts.worker = args[i+1]
			i++
		case "--profiles":
			opts.includeProfiles = true
		case "--workers-config":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		default:
			return doctorOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return doctorOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runDoctor(opts doctorOptions, stdout io.Writer, stderr io.Writer) int {
	report, err := doctorpkg.Build(doctorpkg.Options{
		OutputFormat:      doctorpkg.OutputFormat(opts.outputFormat),
		Worker:            opts.worker,
		IncludeProfiles:   opts.includeProfiles,
		WorkersConfigPath: opts.workersConfigPath,
		StorePath:         opts.storePath,
		ArtifactsDir:      opts.artifactsDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "doctor failed: %v\n", err)
		return 1
	}
	if err := doctorpkg.Write(report, doctorpkg.OutputFormat(opts.outputFormat), stdout); err != nil {
		fmt.Fprintf(stderr, "doctor output failed: %v\n", err)
		return 1
	}
	return 0
}

type workersSmokeOptions struct {
	worker            string
	taskPath          string
	storePath         string
	artifactsDir      string
	domainsPath       string
	memoryPolicyPath  string
	workersConfigPath string
	dryRun            bool
}

type workersSmokeSummary struct {
	worker        string
	envRequiredOK bool
	command       string
	runID         string
	artifactsDir  string
	status        string
}

func parseWorkersSmokeOptions(args []string) (workersSmokeOptions, error) {
	var opts workersSmokeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--worker":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --worker")
			}
			opts.worker = args[i+1]
			i++
		case "--task":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--memory-policy":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --memory-policy")
			}
			opts.memoryPolicyPath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return workersSmokeOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		default:
			return workersSmokeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.worker == "" {
		return workersSmokeOptions{}, fmt.Errorf("missing --worker")
	}
	if opts.worker != "opencode" {
		return workersSmokeOptions{}, fmt.Errorf("workers smoke currently supports only opencode")
	}
	if opts.taskPath == "" {
		return workersSmokeOptions{}, fmt.Errorf("missing --task")
	}
	if opts.storePath == "" {
		return workersSmokeOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return workersSmokeOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.workersConfigPath == "" {
		return workersSmokeOptions{}, fmt.Errorf("missing --workers-config")
	}
	return opts, nil
}

func runWorkersSmoke(opts workersSmokeOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := loadValidatedWorkerTask(opts.taskPath, opts.worker)
	if err != nil {
		writeTaskLoadError(stderr, err)
		return 1
	}

	report, err := doctorpkg.Build(doctorpkg.Options{
		Worker:            opts.worker,
		IncludeProfiles:   task.ModelProfile != "" || task.ModelStrategy != nil,
		WorkersConfigPath: opts.workersConfigPath,
		StorePath:         opts.storePath,
		ArtifactsDir:      opts.artifactsDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "workers doctor failed: %v\n", err)
		return 1
	}
	workerCheck, ok := findWorkerCheck(report, opts.worker)
	if !ok {
		fmt.Fprintf(stderr, "workers doctor failed: worker %s not found\n", opts.worker)
		return 1
	}

	if opts.dryRun {
		return runWorkersSmokeOpenCodeDryRun(opts, task, workerCheck, stdout, stderr)
	}

	workerConfig, envRequirements, err := configuredWorkerDefinitionForTask(opts.workersConfigPath, task, opts.worker)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	missingEnv := workerconfig.MissingRequiredEnv(envRequirements)
	summary := workersSmokeSummary{
		worker:        opts.worker,
		envRequiredOK: len(missingEnv) == 0,
		command:       workerConfig.Command,
	}
	if summary.command == "" {
		summary.command = workerCheck.ConfiguredCommand
	}
	if len(missingEnv) > 0 {
		if opts.dryRun {
			fmt.Fprintf(stderr, "warning: worker %s has missing required env\n", opts.worker)
		} else {
			fmt.Fprintf(stderr, "error: worker %s has missing required env\n", opts.worker)
			summary.status = "env_missing"
			if err := writeWorkersSmokeSummary(stdout, summary); err != nil {
				fmt.Fprintf(stderr, "smoke summary failed: %v\n", err)
			}
			return 1
		}
	}

	return runWorkersSmokeOpenCodeRun(opts, task, summary, envRequirements, stdout, stderr)
}

func runWorkersSmokeOpenCodeDryRun(opts workersSmokeOptions, task *tasks.Task, workerCheck doctorpkg.WorkerCheck, stdout io.Writer, stderr io.Writer) int {
	worker, envRequirements, strategyPlan, err := configuredOpenCodeDryRunWorkerForTask(opts.workersConfigPath, task)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	missingEnv := workerconfig.MissingRequiredEnv(envRequirements)
	summary := workersSmokeSummary{
		worker:        opts.worker,
		envRequiredOK: len(missingEnv) == 0,
		command:       workerCheck.ConfiguredCommand,
	}
	if len(missingEnv) > 0 {
		fmt.Fprintf(stderr, "warning: worker %s has missing required env\n", opts.worker)
	}

	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dry-run failed: %v\n", err)
		return 1
	}

	command := strings.Join(event.Command, " ")
	fmt.Fprintf(stdout, "workspace: %s\n", event.Workspace)
	fmt.Fprintf(stdout, "policy: %s\n", event.Sandbox)
	if strategyPlan != nil {
		fmt.Fprintln(stdout, "model_strategy: planned")
		fmt.Fprintf(stdout, "planned_model_profile: %s\n", strategyPlan.Name)
		fmt.Fprintf(stdout, "provider: %s\n", strategyPlan.Provider)
		fmt.Fprintf(stdout, "model: %s\n", strategyPlan.Model)
		fmt.Fprintf(stdout, "model_arg: %s\n", strategyPlan.ModelArg)
	}
	fmt.Fprintf(stdout, "command: %s\n", command)
	summary.command = command
	summary.status = "dry_run"
	if err := writeWorkersSmokeSummary(stdout, summary); err != nil {
		fmt.Fprintf(stderr, "smoke summary failed: %v\n", err)
		return 1
	}
	return 0
}

func runWorkersSmokeOpenCodeRun(opts workersSmokeOptions, task *tasks.Task, summary workersSmokeSummary, envRequirements []workerconfig.EnvRequirementCheck, stdout io.Writer, stderr io.Writer) int {
	worker, err := configuredOpenCodeWorkerForTask(opts.workersConfigPath, task)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	openCodeRunner := runner.OpenCodeRunner{
		WorkerFactory: func() workers.Worker {
			return worker
		},
		RunIDFactory:            runIDFactory,
		GitDiffRunner:           gitDiffRunner,
		GitSnapshotRunner:       gitSnapshotRunner,
		WorkspaceManagerFactory: workspaceManagerFactory,
	}

	var runOutput bytes.Buffer
	code := openCodeRunner.Run(context.Background(), runner.OpenCodeRunOptions{
		TaskPath:         opts.taskPath,
		StorePath:        opts.storePath,
		ArtifactsDir:     opts.artifactsDir,
		DomainsPath:      opts.domainsPath,
		MemoryPolicyPath: opts.memoryPolicyPath,
		EnvRequirements:  envRequirements,
	}, &runOutput, stderr)
	if _, err := stdout.Write(runOutput.Bytes()); err != nil {
		fmt.Fprintf(stderr, "write run output failed: %v\n", err)
		return 1
	}

	output := runOutput.String()
	summary.runID = outputLineValue(output, "run_id")
	if command := outputLineValue(output, "command"); command != "" {
		summary.command = command
	}
	summary.artifactsDir = outputLineValue(output, "artifacts_dir")
	if code == 0 {
		summary.status = "succeeded"
	} else {
		summary.status = "failed"
	}
	if err := writeWorkersSmokeSummary(stdout, summary); err != nil {
		fmt.Fprintf(stderr, "smoke summary failed: %v\n", err)
		return 1
	}
	return code
}

func findWorkerCheck(report doctorpkg.Report, worker string) (doctorpkg.WorkerCheck, bool) {
	for _, check := range report.Workers {
		if check.Name == worker {
			return check, true
		}
	}
	return doctorpkg.WorkerCheck{}, false
}

func outputLineValue(output string, key string) string {
	prefix := key + ": "
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func writeWorkersSmokeSummary(out io.Writer, summary workersSmokeSummary) error {
	if _, err := fmt.Fprintln(out, "smoke_summary:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "worker: %s\n", summary.worker); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "env_required_ok: %t\n", summary.envRequiredOK); err != nil {
		return err
	}
	if summary.command != "" {
		if _, err := fmt.Fprintf(out, "command: %s\n", summary.command); err != nil {
			return err
		}
	}
	if summary.runID != "" {
		if _, err := fmt.Fprintf(out, "run_id: %s\n", summary.runID); err != nil {
			return err
		}
	}
	if summary.artifactsDir != "" {
		if _, err := fmt.Fprintf(out, "artifacts_dir: %s\n", summary.artifactsDir); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "status: %s\n", summary.status)
	return err
}

type configEnvOptions struct {
	outputFormat string
}

func parseConfigEnvOptions(args []string) (configEnvOptions, error) {
	opts := configEnvOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output-format":
			if i+1 >= len(args) {
				return configEnvOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return configEnvOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return configEnvOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runConfigEnv(opts configEnvOptions, stdout io.Writer, stderr io.Writer) int {
	report := configenvpkg.Build()
	if err := configenvpkg.Write(report, configenvpkg.OutputFormat(opts.outputFormat), stdout); err != nil {
		fmt.Fprintf(stderr, "config env output failed: %v\n", err)
		return 1
	}
	return 0
}

type workerDryRunOptions struct {
	taskPath          string
	workersConfigPath string
}

func parseWorkerDryRunOptions(args []string) (workerDryRunOptions, error) {
	if len(args) < 1 {
		return workerDryRunOptions{}, fmt.Errorf("missing task path")
	}
	opts := workerDryRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--workers-config":
			if i+1 >= len(args) {
				return workerDryRunOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		default:
			return workerDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return opts, nil
}

func runCodexDryRun(opts workerDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := loadValidatedWorkerTask(opts.taskPath, "codex")
	if err != nil {
		writeTaskLoadError(stderr, err)
		return 1
	}

	_, envRequirements, err := configuredWorkerDefinitionForTask(opts.workersConfigPath, task, "codex")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	writeMissingEnvWarnings(stderr, "codex", envRequirements)

	worker, err := configuredCodexWorker(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dry-run failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "workspace: %s\n", event.Workspace)
	fmt.Fprintf(stdout, "command: %s\n", strings.Join(event.Command, " "))
	return 0
}

type codexMaterializedInjectionPreflightOptions struct {
	taskPath                string
	promptPreviewReportPath string
	outputPath              string
	outputFormat            string
}

func parseCodexMaterializedInjectionPreflightOptions(args []string) (codexMaterializedInjectionPreflightOptions, error) {
	opts := codexMaterializedInjectionPreflightOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--prompt-preview-report":
			if i+1 >= len(args) {
				return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing value for --prompt-preview-report")
			}
			opts.promptPreviewReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing --task")
	}
	if opts.promptPreviewReportPath == "" {
		return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing --prompt-preview-report")
	}
	if opts.outputPath == "" {
		return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("missing --output")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionPreflightOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionPreflight(opts codexMaterializedInjectionPreflightOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                opts.taskPath,
		PromptPreviewReportPath: opts.promptPreviewReportPath,
		OutputPath:              opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-preflight failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionPreflightJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionPreflightText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-preflight failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionDryRunOptions struct {
	taskPath                         string
	preflightPath                    string
	promptPreviewPath                string
	outputPath                       string
	promptOutputPath                 string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedInjectionDryRunOptions(args []string) (codexMaterializedInjectionDryRunOptions, error) {
	opts := codexMaterializedInjectionDryRunOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--preflight":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing value for --preflight")
			}
			opts.preflightPath = args[i+1]
			i++
		case "--prompt-preview":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing value for --prompt-preview")
			}
			opts.promptPreviewPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--prompt-output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing value for --prompt-output")
			}
			opts.promptOutputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing --task")
	}
	if opts.preflightPath == "" {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing --preflight")
	}
	if opts.promptPreviewPath == "" {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing --prompt-preview")
	}
	if opts.outputPath == "" {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing --output")
	}
	if opts.promptOutputPath == "" {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing --prompt-output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionDryRunOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionDryRun(opts codexMaterializedInjectionDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionDryRun(retrievalcontext.MaterializedInjectionDryRunOptions{
		TaskPath:                         opts.taskPath,
		PreflightPath:                    opts.preflightPath,
		PromptPreviewPath:                opts.promptPreviewPath,
		OutputPath:                       opts.outputPath,
		PromptOutputPath:                 opts.promptOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-dry-run failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionDryRunJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionDryRunText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-dry-run failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
		fmt.Fprintf(stdout, "prompt-output: %s\n", opts.promptOutputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionDryRunReportOptions struct {
	dryRunPath       string
	promptOutputPath string
	outputFormat     string
}

func parseCodexMaterializedInjectionDryRunReportOptions(args []string) (codexMaterializedInjectionDryRunReportOptions, error) {
	opts := codexMaterializedInjectionDryRunReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("missing value for --dry-run")
			}
			opts.dryRunPath = args[i+1]
			i++
		case "--prompt-output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("missing value for --prompt-output")
			}
			opts.promptOutputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.dryRunPath == "" {
		return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("missing --dry-run")
	}
	if opts.promptOutputPath == "" {
		return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("missing --prompt-output")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionDryRunReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionDryRunReport(opts codexMaterializedInjectionDryRunReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       opts.dryRunPath,
		PromptOutputPath: opts.promptOutputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-dry-run-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionDryRunReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionDryRunReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-dry-run-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionReadinessReportOptions struct {
	preflightPath    string
	dryRunPath       string
	dryRunReportPath string
	outputFormat     string
}

func parseCodexMaterializedInjectionReadinessReportOptions(args []string) (codexMaterializedInjectionReadinessReportOptions, error) {
	opts := codexMaterializedInjectionReadinessReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--preflight":
			if i+1 >= len(args) {
				return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing value for --preflight")
			}
			opts.preflightPath = args[i+1]
			i++
		case "--dry-run":
			if i+1 >= len(args) {
				return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing value for --dry-run")
			}
			opts.dryRunPath = args[i+1]
			i++
		case "--dry-run-report":
			if i+1 >= len(args) {
				return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing value for --dry-run-report")
			}
			opts.dryRunReportPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.preflightPath == "" {
		return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing --preflight")
	}
	if opts.dryRunPath == "" {
		return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing --dry-run")
	}
	if opts.dryRunReportPath == "" {
		return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("missing --dry-run-report")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionReadinessReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionReadinessReport(opts codexMaterializedInjectionReadinessReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    opts.preflightPath,
		DryRunPath:       opts.dryRunPath,
		DryRunReportPath: opts.dryRunReportPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-readiness-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionReadinessReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionReadinessReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-readiness-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionExecutionGateOptions struct {
	taskPath                         string
	readinessReportPath              string
	promptOutputPath                 string
	outputPath                       string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedInjectionExecutionGateOptions(args []string) (codexMaterializedInjectionExecutionGateOptions, error) {
	opts := codexMaterializedInjectionExecutionGateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--prompt-output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing value for --prompt-output")
			}
			opts.promptOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing --task")
	}
	if opts.readinessReportPath == "" {
		return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing --readiness-report")
	}
	if opts.promptOutputPath == "" {
		return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing --prompt-output")
	}
	if opts.outputPath == "" {
		return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionExecutionGateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionExecutionGate(opts codexMaterializedInjectionExecutionGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         opts.taskPath,
		ReadinessReportPath:              opts.readinessReportPath,
		PromptOutputPath:                 opts.promptOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
		OutputPath:                       opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-execution-gate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionExecutionGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionExecutionGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-execution-gate failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedPromptAssemblyDryRunOptions struct {
	executionGatePath                string
	basePromptFixturePath            string
	promptOutputPath                 string
	outputPath                       string
	assembledOutputPath              string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedPromptAssemblyDryRunOptions(args []string) (codexMaterializedPromptAssemblyDryRunOptions, error) {
	opts := codexMaterializedPromptAssemblyDryRunOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--execution-gate":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing value for --execution-gate")
			}
			opts.executionGatePath = args[i+1]
			i++
		case "--base-prompt-fixture":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing value for --base-prompt-fixture")
			}
			opts.basePromptFixturePath = args[i+1]
			i++
		case "--prompt-output":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing value for --prompt-output")
			}
			opts.promptOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--assembled-output":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing value for --assembled-output")
			}
			opts.assembledOutputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.executionGatePath == "" {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing --execution-gate")
	}
	if opts.basePromptFixturePath == "" {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing --base-prompt-fixture")
	}
	if opts.promptOutputPath == "" {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing --prompt-output")
	}
	if opts.outputPath == "" {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing --output")
	}
	if opts.assembledOutputPath == "" {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing --assembled-output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedPromptAssemblyDryRunOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedPromptAssemblyDryRun(opts codexMaterializedPromptAssemblyDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                opts.executionGatePath,
		BasePromptFixturePath:            opts.basePromptFixturePath,
		PromptOutputPath:                 opts.promptOutputPath,
		OutputPath:                       opts.outputPath,
		AssembledOutputPath:              opts.assembledOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-prompt-assembly-dry-run failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedPromptAssemblyJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedPromptAssemblyText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-prompt-assembly-dry-run failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
		fmt.Fprintf(stdout, "assembled-output: %s\n", opts.assembledOutputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedPromptAssemblyReportOptions struct {
	assemblyDryRunPath  string
	assembledOutputPath string
	outputFormat        string
}

func parseCodexMaterializedPromptAssemblyReportOptions(args []string) (codexMaterializedPromptAssemblyReportOptions, error) {
	opts := codexMaterializedPromptAssemblyReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--assembly-dry-run":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("missing value for --assembly-dry-run")
			}
			opts.assemblyDryRunPath = args[i+1]
			i++
		case "--assembled-output":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("missing value for --assembled-output")
			}
			opts.assembledOutputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.assemblyDryRunPath == "" {
		return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("missing --assembly-dry-run")
	}
	if opts.assembledOutputPath == "" {
		return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("missing --assembled-output")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedPromptAssemblyReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedPromptAssemblyReport(opts codexMaterializedPromptAssemblyReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  opts.assemblyDryRunPath,
		AssembledOutputPath: opts.assembledOutputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-prompt-assembly-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedPromptAssemblyReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedPromptAssemblyReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-prompt-assembly-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionRuntimeValidateOptions struct {
	configPath   string
	outputFormat string
}

func parseCodexMaterializedInjectionRuntimeValidateOptions(args []string) (codexMaterializedInjectionRuntimeValidateOptions, error) {
	opts := codexMaterializedInjectionRuntimeValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRuntimeValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRuntimeValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionRuntimeValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexMaterializedInjectionRuntimeValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionRuntimeValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionRuntimeValidate(opts codexMaterializedInjectionRuntimeValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-runtime validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-runtime validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionRuntimeValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionRuntimeValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-runtime validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionRunPlanOptions struct {
	taskPath                         string
	runtimeConfigPath                string
	executionGatePath                string
	assemblyReportPath               string
	assembledOutputPath              string
	outputPath                       string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedInjectionRunPlanOptions(args []string) (codexMaterializedInjectionRunPlanOptions, error) {
	opts := codexMaterializedInjectionRunPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--execution-gate":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --execution-gate")
			}
			opts.executionGatePath = args[i+1]
			i++
		case "--assembly-report":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --assembly-report")
			}
			opts.assemblyReportPath = args[i+1]
			i++
		case "--assembled-output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --assembled-output")
			}
			opts.assembledOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --task")
	}
	if opts.runtimeConfigPath == "" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --runtime-config")
	}
	if opts.executionGatePath == "" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --execution-gate")
	}
	if opts.assemblyReportPath == "" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --assembly-report")
	}
	if opts.assembledOutputPath == "" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --assembled-output")
	}
	if opts.outputPath == "" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionRunPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionRunPlan(opts codexMaterializedInjectionRunPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         opts.taskPath,
		RuntimeConfigPath:                opts.runtimeConfigPath,
		ExecutionGatePath:                opts.executionGatePath,
		AssemblyReportPath:               opts.assemblyReportPath,
		AssembledOutputPath:              opts.assembledOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
		OutputPath:                       opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-run-plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionRunPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionRunPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-run-plan failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionExecutionEnableValidateOptions struct {
	configPath   string
	outputFormat string
}

func parseCodexMaterializedInjectionExecutionEnableValidateOptions(args []string) (codexMaterializedInjectionExecutionEnableValidateOptions, error) {
	opts := codexMaterializedInjectionExecutionEnableValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionEnableValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionExecutionEnableValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionExecutionEnableValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexMaterializedInjectionExecutionEnableValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionExecutionEnableValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionExecutionEnableValidate(opts codexMaterializedInjectionExecutionEnableValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-execution-enable validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-execution-enable validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionExecutionEnableValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionExecutionEnableValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-execution-enable validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedInjectionProviderRunPlanOptions struct {
	taskPath                         string
	enableConfigPath                 string
	executionGatePath                string
	assemblyReportPath               string
	assembledOutputPath              string
	outputPath                       string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedInjectionProviderRunPlanOptions(args []string) (codexMaterializedInjectionProviderRunPlanOptions, error) {
	opts := codexMaterializedInjectionProviderRunPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--enable-config":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --enable-config")
			}
			opts.enableConfigPath = args[i+1]
			i++
		case "--execution-gate":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --execution-gate")
			}
			opts.executionGatePath = args[i+1]
			i++
		case "--assembly-report":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --assembly-report")
			}
			opts.assemblyReportPath = args[i+1]
			i++
		case "--assembled-output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --assembled-output")
			}
			opts.assembledOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --task")
	}
	if opts.enableConfigPath == "" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --enable-config")
	}
	if opts.executionGatePath == "" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --execution-gate")
	}
	if opts.assemblyReportPath == "" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --assembly-report")
	}
	if opts.assembledOutputPath == "" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --assembled-output")
	}
	if opts.outputPath == "" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedInjectionProviderRunPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedInjectionProviderRunPlan(opts codexMaterializedInjectionProviderRunPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         opts.taskPath,
		EnableConfigPath:                 opts.enableConfigPath,
		ExecutionGatePath:                opts.executionGatePath,
		AssemblyReportPath:               opts.assemblyReportPath,
		AssembledOutputPath:              opts.assembledOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
		OutputPath:                       opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-provider-run-plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedInjectionProviderRunPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedInjectionProviderRunPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-injection-provider-run-plan failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderCallApprovalNewOptions struct {
	readinessReportPath  string
	providerCallGatePath string
	payloadReportPath    string
	outputPath           string
}

func parseCodexProviderCallApprovalNewOptions(args []string) (codexProviderCallApprovalNewOptions, error) {
	opts := codexProviderCallApprovalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderCallApprovalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.readinessReportPath == "" {
		return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing --readiness-report")
	}
	if opts.providerCallGatePath == "" {
		return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing --provider-call-gate")
	}
	if opts.payloadReportPath == "" {
		return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing --payload-report")
	}
	if opts.outputPath == "" {
		return codexProviderCallApprovalNewOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runCodexProviderCallApprovalNew(opts codexProviderCallApprovalNewOptions, stdout io.Writer, stderr io.Writer) int {
	request, err := retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath:  opts.readinessReportPath,
		ProviderCallGatePath: opts.providerCallGatePath,
		PayloadReportPath:    opts.payloadReportPath,
		OutputPath:           opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-approval new failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-call-approval new: ok")
	fmt.Fprintf(stdout, "status: %s\n", request.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "provider_payload_sha256: %s\n", request.ProviderPayloadSHA256)
	fmt.Fprintf(stdout, "provider_call_allowed_now: %t\n", request.ProviderCallAllowedNow)
	fmt.Fprintf(stdout, "sent_to_provider: %t\n", request.SentToProvider)
	return 0
}

type codexProviderCallApprovalApproveOptions struct {
	requestPath                  string
	outputPath                   string
	confirmProviderPayloadSHA256 string
}

func parseCodexProviderCallApprovalApproveOptions(args []string) (codexProviderCallApprovalApproveOptions, error) {
	opts := codexProviderCallApprovalApproveOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return codexProviderCallApprovalApproveOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallApprovalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-provider-payload-sha256", "--confirm-payload-output-sha256":
			if i+1 >= len(args) {
				return codexProviderCallApprovalApproveOptions{}, fmt.Errorf("missing value for %s", args[i])
			}
			opts.confirmProviderPayloadSHA256 = args[i+1]
			i++
		default:
			return codexProviderCallApprovalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.requestPath == "" {
		return codexProviderCallApprovalApproveOptions{}, fmt.Errorf("missing --request")
	}
	if opts.outputPath == "" {
		return codexProviderCallApprovalApproveOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runCodexProviderCallApprovalApprove(opts codexProviderCallApprovalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	approval, err := retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath:                  opts.requestPath,
		OutputPath:                   opts.outputPath,
		ConfirmProviderPayloadSHA256: opts.confirmProviderPayloadSHA256,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-approval approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-call-approval approve: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "provider_call_authorized_for_future: %t\n", approval.ProviderCallAuthorizedForFuture)
	fmt.Fprintf(stdout, "provider_call_allowed_now: %t\n", approval.ProviderCallAllowedNow)
	fmt.Fprintf(stdout, "sent_to_provider: %t\n", approval.SentToProvider)
	return 0
}

type codexProviderCallApprovalInspectOptions struct {
	approvalPath string
	requestPath  string
	outputFormat string
}

func parseCodexProviderCallApprovalInspectOptions(args []string) (codexProviderCallApprovalInspectOptions, error) {
	opts := codexProviderCallApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return codexProviderCallApprovalInspectOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return codexProviderCallApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	return opts, nil
}

func runCodexProviderCallApprovalInspect(opts codexProviderCallApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectProviderCallApproval(opts.approvalPath, retrievalcontext.InspectProviderCallApprovalOptions{
		RequestPath: opts.requestPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-approval inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteInspectProviderCallApprovalJSON(result, stdout)
	default:
		err = retrievalcontext.WriteInspectProviderCallApprovalText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-approval inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderCallExecutionBundleOptions struct {
	dispatchConfigPath   string
	payloadReportPath    string
	providerCallGatePath string
	readinessReportPath  string
	approvalPath         string
	outputPath           string
	outputFormat         string
}

func parseCodexProviderCallExecutionBundleOptions(args []string) (codexProviderCallExecutionBundleOptions, error) {
	opts := codexProviderCallExecutionBundleOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.dispatchConfigPath == "" {
		return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing --dispatch-config")
	}
	if opts.payloadReportPath == "" {
		return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing --payload-report")
	}
	if opts.providerCallGatePath == "" {
		return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing --provider-call-gate")
	}
	if opts.readinessReportPath == "" {
		return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing --readiness-report")
	}
	if opts.approvalPath == "" {
		return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputPath == "" {
		return codexProviderCallExecutionBundleOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runCodexProviderCallExecutionBundle(opts codexProviderCallExecutionBundleOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath:   opts.dispatchConfigPath,
		PayloadReportPath:    opts.payloadReportPath,
		ProviderCallGatePath: opts.providerCallGatePath,
		ReadinessReportPath:  opts.readinessReportPath,
		ApprovalPath:         opts.approvalPath,
		OutputPath:           opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-execution-bundle failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCallExecutionBundleJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCallExecutionBundleText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-execution-bundle failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runOpenCodeDryRun(opts workerDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := loadValidatedWorkerTask(opts.taskPath, "opencode")
	if err != nil {
		writeTaskLoadError(stderr, err)
		return 1
	}

	worker, envRequirements, strategyPlan, err := configuredOpenCodeDryRunWorkerForTask(opts.workersConfigPath, task)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	writeMissingEnvWarnings(stderr, "opencode", envRequirements)

	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dry-run failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "workspace: %s\n", event.Workspace)
	fmt.Fprintf(stdout, "policy: %s\n", event.Sandbox)
	if strategyPlan != nil {
		fmt.Fprintln(stdout, "model_strategy: planned")
		fmt.Fprintf(stdout, "planned_model_profile: %s\n", strategyPlan.Name)
		fmt.Fprintf(stdout, "provider: %s\n", strategyPlan.Provider)
		fmt.Fprintf(stdout, "model: %s\n", strategyPlan.Model)
		fmt.Fprintf(stdout, "model_arg: %s\n", strategyPlan.ModelArg)
	}
	fmt.Fprintf(stdout, "command: %s\n", strings.Join(event.Command, " "))
	return 0
}

type codexRunOptions struct {
	taskPath          string
	storePath         string
	artifactsDir      string
	domainsPath       string
	memoryPolicyPath  string
	workersConfigPath string
	runtimeConfigPath string
	validationRuntime string
	workerRuntime     string
}

func parseCodexRunOptions(args []string) (codexRunOptions, error) {
	if len(args) < 1 {
		return codexRunOptions{}, fmt.Errorf("missing task path")
	}

	opts := codexRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--memory-policy":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --memory-policy")
			}
			opts.memoryPolicyPath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--validation-runtime":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --validation-runtime")
			}
			opts.validationRuntime = args[i+1]
			i++
		case "--worker-runtime":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --worker-runtime")
			}
			opts.workerRuntime = args[i+1]
			i++
		default:
			return codexRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return codexRunOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return codexRunOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.validationRuntime != "" && opts.validationRuntime != tasks.ValidationRuntimeLocal && opts.validationRuntime != tasks.ValidationRuntimeDocker {
		return codexRunOptions{}, fmt.Errorf("unsupported validation runtime %q", opts.validationRuntime)
	}
	if opts.workerRuntime != "" && opts.workerRuntime != runner.WorkerRuntimeLocal && opts.workerRuntime != runner.WorkerRuntimeDocker {
		return codexRunOptions{}, fmt.Errorf("unsupported worker runtime %q", opts.workerRuntime)
	}
	return opts, nil
}

func runCodexRun(opts codexRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := loadValidatedWorkerTask(opts.taskPath, "codex")
	if err != nil {
		writeTaskLoadError(stderr, err)
		return 1
	}
	_, envRequirements, err := configuredWorkerDefinitionForTask(opts.workersConfigPath, task, "codex")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateRequiredWorkerEnv("codex", envRequirements); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if opts.workerRuntime == runner.WorkerRuntimeDocker {
		fmt.Fprintln(stderr, "error: Codex Docker worker runtime is not implemented; OpenCode Docker is experimental")
		return 1
	}
	runtimeCfg, err := loadRunRuntimeConfig(opts.runtimeConfigPath, opts.validationRuntime, opts.workerRuntime, task)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	worker, err := configuredCodexWorker(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	codexRunner := runner.CodexRunner{
		WorkerFactory: func() workers.Worker {
			return worker
		},
		RunIDFactory:            runIDFactory,
		GitDiffRunner:           gitDiffRunner,
		GitSnapshotRunner:       gitSnapshotRunner,
		WorkspaceManagerFactory: workspaceManagerFactory,
	}
	return codexRunner.Run(context.Background(), runner.CodexRunOptions{
		TaskPath:          opts.taskPath,
		StorePath:         opts.storePath,
		ArtifactsDir:      opts.artifactsDir,
		DomainsPath:       opts.domainsPath,
		MemoryPolicyPath:  opts.memoryPolicyPath,
		EnvRequirements:   envRequirements,
		WorkerRuntime:     opts.workerRuntime,
		ValidationRuntime: opts.validationRuntime,
		RuntimeConfig:     runtimeCfg,
		RuntimeConfigPath: opts.runtimeConfigPath,
	}, stdout, stderr)
}

type openCodeRunOptions struct {
	taskPath          string
	storePath         string
	artifactsDir      string
	domainsPath       string
	memoryPolicyPath  string
	workersConfigPath string
	runtimeConfigPath string
	validationRuntime string
	workerRuntime     string
}

func parseOpenCodeRunOptions(args []string) (openCodeRunOptions, error) {
	if len(args) < 1 {
		return openCodeRunOptions{}, fmt.Errorf("missing task path")
	}

	opts := openCodeRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--memory-policy":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --memory-policy")
			}
			opts.memoryPolicyPath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		case "--runtime-config":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --runtime-config")
			}
			opts.runtimeConfigPath = args[i+1]
			i++
		case "--validation-runtime":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --validation-runtime")
			}
			opts.validationRuntime = args[i+1]
			i++
		case "--worker-runtime":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --worker-runtime")
			}
			opts.workerRuntime = args[i+1]
			i++
		default:
			return openCodeRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return openCodeRunOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return openCodeRunOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.validationRuntime != "" && opts.validationRuntime != tasks.ValidationRuntimeLocal && opts.validationRuntime != tasks.ValidationRuntimeDocker {
		return openCodeRunOptions{}, fmt.Errorf("unsupported validation runtime %q", opts.validationRuntime)
	}
	if opts.workerRuntime != "" && opts.workerRuntime != runner.WorkerRuntimeLocal && opts.workerRuntime != runner.WorkerRuntimeDocker {
		return openCodeRunOptions{}, fmt.Errorf("unsupported worker runtime %q", opts.workerRuntime)
	}
	return opts, nil
}

func runOpenCodeRun(opts openCodeRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := loadValidatedWorkerTask(opts.taskPath, "opencode")
	if err != nil {
		writeTaskLoadError(stderr, err)
		return 1
	}
	_, envRequirements, err := configuredWorkerDefinitionForTask(opts.workersConfigPath, task, "opencode")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateRequiredWorkerEnv("opencode", envRequirements); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	runtimeCfg, err := loadRunRuntimeConfig(opts.runtimeConfigPath, opts.validationRuntime, opts.workerRuntime, task)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	worker, err := configuredOpenCodeWorkerForTask(opts.workersConfigPath, task)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	openCodeRunner := runner.OpenCodeRunner{
		WorkerFactory: func() workers.Worker {
			return worker
		},
		RunIDFactory:            runIDFactory,
		GitDiffRunner:           gitDiffRunner,
		GitSnapshotRunner:       gitSnapshotRunner,
		WorkspaceManagerFactory: workspaceManagerFactory,
	}
	return openCodeRunner.Run(context.Background(), runner.OpenCodeRunOptions{
		TaskPath:          opts.taskPath,
		StorePath:         opts.storePath,
		ArtifactsDir:      opts.artifactsDir,
		DomainsPath:       opts.domainsPath,
		MemoryPolicyPath:  opts.memoryPolicyPath,
		EnvRequirements:   envRequirements,
		WorkerRuntime:     opts.workerRuntime,
		ValidationRuntime: opts.validationRuntime,
		RuntimeConfig:     runtimeCfg,
		RuntimeConfigPath: opts.runtimeConfigPath,
	}, stdout, stderr)
}

func loadRunRuntimeConfig(runtimeConfigPath string, validationRuntime string, workerRuntime string, task *tasks.Task) (*runtimeconfig.Config, error) {
	validationRuntimeName := strings.TrimSpace(validationRuntime)
	if validationRuntimeName == "" && task != nil {
		validationRuntimeName = strings.TrimSpace(task.Validation.Runtime)
	}
	workerRuntimeName := strings.TrimSpace(workerRuntime)
	if validationRuntimeName != tasks.ValidationRuntimeDocker && workerRuntimeName != runner.WorkerRuntimeDocker {
		return nil, nil
	}
	if strings.TrimSpace(runtimeConfigPath) == "" {
		if workerRuntimeName == runner.WorkerRuntimeDocker {
			return nil, fmt.Errorf("worker runtime docker requires --runtime-config")
		}
		return nil, fmt.Errorf("validation runtime docker requires --runtime-config")
	}
	cfg, err := runtimeconfig.Load(runtimeConfigPath)
	if err != nil {
		return nil, err
	}
	if _, err := runtimeconfig.PlanDocker(cfg, task.Workspace.Path); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func configuredCodexWorker(workersConfigPath string) (workers.Worker, error) {
	if strings.TrimSpace(workersConfigPath) == "" {
		return codexWorkerFactory(), nil
	}
	workerConfig, err := configuredWorkerDefinition(workersConfigPath, "codex")
	if err != nil {
		return nil, err
	}
	return codex.NewWithCommand(workerConfig.Command), nil
}

func configuredOpenCodeWorker(workersConfigPath string) (workers.Worker, error) {
	if strings.TrimSpace(workersConfigPath) == "" {
		return opencodeWorkerFactory(), nil
	}
	workerConfig, err := configuredWorkerDefinition(workersConfigPath, "opencode")
	if err != nil {
		return nil, err
	}
	return opencode.NewWithCommand(workerConfig.Command), nil
}

type plannedModelProfile struct {
	Name     string
	Provider string
	Model    string
	ModelArg string
}

func plannedModelProfileFromResolved(resolved workerconfig.ResolvedModelStrategy) (*plannedModelProfile, error) {
	planned, ok := resolved.PlannedModelProfile()
	if !ok {
		return nil, fmt.Errorf("model_strategy.preferred must not be empty")
	}
	return &plannedModelProfile{
		Name:     planned.Name,
		Provider: planned.Profile.Provider,
		Model:    planned.Profile.Model,
		ModelArg: planned.Profile.ModelArg,
	}, nil
}

func configuredOpenCodeDryRunWorkerForTask(workersConfigPath string, task *tasks.Task) (workers.Worker, []workerconfig.EnvRequirementCheck, *plannedModelProfile, error) {
	cfg, err := configuredWorkersConfig(workersConfigPath)
	if err != nil {
		return nil, nil, nil, err
	}
	workerConfig := cfg.Worker("opencode")
	opts := opencode.Options{Command: workerConfig.Command}
	profileName := ""
	var strategyPlan *plannedModelProfile

	if task != nil && strings.TrimSpace(task.ModelProfile) != "" {
		profile, err := cfg.ResolveModelProfile("opencode", task.ModelProfile)
		if err != nil {
			return nil, nil, nil, err
		}
		profileName = task.ModelProfile
		opts.ModelProfile = task.ModelProfile
		opts.Provider = profile.Provider
		opts.Model = profile.Model
		opts.ModelArg = profile.ModelArg
	}
	if task != nil && task.ModelStrategy != nil {
		resolved, err := cfg.ResolveModelStrategy("opencode", task.ModelStrategy)
		if err != nil {
			return nil, nil, nil, err
		}
		strategyPlan, err = plannedModelProfileFromResolved(resolved)
		if err != nil {
			return nil, nil, nil, err
		}
		profileName = strategyPlan.Name
		opts.ModelArg = strategyPlan.ModelArg
	}

	envRequirements, err := cfg.EnvRequirementChecksFor("opencode", profileName)
	if err != nil {
		return nil, nil, nil, err
	}
	return opencode.NewWithOptions(opts), envRequirements, strategyPlan, nil
}

func configuredOpenCodeWorkerForTask(workersConfigPath string, task *tasks.Task) (workers.Worker, error) {
	if strings.TrimSpace(workersConfigPath) == "" && (task == nil || (strings.TrimSpace(task.ModelProfile) == "" && task.ModelStrategy == nil)) {
		return opencodeWorkerFactory(), nil
	}
	cfg, err := configuredWorkersConfig(workersConfigPath)
	if err != nil {
		return nil, err
	}
	workerConfig := cfg.Worker("opencode")
	opts := opencode.Options{Command: workerConfig.Command}
	if task != nil && strings.TrimSpace(task.ModelProfile) != "" {
		profile, err := cfg.ResolveModelProfile("opencode", task.ModelProfile)
		if err != nil {
			return nil, err
		}
		opts.ModelProfile = task.ModelProfile
		opts.Provider = profile.Provider
		opts.Model = profile.Model
		opts.ModelArg = profile.ModelArg
	}
	if task != nil && task.ModelStrategy != nil {
		resolved, err := cfg.ResolveModelStrategy("opencode", task.ModelStrategy)
		if err != nil {
			return nil, err
		}
		selected, err := plannedModelProfileFromResolved(resolved)
		if err != nil {
			return nil, err
		}
		opts.ModelStrategy = "selected"
		opts.SelectedModelProfile = selected.Name
		opts.Provider = selected.Provider
		opts.Model = selected.Model
		opts.ModelArg = selected.ModelArg
	}
	return opencode.NewWithOptions(opts), nil
}

func configuredWorkerDefinition(workersConfigPath string, worker string) (workerconfig.Worker, error) {
	cfg, err := configuredWorkersConfig(workersConfigPath)
	if err != nil {
		return workerconfig.Worker{}, err
	}
	return cfg.Worker(worker), nil
}

func configuredWorkerDefinitionForTask(workersConfigPath string, task *tasks.Task, worker string) (workerconfig.Worker, []workerconfig.EnvRequirementCheck, error) {
	cfg, err := configuredWorkersConfig(workersConfigPath)
	if err != nil {
		return workerconfig.Worker{}, nil, err
	}
	profileName := ""
	if task != nil {
		profileName = task.ModelProfile
		if task.ModelStrategy != nil {
			resolved, err := cfg.ResolveModelStrategy(worker, task.ModelStrategy)
			if err != nil {
				return workerconfig.Worker{}, nil, err
			}
			planned, err := plannedModelProfileFromResolved(resolved)
			if err != nil {
				return workerconfig.Worker{}, nil, err
			}
			profileName = planned.Name
		}
	}
	envRequirements, err := cfg.EnvRequirementChecksFor(worker, profileName)
	if err != nil {
		return workerconfig.Worker{}, nil, err
	}
	return cfg.Worker(worker), envRequirements, nil
}

func configuredWorkersConfig(workersConfigPath string) (workerconfig.Config, error) {
	cfg := workerconfig.Default()
	if strings.TrimSpace(workersConfigPath) != "" {
		loaded, err := workerconfig.Load(workersConfigPath)
		if err != nil {
			return workerconfig.Config{}, err
		}
		cfg = loaded
	}
	return cfg, nil
}

func loadValidatedWorkerTask(taskPath string, worker string) (*tasks.Task, error) {
	task, err := tasks.LoadFromFile(taskPath)
	if err != nil {
		return nil, err
	}
	if err := tasks.Validate(task); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if _, err := retrievalcontext.ValidateTaskMaterializedInjection(task.RetrievalContext.MaterializedInjection); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if err := ensureTaskWorker(task, worker); err != nil {
		return nil, err
	}
	return task, nil
}

func writeTaskLoadError(stderr io.Writer, err error) {
	if strings.HasPrefix(err.Error(), "validation failed:") || strings.HasPrefix(err.Error(), "task worker ") {
		fmt.Fprintf(stderr, "%v\n", err)
		return
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
}

func validateRequiredWorkerEnv(worker string, envRequirements []workerconfig.EnvRequirementCheck) error {
	missing := workerconfig.MissingRequiredEnv(envRequirements)
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for _, check := range missing {
		names = append(names, check.Name)
	}
	return fmt.Errorf("worker %s missing required env: %s", worker, strings.Join(names, ", "))
}

func writeMissingEnvWarnings(stderr io.Writer, worker string, envRequirements []workerconfig.EnvRequirementCheck) {
	for _, check := range workerconfig.MissingRequiredEnv(envRequirements) {
		fmt.Fprintf(stderr, "warning: worker %s required env %s is missing\n", worker, check.Name)
	}
}

type runsReportOptions struct {
	storePath    string
	by           string
	worker       string
	status       runs.RunStatus
	since        *time.Time
	outputFormat string
}

func parseRunsReportOptions(args []string) (runsReportOptions, error) {
	opts := runsReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return runsReportOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--by":
			if i+1 >= len(args) {
				return runsReportOptions{}, fmt.Errorf("missing value for --by")
			}
			opts.by = args[i+1]
			i++
		case "--worker":
			if i+1 >= len(args) {
				return runsReportOptions{}, fmt.Errorf("missing value for --worker")
			}
			opts.worker = args[i+1]
			i++
		case "--status":
			if i+1 >= len(args) {
				return runsReportOptions{}, fmt.Errorf("missing value for --status")
			}
			status, err := parseRunsReportStatus(args[i+1])
			if err != nil {
				return runsReportOptions{}, err
			}
			opts.status = status
			i++
		case "--since":
			if i+1 >= len(args) {
				return runsReportOptions{}, fmt.Errorf("missing value for --since")
			}
			since, err := parseRunsReportSince(args[i+1])
			if err != nil {
				return runsReportOptions{}, err
			}
			opts.since = &since
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return runsReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return runsReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return runsReportOptions{}, fmt.Errorf("missing --store")
	}
	if opts.by != "" && opts.by != runreport.GroupByModelProfile {
		return runsReportOptions{}, fmt.Errorf("unsupported --by %q", opts.by)
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return runsReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseRunsReportStatus(value string) (runs.RunStatus, error) {
	switch runs.RunStatus(value) {
	case runs.StatusSucceeded, runs.StatusFailed, runs.StatusPolicyFailed:
		return runs.RunStatus(value), nil
	default:
		return "", fmt.Errorf("unsupported --status %q", value)
	}
}

func parseRunsReportSince(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q", value)
	}
	return parsed.UTC(), nil
}

func runRunsReport(opts runsReportOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	report, err := runreport.Build(context.Background(), db, runreport.Options{
		GroupBy: opts.by,
		Worker:  opts.worker,
		Status:  opts.status,
		Since:   opts.since,
	})
	if err != nil {
		fmt.Fprintf(stderr, "runs report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = runreport.WriteJSON(report, stdout)
	default:
		err = runreport.WriteText(report, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "runs report failed: %v\n", err)
		return 1
	}
	return 0
}

type runsRetrievalReportOptions struct {
	storePath    string
	runID        string
	outputFormat string
}

func parseRunsRetrievalReportOptions(args []string) (runsRetrievalReportOptions, error) {
	opts := runsRetrievalReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return runsRetrievalReportOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return runsRetrievalReportOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return runsRetrievalReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return runsRetrievalReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return runsRetrievalReportOptions{}, fmt.Errorf("missing --store")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return runsRetrievalReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRunsRetrievalReport(opts runsRetrievalReportOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "runs retrieval-report failed: %v\n", err)
		return 1
	}
	defer db.Close()

	report, err := retrievalcontext.BuildRetrievalReport(context.Background(), db, retrievalcontext.RetrievalReportOptions{
		RunID: opts.runID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "runs retrieval-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteRetrievalReportJSON(report, stdout)
	default:
		err = retrievalcontext.WriteRetrievalReportText(report, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "runs retrieval-report failed: %v\n", err)
		return 1
	}
	return 0
}

type retrievalContextInspectOptions struct {
	artifactPath string
	outputFormat string
}

func parseRetrievalContextInspectOptions(args []string) (retrievalContextInspectOptions, error) {
	opts := retrievalContextInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) {
				return retrievalContextInspectOptions{}, fmt.Errorf("missing value for --artifact")
			}
			opts.artifactPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.artifactPath == "" {
		return retrievalContextInspectOptions{}, fmt.Errorf("missing --artifact")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextInspect(opts retrievalContextInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectArtifact(opts.artifactPath)
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteInspectJSON(result, stdout)
	default:
		err = retrievalcontext.WriteInspectText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type retrievalContextMaterializeOptions struct {
	retrievalContextPath    string
	chunksPath              string
	outputPath              string
	summaryPath             string
	maxCharsPerChunk        int
	maxTotalChars           int
	confirmIncludeChunkText bool
}

func parseRetrievalContextMaterializeOptions(args []string) (retrievalContextMaterializeOptions, error) {
	opts := retrievalContextMaterializeOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--retrieval-context":
			if i+1 >= len(args) {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("missing value for --retrieval-context")
			}
			opts.retrievalContextPath = args[i+1]
			i++
		case "--chunks":
			if i+1 >= len(args) {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("missing value for --chunks")
			}
			opts.chunksPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--summary":
			if i+1 >= len(args) {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("missing value for --summary")
			}
			opts.summaryPath = args[i+1]
			i++
		case "--max-chars-per-chunk":
			if i+1 >= len(args) {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("missing value for --max-chars-per-chunk")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("invalid --max-chars-per-chunk %q", args[i+1])
			}
			opts.maxCharsPerChunk = value
			i++
		case "--max-total-chars":
			if i+1 >= len(args) {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("missing value for --max-total-chars")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil {
				return retrievalContextMaterializeOptions{}, fmt.Errorf("invalid --max-total-chars %q", args[i+1])
			}
			opts.maxTotalChars = value
			i++
		case "--confirm-include-chunk-text":
			opts.confirmIncludeChunkText = true
		default:
			return retrievalContextMaterializeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.retrievalContextPath == "" {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing --retrieval-context")
	}
	if opts.chunksPath == "" {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing --chunks")
	}
	if opts.outputPath == "" {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing --output")
	}
	if opts.summaryPath == "" {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing --summary")
	}
	if opts.maxCharsPerChunk <= 0 {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing or invalid --max-chars-per-chunk")
	}
	if opts.maxTotalChars <= 0 {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing or invalid --max-total-chars")
	}
	if !opts.confirmIncludeChunkText {
		return retrievalContextMaterializeOptions{}, fmt.Errorf("missing --confirm-include-chunk-text")
	}
	return opts, nil
}

func runRetrievalContextMaterialize(opts retrievalContextMaterializeOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.Materialize(retrievalcontext.MaterializeOptions{
		RetrievalContextPath:    opts.retrievalContextPath,
		ChunksPath:              opts.chunksPath,
		OutputPath:              opts.outputPath,
		SummaryPath:             opts.summaryPath,
		MaxCharsPerChunk:        opts.maxCharsPerChunk,
		MaxTotalChars:           opts.maxTotalChars,
		ConfirmIncludeChunkText: opts.confirmIncludeChunkText,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context materialize failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context materialize: ok")
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "summary: %s\n", opts.summaryPath)
	fmt.Fprintf(stdout, "included_chunk_count: %d\n", result.IncludedChunkCount)
	fmt.Fprintf(stdout, "omitted_chunk_count: %d\n", result.OmittedChunkCount)
	fmt.Fprintf(stdout, "total_chars_included: %d\n", result.TotalCharsIncluded)
	return 0
}

type retrievalContextMaterializedReportOptions struct {
	artifactPath string
	outputFormat string
}

func parseRetrievalContextMaterializedReportOptions(args []string) (retrievalContextMaterializedReportOptions, error) {
	opts := retrievalContextMaterializedReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) {
				return retrievalContextMaterializedReportOptions{}, fmt.Errorf("missing value for --artifact")
			}
			opts.artifactPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextMaterializedReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextMaterializedReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.artifactPath == "" {
		return retrievalContextMaterializedReportOptions{}, fmt.Errorf("missing --artifact")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextMaterializedReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextMaterializedReport(opts retrievalContextMaterializedReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedReport(opts.artifactPath)
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context materialized-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context materialized-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type retrievalContextBundleOptions struct {
	retrievalContextPath string
	materializedPath     string
	outputPath           string
	summaryPath          string
	outputFormat         string
}

func parseRetrievalContextBundleOptions(args []string) (retrievalContextBundleOptions, error) {
	opts := retrievalContextBundleOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--retrieval-context":
			if i+1 >= len(args) {
				return retrievalContextBundleOptions{}, fmt.Errorf("missing value for --retrieval-context")
			}
			opts.retrievalContextPath = args[i+1]
			i++
		case "--materialized":
			if i+1 >= len(args) {
				return retrievalContextBundleOptions{}, fmt.Errorf("missing value for --materialized")
			}
			opts.materializedPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextBundleOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--summary":
			if i+1 >= len(args) {
				return retrievalContextBundleOptions{}, fmt.Errorf("missing value for --summary")
			}
			opts.summaryPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextBundleOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextBundleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.retrievalContextPath == "" {
		return retrievalContextBundleOptions{}, fmt.Errorf("missing --retrieval-context")
	}
	if opts.materializedPath == "" {
		return retrievalContextBundleOptions{}, fmt.Errorf("missing --materialized")
	}
	if opts.outputPath == "" {
		return retrievalContextBundleOptions{}, fmt.Errorf("missing --output")
	}
	if opts.summaryPath == "" {
		return retrievalContextBundleOptions{}, fmt.Errorf("missing --summary")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextBundleOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextBundle(opts retrievalContextBundleOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.Bundle(retrievalcontext.BundleOptions{
		RetrievalContextPath: opts.retrievalContextPath,
		MaterializedPath:     opts.materializedPath,
		OutputPath:           opts.outputPath,
		SummaryPath:          opts.summaryPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context bundle failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteBundleJSON(result, stdout)
	default:
		err = retrievalcontext.WriteBundleText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context bundle failed: %v\n", err)
		return 1
	}
	return 0
}

type retrievalContextApprovalNewOptions struct {
	bundlePath string
	outputPath string
}

func parseRetrievalContextApprovalNewOptions(args []string) (retrievalContextApprovalNewOptions, error) {
	opts := retrievalContextApprovalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bundle":
			if i+1 >= len(args) {
				return retrievalContextApprovalNewOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextApprovalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return retrievalContextApprovalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.bundlePath == "" {
		return retrievalContextApprovalNewOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.outputPath == "" {
		return retrievalContextApprovalNewOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runRetrievalContextApprovalNew(opts retrievalContextApprovalNewOptions, stdout io.Writer, stderr io.Writer) int {
	request, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: opts.bundlePath,
		OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context approval new failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context approval new: ok")
	fmt.Fprintf(stdout, "status: %s\n", request.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "bundle_sha256: %s\n", request.BundleSHA256)
	fmt.Fprintf(stdout, "included_chunk_count: %d\n", request.IncludedChunkCount)
	fmt.Fprintf(stdout, "omitted_chunk_count: %d\n", request.OmittedChunkCount)
	return 0
}

type retrievalContextApprovalApproveOptions struct {
	requestPath                       string
	outputPath                        string
	confirmApproveMaterializedContext bool
}

func parseRetrievalContextApprovalApproveOptions(args []string) (retrievalContextApprovalApproveOptions, error) {
	opts := retrievalContextApprovalApproveOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return retrievalContextApprovalApproveOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextApprovalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-approve-materialized-context":
			opts.confirmApproveMaterializedContext = true
		default:
			return retrievalContextApprovalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.requestPath == "" {
		return retrievalContextApprovalApproveOptions{}, fmt.Errorf("missing --request")
	}
	if opts.outputPath == "" {
		return retrievalContextApprovalApproveOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmApproveMaterializedContext {
		return retrievalContextApprovalApproveOptions{}, fmt.Errorf("missing --confirm-approve-materialized-context")
	}
	return opts, nil
}

func runRetrievalContextApprovalApprove(opts retrievalContextApprovalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	approval, err := retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath:                       opts.requestPath,
		OutputPath:                        opts.outputPath,
		ConfirmApproveMaterializedContext: opts.confirmApproveMaterializedContext,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context approval approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context approval approve: ok")
	fmt.Fprintf(stdout, "approved: %t\n", approval.Approved)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "allowed_use: %s\n", approval.AllowedUse)
	fmt.Fprintf(stdout, "runner_injection_allowed: %t\n", approval.RunnerInjectionAllowed)
	return 0
}

type retrievalContextApprovalInspectOptions struct {
	approvalPath string
	outputFormat string
}

func parseRetrievalContextApprovalInspectOptions(args []string) (retrievalContextApprovalInspectOptions, error) {
	opts := retrievalContextApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return retrievalContextApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return retrievalContextApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextApprovalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextApprovalInspect(opts retrievalContextApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectApproval(opts.approvalPath, retrievalcontext.InspectApprovalOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context approval inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteInspectApprovalJSON(result, stdout)
	default:
		err = retrievalcontext.WriteInspectApprovalText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context approval inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type retrievalContextInjectionPlanOptions struct {
	approvalPath     string
	requestPath      string
	bundlePath       string
	materializedPath string
	outputPath       string
	summaryPath      string
}

func parseRetrievalContextInjectionPlanOptions(args []string) (retrievalContextInjectionPlanOptions, error) {
	opts := retrievalContextInjectionPlanOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--bundle":
			if i+1 >= len(args) {
				return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--materialized":
			if i+1 >= len(args) {
				return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing value for --materialized")
			}
			opts.materializedPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--summary":
			if i+1 >= len(args) {
				return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing value for --summary")
			}
			opts.summaryPath = args[i+1]
			i++
		default:
			return retrievalContextInjectionPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.requestPath == "" {
		return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing --request")
	}
	if opts.bundlePath == "" {
		return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.materializedPath == "" {
		return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing --materialized")
	}
	if opts.outputPath == "" {
		return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing --output")
	}
	if opts.summaryPath == "" {
		return retrievalContextInjectionPlanOptions{}, fmt.Errorf("missing --summary")
	}
	return opts, nil
}

func runRetrievalContextInjectionPlan(opts retrievalContextInjectionPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InjectionPlan(retrievalcontext.InjectionPlanOptions{
		ApprovalPath:     opts.approvalPath,
		RequestPath:      opts.requestPath,
		BundlePath:       opts.bundlePath,
		MaterializedPath: opts.materializedPath,
		OutputPath:       opts.outputPath,
		SummaryPath:      opts.summaryPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-plan failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context injection-plan: ok")
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "summary: %s\n", opts.summaryPath)
	fmt.Fprintf(stdout, "can_inject_now: %t\n", result.CanInjectNow)
	fmt.Fprintf(stdout, "reason: %s\n", result.Reason)
	fmt.Fprintf(stdout, "estimated_prompt_chars: %d\n", result.EstimatedPromptChars)
	fmt.Fprintf(stdout, "required_future_flag: %s\n", result.RequiredFutureFlag)
	return 0
}

type retrievalContextInjectionExecutionPlanOptions struct {
	policyPath                   string
	governanceReportPath         string
	injectionApprovalPath        string
	injectionApprovalRequestPath string
	materializedPath             string
	outputPath                   string
	summaryPath                  string
}

func parseRetrievalContextInjectionExecutionPlanOptions(args []string) (retrievalContextInjectionExecutionPlanOptions, error) {
	opts := retrievalContextInjectionExecutionPlanOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--governance-report":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --governance-report")
			}
			opts.governanceReportPath = args[i+1]
			i++
		case "--injection-approval":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --injection-approval")
			}
			opts.injectionApprovalPath = args[i+1]
			i++
		case "--injection-approval-request":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --injection-approval-request")
			}
			opts.injectionApprovalRequestPath = args[i+1]
			i++
		case "--materialized":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --materialized")
			}
			opts.materializedPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--summary":
			if i+1 >= len(args) {
				return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing value for --summary")
			}
			opts.summaryPath = args[i+1]
			i++
		default:
			return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.governanceReportPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --governance-report")
	}
	if opts.injectionApprovalPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --injection-approval")
	}
	if opts.injectionApprovalRequestPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --injection-approval-request")
	}
	if opts.materializedPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --materialized")
	}
	if opts.outputPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --output")
	}
	if opts.summaryPath == "" {
		return retrievalContextInjectionExecutionPlanOptions{}, fmt.Errorf("missing --summary")
	}
	return opts, nil
}

func runRetrievalContextInjectionExecutionPlan(opts retrievalContextInjectionExecutionPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InjectionExecutionPlan(retrievalcontext.InjectionExecutionPlanOptions{
		PolicyPath:                   opts.policyPath,
		GovernanceReportPath:         opts.governanceReportPath,
		InjectionApprovalPath:        opts.injectionApprovalPath,
		InjectionApprovalRequestPath: opts.injectionApprovalRequestPath,
		MaterializedPath:             opts.materializedPath,
		OutputPath:                   opts.outputPath,
		SummaryPath:                  opts.summaryPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-execution-plan failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context injection-execution-plan: ok")
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "summary: %s\n", opts.summaryPath)
	fmt.Fprintf(stdout, "would_execute_runner: %t\n", result.WouldExecuteRunner)
	fmt.Fprintf(stdout, "would_inject_materialized_context: %t\n", result.WouldInjectMaterializedContext)
	fmt.Fprintf(stdout, "execution_supported_now: %t\n", result.ExecutionSupportedNow)
	fmt.Fprintf(stdout, "reason: %s\n", result.Reason)
	fmt.Fprintf(stdout, "total_chars_included: %d\n", result.TotalCharsIncluded)
	fmt.Fprintf(stdout, "required_future_flag: %s\n", result.RequiredFutureFlag)
	return 0
}

type retrievalContextPromptPreviewOptions struct {
	executionPlanPath                string
	policyPath                       string
	materializedPath                 string
	outputPath                       string
	manifestPath                     string
	confirmRenderMaterializedContext bool
}

func parseRetrievalContextPromptPreviewOptions(args []string) (retrievalContextPromptPreviewOptions, error) {
	opts := retrievalContextPromptPreviewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--execution-plan":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing value for --execution-plan")
			}
			opts.executionPlanPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--materialized":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing value for --materialized")
			}
			opts.materializedPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--manifest":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing value for --manifest")
			}
			opts.manifestPath = args[i+1]
			i++
		case "--confirm-render-materialized-context":
			opts.confirmRenderMaterializedContext = true
		default:
			return retrievalContextPromptPreviewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.executionPlanPath == "" {
		return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing --execution-plan")
	}
	if opts.policyPath == "" {
		return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.materializedPath == "" {
		return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing --materialized")
	}
	if opts.outputPath == "" {
		return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing --output")
	}
	if opts.manifestPath == "" {
		return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing --manifest")
	}
	if !opts.confirmRenderMaterializedContext {
		return retrievalContextPromptPreviewOptions{}, fmt.Errorf("missing --confirm-render-materialized-context")
	}
	return opts, nil
}

func runRetrievalContextPromptPreview(opts retrievalContextPromptPreviewOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.PromptPreview(retrievalcontext.PromptPreviewOptions{
		ExecutionPlanPath:                opts.executionPlanPath,
		PolicyPath:                       opts.policyPath,
		MaterializedPath:                 opts.materializedPath,
		OutputPath:                       opts.outputPath,
		ManifestPath:                     opts.manifestPath,
		ConfirmRenderMaterializedContext: opts.confirmRenderMaterializedContext,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context prompt-preview failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context prompt-preview: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "manifest: %s\n", opts.manifestPath)
	fmt.Fprintf(stdout, "contains_text: %t\n", result.Manifest.ContainsText)
	fmt.Fprintf(stdout, "preview_only: %t\n", result.Manifest.PreviewOnly)
	fmt.Fprintf(stdout, "runner_execution: %t\n", result.Manifest.RunnerExecution)
	fmt.Fprintf(stdout, "total_chars_rendered: %d\n", result.Manifest.TotalCharsRendered)
	fmt.Fprintf(stdout, "chunk_count: %d\n", result.Manifest.ChunkCount)
	return 0
}

type retrievalContextPromptPreviewReportOptions struct {
	previewPath       string
	manifestPath      string
	executionPlanPath string
	outputFormat      string
}

func parseRetrievalContextPromptPreviewReportOptions(args []string) (retrievalContextPromptPreviewReportOptions, error) {
	opts := retrievalContextPromptPreviewReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--preview":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing value for --preview")
			}
			opts.previewPath = args[i+1]
			i++
		case "--manifest":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing value for --manifest")
			}
			opts.manifestPath = args[i+1]
			i++
		case "--execution-plan":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing value for --execution-plan")
			}
			opts.executionPlanPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.previewPath == "" {
		return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing --preview")
	}
	if opts.manifestPath == "" {
		return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing --manifest")
	}
	if opts.executionPlanPath == "" {
		return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("missing --execution-plan")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextPromptPreviewReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextPromptPreviewReport(opts retrievalContextPromptPreviewReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.PromptPreviewReport(retrievalcontext.PromptPreviewReportOptions{
		PreviewPath:       opts.previewPath,
		ManifestPath:      opts.manifestPath,
		ExecutionPlanPath: opts.executionPlanPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context prompt-preview-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WritePromptPreviewReportJSON(result, stdout)
	default:
		err = retrievalcontext.WritePromptPreviewReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context prompt-preview-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type retrievalContextInjectionGovernanceBundleOptions struct {
	governanceReportPath         string
	policyPath                   string
	injectionApprovalRequestPath string
	injectionApprovalPath        string
	executionPlanPath            string
	promptPreviewManifestPath    string
	promptPreviewReportPath      string
	outputPath                   string
	summaryPath                  string
}

func parseRetrievalContextInjectionGovernanceBundleOptions(args []string) (retrievalContextInjectionGovernanceBundleOptions, error) {
	opts := retrievalContextInjectionGovernanceBundleOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--governance-report":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --governance-report")
			}
			opts.governanceReportPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--injection-approval-request":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --injection-approval-request")
			}
			opts.injectionApprovalRequestPath = args[i+1]
			i++
		case "--injection-approval":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --injection-approval")
			}
			opts.injectionApprovalPath = args[i+1]
			i++
		case "--execution-plan":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --execution-plan")
			}
			opts.executionPlanPath = args[i+1]
			i++
		case "--prompt-preview-manifest":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --prompt-preview-manifest")
			}
			opts.promptPreviewManifestPath = args[i+1]
			i++
		case "--prompt-preview-report":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --prompt-preview-report")
			}
			opts.promptPreviewReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--summary":
			if i+1 >= len(args) {
				return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing value for --summary")
			}
			opts.summaryPath = args[i+1]
			i++
		default:
			return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.governanceReportPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --governance-report")
	}
	if opts.policyPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.injectionApprovalRequestPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --injection-approval-request")
	}
	if opts.injectionApprovalPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --injection-approval")
	}
	if opts.executionPlanPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --execution-plan")
	}
	if opts.promptPreviewManifestPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --prompt-preview-manifest")
	}
	if opts.promptPreviewReportPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --prompt-preview-report")
	}
	if opts.outputPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --output")
	}
	if opts.summaryPath == "" {
		return retrievalContextInjectionGovernanceBundleOptions{}, fmt.Errorf("missing --summary")
	}
	return opts, nil
}

func runRetrievalContextInjectionGovernanceBundle(opts retrievalContextInjectionGovernanceBundleOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InjectionGovernanceBundle(retrievalcontext.InjectionGovernanceBundleOptions{
		GovernanceReportPath:         opts.governanceReportPath,
		PolicyPath:                   opts.policyPath,
		InjectionApprovalRequestPath: opts.injectionApprovalRequestPath,
		InjectionApprovalPath:        opts.injectionApprovalPath,
		ExecutionPlanPath:            opts.executionPlanPath,
		PromptPreviewManifestPath:    opts.promptPreviewManifestPath,
		PromptPreviewReportPath:      opts.promptPreviewReportPath,
		OutputPath:                   opts.outputPath,
		SummaryPath:                  opts.summaryPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-governance-bundle failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context injection-governance-bundle: ok")
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "summary: %s\n", opts.summaryPath)
	fmt.Fprintf(stdout, "contains_text: %t\n", result.ContainsText)
	fmt.Fprintf(stdout, "runner_execution: %t\n", result.RunnerExecution)
	fmt.Fprintf(stdout, "injection_authorized_for_future: %t\n", result.InjectionAuthorizedForFuture)
	fmt.Fprintf(stdout, "execution_supported_now: %t\n", result.ExecutionSupportedNow)
	fmt.Fprintf(stdout, "materialized_sha256: %s\n", result.MaterializedSHA256)
	return 0
}

type retrievalContextGovernanceReportOptions struct {
	retrievalContextPath string
	materializedPath     string
	bundlePath           string
	requestPath          string
	approvalPath         string
	injectionPlanPath    string
	outputFormat         string
}

func parseRetrievalContextGovernanceReportOptions(args []string) (retrievalContextGovernanceReportOptions, error) {
	opts := retrievalContextGovernanceReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--retrieval-context":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --retrieval-context")
			}
			opts.retrievalContextPath = args[i+1]
			i++
		case "--materialized":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --materialized")
			}
			opts.materializedPath = args[i+1]
			i++
		case "--bundle":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--injection-plan":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --injection-plan")
			}
			opts.injectionPlanPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextGovernanceReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.retrievalContextPath == "" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing --retrieval-context")
	}
	if opts.materializedPath == "" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing --materialized")
	}
	if opts.bundlePath == "" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.requestPath == "" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing --request")
	}
	if opts.approvalPath == "" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.injectionPlanPath == "" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("missing --injection-plan")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextGovernanceReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextGovernanceReport(opts retrievalContextGovernanceReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.GovernanceReport(retrievalcontext.GovernanceReportOptions{
		RetrievalContextPath: opts.retrievalContextPath,
		MaterializedPath:     opts.materializedPath,
		BundlePath:           opts.bundlePath,
		RequestPath:          opts.requestPath,
		ApprovalPath:         opts.approvalPath,
		InjectionPlanPath:    opts.injectionPlanPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context governance-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteGovernanceReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteGovernanceReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context governance-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type retrievalContextInjectionPolicyOptions struct {
	policyPath string
}

func parseRetrievalContextInjectionPolicyOptions(args []string) (retrievalContextInjectionPolicyOptions, error) {
	opts := retrievalContextInjectionPolicyOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return retrievalContextInjectionPolicyOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return retrievalContextInjectionPolicyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return retrievalContextInjectionPolicyOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

type retrievalContextInjectionPolicyPlanOptions struct {
	policyPath   string
	outputFormat string
}

func parseRetrievalContextInjectionPolicyPlanOptions(args []string) (retrievalContextInjectionPolicyPlanOptions, error) {
	opts := retrievalContextInjectionPolicyPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return retrievalContextInjectionPolicyPlanOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextInjectionPolicyPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextInjectionPolicyPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.policyPath == "" {
		return retrievalContextInjectionPolicyPlanOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextInjectionPolicyPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextInjectionPolicyValidate(opts retrievalContextInjectionPolicyOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadInjectionPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-policy validate failed: %v\n", err)
		return 1
	}
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-policy validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context injection-policy validate: ok")
	return 0
}

func runRetrievalContextInjectionPolicyPlan(opts retrievalContextInjectionPolicyPlanOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadInjectionPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-policy plan failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.InjectionPolicyPlan(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-policy plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteInjectionPolicyPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteInjectionPolicyPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-policy plan failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type retrievalContextInjectionApprovalNewOptions struct {
	governanceReportPath string
	policyPath           string
	outputPath           string
}

func parseRetrievalContextInjectionApprovalNewOptions(args []string) (retrievalContextInjectionApprovalNewOptions, error) {
	opts := retrievalContextInjectionApprovalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--governance-report":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("missing value for --governance-report")
			}
			opts.governanceReportPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.governanceReportPath == "" {
		return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("missing --governance-report")
	}
	if opts.policyPath == "" {
		return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputPath == "" {
		return retrievalContextInjectionApprovalNewOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runRetrievalContextInjectionApprovalNew(opts retrievalContextInjectionApprovalNewOptions, stdout io.Writer, stderr io.Writer) int {
	request, err := retrievalcontext.NewInjectionApprovalRequest(retrievalcontext.NewInjectionApprovalRequestOptions{
		GovernanceReportPath: opts.governanceReportPath,
		PolicyPath:           opts.policyPath,
		OutputPath:           opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-approval new failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context injection-approval new: ok")
	fmt.Fprintf(stdout, "status: %s\n", request.Status)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "governance_report_sha256: %s\n", request.GovernanceReportSHA256)
	fmt.Fprintf(stdout, "policy_sha256: %s\n", request.PolicySHA256)
	fmt.Fprintf(stdout, "materialized_sha256: %s\n", request.MaterializedSHA256)
	fmt.Fprintf(stdout, "max_total_chars: %d\n", request.MaxTotalChars)
	return 0
}

type retrievalContextInjectionApprovalApproveOptions struct {
	requestPath                 string
	outputPath                  string
	confirmAllowRunnerInjection bool
}

func parseRetrievalContextInjectionApprovalApproveOptions(args []string) (retrievalContextInjectionApprovalApproveOptions, error) {
	opts := retrievalContextInjectionApprovalApproveOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalApproveOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-allow-runner-injection":
			opts.confirmAllowRunnerInjection = true
		default:
			return retrievalContextInjectionApprovalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.requestPath == "" {
		return retrievalContextInjectionApprovalApproveOptions{}, fmt.Errorf("missing --request")
	}
	if opts.outputPath == "" {
		return retrievalContextInjectionApprovalApproveOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmAllowRunnerInjection {
		return retrievalContextInjectionApprovalApproveOptions{}, fmt.Errorf("missing --confirm-allow-runner-injection")
	}
	return opts, nil
}

func runRetrievalContextInjectionApprovalApprove(opts retrievalContextInjectionApprovalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	approval, err := retrievalcontext.ApproveRunnerInjection(retrievalcontext.ApproveRunnerInjectionOptions{
		RequestPath:                 opts.requestPath,
		OutputPath:                  opts.outputPath,
		ConfirmAllowRunnerInjection: opts.confirmAllowRunnerInjection,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-approval approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "retrieval context injection-approval approve: ok")
	fmt.Fprintf(stdout, "approved: %t\n", approval.Approved)
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "allowed_use: %s\n", approval.AllowedUse)
	fmt.Fprintf(stdout, "runner_injection_allowed: %t\n", approval.RunnerInjectionAllowed)
	return 0
}

type retrievalContextInjectionApprovalInspectOptions struct {
	approvalPath string
	requestPath  string
	outputFormat string
}

func parseRetrievalContextInjectionApprovalInspectOptions(args []string) (retrievalContextInjectionApprovalInspectOptions, error) {
	opts := retrievalContextInjectionApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalInspectOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return retrievalContextInjectionApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return retrievalContextInjectionApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return retrievalContextInjectionApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return retrievalContextInjectionApprovalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runRetrievalContextInjectionApprovalInspect(opts retrievalContextInjectionApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectInjectionApproval(opts.approvalPath, retrievalcontext.InspectInjectionApprovalOptions{
		RequestPath: opts.requestPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-approval inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteInspectInjectionApprovalJSON(result, stdout)
	default:
		err = retrievalcontext.WriteInspectInjectionApprovalText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "retrieval context injection-approval inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type artifactListOptions struct {
	storePath string
	runID     string
	status    runs.RunStatus
}

func parseArtifactListOptions(args []string) (artifactListOptions, error) {
	var opts artifactListOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--status":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --status")
			}
			opts.status = runs.RunStatus(args[i+1])
			i++
		default:
			return artifactListOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return artifactListOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

type artifactPruneOptions struct {
	storePath    string
	artifactsDir string
	olderThan    time.Duration
	olderThanRaw string
	dryRun       bool
}

func parseArtifactPruneOptions(args []string) (artifactPruneOptions, error) {
	var opts artifactPruneOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--older-than":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --older-than")
			}
			duration, err := parseRetentionDuration(args[i+1])
			if err != nil {
				return artifactPruneOptions{}, err
			}
			opts.olderThan = duration
			opts.olderThanRaw = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		default:
			return artifactPruneOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return artifactPruneOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return artifactPruneOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.olderThan == 0 {
		return artifactPruneOptions{}, fmt.Errorf("missing --older-than")
	}
	return opts, nil
}

func runArtifactList(opts artifactListOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := db.ListArtifacts(context.Background(), storepkg.ArtifactListFilter{
		RunID:  opts.runID,
		Status: opts.status,
	})
	if err != nil {
		fmt.Fprintf(stderr, "artifact list failed: %v\n", err)
		return 1
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "artifact_id\trun_id\tkind\tpath\tsize_bytes\tsha256\tkeep\tcreated_at")
	for _, artifact := range result {
		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%d\t%s\t%t\t%s\n",
			artifact.ID,
			artifact.RunID,
			artifact.Kind,
			artifact.Path,
			artifact.SizeBytes,
			artifact.SHA256,
			artifact.Keep,
			artifact.CreatedAt.Format(time.RFC3339Nano),
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "artifact list failed: %v\n", err)
		return 1
	}
	return 0
}

func parseRetentionDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("missing --older-than")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid --older-than %q", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --older-than %q", value)
	}
	return duration, nil
}

func runArtifactPrune(opts artifactPruneOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := artifactspkg.Prune(context.Background(), db, artifactspkg.RetentionConfig{
		ArtifactsDir: opts.artifactsDir,
		OlderThan:    opts.olderThan,
		DryRun:       opts.dryRun,
	})
	if err != nil {
		fmt.Fprintf(stderr, "artifact prune failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "dry_run: %t\n", result.DryRun)
	fmt.Fprintf(stdout, "older_than: %s\n", opts.olderThanRaw)
	fmt.Fprintf(stdout, "cutoff: %s\n", result.Cutoff.Format(time.RFC3339Nano))
	fmt.Fprintf(stdout, "candidates: %d\n", result.Candidates)
	fmt.Fprintf(stdout, "deleted: %d\n", result.Deleted)
	fmt.Fprintf(stdout, "skipped: %d\n", result.Skipped)
	fmt.Fprintf(stdout, "bytes: %d\n", result.SizeBytes)
	return 0
}

func captureGitDiff(ctx context.Context, workspace string) ([]byte, error) {
	return runner.CaptureGitDiff(ctx, workspace)
}

func ensureTaskWorker(task *tasks.Task, requested string) error {
	if task.Worker != requested {
		return fmt.Errorf("task worker %q does not match requested worker %q", task.Worker, requested)
	}
	return nil
}
