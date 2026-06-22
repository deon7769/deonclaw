# OpenClaw Migration Plan

## Purpose

This document defines how DeonClaw will migrate selected OpenClaw assets and workflows into its own stable personal harness without forcing a risky cutover.

The migration is incremental and reversible. DeonClaw should first inventory and shadow OpenClaw behavior, then import passive assets, then migrate agents and sessions, and only later take over cron/heartbeat execution.

## Goals

- Inventory OpenClaw agents, workspaces, skills, memory, cron jobs, heartbeat config, channels, tools, and permissions.
- Convert OpenClaw skills into DeonClaw's universal `SKILL.md` registry.
- Convert OpenClaw heartbeat and cron behavior into DeonClaw proactive runtime policies.
- Preserve source provenance and rollback paths.
- Avoid copying secrets or tokens.
- Support shadow mode before cutover.
- Allow OpenClaw and DeonClaw to run side by side during migration.

## Non-goals

- No immediate replacement of all OpenClaw channels.
- No direct mutation of OpenClaw state during inspect/plan.
- No migration of secret values.
- No automatic cutover.
- No destructive cleanup until rollback has been validated.

## Migration phases

```text
1. inspect
2. plan
3. passive import
4. shadow mode
5. cutover
6. verify
7. rollback if needed
```

## CLI commands

```bash
deonctl migrate openclaw inspect --home ~/.openclaw --output openclaw-inventory.json
deonctl migrate openclaw plan --inventory openclaw-inventory.json --output openclaw-migration-plan.json
deonctl migrate openclaw apply --plan openclaw-migration-plan.json --mode passive
deonctl migrate openclaw shadow --plan openclaw-migration-plan.json
deonctl migrate openclaw cutover --plan openclaw-migration-plan.json --approval <approval.json>
deonctl migrate openclaw verify --plan openclaw-migration-plan.json
deonctl migrate openclaw rollback --plan openclaw-migration-plan.json --approval <approval.json>
```

All commands must emit machine-readable artifacts and human-readable summaries.

## Inventory scope

The inventory command collects metadata only.

### Agents

```json
{
  "agent_id": "ops",
  "workspace": "/path/to/workspace",
  "model": "...",
  "skills": ["github", "browser-automation"],
  "heartbeat": {"every": "30m", "target": "last"},
  "channels": ["telegram", "slack"],
  "permissions": {}
}
```

### Skills

Fields:

- name;
- source root;
- resolved path;
- hash;
- scope;
- frontmatter;
- scripts/assets present;
- plugin/bundled/managed/local origin;
- install source when known;
- security warnings.

### Memory and instructions

Inventory:

- workspace instructions;
- `HEARTBEAT.md` files;
- standing orders;
- agent context files;
- memory-like Markdown files;
- references to external providers.

Do not import memory directly into canonical DeonClaw memory. Create memory proposals.

### Schedules and heartbeat

Inventory:

- cron jobs;
- one-shot reminders;
- interval jobs;
- webhook triggers;
- heartbeat per agent;
- active hours;
- delivery target;
- skip/defer policies;
- session style.

### Channels and secrets

Inventory names only:

- channel type;
- account IDs;
- configured target references;
- secret variable names;
- whether a secret appears configured.

Never read or store secret values.

## Migration plan artifact

```json
{
  "migration_plan_id": "mig_...",
  "created_at": "...",
  "inventory_sha256": "...",
  "summary": {
    "agents": 3,
    "skills": 12,
    "cron_jobs": 4,
    "heartbeats": 2,
    "memory_proposals": 8
  },
  "actions": [
    {
      "type": "skill_import",
      "source": "~/.openclaw/skills/github",
      "target": "managed:github",
      "requires_approval": true
    },
    {
      "type": "agent_create",
      "source_agent": "ops",
      "target_agent": "ops",
      "requires_approval": true
    },
    {
      "type": "schedule_shadow",
      "source_schedule": "daily-report",
      "target_schedule": "daily-report",
      "mode": "shadow"
    }
  ],
  "blocked_actions": [],
  "warnings": []
}
```

## Passive import

Passive import is safe to run before any runtime cutover.

Allowed:

- install skills into DeonClaw registry as inactive or pending approval;
- create memory proposals;
- create agent configs as paused;
- create schedule configs as disabled/shadow;
- create compatibility reports;
- create migration lockfile.

Not allowed:

- disable OpenClaw jobs;
- start DeonClaw scheduled execution;
- copy secret values;
- auto-enable migrated skills;
- mutate OpenClaw config.

## Shadow mode

Shadow mode compares what DeonClaw would do with what OpenClaw is still doing.

