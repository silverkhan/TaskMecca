# A-50 Controller 검수 상태 신뢰성 구현 보고

## 작업 근거
- 구현 worktree: `/Users/hanati/.codex/worktrees/controller-live-review-health/TaskMecca`
- 기준: `origin/dev` / `32c61719abe0b47f2812fe10ba6b59da3399fcbc`
- Worker runtime (Controller가 제공한 identity): `run-fda32634067879e5`, native UUID `01a12db7-f184-7f80-afdc-8b25c13e8a04`
- 혼동 금지: unrelated attempt `run-f33def2a4037e276`는 사용하지 않음.

## 변경
- exact `worker_done` prepared handoff 및 claim Controller/attempt가 target과 일치하고, 동일 project execution ledger에서 provider, runtime agent ID, session, bound state, Hook 관측, running/nonterminal 상태가 모두 맞을 때 검수 진행을 표시.
- Controller `LastActivityAt`는 미래/불량 clock이면 fail-closed. 15분 claim grace 안에서 claim 시각보다 최대 15분 앞선 activity는 허용해 11분 전 활동 시나리오도 `controller_review`로 남김. Controller stage clock은 `claimed_at`이며 15분 초과는 recovery.
- backlog completion review는 coordinate/workload/Web summary가 공유하는 control tower projection을 유지. Web operations stage도 동일한 completion review 결과를 적용하고, raw handoff fallback은 complete target identity와 exact running Hook attempt를 요구.
- 회귀: exact claim phase 11분-old activity, stale/mismatched session fail-closed, 실제 synthetic Web stage scan, handoff review grace 및 backlog doing 보존.

## 검증
- 통과: `go test ./...`
- 통과: `go test ./internal/backlog -run 'CompletionReview' -count=1`
- 통과: `go test ./internal/webui -run 'ExactControllerStageLive|OperationStagesBoundedGraceAndFailures|OperationStageScanTransitionDedupAndBacklogPreservation' -count=1`
- 통과: `go vet ./...`
- 통과: `node --check goassets/template/_task_mecca/framework/web/app.js`
- 통과: `node --check goassets/template/_task_mecca/framework/web/sw.js`
- 통과: `go build ./...`, `git diff --check`
- Web subprocess의 `TestOperationWebProcessContinuesAfterBrowserClose`는 단독/패키지 반복에서 간헐 timeout이 한 차례씩 났으나 마지막 전체 `go test ./...`는 통과. 해당 test의 재시작/포트/프로세스 대기 flaky 특성은 잔여 위험.

## 미수행/한계
- 사용자가 수행할 Dev 실사용 및 브라우저 UI 확인은 하지 않음.
- Python 별도 runtime projection은 저장소에서 확인하지 못했으며 Go CLI coordinate 및 Go Web projection 경로를 검사함.
- canonical backlog/lifecycle, Linear, 실제 Telegram, 운영 DB/데이터는 수정/접근하지 않음.

## Commit
- `f47e4e029a72956a0d1695416d01693938fe9f52`
- detached HEAD에서 생성, 원격 push는 수행하지 않음. Controller 통합 대상.
