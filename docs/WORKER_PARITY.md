# Worker Parity

This document tracks operational parity between the implemented workers.

It is a snapshot of the current MVP contract, not a provider roadmap.

## Status Legend

- yes: implemented and covered by the shared runner or worker adapter
- partial: implemented with worker-specific behavior or remaining caveat
- no: not implemented
- inspiration_only: future reference only, with no operational config example, doctor entry, smoke path, or worker adapter

## Codex vs OpenCode

| Capability | Codex | OpenCode | Notes |
| --- | --- | --- | --- |
| dry-run | yes | yes | Both expose deonctl worker WORKER dry-run and validate task worker matching. |
| run | yes | yes | Both run through the shared runner. OpenCode uses the OpenCode adapter. |
| isolated workspace | yes | yes | Shared runner prepares an isolated workspace before worker execution. |
| dirty baseline | yes | yes | Shared runner refuses runs when the source workspace is dirty. |
| context pack | yes | yes | Both accept --domains and preserve context-pack.md when provided. |
| memory policy | yes | yes | Both accept --memory-policy and lint memory-proposal.json when present. |
| artifacts | yes | yes | Both persist standard run artifacts and artifact metadata in SQLite. |
| validation | yes | yes | Shared runner applies task validation commands after successful worker execution. |
| path policy | yes | yes | Shared runner evaluates changed paths and can mark policy_failed. |
| cleanup | yes | yes | succeeded removes workspace; failed and policy_failed keep workspace. |
| event parsing | partial | yes | Codex expects JSONL from codex exec --json. OpenCode accepts jsonl, mixed, text, and empty stdout. |
| workers-config | yes | yes | Both support --workers-config and fallback commands. |
| model profiles | yes | yes | Both can resolve task model_profile entries from workers.yaml, validate worker/profile matching, and apply profile env requirements before run. The real command is unchanged for now. |
| doctor | yes | yes | deonctl workers doctor can check configured commands for both workers. |
| parity smoke | no | yes | deonctl workers smoke currently supports OpenCode only and runs task validation, worker doctor, env requirement validation, and dry-run/run dispatch. |

## OpenCode Event Parsing Parity

OpenCode now has hardened stdout handling:

| Stdout case | Artifact | Events | Run failure |
| --- | --- | --- | --- |
| JSONL only | stdout.jsonl | JSON lines become WorkerEvent entries | no |
| mixed JSON and text | stdout.log | JSON lines become events; parse warning event is added | no |
| text only | stdout.log | worker.stdout.log event | no |
| empty | stdout.log | no stdout event | no |
| invalid JSON only | stdout.log | worker.stdout.log event | no |
| large payload | raw stdout artifact preserved | event payload truncated safely | no |

stderr.log is always preserved for OpenCode, even when stderr is empty.

OpenCode summary lines include:

- OpenCode stdout format
- OpenCode parsed events
- OpenCode parse warnings

## Codex Event Parsing Caveat

Codex is still treated as a JSONL-first worker because the command contract is codex exec --json.

Invalid Codex JSONL can fail parsing. This is intentional for now because Codex has a stricter event contract than OpenCode.

## Future Inspiration

Kimi was used as inspiration for UX, MCP, and ACP ideas. It is not an implemented DeonClaw worker and has no operational contract.

Current status:

- worker adapter: no
- run command: no
- dry-run command: no
- workers.yaml command entry: no example and no operational fallback
- doctor command check: no, and no default diagnostic worker entry
- smoke participation: no
- workers-config operational entry: no
- status label: inspiration_only

Future inspiration references must stay in this section only. Kimi must remain out of operational config, doctor output, smoke commands, and worker fallback lists until a dedicated worker adapter is explicitly implemented.

## Current Shared Runner Guarantees

Codex and OpenCode share:

- task load and validation
- worker mismatch rejection
- context pack generation
- memory proposal lint wiring
- run persistence
- event persistence
- artifact persistence
- isolated workspace preparation
- dirty baseline protection
- validation command execution
- git diff capture
- changed-files capture
- path policy evaluation
- cleanup behavior
- summary generation

Worker adapters should stay small. New worker-specific behavior should return normalized events, artifacts, stderr, and metadata without duplicating the runner lifecycle.
