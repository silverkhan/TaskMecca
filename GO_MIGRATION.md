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
