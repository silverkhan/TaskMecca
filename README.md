# Task Mecca

**Local-first orchestration and observability for multi-agent coding workflows.**

Task Mecca keeps its orchestration framework inside the project while keeping project-owned task data physically separate from framework files. A Git clone can therefore preserve both the exact Task Mecca operating rules and the project's durable backlog state without depending on a central service.

> Public alpha: `0.2.1`. The project-contained runtime is based on the current Task Mecca v2.17 line, with the public packaging/update layer added around it.

[한국어 README](README.ko.md)

## Quick start

### 0. Install the framework

Task Mecca requires **Python 3.11+ and Git**. `uv` is optional.

Before PyPI publication, the primary install path is standard Python packaging:

```bash
python -m pip install "git+https://github.com/silverkhan/TaskMecca.git"
python -m task_mecca init
```

After PyPI publication:

```bash
python -m pip install task-mecca
python -m task_mecca init
```

Installation creates only Task Mecca framework/metadata. It does **not** create project data, does not create a backlog, and does not modify the project-level `AGENTS.md`.

If `uv` is already installed, `uvx` is an optional convenience:

```bash
uvx --from git+https://github.com/silverkhan/TaskMecca.git task-mecca init
```

Task Mecca does not install or require `uv`.

Installed layout:

```text
_task_mecca/
├── VERSION
├── manifest.json
├── ROOT_PROMPT.md
├── framework/
│   ├── collab_tools.py
│   ├── runtime_metadata.py
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

Run the **installed project-local runtime directly**:

```bash
python _task_mecca/framework/collab_tools.py web
```

This does not contact GitHub, does not require `uv`, and does not depend on the bootstrap package remaining installed in the Python environment.

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

Before PyPI publication:

```bash
python -m pip install --upgrade "git+https://github.com/silverkhan/TaskMecca.git"
python -m task_mecca update
```

After PyPI publication:

```bash
python -m pip install --upgrade task-mecca
python -m task_mecca update
```

If `uv` is already installed, the equivalent optional convenience command is:

```bash
uvx --refresh --from git+https://github.com/silverkhan/TaskMecca.git task-mecca update
```

Network access is needed for **install/update** because a new Task Mecca package must be retrieved. Normal project runtime commands such as Web UI, doctor, preflight, and backlog operations run from `_task_mecca/framework/` and do not require `uv` or package download.

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

- Python 3.11+
- Git
- `pip` for the standard bootstrap package installation path
- `uv` / `uvx`: optional convenience only

The installed project runtime has no third-party Python runtime dependencies.

## Development

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
