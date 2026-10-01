# Task Mecca

**Local-first orchestration and observability for multi-agent work, built around a durable backlog and a single user-facing Root.**

Task Mecca is not just a way to run several agents at once. Its goal is to let a user describe, discuss, and follow work in their normal language and domain context without having to mentally translate internal agent mechanics.

A user-facing **Root** owns the conversation and task definition. Registration, orchestration, and implementation are separated into distinct roles, while task contracts, lifecycle transitions, results, and verification remain durable inside the project.

[한국어 README](README.ko.md)

---

## What problem does Task Mecca solve?

Long-running agent workflows tend to develop the same failure modes:

- task definitions and decisions live only in chat history;
- multiple agents make ownership and blocked work hard to see;
- requirements are compressed or altered as they are relayed between agents;
- “working” and “done” are conversational labels rather than durable lifecycle facts;
- work that started in Linear or GitHub Issues becomes disconnected from the agent execution layer;
- internal agent terminology leaks into user-facing language.

Task Mecca addresses these problems with a durable collaboration model:

1. **Root** is the single user-facing conversation owner.
2. Work is defined as a **Simple Task** or **Defined Task**.
3. **Registrar** writes the confirmed contract to the durable backlog without changing its meaning.
4. **Controller** coordinates dependencies, continuity, capacity, and workers.
5. **Workers** read the canonical backlog contract directly instead of relying on a relay summary.
6. State transitions, results, and verification are preserved in Markdown + Git lifecycle evidence.
7. The Web UI provides backlog, lifecycle, workload, and attention observability.
8. Linear / GitHub Issue work can remain linked to its external source and receive meaningful status/result updates.

---

## Architecture

```text
User
  ↕
/root                         user-facing · task definition owner
  ├─ /root/registrar         lossless registration
  └─ /root/controller        scheduling / orchestration
       ├─ /root/controller/<worker>
       ├─ /root/controller/<worker>
       └─ ...                bounded implementation workers

                    ↓
       Markdown backlog + Git lifecycle
                    ↓
            Task Mecca Web UI
```

Task Mecca does not replace the underlying agent runtime. Agent spawning, messaging, and waiting are provided by the current runtime such as Codex or Claude. Task Mecca provides the **task contract, durable state, scheduling context, and observability surface**.

### Roles

| Role | Responsibility |
|---|---|
| **Root** | User conversation, task definition, Simple/Defined classification, user decisions, external-source synchronization |
| **Registrar** | Lossless registration of the confirmed contract |
| **Controller** | Readiness, dependencies, worker allocation, parallelism, continuity/recovery, completion verification |
| **Worker** | Bounded implementation based on the canonical backlog contract |

The installed `_task_mecca/framework/roles/` documents are the authoritative operating contract for the current project.

---

## Task contracts: Simple vs Defined

Task Mecca separates work by **whether it can be executed and verified without additional interpretation**, not by raw size.

### Simple Task

Use when the goal and acceptance criteria are already clear.

```markdown
## Task Definition
### Goal
### Acceptance Criteria
```

No unnecessary requirement-definition round trip is added.

### Defined Task

Use when scope, design, product behavior, user choices, or multiple acceptance criteria need to be agreed first.

```markdown
## Requirements
### Background & Problem
### Goal
### Requirements
### Scope
#### In
#### Out
### Acceptance Criteria
### Constraints & Preservation
```

Root clarifies the work with the user, obtains confirmation, then Registrar records the contract.

---

## Durable backlog

The canonical backlog for a new project is:

```text
_task_mecca/data/backlog/
```

The installer intentionally does not create empty project data. Registrar creates the ledger when the first task is registered.

```text
_task_mecca/
├── VERSION
├── manifest.json
├── ROOT_PROMPT.md
├── framework/
│   ├── SESSION_GUIDE.md
│   ├── collab.md
│   ├── HUMAN_READABLE_BACKLOG.md
│   ├── TAGS.md
│   ├── roles/
│   └── web/
├── data/
│   ├── backlog/
│   │   ├── 000143.A-143.example.todo.md
│   │   └── archive/
│   │       └── 2026-09/
│   └── tags/
│       └── registry.json
├── .runtime/
└── backups/
```

Legacy ledgers whose basename begins with `backlog` remain discoverable for compatibility.

### Framework vs project data

| Area | Ownership | Update behavior |
|---|---|---|
| `_task_mecca/framework/**` | Task Mecca framework | managed by migrator |
| `_task_mecca/ROOT_PROMPT.md` | managed/customizable | protected on local/upstream conflicts |
| `_task_mecca/data/**` | project/agent durable data | never overwritten by migrator |
| legacy `_task_mecca/backlog*/**` | project data | compatibility path |
| `_task_mecca/.runtime/**` | ephemeral runtime state | not a durable Git source |
| `_task_mecca/backups/**` | migration safety copy | not framework-managed |

This separation allows a repository clone to carry both the Task Mecca operating contract and the project’s durable task state.

---

## Human-readable backlog

A backlog item is both an execution record for agents and a document a person should be able to understand quickly.

Task Mecca uses a `## 핵심 요약` / human-summary projection alongside the canonical contract:

