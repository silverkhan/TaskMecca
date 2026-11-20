# Analysis and review efficiency

B-2 extends the existing read-only `analysisdepth` advice and A-90 role policy.
It does not replace A-91 review continuity, acceptance, execution permission, or
the required/recommended/optional review decision. Use the existing
`flow next --role worker --event analysis_requested --task ID --analysis-input FILE --json`
boundary. Its JSON input accepts optional `counterexamples`, `review_preflight`,
and `review_followup` objects; the CLI and `ReviewRequest` schema stay unchanged.

## Distinguish work, time and evidence

Use `requirements` for interpreting the task/AC, `research` for investigating
references, `environment_identity` for environment and actual native identity,
`implementation` for product changes, `verification` for selected checks,
`independent_review` for independent judgment, `result_readback` for checking
the result against the original evidence, and `actual_wait` for evidenced wait.
`coordination_handoff` remains supported. Unknown phases and gaps remain
unmeasured. These advice labels do not create or rewrite Activity events: use
the existing collector's supported current boundaries, preserving raw phases.
Do not add per-tool comments, heartbeats or automatic LLM summaries.

Consume the existing activity workflow read projection:

| Quantity | Existing field | Meaning |
| --- | --- | --- |
| Total elapsed | `axis.elapsed_ms` | Time between the observed task boundaries; boundary quality still matters |
| Observed wall coverage | `measurement.observed_wall_ms` | Union of measured intervals, without concurrent overlap duplication |
| Cumulative actor time | `measurement.agent_time_ms`, row `observed_activity_ms` | Sum of measured actor intervals; concurrent work can exceed wall time |
| Event/segment counts | `measurement.*.event_count`, `segment_count` | Recorded events/segments; neither is automatically a redundant analysis count |

A repeated confirmation requires the same task/source/assumptions and an actual
finding/reference. A source change, new uncertainty, necessary safety research,
or checkpoint is not avoidable repetition merely because it takes time.
Unmeasured work cannot be reconstructed from elapsed time, file counts or names.

Historical read-only cases are in the original
[`20261120-overnight-workflow-review/report.md`](../_task_mecca/data/audits/20261120-overnight-workflow-review/report.md),
with `evidence-manifest.json`, `review-dispatch-counts.json` and `snapshots/` in
that directory. A-88's early independent reviews found real authorization
defects; its dispatch/name/purpose counts do not count accepted judgments.
B-651's proposal investigated deletion/preservation/reference relationships
for 21 targets and 7,933 metadata records. Its reported requirements duration
alone does not establish waste. B-652's reported phases and completion evidence
must likewise be read with the work performed, not converted into an efficiency
score. These historical gaps do not establish a dev.88 failure. A-93 remains
outside this change's completion decisions.

## Reuse and counterexamples

Keep A-94's task, contract, AC, source hashes, decisions, open issues and context
identity. An exact prior artifact can resolve its recorded checks; a changed
source yields the changed paths and retains required checks conservatively.
Unknown context, changed decisions/risks, stale evidence or long-context
protection still require the existing checkpoint/handoff and fresh checks.
There is no fixed time cap, new estimator, or elapsed-time authorization input.

The optional `counterexamples` matrix binds `task_id` and `source` to the
analysis context. The author chooses `required` case IDs from the actual change
and risk. Each case records `id`, `question`, `input`, `expected`, `actual`,
`result` (`pass`, `fail`, `not_run`) and `evidence`. Relevant examples include
negative/uncertain requests, wrong targets, legacy continuity, stale source,
missing fields, and identity conflicts. These are examples, not a mandatory
global suite. A claimed pass without actual result/evidence is incomplete;
`not_run` remains `not_run`. The helper checks structure, not semantic truth.

## Reviewer preflight and follow-up

Before the Reviewer terminates, `review_preflight` checks the existing request,
current policy/requirement, actual own identity, workspace, contract/source,
verified implementation history, proposed `ReviewStatement` and independently
computed report hash. It reuses `EvaluateReviewer`, the risk requirement and
history checks. It catches exact scope/hash/native/attempt errors early.
It neither writes the Reviewer's judgment nor creates missing native fields.
`ready` means structurally complete. Even a running Reviewer can check a draft;
it is never a final accepted result. Every output remains advisory with
`execution_authorized:false` and `acceptance_granted:false`.

`review_followup` preserves the original preflight input (including the actual
statement, source, identities and report hash), `original_errors`, original
native final and terminal evidence references, and observed state:

| Cause/evidence | Advice |
| --- | --- |
| Source/contract changed or new defect | `new_review`; T3 new review/recheck uses a new `delegate_task` with prior evidence |
| Identity recovery | `blocked`; recover and verify actual original identity first |
| Unknown source, errors, judgment, native final or terminal evidence | `blocked`, with the actual missing requirement |
| Unchanged source, structurally valid original independent accepted statement, observed completed run, final/terminal references, known original errors | `verify_original_then_supplement_transport` |

The last action requests readback of the original attested evidence, followed
by transport/format supplementation through the existing collection gates.
It requests no new judgment, never resumes a terminal Reviewer as a new review,
and never substitutes a Controller-authored statement. References and supplied
`bound`/`observed` facts do not prove provenance: independently verify them at
the existing runtime/native/source boundary. If that readback cannot establish
the original attestation, stop and report the missing evidence; this advice
cannot waive the gate. An original `needs_changes` result cannot become an
accepted result through transport repair; its findings remain unresolved.
Mandatory independent review and destructive/safety investigation remain.

## Bounded comparison, not a speed estimate

Selected tests compare the same fixtures and report planned work:

| Case | Control | Reuse/advice | Required work retained |
| --- | --- | --- | --- |
| Unchanged analysis resume | Two unresolved checks without a prior artifact | Zero unresolved checks with an exact artifact resolving those same two checks | Fresh authorization/native binding and strict gate checks |
| Source change | Two required checks | Two required checks and one changed path | Current source/affected decisions; no stale check reuse or claimed reduction |
| Report transport supplement | Routing a new T3 review/recheck requires a new delegate under A-91 | B-2 identifies an existing attested result for original-evidence readback and transport only | Original judgment/hash/source/native/terminal/independence verification and existing acceptance gates |

The first row demonstrates two avoided repeat check items in a bounded plan;
the source-change row is a negative control. The transport row avoids requesting
a new judgment only after original-evidence verification. These are fixture
work decisions, not observed host API call totals or measured minutes saved.
Actual production savings are unknown. Do not mark unrun checks PASS or invent
speed gains. Controller review, integration, CI/release and final acceptance
are separate outcomes.
