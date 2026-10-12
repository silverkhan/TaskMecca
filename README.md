# Task Mecca

**Manage AI-agent work in a persistent backlog and monitor execution and results from the Web UI.**  
Talk to one Root in natural language; Registrar, Controller, and Workers handle registration, execution, and verification.

[한국어](README.ko.md) · [User manual](goassets/template/_task_mecca/framework/README.en.md)

> [!IMPORTANT]
> **Project origin & acknowledgements**  
> Task Mecca began with **initial source code and a backlog-driven collaboration idea** shared by my colleague [gyusu](https://github.com/gyusu). [Read more →](#project-origin)

## Quick start

### 1. Install

**macOS / Linux**
```bash
curl -fsSL https://raw.githubusercontent.com/silverkhan/TaskMecca/main/install.sh | sh
```

**Windows PowerShell**
```powershell
irm https://raw.githubusercontent.com/silverkhan/TaskMecca/main/install.ps1 | iex
```

### 2. Start the Web UI

```bash
task-mecca web
```

Open **http://127.0.0.1:18765/** in your browser. Initialize a working project in step 3.

The Web UI brings together **backlog progress**, **Root/Worker sessions and recent activity**, and **execution stages and actionable warnings**. You can configure project-level **Telegram and browser notifications**, manage projects, and update Task Mecca.

**Web service commands**

| Command | Purpose |
| --- | --- |
| `task-mecca web` | Start Web UI and open browser |
| `task-mecca web status` | Check service status, version and URLs |
| `task-mecca web restart` | Restart Web service |
| `task-mecca web stop` | Stop Web service |
| `task-mecca web logs` | View logs (`--follow` for live output) |

![Task Mecca Web UI — English project backlog list](handoff-evidence/a24-raichyu/final-desktop-dark-en.jpg)

<sub>English · Dark theme · Backlog list for a sample project (Needs attention: 0). Actual capture from an earlier development build; the current UI may differ.</sub>

### 3. Initialize a project

Run this in the **folder you want to manage**.

```bash
cd /path/to/your/project
task-mecca init
```

Key installed files:

```text
project/
└── _task_mecca/
    ├── ROOT_PROMPT.md           # Root startup prompt
    └── framework/
        ├── SESSION_GUIDE.md     # Session guide
        ├── EXECUTION_PROTOCOL.md # Execution and handoff rules
        ├── collab.md            # Collaboration rules
        └── roles/               # Root, Registrar, Controller, Worker
```

> [!NOTE]
> **The task backlog (`data/backlog/`) is created when the first task is registered, not by initialization.** For an existing installation, use migration in step 5 instead of rerunning `init`.

### 4. Activate your Root

Open Codex or Claude Code **in the same project folder**. Designate **one user-facing session** as Root, paste the prompt below, and describe work naturally.

<details>
<summary><strong>Expand & copy Root prompt</strong></summary>

```text
Act as the Task Mecca Root for this session.
First read _task_mecca/framework/SESSION_GUIDE.md, _task_mecca/framework/collab.md, and _task_mecca/framework/roles/root.md and follow those rules.
For all user-facing conversation and backlog prose—titles, summaries, requirements, status, and results—write natively in the user's primary language and vocabulary. Keep exact English identifiers, commands, paths, or technical terms when they improve precision, but do not mechanically leak internal English modifiers or word order into another language; restate the whole expression naturally in the user's language.
When work originates from a connected external item such as a Linear or GitHub issue, read that source and preserve a clickable source reference in the backlog. Root writes back important decisions confirmed before registration or during active user discussion. After registration, Controller is the single operational writer for real lifecycle transitions and completion results through the connected capability. Workers do not edit the external issue directly; they report implementation and verification evidence to Controller. If the external source changes in a way that conflicts with the current Task Mecca contract and requires user judgment, Controller must not merge it silently: put the task on hold, record the difference and resume condition in follow-up, and let the next Root interaction reconcile it with the user.
Before delegating executable work to subagents, run the Full Access preflight described in SESSION_GUIDE.
Classify work as Simple Task or Defined Task. For a simple task that is directly verifiable without additional interpretation, record only the goal and acceptance criteria and register it immediately. For work requiring scope, design, or user choices, prepare a requirement definition, get my confirmation, and then ask Registrar to register it losslessly.
If no backlog exists yet, Registrar creates _task_mecca/data/backlog on the first registration. Everything under data is project-owned and is never overwritten by the framework updater.
Before sending execution-authorized work to Registrar, prepare an exact reusable Controller runtime identity and pass it to Registrar so successful registration can hand off directly to Controller without another Root turn. Workers must likewise hand DONE/BLOCKED events directly back to Controller, and any resume that starts a completed agent's new turn requires a fresh Full Access preflight.
After registration, let Controller allocate work to Workers based on dependency and continuity, while Root remains the user-facing interface.
```

</details>

Example: `Fix the login API error and verify the relevant tests.`

### 5. Migrate an existing project

Use migration **after a Task Mecca update when the project's installed operating instructions need refreshing**. The Web UI or an `init` attempt in an existing project will guide you when migration is needed.

```bash
cd /path/to/your/project
task-mecca migrate
```

- **Updated:** Task Mecca-managed instructions and framework files
- **Preserved:** Your backlog, work history, configuration, notifications, and execution data
- **Customized files:** Choose overwrite / back up changes and continue / cancel

Use `task-mecca upgrade` to **update the application**, and `task-mecca migrate` to **update a project's instructions**.

## How it works

```mermaid
%%{init: {"theme": "base", "themeVariables": {"fontFamily": "Inter, sans-serif", "primaryColor": "#241b39", "primaryTextColor": "#ffffff", "primaryBorderColor": "#9b79d2", "lineColor": "#9477c6", "background": "#141426"}}}%%
flowchart LR
    U(["You<br/>Natural-language request"]):::person --> R["Root<br/>Discuss & define"]:::root
    R --> G["Registrar<br/>Register work"]:::agent
    G --> B[("Markdown backlog<br/>Source & history")]:::record
    B --> C["Controller<br/>Assign · verify · finalize"]:::controller
    C <--> W["Workers<br/>Read source & execute"]:::worker
    C --> B
    B --> V["Web UI<br/>Sessions · status · alerts"]:::web
    R -. Decisions .-> E["Linear · GitHub<br/>Source issues"]:::external
    C -. Results .-> E

    classDef person fill:#FFF0C2,stroke:#E3AC3B,color:#43300A,stroke-width:2px
    classDef root fill:#EADFFF,stroke:#9A66D2,color:#48216C,stroke-width:2px
    classDef agent fill:#F2EAFE,stroke:#B49AE6,color:#45336A,stroke-width:2px
    classDef controller fill:#56417D,stroke:#A78BFA,color:#FFFFFF,stroke-width:2px
    classDef worker fill:#DAF5EF,stroke:#56BDA7,color:#165C50,stroke-width:2px
    classDef record fill:#DFEAFD,stroke:#719DE5,color:#233D72,stroke-width:2px
    classDef web fill:#241B3B,stroke:#D6AEFF,color:#FFFFFF,stroke-width:2px
    classDef external fill:#F7E7F2,stroke:#BD78AE,color:#733B62,stroke-width:2px
```

## Version & updates

| Command | Purpose |
| --- | --- |
| `task-mecca --version` | Installed version, channel and latest release |
| `task-mecca upgrade` | Update from the selected channel |
| `task-mecca channel` | Show update channel |
| `task-mecca channel stable` | Select stable releases |
| `task-mecca channel dev` | Select development releases |
| `task-mecca migrate` | Refresh an existing project's operating instructions (step 5) |

For more details, see the Web **User Manual**, [session guide](goassets/template/_task_mecca/framework/SESSION_GUIDE.en.md) and [collaboration rules](goassets/template/_task_mecca/framework/collab.md).

<a id="project-origin"></a>

## Project origin & acknowledgements

Task Mecca grew from an **idea and usable initial source code** shared by my colleague [gyusu](https://github.com/gyusu): separate backlog registration and execution sessions, and let Workers continue through remaining tasks. The idea evolved into today's Root, Registrar, Controller, and Worker collaboration model.

Many thanks to [gyusu](https://github.com/gyusu) for sharing the inspiration and source material that started this project.

## Development & license

See [CONTRIBUTING.md](CONTRIBUTING.md) · [MIT License](LICENSE)

## Complete CLI command reference

Common commands are covered above. This is the complete list of CLI command families and subcommands; `|` separates alternatives.

### Start & update

| Command | Purpose |
| --- | --- |
| `task-mecca` | Start Web UI (default) |
| `task-mecca web [start\|status\|restart\|stop\|logs]` | Start, inspect, restart, stop Web or read logs |
| `task-mecca --version` (or `version`) | Installed/latest version and channel |
| `task-mecca upgrade` | Update the application |
| `task-mecca channel [stable\|dev]` | View or switch update channel |
| `task-mecca init` | Initialize a new project |
| `task-mecca migrate` | Refresh an existing project's framework |

### Backlog & project management

| Command | Purpose |
| --- | --- |
| `task-mecca status` | Backlog progress |
| `task-mecca ready` | Ready tasks |
| `task-mecca search <query>` | Search the backlog |
| `task-mecca inspect <ID>` | Task details |
| `task-mecca workload` | Agent workload |
| `task-mecca coordinate` | Scheduling snapshot |
| `task-mecca projects list\|archived\|add\|register\|unregister\|archive\|restore\|forget` | Register, monitor and archive projects |
| `task-mecca tags list\|search\|discover\|show\|resolve\|tasks\|query\|stats\|catalog\|assign\|remove\|set\|define\|rename\|merge\|retire\|rebuild` | Search and manage tags |
| `task-mecca task tags\|tag-add\|tag-remove\|tag-set` | Manage task tags |
| `task-mecca audit` | Required backlog fields |
| `task-mecca check` | Documentation links and dependencies |
| `task-mecca doctor` | Project diagnostics |

### Agents & execution records (advanced)

| Command | Purpose |
| --- | --- |
| `task-mecca monitor` | Web-backed monitoring (`--once` for one snapshot) |
| `task-mecca preflight` | Check permissions and execution readiness |
| `task-mecca worker-name` | Select available Worker identity |
| `task-mecca agent <path>` | Validate Agent path |
| `task-mecca ensure-backlog` | Check or create backlog |
| `task-mecca next-id <prefix>` | Allocate a task ID |
| `task-mecca lifecycle list\|record` | Inspect or record lifecycle events |
| `task-mecca runtime list\|reconcile\|observe\|enrich-name\|assign\|bind\|bind-assignment\|bind-agent\|hooks` | Observe and bind runtimes, manage Hooks |
| `task-mecca runtime-spike observe\|report` | Runtime diagnostic reports |
| `task-mecca handoff prepare\|claim\|mark\|inspect\|reconcile\|capability` | Manage Agent handoff evidence |
| `task-mecca deliveries [task ID] [event ID]` | Notification delivery history |
| `task-mecca notifications resume-status\|suppress-before-resume` | Telegram resumption controls |
| `task-mecca operations reconcile` | Reconcile execution warnings with completion evidence |

> [!NOTE]
> `|` means **choose one subcommand**, not a literal pipe. Some advanced commands require additional parameters and permissions; use them through your Root/Controller under the established operating rules.