Examples:

- cron job due at the same time but DeonClaw does not execute;
- heartbeat prompt planned but not sent;
- schedule result predicted but not delivered;
- skills snapshot materialized but not used by a live run.

Shadow report fields:

```json
{
  "shadow_ready": true,
  "source": "openclaw",
  "deonclaw_would_run": true,
  "openclaw_current_owner": true,
  "cutover_allowed_now": false,
  "mismatches": [],
  "warnings": []
}
```

## Cutover

Cutover requires explicit approval.

Cutover may:

- enable DeonClaw schedule ownership;
- mark OpenClaw schedule as externally owned if supported;
- enable DeonClaw agent heartbeat;
- enable skill snapshot usage;
- mark DeonClaw as canonical for a subset of agents.

Cutover must not:

- delete OpenClaw state;
- delete OpenClaw skills;
- copy secret values;
- disable rollback;
- enable unreviewed provider/network actions.

## Rollback

Rollback should restore ownership to OpenClaw or disable DeonClaw ownership.

Rollback artifact:

```json
{
  "rollback_ready": true,
  "actions": [
    "disable_deonclaw_schedule:daily-report",
    "pause_agent:ops",
    "restore_openclaw_owner:daily-report"
  ],
  "requires_approval": true
}
```

## Mapping rules

### Skills

OpenClaw skill roots map to DeonClaw managed or workspace skills.

```text
OpenClaw workspace skills -> DeonClaw workspace `.agents/skills`
OpenClaw managed skills   -> DeonClaw managed registry
OpenClaw plugin skills    -> DeonClaw imported-plugin source, disabled by default
```

### Agents

```text
OpenClaw agent id     -> DeonClaw agent id
workspace             -> agent workspace policy
skills allowlist      -> DeonClaw skill policy
heartbeat             -> heartbeat policy
model/provider config -> worker/model profile binding
```

### Heartbeat

```text
OpenClaw heartbeat.every          -> DeonClaw heartbeat policy every
OpenClaw target/delivery          -> DeonClaw delivery policy
OpenClaw active hours             -> DeonClaw active hours
OpenClaw skipWhenBusy             -> DeonClaw skip_when_busy
HEARTBEAT.md                      -> heartbeat instruction / skill snapshot
```

### Cron

```text
OpenClaw cron at/every/cron -> DeonClaw schedule
OpenClaw isolated session   -> DeonClaw cron session kind
OpenClaw delivery target    -> DeonClaw delivery policy
OpenClaw run history        -> imported audit references, not active state
```

## Required prerequisites

Passive inventory can start immediately.

Active cutover requires:

- Universal Skill Registry implemented;
- Persistent Agents implemented;
- Proactive Runtime implemented;
- Work Queue and Leases implemented;
- Budgets and Costs implemented or explicitly disabled by policy;
- rollback tested.

## Safety checks

- No secret values in inventory or plan.
- No path escapes from OpenClaw home/workspace.
- No symlink target outside trusted roots.
- No imported skill active without approval.
- No schedule enabled in DeonClaw while OpenClaw is still owner unless shadow mode.
- No channel delivery enabled during shadow.
- No irreversible mutation during cutover.

## Tests

Minimum tests:

- inventory reads sample OpenClaw home;
- inventory redacts secret-like values;
- plan maps skills, agents, heartbeat, and cron;
- passive apply installs inactive skills only;
- memory import creates proposals only;
- shadow mode produces no worker run;
- cutover requires approval;
- rollback disables DeonClaw schedule ownership;
- symlink escape is rejected;
- duplicate skill names generate deterministic precedence warning.

## Implementation sequence

### 23.20 — Inventory

- OpenClaw home scanner;
- skills, heartbeat, cron, agents, memory metadata;
- no mutation;
- inventory report.

### 23.21 — Plan and passive import

- migration plan;
- inactive skill import;
- memory proposals;
- paused agent configs;
- disabled schedule configs.

### 23.22 — Shadow mode

- compare due schedules;
- heartbeat plan without execution;
- shadow reports;
- mismatch diagnostics.

### 23.23 — Cutover and rollback

- approval flow;
- enable DeonClaw ownership for selected agents/schedules;
- rollback artifact and execution;
- end-to-end migration fixture.

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` status;
- `docs/EPIC_23_ROADMAP.md`;
- `docs/UNIVERSAL_SKILLS.md` for skill import behavior;
- `docs/PERSISTENT_AGENTS.md` for agent mapping;
- `docs/PROACTIVE_RUNTIME.md` for schedule/heartbeat mapping;
- examples under `configs/examples/openclaw-migration/`;
- migration checklist documentation.