# Activity workflow read model

`GET /api/activity?task=ID` keeps all existing fields and adds `workflow` version 1.
The model combines immutable Activity, Lifecycle and Handoff evidence; it does not
write events, authorize work, run Runtime reconciliation or change backlog state.
Control Tower decisions are read from its existing snapshot only.

| Field | Meaning |
|---|---|
| `axis` | Full registration to current completion, or registration to `computed_at` while open. `boundary_status` exposes missing/conflicting bounds. Activity filtering never changes this range. |
| `lifecycle.completion_history` | Historical completion boundaries. A later assignment/start/resume clears only the current completion boundary; past completion stays visible. |
| `measurement` | `not_recorded`, `partial`, `observed`, or `conflict`. Role/review/test metrics have an optional `duration_ms`: absent means unmeasured; a present 0 means an actually closed zero-duration interval. Existing numeric summary fields remain compatible. |
| `rows` | One role × Agent × grounded work round, with separate intervals. `lane` preserves concurrency. Attempt IDs identify execution sessions in detail, not new work rounds. |
| `rows[].intervals[].kind` | `activity` is a closed reported measurement; `activity_open` has no measured end; `execution_report` and `handoff_review` are independently reported execution/claim boundaries, never pure work time. |
| `unmeasured_intervals` | Complement of measured Activity within the full axis, including intervals whose execution is known but activity type/time is not measured. Lifecycle waits stay separately classified. |
| `rounds` | Initial work is 0. An explicit Controller review rejection/needs-changes or failed Handoff acceptance grounds the next rework boundary. The same Handoff's Activity and acceptance outcome count once. No rejection means attempt replacement, resume, phase switching and failed tests do not create a new round. |
| `current` | Evidence-based phase, processing actor, assigned Worker and next action. A claim proves an owner, not continuous execution. Generic waits stay unclassified; external CI waits require an explicit `external_ci_wait_started`/`external_ci_wait` Activity report. |
| `reporting` | Expected execution/review boundaries without matching Activity reports. This is an omission diagnostic, not permission to fill in past activity. |
| `control_snapshot` | Existing snapshot version, source revision, computed/validated times, trust, age and errors, plus Handoff checked/computed times. Stale/error evidence remains distinguishable from current facts. |

The Activity/Handoff reads have short, bounded caches. The workflow response is
reused for up to two seconds for unchanged source versions. Different projects
do not share an I/O lock. Reopening an Activity panel does not reconcile control
state or rescan the Runtime ledger. Source failures retain the last Handoff
snapshot and expose its error/check time; the response does not pretend missing
data is zero.

`lifecycle record started|resumed`, `handoff claim` for a Worker report, and
`handoff mark --step acceptance` return an additive `activity_reporting` reminder.
It identifies a missing actual report and supplies exact bound actor arguments
when available. `activity report` still independently validates role, attempt,
assignment and claim. A terminal/unverified actor is never offered as a way to
backfill history. The check itself records no Activity and does not block work.

Approval and Handoff fingerprints ignore only checkbox state under `### 수용 기준`.
Requirement wording, scope, constraints and the immutable source quotation still
affect authorization. Controller `reconciled` state is verified using real Hook
activity folded into the same exact explicitly bound attempt; the source string
alone is insufficient.

Older approvals may store a checksum that included checked criteria. The reader
first accepts an exact unchanged legacy scope hash, then tries only acceptance
check-state variants, capped at 4,096 candidates and memoized per base hash within
the decision projection. A proven match verifies the original approval anchor
against the current supplement/policy history; it never rewrites an event or
creates new consent. `approval_compatibility`/`approval_anchor_sha256` disclose
that proof. If a legacy contract has no stored text snapshot and the bounded
search cannot establish a match, `LEGACY_CHECK_STATE_UNVERIFIED` keeps it
unauthorized. Actual wording or immutable source changes are not normalized away.

The A-58 UI should show the full `axis`, group `rows`, label round 0 as initial
work and a missing round as unclassified, and keep measurements distinct from
execution evidence. It should use stable role colors, a separate outcome badge,
and the current phase/actor/next action and observation coverage in its summary.
Private user evidence is not part of public fixtures.
