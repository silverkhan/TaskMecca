# Task Mecca

**Local-first orchestration and observability for multi-agent coding workflows.**

Task Mecca keeps requirements, backlog state, worker allocation, lifecycle timing, and a local web dashboard inside the project repository. It is designed for agent-driven coding workflows where you want the project to remain reproducible from Git instead of depending on a central service.

> Public alpha: `0.1.1`. The project-contained runtime is based on the current Task Mecca v2.17 implementation.

[한국어 README](README.ko.md)

## Quick start

### 0. Install Task Mecca into a project

Until the package is published on PyPI, run it directly from this repository:

```bash
uvx --from git+https://github.com/silverkhan/TaskMecca.git task-mecca init
```

After PyPI publication, the intended shorter command is:

```bash
uvx task-mecca init
```

This installs a self-contained `_task_mecca/` directory into the current project. The project runtime remains local to the project; `uvx` is only the installer/updater layer. Installation does **not** create or modify the project-level `AGENTS.md`.

### 1. Launch the dashboard

```bash
uv run _task_mecca/collab_tools.py web
```

Without `uv`:

```bash
python _task_mecca/collab_tools.py web
```

### 2. Activate the Root session

Task Mecca installation and Root activation are deliberately separate:

```text
Task Mecca installed in the project ≠ this session is Root
```

`task-mecca init` does **not** create or modify `AGENTS.md`. Project-wide instructions would blur the boundary between the single user-facing Root session and other sessions/agents.

Open the session you want to use as Root and paste the appropriate prompt from:

```text
_task_mecca/ROOT_PROMPT.md
```

Only that explicitly activated session should act as `/root`.

### 3. Give Root work

No special syntax is required. Describe the task naturally.

```text
Fix the incorrect command in README.
```

Task Mecca classifies directly verifiable work as a **Simple Task** and work requiring scope/design/user choices as a **Defined Task**. Defined Tasks go through requirement clarification and user confirmation before registration.

## Update

The user-facing update command is intentionally simple:

```bash
uvx --from git+https://github.com/silverkhan/TaskMecca.git task-mecca update
```

After PyPI publication:

```bash
uvx task-mecca update
```

If an update would overwrite locally modified managed files, Task Mecca:

1. shows the modified file list,
2. recommends creating a backup,
3. creates `_task_mecca/backups/<timestamp>/` when you accept,
4. clearly warns that the modified project copies will be overwritten,
5. asks for final confirmation before applying the update.

You do not need to remember force/backup command-line flags.

## File ownership model

Task Mecca makes the update boundary explicit to both the updater and the user.

| Area | Examples | Update behavior |
|---|---|---|
| **Framework managed** | `collab_tools.py`, `runtime_metadata.py`, `web/*` | Updated automatically when upstream changes |
| **Customizable managed** | `roles/*`, `SESSION_GUIDE*.md`, `collab.md`, `_template.md`, local manuals | If both local and upstream changed, Task Mecca requires a backup + confirmation before overwrite |
| **Project owned** | `backlog_*`, archive contents, project config | Never overwritten by the updater |
| **Runtime/backup** | `.runtime/*`, `backups/*` | Local ephemeral/safety data; excluded from Git by default |

If a customizable managed file was changed locally but the new Task Mecca release did **not** change that file upstream, the local version is preserved without prompting.

The installed `_task_mecca/manifest.json` records the installed version and baseline hashes used to detect these cases.

## What the dashboard provides

- automatic backlog-folder discovery and manual switching
- active + `archive/YYYY-MM/` backlog reading
- combined status filters and keyboard navigation
- ID/update sorting and adaptive page sizing
- Simple / Defined / legacy backlog rendering
- lifecycle timeline and Queue / Active / Wait / Lead timing
- subagent workload and stale/worker-missing signals
- Full Access preflight status and dispatch-time verification
- Light / Dark / System themes
- Korean / English UI and manuals
- Markdown tables, code-copy buttons, and Mermaid diagrams
- local-only, read-only web UI

## Repository layout

```text
TaskMecca/
├── src/task_mecca/              # uvx bootstrap package
│   ├── cli.py                   # init / update / doctor
│   ├── installer.py             # manifest + safe update engine
│   └── template/_task_mecca/    # project-contained runtime template
├── tests/                       # regression tests
├── demo/                        # example backlog data
└── .github/workflows/ci.yml     # Python 3.11–3.13 CI
```

Installed into another project:

```text
my-project/
├── AGENTS.md
├── source...
└── _task_mecca/
    ├── VERSION
    ├── manifest.json
    ├── collab_tools.py
    ├── roles/
    ├── web/
    ├── backlog_*/
    └── .runtime/
```

## Requirements

- Python 3.11+
- Git
- `uv` is recommended for installation and launch convenience, but the project-contained dashboard can also run with Python directly.

The installed runtime has no third-party Python runtime dependencies.

## Development

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT. See [LICENSE](LICENSE).
