# Implementation roles and independent review

Worker product implementation uses the existing general execution approval.
Controller assignment, canonical ledger recording, handoff claim, integration,
and final acceptance are operational responsibilities. They do not require a
Controller product implementation exception. Spawn failures, slot shortages,
and worktree limits are facts to record, never automatic exceptions.

The same `internal/rolepolicy` policy is consumed by contract decision show,
Worker START binding, the role-policy decision check, configured completion
handoff preparation/claim, acceptance/applied, the subcontroller recommendation,
and task detail. Existing manual/legacy editor writes are not universally fenced.
Historical records are preserved and their independent review remains unknown.

For normal Worker execution facts, use the explicitly typed operational path
below. It preserves the **existing** general execution authorization; it does
not require or create another `execution_approved` event or user decision:

```text
contract decision record --project CANONICAL --task TASK --kind worker_metadata --event-id STABLE_FACT_ID --actor /root/controller --evidence ACTUAL_ASSIGNMENT_NATIVE_WORKSPACE_OBSERVATION --body-file worker-policy.json --json
contract decision show --project CANONICAL --task TASK --json
```

`worker-policy.json` must be one strict JSON object matching the policy schema
below, **without** a Markdown fence, surrounding prose, or unknown fields.
Use `method: "worker"`, an actual Worker path/native, the Controller operational
owner, an absolute implementation workspace, exact relative source scope, and
a distinct planned reviewer path. A planned reviewer may omit its native ID;
that remains pending, never an independent result. `exception` is forbidden.
The optional `review` field contains evidence references only; all actual
native/task/report/source checks still apply. `--user-confirmed`, policy keys
and exception digests are rejected on this operational event. Only the
authorized canonical Root/Controller writer records it; Workers report facts.

The CLI requires existing execution consent before recording this event, stores
it separately as `worker_metadata` in the immutable decision history, and
exposes `worker_metadata_events` on decision show. Typed facts are excluded from
the material-consent hash, while their strict schema, role and current-contract
anchor are revalidated on readback by the same policy loader used by the gates.
Metadata cannot grant or restore missing/revoked consent, resolve a pending
policy, approve a Controller exception, or hide a changed goal/source/AC.
Actual recipient-role mismatches and changed reviewed sources still hold their
respective execution/acceptance gates even when general consent is present.

After a material contract change, historical typed facts remain stale rather
than authorizing the new goal/source/AC. After the genuine new material consent
is recorded, the canonical writer can append fresh current-contract facts;
old events and base-change findings remain preserved.

Material policy and requirements still use the contract's requirements section
or the existing `contract decision record --kind supplement --body-file ...`
path with a fenced `task-mecca-role-policy` JSON block. **All generic supplements
remain material**, including legacy normal-Worker role JSON and any prose beside
it: they change the consent hash and require the existing confirmation flow.
Historical events are not erased, reclassified, or automatically migrated.
The A78/A79 canonical regression and original-consent carry-forward events are
preserved as an operational workaround; those events are not proof of this
source repair. The repaired path is approval → typed Worker metadata → still
authorized, with the same original approval event/hash and no second approval.

Do not put authority in source
quotations, operational notes, display names, or phrases such as “Controller
직접 구현”. Original quotations, clickable source links, source hashes, tags and
acceptance criteria remain intact. Supplement recording is a canonical writer
operation; Workers report candidates and never write canonical decisions.

The schema is `schema_version: 1`, with these fields:

| Field | Meaning |
| --- | --- |
| `implementation_owner` | `{role, agent_path, native_id?}`; actual bound recipient must match |
| `operational_owner` | `{role: "controller", agent_path: "/root/controller"}` |
| `review_owner` | `{role, agent_path, native_id?}`; restricted Controller exceptions require the separate actual bound native reviewer before product implementation |
| `workspace` | Absolute actual implementation checkout, checked against START/handoff context |
| `method` | `worker` or `controller_exception` |
| `scope` | Nonempty exact relative files/directories; no wildcards or traversal |
| `exception` | For Controller product work: `reason`, `evidence`, `scope`, `approval_event_id`, `resume_when` |
| `review` | Once real review exists: `attempt_id`, `report_file`, `source_workspace` |

