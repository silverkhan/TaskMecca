# Go runtime migration (in progress)

The Go binary is not a replacement for the current Python distribution yet. This branch starts with the installer and keeps the Python runtime as the behavior reference. Do not distribute its binary as a complete Task Mecca release.

## Compatibility boundary

- Preserve the existing `_task_mecca` layout and manifest schema; never modify `data/` or other project-owned paths on update.
- Preserve CLI commands and exit codes: `init`, `update`, `doctor`, `web`, then the project runtime's `agent`, `worker-name`, `ensure-backlog`, `preflight`, `next-id`, `search`, `inspect`, `ready`, `workload`, `coordinate`, `audit`, `check`, `status`, and `monitor`.
- Preserve JSON fields and ordering where callers consume them. Compare against the Python implementation using the existing fixtures and Git-history tests.
- Preserve the local Web UI routes `/api/backlog-folders`, `/api/snapshot`, `/api/manual`, and `/api/tasks/{id}`, static assets, backlog selection, and read-only behavior.
- Preserve direct worker identity `/root/controller/<worker_name>`, registrar/controller roles, Controller Drain, Parallel Fill, and Adaptive Worker Allocation.
- Only remove the Python runtime and Python usage instructions after the Go command surface and behavior pass parity tests on Windows and macOS.

## Current migration status

- Go module and embedded project template: started.
- `init` and conservative `update`: implemented, pending CI and parity verification.
- Backlog parser, scheduling reports, preflight, lifecycle history, Web UI, terminal UI, documentation and release packages: pending.

## Second checkpoint

- Go backlog discovery, canonical ledger creation, archive-aware catalog, and `next-id` are implemented with initial tests.
- The Go catalog currently parses only basic identity/title/fields. It is not yet suitable for scheduling, audits or the Web UI until the full document and lifecycle model is migrated.
- The `agent` identity validation command now has a Go implementation and a Python/Go JSON parity check in CI.
- `worker-name` now uses active doing assignments and live `--used` reservations. CI compares its JSON with Python on empty and reserved pools.


## Third checkpoint

- The Go backlog record now carries the normalized document model used by the Python runtime: Simple/Defined task schema, sections, requirements, acceptance items, result/verification, raw Markdown, archive month, and runtime metadata fields.
- `search` is implemented in the Go CLI with the existing scoring contract and human/JSON output.
- CI includes a Python/Go JSON parity check for search in addition to agent, worker-name, and archive-aware next-id.
- Next migration layer: dependency/ready reports, then inspect/workload on top of lifecycle, hold-audit, and continuity primitives. Do not duplicate those behaviors with simplified one-off implementations.


## Fourth checkpoint

- Go now implements assignment views, released-hold audit semantics, continuity evidence, durable Git lifecycle reconstruction, and provisional lifecycle observations in `.runtime/lifecycle_observations.json`.
- `inspect` and `workload` are implemented on top of the shared parser/dependency/lifecycle primitives rather than simplified one-off logic.
- `coordinate` now builds a single scheduling snapshot with shared lifecycle timings, scope-conflict detection, parallel-fill accounting, the deterministic Pokémon worker pool, hold review, and the Controller Drain / Parallel Fill / Adaptive Worker Allocation contract fields.
- `audit` is implemented with Python-compatible state-specific ownership and required-field checks.
- CI parity checks cover inspect/workload and coordinate/audit structurally; only live clock-derived duration fields and snapshot timestamps are excluded from exact comparison.
- GitHub Actions visibility for the newest commits is still pending at this checkpoint, so these commands remain migration work until the branch CI actually reports success.
