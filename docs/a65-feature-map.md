# A-65 feature correspondence and stage limits

This map compares the `f0a732e3` application with the Smoke core prototype. “Core” means the working prototype surface; it does not mark every A-65 acceptance criterion complete. “Residual” means the old server/API or compatibility source is retained, while the replacement UI still requires the user-reviewed next stage. A retained API is not proof of a completed new screen.

| Existing capability | Smoke destination | Current evidence / remaining work |
| --- | --- | --- |
| Global project hub | Home → Projects | Core: single server aggregate, all registered projects through pagination; unknown/paused/missing/error states distinct. |
| Cross-project attention | Home, first section | Core: canonical reason/evidence, project task links, independent opaque cursor and coverage. |
| Working tasks | Home, second section | Core: observed state and provisional file state distinguished. Full role/agent workload remains residual. |
| Project list/open project | Projects → dedicated backlog | Core: real Go compact-list wire contract, registry changes recover expired cursors once. |
| Backlog search | Backlog search | Core: debounce, foreground cancellation, focus and URL context. |
| Status filtering | Backlog state control | Core: individual states and incoming comma-separated API status values. A full multi-status editing control remains residual. |
| Tag filtering / catalog | Backlog tag disclosure | Core: comma-separated multiple tags; actual catalog stats/registry canonical names. Rich namespace explorer is residual. |
| Sorting | Backlog sort | Core: existing supported ID and modification-time directions. No unsupported title sort. |
| Page selection and size | Backlog pagination / 10, 20, 50 rows | Core: server clamping, URL/back/refresh restoration. Automatic viewport-driven row count is residual; do not claim it is preserved in this first prototype. |
| List keyboard selection | Backlog row links | Core: arrows, Enter, search shortcut, PageUp/PageDown; dedicated detail Escape/ArrowLeft outside chart. |
| Task source and summary | Dedicated detail → Overview | Core: recorded summary, source identity, tags, related tasks, raw source. Missing summary stays “Not recorded”. |
| Defined/simple/legacy requirements | Detail → Requirements | Core: populated contract fields; actual legacy records with empty defined keys fall back to original source. |
| Execution / validation / result / notes | Detail → Execution & validation | Core: existing fields, source/result/verification, measured/unknown timings. Full provider/runtime dashboards remain residual. |
| Related/dependency task links | Detail → Overview | Core: dedicated task navigation within project/backlog context. |
| Lifecycle event provenance | Detail → Lifecycle | Core: immutable actor and historical assignee remain separate; pending/stale/error retries preserve prior evidence. |
| Connected workflow intervals | Detail → Lifecycle | Core: original component in a native module; fit entry, manual window, wheel/pinch, keyboard, edge navigation, exact events and pinned nonmodal reader. |
| Source-change observation | One visible-screen SSE | Core: existing project attention producer and global invalidation hint; document changes prompt, lifecycle patches preserve body, safe first list page refreshes and filtered/later pages prompt. |
| Read-position / deep links | Backlog URLs, `/tasks/ID` | Core paths: query/filter/page, direct task links, legacy project-only entry, hash lifecycle entry, refresh/back/focus. Legacy operations/settings routes still require explicit replacement in the residual stage. |
| Appearance modes / language | Header, Preferences | Core: Smoke Light/Dark/System; one-time valid legacy mode migration before first paint; saved ko/en and unsaved navigator locale policy. |
| Old palette choice / sidebar appearance | Smoke shell | User chose full replacement and Smoke only; no old/new UI toggle. Old compatibility assets remain for server/terminal continuity. |
| Workload, agent roles, root sessions | Operations area after review | Residual. Existing APIs and source are retained. |
| Notifications: current attention / journal | Home, Recent activity | Core read evidence. It is explicitly the notification journal, not all execution logs. |
| Notification history, delivery, web push, Telegram, settings | Notifications / Preferences after review | Residual; shell describes stage instead of claiming working delivery/configuration controls. |
| Project register/pause/archive/restore, backlog selection management | Projects / Preferences after review | Residual UI; project lifecycle APIs retained. |
| Preview/channel/service upgrade/restart and release notes | Preferences / release area after review | Residual. No user service or Preview was switched by this work. |
| Runtime storage / cleanup | Preferences after review | Residual; no automatic cleanup is added. |
| Terminal / security controls | Operations after review | Residual; existing standalone terminal route and compatibility assets retained. |
| Project Activity filters/reports/advice and diagnostics/issues | Activity / operations after review | Residual. Core recent activity does not claim to include this data. |
| Manual/help and release/update prompts | Preferences after review | Residual; old server/API/compatibility source retained. |
| Backlog create/edit; Agent execution/approval | None | Excluded by the approved redesign scope. |

Before overall A-65 completion, the residual screens and behavior must be implemented and verified under the same contract after user design review. No operation is considered complete merely because an old API or compatibility asset still exists.
