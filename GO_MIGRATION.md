# Go runtime migration (in progress)

This branch migrates Task Mecca to a standalone Go runtime. The Python implementation remains in `src/` as the behavior/parity reference during the migration, but the embedded project install template is now Python-free. Keep the PR Draft until cross-platform artifact and parity verification is complete.

## Compatibility boundary

- Preserve the existing `_task_mecca` layout and manifest schema; never modify `data/` or other project-owned paths on update.
- Preserve CLI commands and exit codes: `init`, `update`, `doctor`, `web`, then the project runtime's `agent`, `worker-name`, `ensure-backlog`, `preflight`, `next-id`, `search`, `inspect`, `ready`, `workload`, `coordinate`, `audit`, `check`, `status`, and `monitor`.
- Preserve JSON fields and ordering where callers consume them. Compare against the Python implementation using the existing fixtures and Git-history tests.
- Preserve the local Web UI routes `/api/backlog-folders`, `/api/snapshot`, `/api/manual`, and `/api/tasks/{id}`, static assets, backlog selection, and read-only behavior.
- Preserve direct worker identity `/root/controller/<worker_name>`, registrar/controller roles, Controller Drain, Parallel Fill, and Adaptive Worker Allocation.
- Installed projects must remain Python-free; keep the Python reference implementation only in the repository until final parity sign-off.

## Current migration status

- Go module and Python-free embedded project template: implemented.
- `init` and conservative `update`: implemented.
- Backlog parser, scheduling/health reports, effective preflight, lifecycle history, monitor, and local Web UI: implemented.
- Standalone Windows/macOS/Linux artifact workflow: implemented.
- Remaining work is focused on full parity sign-off, update-conflict UX review, packaging/release polish, and direct Windows/macOS behavior verification.

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


## Fifth checkpoint

- Go now implements `check`, `doctor`, `status`, and `preflight`.
- `doctor` reuses the shared dependency, audit, filename, agent-path, scope-conflict, contract, hold-review, and runtime-metadata checks and remains scoped to exactly one selected backlog ledger.
- `status` reuses the same lifecycle snapshot and assignment semantics as `inspect`, `workload`, and `coordinate`, including optional completed-history display.
- `preflight` performs effective workspace, Git-metadata, subprocess, and outside-workspace write probes; current sandbox markers can still force restricted status, and network restriction is reported independently.
- CI definitions now include Python/Go parity coverage for check/doctor/status/preflight in addition to the earlier command surface. Clock-derived status/lifecycle fields and probe timestamps are excluded from exact structural comparison.
- GitHub has still not exposed a workflow run for the newest Contents-API commits on this Draft PR. Keep the migration Draft and treat CI success as pending until an actual run is visible.


## Sixth checkpoint

- The local read-only Web UI is now served directly by Go from embedded HTML/CSS/JS assets.
- Go implements the existing HTTP routes: `/api/backlog-folders`, `/api/snapshot`, `/api/manual`, `/api/tasks/{id}`, static assets, and SPA fallback.
- The dashboard snapshot includes backlog selection, tree rows, task details, lifecycle timings, runtime activity signals, health, workload, hold review, passive access observation, attention items, and counts.
- `web`, `monitor`, and `status --watch` are wired to the Go runtime; invoking `task-mecca` with no arguments opens the Web UI, matching the current human-facing behavior.
- The embedded install template no longer contains `collab_tools.py` or `runtime_metadata.py`; installer tests assert that a fresh standalone install contains no `.py` runtime files.
- Framework and top-level documentation now use `task-mecca ...` commands and describe the standalone binary distribution.
- A dedicated Actions workflow builds downloadable Windows amd64, macOS amd64/arm64, and Linux amd64 binaries with SHA-256 sums.
- The first standalone-artifact run reached and passed `go test ./...`; cross-platform builds were still running when this checkpoint was written.

### Direct test flow

After downloading the artifact for your platform:

```bash
task-mecca --version
task-mecca init
task-mecca doctor --json
task-mecca preflight --json
task-mecca web
```

For an existing Task Mecca project, back it up or use a disposable clone first, replace the executable, then run:

```bash
task-mecca update
task-mecca doctor --json
task-mecca status --json
task-mecca web
```
