# Multi-worker backlog assignments (v1: declarative roster)

## Rationale
A backlog remains **one acceptance unit** even when multiple workers contribute non-overlapping implementation and independent validation. This does not make it a shared mutable worktree or allow a single worker to finalize the backlog.

## Backlog Markdown contract
Place a section immediately after the task overview / near the top of the body. The detail UI recognizes either `## 작업 참여자` or `## Worker Assignments`. Columns are Worker, Role, Scope, Assignment origin. Use one row per participant. A single-worker backlog does not require this section.

```markdown
## 작업 참여자

| Worker | 역할 | 담당 범위 | 배정 경위 |
| --- | --- | --- | --- |
| raichyu | 주 구현 | BM 포지션 및 회전율 계산, 저장, API 연결 | Controller 배정 |
| naong | 독립 검수 | BM 정책, 10개 path의 수익률·비중 대사 | Controller 배정 |
| metamong | 보조 구현 | 과거 BM 실행 근거 복원 도구, 분리된 파일 3개 | raichyu 요청, Controller 승인 |
```

The roster declares **responsibility**, not observed runtime state. The existing Runtime and Lifecycle are the authority on actual assignment attempts, activity, and completion. Do not manually fabricate start/finish timestamps or assignment IDs.

## Rules for Controller
1. Keep exactly one lead for integration, and retain Controller-only final acceptance/transition authority.
2. Assign another worker under the same backlog only when there is one shared acceptance criterion and an explicit contribution or independent validation need. Otherwise register a separate linked backlog.
3. Explicitly state each worker's scope and file boundary before parallel work, and prevent conflicting writes. Worker-initiated delegation must be authorized by the Controller.
4. Capture every participation change in the canonical assignment/runtime history. If evidence cannot be verified, show it as unknown; a Markdown table alone is not evidence of execution.
5. Do not mark the overall backlog done because the lead finishes. Wait for required independent checks, supporting outputs, conflict resolution, and Controller acceptance.
6. Preserve existing single-worker tasks and historical Markdown without migration.

## Delivery boundaries
- **Implemented in this PR:** read-only roster presentation in backlog detail, Markdown authoring convention, no change to single-worker execution, status transition, or lifecycle.
- **Required next integration:** structured assignment roster storage / event correlation with existing assignment IDs, automated participant join/leave evidence, Controller completion gate tied to all required contributions and validation. These are deliberately **not** inferred from prose.

## Acceptance checks
- Existing single-worker details do not render a roster.
- Multiple rows show worker/role/scope/origin without replacing canonical agent or lifecycle data.
- Missing / malformed Markdown does not alter state or cause a fabricated assignment.
- Changing roster text cannot trigger runtime actions, notifications, or completion.
- A second worker with overlapping write scope must not be automatically authorized.