- purpose;
- key change;
- current status or result;
- follow-up, blocking issue, approval, or important unverified point when relevant.

Long sections may optionally start with a semantic summary:

```markdown
> Summary: The meaning of the entire section in one or two sentences.
```

- If a semantic summary exists, the Web UI keeps it visible and collapses only the detail body.
- If there is no semantic summary, the body is shown directly with no forced disclosure interaction.
- Short or already-clear sections are not forced to have summaries.

User-facing prose follows the user’s actual language and terminology. Exact code, paths, identifiers, and technical terms may remain unchanged, but internal English word order or compressed agent jargon should not leak mechanically into another language.

---

## Source-linked tasks: Linear and GitHub Issues

When work begins from an external item such as Linear or GitHub Issue, Task Mecca treats it as a **Source-linked Task**.

```text
Linear / GitHub Issue
        ↓
Root reads the source through the connected MCP/tool
        ↓
User and Root agree scope / design / acceptance criteria
        ↓
Task Mecca backlog records source + execution contract
        ↓
Controller / Worker execute
        ↓
Meaningful decisions, lifecycle changes, and completion results
are written back to the external source
```

Recommended backlog source metadata:

```markdown
- Source: [Linear · ENG-123](https://linear.app/...)
- Source: [GitHub · owner/repo#84](https://github.com/owner/repo/issues/84)
```

Task Mecca does not duplicate the external issue tracker.

- **External issue**: shared human/team work source and collaboration surface.
- **Task Mecca backlog**: current confirmed execution contract and lifecycle source for agents.
- **Root**: interprets and synchronizes meaningful decisions, state transitions, and completion results.

An external issue edit never silently overwrites the Task Mecca contract. If it materially changes scope or acceptance criteria, Root reconciles the difference with the user first.

External status names are not globally hardcoded. Root reads the connected system’s actual workflow and chooses a semantically appropriate state.

---

## Lifecycle and observability

The durable backlog state comes from the filename:

```text
<sort-key>.<ID>.<slug>.<todo|doing|hold|done>.md
```

| State | Meaning |
|---|---|
| `todo` | registered but not currently assigned |
| `doing` | actively owned by a worker |
| `hold` | genuinely waiting on user/external/dependency conditions |
| `done` | acceptance criteria verified and results recorded |

Git lifecycle transitions plus runtime observations support:

- **Queue Time**: registration → first doing
- **Active Time**: cumulative doing time
- **Wait Time**: cumulative hold time
- **Lead Time**: registration → completion/current time

Lifecycle and worker liveness are deliberately separate. `Quiet`, `Stale`, and `Worker missing` are observability signals, not automatic lifecycle transitions.

Controller evaluates dependencies, write-scope conflicts, worker continuity, and available capacity, fills independent ready work in parallel where appropriate, and treats completed/missing-worker `doing` items as recovery work rather than silently leaving continuity gaps.

---

## Full Access preflight

Task Mecca does not use “spawn first and see whether permissions fail” as its normal execution model.

Immediately before worker dispatch:

```bash
task-mecca preflight --require-full-access --json
```

checks effective Full Access with a fresh probe.

- full → dispatch can proceed;
- restricted/unknown → no worker spawn or doing claim;
- Root asks the user for the required permission change;
- after the change, preflight is executed again.

A cached Web UI observation is informational only and is never treated as dispatch authorization.

---

## Project taxonomy / Tags

Task Mecca supports a project-owned taxonomy:

```text
_task_mecca/data/tags/registry.json
```

Tags describe work meaning. They should not duplicate state, dependencies, agent assignment, or write scope.

Common commands:

```bash
task-mecca tags list
task-mecca tags search <query>
task-mecca tags resolve <query>
task-mecca tags define <namespace:name> [description] [aliases]
task-mecca tags tasks <expression>
task-mecca tags stats
```

New canonical tags are allowed when they represent genuinely new meaning; the goal is to avoid duplicate semantics and spelling fragmentation, not to force every task into a pre-existing vocabulary.

---

## Web UI

From the project root:

```bash
task-mecca web
```

Task Mecca Web runs as a user-level singleton background service on port `18765` by default.

### Current capabilities

- automatic backlog discovery and manual switching;
- active + `archive/YYYY-MM/` history;
- Simple / Defined / legacy rendering;
- human summary, source links, related work, execution evidence, lifecycle;
- Queue / Active / Wait / Lead Time;
- dependency/readiness and Controller coordination information;
- Subagent Workload;
- Needs Attention, stale, worker-missing signals;
- project taxonomy/tag visibility;
- Global Hub for registered projects and framework versions;
- CLI update and project-framework migration surfaces;
- Light / Dark / System themes;
- Korean / English UI and manual;
- Markdown tables, code copy, Mermaid;
- floating TOC;
- adaptive page size and keyboard navigation;
- non-disruptive refresh notification when content changes while the user is reading;
- browser notifications when running in a secure origin.

Backlog content is observed from the durable Markdown/Git source rather than edited as a generic Web document. Maintenance actions such as upgrade and migration require an explicit user action.

### Web service control

