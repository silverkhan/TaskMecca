# Task Mecca

**Local-first orchestration and observability for multi-agent coding workflows.**

Task Mecca keeps its orchestration framework inside the project while keeping project-owned task data physically separate from framework files. A Git clone can therefore preserve both the exact Task Mecca operating rules and the project's durable backlog state without depending on a central service.

> Public alpha: `0.2.1`. The project-contained runtime is based on the current Task Mecca v2.17 line, with the public packaging/update layer added around it.

[한국어 README](README.ko.md)

## Quick start

### 0. Install the framework

Task Mecca requires **Git** and the standalone `task-mecca` executable for your operating system. End users do not need Python, `uv`, or the Go toolchain.

Put the downloaded binary on PATH, then initialize the current project:

```bash
task-mecca init
```

Installation creates only Task Mecca framework/metadata. It does **not** create project data, does not create a backlog, and does not modify the project-level `AGENTS.md`.

Installed layout:

```text
_task_mecca/
├── VERSION
├── manifest.json
├── ROOT_PROMPT.md
├── framework/
│   ├── README*.md
│   ├── SESSION_GUIDE*.md
│   ├── collab.md
│   ├── _template.md
│   ├── roles/
│   └── web/
├── .runtime/        # created only when runtime state is needed
└── backups/         # created only when updater backup is needed
```

There is intentionally no `data/` directory immediately after installation.

### 1. Activate one Root session explicitly

Task Mecca installation and Root activation are separate:

```text
Task Mecca installed in the project ≠ this session is Root
```

Choose the single user-facing session that should act as Root and paste the prompt from:

```text
_task_mecca/ROOT_PROMPT.md
```

Only that explicitly activated session acts as `/root`.

### 2. Give Root work

Describe work naturally. Root classifies directly verifiable work as a **Simple Task** and work requiring scope/design/user choices as a **Defined Task**.

On the first registration, Registrar creates the canonical project-owned ledger:

```text
_task_mecca/data/backlog/
```

The installer does not create it. Existing ledgers such as `backlog_b`, `backlog-team`, or other names beginning with `backlog` remain discoverable for compatibility.

### 3. Launch the dashboard

Run the standalone CLI:

```bash
task-mecca web
```

This serves the embedded read-only Web UI locally. It does not require Python, `uv`, or a network connection.

The Web UI is read-only and localhost-only. It can start before the first backlog is created and reports the ledger as uninitialized.

## Framework vs project data

The filesystem boundary is intentional.

| Area | Ownership | Update behavior |
|---|---|---|
| `_task_mecca/framework/**` | Task Mecca framework | Managed by updater |
| `_task_mecca/ROOT_PROMPT.md` | Task Mecca managed/customizable | Backup + confirmation when both local and upstream changed |
| `_task_mecca/data/**` | Project/agent durable data | Never overwritten by updater |
| legacy `_task_mecca/backlog*/**` | Project data compatibility | Never overwritten by updater |
| `_task_mecca/.runtime/**` | Ephemeral runtime state | Not durable project data |
| `_task_mecca/backups/**` | Local updater safety copies | Never framework-managed |

Agents may create additional durable artifacts under `data/` when a task genuinely needs them, for example measurements or audit evidence. Task Mecca does not pre-create or standardize arbitrary artifact folders.

## Backlog convention

New projects use one canonical path:

```text
_task_mecca/data/backlog/
```

Registrar creates it only on first registration. Discovery remains compatible with existing folders whose names begin with `backlog`.

For migrated projects, keeping the legacy basename is allowed:

```text
_task_mecca/backlog_b/
    ↓
_task_mecca/data/backlog_b/
```

Task Mecca includes lifecycle-history compatibility so Git events recorded under the pre-0.2 direct path are also read when the same ledger basename is moved under `data/`.

## Update

Replace the standalone executable with a newer Task Mecca binary, then update the managed project framework:

```bash
task-mecca update
```

Downloading a newer binary requires whatever network/file-transfer method you choose, but normal runtime commands such as Web UI, doctor, preflight, and backlog operations execute entirely from the standalone binary and project files.

If an update would overwrite or retire locally modified managed files, Task Mecca:

1. lists the affected files,
2. recommends a backup,
3. creates `_task_mecca/backups/<timestamp>/` after confirmation,
4. clearly warns what will be overwritten/retired,
5. asks for final confirmation before applying the update.

No force/backup flags are required for the normal flow.

## Dashboard and orchestration features

- automatic backlog discovery and manual switching
- active + `archive/YYYY-MM/` history
- Simple / Defined / legacy backlog rendering
- lifecycle timeline and Queue / Active / Wait / Lead timing
- historical lifecycle compatibility across the 0.2 data-layout migration
- dependency/readiness checks and Controller coordination snapshots
- subagent workload and stale/worker-missing signals
- dispatch-time Full Access preflight
- Light / Dark / System themes
- Korean / English UI and manuals
- Markdown tables, code-copy buttons, Mermaid diagrams
- adaptive page sizing and keyboard navigation

## Requirements

- Git
- standalone `task-mecca` binary for Windows, macOS, or Linux

The installed project runtime has no third-party Python runtime dependencies.

## Development

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