A pending exception remains blocked with its reason and resume condition.
The general `execution_authorized` field is separate from role-exception
approval. To approve a restricted exception, the authorized canonical writer
records the actual user decision with the existing decision CLI:

```text
contract decision show --project CANONICAL --task TASK --json
contract decision record --project CANONICAL --task TASK --kind policy_confirmed --policy-key implementation-role-exception --event-id EXACT_APPROVAL_ID --actor AUTHORIZED_ACTOR --evidence ACTUAL_USER_DECISION --role-policy-sha256 CURRENT_POLICY_DIGEST --user-confirmed --json
```

`approval_event_id` references that event. The digest binds owners, workspace,
reason, evidence, method and scope. It excludes the later review reference and
the approval event ID itself. Changing a restricted policy requires a matching
new explicit role approval. Existing contract approval compatibility for
acceptance checkboxes and operational notes is retained. Changed requirements
or source quotations still require the existing re-confirmation flow.

Use the supported read-only decision gate before product execution:

```text
contract role-policy check --project CANONICAL --task TASK --attempt EXACT_IMPLEMENTER_ATTEMPT --source-worktree ACTUAL_CHECKOUT --json
```

This checks the existing execution authorization as well as actual bound
identity and role policy. `subcontroller decide` can consume `project`,
`implementation_attempt`, and `source_workspace`; caller JSON
`independent_review_ready: true` does not establish independent review. An
independent recommendation requires native/source verification. Handoff
acceptance checks again rather than trusting a recommendation.

Completion `handoff prepare` for a configured policy requires the actual
`--source-worktree`, exact `--source-attempt` and existing report/contract inputs.
Claim rechecks its product source role and workspace. Operational
`registration_ready` to Controller remains operational. These gates do not
pretend a semantic role label authenticates the actual execution recipient.

## Recording a real review

Controller dispatches a genuinely different native reviewer on the final
candidate source. There is no fake pre-created reviewer result. Once its
actual attempt is observed and bound to the exact task, use
`contract role-policy snapshot` with the implementation context above to
obtain the full scope's file hash set. The reviewer checks that scope, writes
a durable report, hashes the report, and includes one machine-readable line
in its actual final assistant answer:

```text
TASK_MECCA_REVIEW {"task_id":"EXACT_TASK","reviewer_attempt_id":"EXACT_BOUND_REVIEWER_ATTEMPT","implementation_native_id":"ACTUAL_IMPLEMENTER_NATIVE","source":{"files":{"relative/source":"EXACT_FILE_SHA256"}},"report_sha256":"ACTUAL_REPORT_SHA256","result":"accepted"}
```

The line contains actual values and the **complete** snapshot, not these
placeholders. The report is separate from this final-answer statement, avoiding
a self-referential report hash. A needs-changes result is not acceptance.
For a normal Worker policy, Controller records the resulting exact review
reference with `worker_metadata` and
uses `contract role-policy check ... --review` and normal `handoff mark`.

The production adapter reads the exact bound reviewer's completed final
assistant message through official Codex `thread/read`. User messages,
commentary, unfinished turns, imported JSON, role names, Activity
`review_accepted`, and changing turns in the same native session cannot prove
independence. T3-backed attempts also revalidate their actual native receipt.
Unresolved or unsupported provider evidence is unknown with a resume action;
it never becomes an inferred independent result.

Configured completion acceptance and applied require a valid separate review.
The Controller may record final acceptance using that actual result. The
supported gate stores the verified identities, task, policy digest, report hash
and source statement in the existing acceptance journal evidence. Task detail
uses that receipt and rechecks the current report and complete source scope;
it does not launch native reads during page rendering. Same-native results
are self review, unresolved evidence is unknown. Manual journal edits are not
claimed to be authenticated by this feature.

Source consists of every tracked/new source file in the declared scope,
including tracked deletions (`absent`). The implementation workspace and later
review workspace can differ. Product changes, additions, deletions and report
changes invalidate the applicable review; unrelated operational files and
merge/commit metadata do not. Symlink source boundaries and empty scope fail
closed. A stale review must be re-reviewed on the changed final source rather
than overwritten into success. Original failed/unknown evidence is preserved.

