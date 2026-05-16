# Skills Lifecycle

## Purpose

Skills are operational playbooks that guide agents through recurring work.

They are not just prompts.

A skill should encode:

- when to use it
- what to read first
- how to perform the task
- what to avoid
- what good output looks like
- how to validate the result

## Skill states

```text
draft
  -> active
  -> deprecated
  -> archived
```

## Skill sources

A skill can come from:

- repeated manual workflow
- repeated agent failure
- successful agent run
- domain-specific operation
- new tool integration
- changed memory policy
- changed project direction

## Skill proposal format

```yaml
proposal_id: skill-2026-05-16-001
skill_name: escalasoft-analysis
domain: escalasoft
operation: update
status: proposed
source_run_id: run-123

reason: >
  Agent repeatedly started from raw docs instead of domain index.

evidence:
  - artifacts/run-123/events.jsonl
  - artifacts/run-123/summary.md

expected_change: >
  Force the agent to start from _meta/index.md and only then descend.
```

## Renewal triggers

Renew a skill when:

- a tool changes behavior
- a model frequently fails a workflow
- a domain index changes
- a process becomes stable
- a safer route exists
- a better validation method exists
- user corrections repeat

## Skill validation

Before a skill becomes active:

- it must define scope
- it must define non-goals
- it must define expected output
- it must define safety limits
- it should have at least one example task
- it must not grant broad permissions implicitly

## Skill inventory command

Future command:

```bash
deonctl skills list
deonctl skills lint
deonctl skills propose
deonctl skills approve
deonctl skills renew --domain escalasoft
```

## Relationship with memory

Skills can refer to memory.

Skills must not become memory dumps.

If a skill starts containing too much domain knowledge, move that knowledge into the correct domain memory and leave only the procedure in the skill.

## Relationship with workers

Different workers may use the same skill differently.

DeonClaw should eventually map:

```text
skill + domain + worker -> context pack + tool permissions + validation
```

Example:

```text
escalasoft-analysis + escalasoft + codex
  -> read-only context pack

escalasoft-analysis + escalasoft + opencode
  -> read-only context pack + SQLite query helper

path-policy-implementation + deonclaw + codex
  -> workspace-write + Go tests
```