```bash
task-mecca web status
task-mecca web restart
task-mecca web stop
task-mecca web logs
task-mecca web logs --follow
task-mecca web --foreground
```

### Tailscale HTTPS

With the default `--host auto`, Task Mecca serves local HTTP and, when Tailscale is detected with MagicDNS and HTTPS Certificates enabled, provides a direct HTTPS endpoint on the same port.

```text
Local      http://127.0.0.1:18765/
Tailscale  https://<machine>.<tailnet>.ts.net:18765/
```

No separate `tailscale serve` command is required for this mode.

---

## Installation

### Requirements

- Git
- Windows, macOS, or Linux
- no Python, `uv`, or Go toolchain required for end users

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/silverkhan/TaskMecca/main/install.sh | sh
```

Then initialize the project:

```bash
task-mecca init
```

### Windows

Download the rolling standalone asset `task-mecca-windows-amd64.exe`, place it in a directory on PATH, and use it as `task-mecca.exe`.

No Go or Python runtime is required.

---

## First use

### 1. Initialize

```bash
task-mecca init
```

Only framework and installation metadata are created initially. No empty backlog is created.

### 2. Start Web

```bash
task-mecca web
```

The dashboard can start before the first task exists.

### 3. Activate one Root session

Task Mecca installation and Root activation are intentionally separate.

```text
Task Mecca installed in the project ≠ this session is Root
```

Choose one user-facing AI session to act as Root.

The actual prompt is available in **Web → User Manual → Quick Start → Root session prompt**, or directly from:

```text
_task_mecca/ROOT_PROMPT.md
```

Paste that prompt into the selected session, then describe work naturally.

---

## Upgrade and migration

Upgrade the standalone CLI:

```bash
task-mecca upgrade
```

The Web UI also surfaces available CLI upgrades.

- On macOS/Linux, if Task Mecca Web is running during a CLI upgrade, it is restarted with the upgraded executable.
- On Windows, the running Web service is stopped when necessary so the executable can be replaced safely, then a precise restart instruction is shown.
- The CLI itself has no separate restart concept; subsequent commands use the replaced binary.

Update the managed project framework:

```bash
task-mecca migrate
```

or use the migration action in Web Global Hub.

If operating instructions such as `ROOT_PROMPT.md`, `SESSION_GUIDE.md`, `collab.md`, or `roles/*.md` changed, Web surfaces whether the active Root session should be resynchronized and provides a copyable resync prompt.

Project-owned `data/**` is never overwritten by migration.

---

## Common CLI commands

| Command | Purpose |
|---|---|
| `task-mecca init` | install framework into the current project |
| `task-mecca upgrade` | update the standalone CLI |
| `task-mecca migrate` | update managed framework files |
| `task-mecca web` | launch the Web dashboard |
| `task-mecca web status` | inspect Web service status |
| `task-mecca status` | backlog status summary |
| `task-mecca inspect <ID>` | inspect one task |
| `task-mecca ready` | list execution-ready work |
| `task-mecca coordinate` | scheduling snapshot |
| `task-mecca workload` | worker workload |
| `task-mecca doctor` | comprehensive consistency checks |
| `task-mecca preflight --require-full-access` | dispatch permission preflight |
| `task-mecca tags ...` | taxonomy operations |

Most users should not need to manually orchestrate these primitives. The normal interaction model is to talk to Root in natural language and let Root/agents use deterministic commands underneath.

---

## Installed manuals

The project-local framework contains more detailed operating contracts than this README:

| Document | Purpose |
|---|---|
| `_task_mecca/ROOT_PROMPT.md` | Root activation prompt |
| `framework/SESSION_GUIDE.md` | user quick start + operational rules |
| `framework/collab.md` | roles, contracts, lifecycle, Source-linked Tasks |
| `framework/HUMAN_READABLE_BACKLOG.md` | human-readable backlog principles |
| `framework/TAGS.md` | taxonomy/tag policy |
| `framework/roles/*.md` | Root / Registrar / Controller / Worker contracts |
| `framework/_template.md` | canonical backlog template |

The same user guidance is also exposed through the Web **User Manual**.

---

## Project origin and acknowledgements

Task Mecca began with ideas and inspiration shared by my colleague [gyusu](https://github.com/gyusu).

The initial concept separated a backlog-registration session from Worker sessions that execute backlog items, with Workers repeatedly selecting and performing remaining work in a `/goal`-style loop until the backlog was exhausted. Reference source material for exploring and implementing that collaboration pattern was also shared with me.

That idea became an important starting point for Task Mecca.

Task Mecca has since expanded the original concept into the Root / Registrar / Controller / Worker role model, task orchestration and parallel execution, backlog lifecycle management, Web UI, external-source synchronization, and installation/release infrastructure.

Many thanks to [gyusu](https://github.com/gyusu) for sharing the early ideas and inspiration that helped start this project.

---

## Development

The standalone Go runtime is the primary end-user distribution. The Python package remains as a compatibility path.

Go:

```bash
go test ./...
go build ./cmd/task-mecca
```

Python compatibility:

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

MIT License. See [LICENSE](LICENSE).