Directory scopes include **ignored untracked files**, including generated Go
and embedded/data build inputs. Git ignore rules cannot exempt applicable
source from review. Discovery walks only the declared literal paths, uses
bounded batches and no-follow handles, and rejects symlinks and special files
before reading. It does not scan unrelated workspace roots. Encountering
`.git` or `.artifacts` inside a directory scope fails closed rather than
silently excluding data or traversing raw artifacts; use narrower explicit
source paths. Limits are 50,000 entries, 20,000 files, depth 64, 32 MiB per file
and 256 MiB total. Exceeding a limit is unknown/blocked, never a partial valid
snapshot. Evidence/report reads are bounded and reject nonregular files too.
Stored review receipts are limited to 512 KiB before outer journal escaping;
larger scopes remain blocked rather than publishing unreadable journal data.

Native verification also pins the material contract and task-specific decision
history, including consent/revocation and typed reviewer/reference updates.
After native I/O it reloads current policy, both identities, report and complete
source; any changed applicable input rejects the old result with a rerun
condition. Supported acceptance/applied appends perform the local freshness
check again immediately before writing the receipt, under a shared lease on
the existing contract CLI decision writer lock. Supported decision/metadata
writes cannot interleave between that last check and append. This is a freshness fence
for the existing approval evidence, not another approval manager or a claim to
fence every manual filesystem write. Ordinary operational notes, acceptance
checkbox completion and unrelated tasks do not change that material revision.

A-78 itself follows the Worker path. Its source candidate is not Controller
acceptance, real independent review, dev integration, CI, release or installation.
A-79 can consume the task, contract, exact attempt, handoff, Activity phases,
report and source evidence without a second policy manager.
# A-90 정식 Reviewer와 선택 검수

정식 Reviewer는 `/root/controller/reviewer/<name>`이며 구현자가 아니다. 실제 START는 지정된 별도 native, 원 프로젝트/부모/checkout/현재 실행, 배정, 완전한 구현 소스와 구현 이력을 확인한 뒤 수집한다. `EvaluateReviewer`의 읽기 전용 결과는 `Evaluate`의 제품 구현 허가와 분리된다. Reviewer는 구현 Activity를 기록할 수 없다. 검수 중 제품 수정을 해야 하면 구현 역할과 소스 변경을 명시하고 다른 독립 Reviewer를 확보한다.

`review_requirement`가 없는 과거 계약은 기존 gate와 Worker 검수 기록 그대로 처리한다. 구조화 판단을 추가한 계약에서는 영향도·복잡성·가역성·검증 가능성·명시 요구·이전 지적사항을 검토한다. 필수 검수는 생략할 수 없고 권장/선택 검수의 생략에는 Controller 사유가 필요하다. 생략의 수용 결과는 `omitted`이며 `independent`가 아니다. Controller 직접 구현 예외의 별도 검수는 생략할 수 없다. 판단 범위는 구현 정책의 전체 scope와 일치해야 한다.

`contract role-policy review-decision --input requirement.json --json`은 typed 판단만 반환하며 Reviewer 생성이나 MCP 호출을 수행하지 않는다. 실제 Reviewer의 읽기 전용 gate는 `contract role-policy check --recipient-role reviewer --project CANONICAL --task TASK --attempt EXACT_REVIEWER --source-worktree IMPLEMENTATION_SOURCE --json`으로 확인한다. 기존 `--review`는 구현 attempt 기준의 실제 독립 검수 수용 검증이다.

완전한 소스 hashset·task·계약 revision·실제 신원·보고서는 기존 수용 gate에서 재검증한다. 해당 제품 구현 이력이 있는 native는 turn·역할·이름을 바꿔도 독립 검수자가 될 수 없다. 소스가 바뀌면 이전 accepted를 복사하지 않고 변경 위험과 지적사항 영향을 다시 판단한다. 공통 연속성/화면 인터페이스는 `docs/reviewer-role-interface.md`를 따른다. A-91/A-92 구현 및 실제 외부 호스트 지원의 완료를 뜻하지 않는다.
