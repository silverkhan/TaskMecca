# A-51 Worker 결과

- 작업 항목: A-51 `link progress review and workload evidence`
- Worker identity: `/root/controller/pairi`
- assignment_id: `assignment-e63d73aa77759960dabd6156c5ea9366`
- attempt_id: `run-b80c5261a87b61ad`
- 상태: DONE
- 브랜치: `codex/a51-progress-review-workload`
- 커밋: `9ec8bf441c14c59a14f333725db33fd159056f9b`

## 변경 사항

- exact `worker_done` handoff에서 Controller가 claim한 경우에만 lifecycle 타임라인에 `review_started` projection을 추가했습니다. actor, 시각, handoff ID, Worker assignment와 Controller attempt를 보존합니다.
- 이 projection은 내구성 있는 lifecycle 이벤트를 쓰지 않으며 Worker의 `started_at` 및 active duration 계산에 포함되지 않습니다.
- 같은 task/assignment의 live Controller attempt가 hook 증거와 전체 identity 일치로 확인될 때 일반 stale/missing/unknown health를 review 상태로 억제합니다. 사용자 대기와 확인된 실행 실패는 그대로 둡니다.
- 회귀 테스트 `TestLifecycleProjectsExactControllerReviewClaimSeparately`를 추가했습니다.

## 검증

- PASS: `go test ./internal/backlog ./internal/webui` (변경 중 실행)
- PASS: `go vet ./...`
- PASS: `go build ./...`
- PASS: Web UI JavaScript `node --check`
- PASS: `git diff --check`
- 전체 `go test ./...`는 기존 `TestOperationWebProcessContinuesAfterBrowserClose`가 간헐적으로 context deadline으로 실패했습니다. 해당 케이스 단독 재실행은 PASS였으나, 전체 suite 재실행에서 같은 timeout이 다시 발생했습니다.

## 미완료 / 제한

- dev 통합·배포는 Controller 담당입니다.
- 실사용 검증은 사용자가 dev 반영 뒤 수행합니다.
- canonical backlog, assignments, runtime 및 handoff 원장은 Worker가 수정하지 않았습니다.
- worktree에 기존 untracked `.serena/` 및 `_task_mecca/`가 있어 보존했습니다.

## Controller handoff transport

- 2026-10-13 15:33 KST에 fresh `task-mecca preflight --require-full-access --json` 실행: `status=full`, `orchestration_ready=true`.
- 현재 worktree runtime 조회에는 등록된 attempt가 없어 `run-b80c5261a87b61ad`의 live/completed target을 검증할 수 없었고, `task-mecca agent /root/controller --json`에서 Controller identity 경로는 유효함을 확인했습니다.
- 이 환경의 native `collaboration` transport 대상은 root child agent로 한정되어 있어 `/root/controller`에 직접 message/follow-up을 전송할 API가 없습니다. Root ACK나 CLI 성공으로 꾸미지 않았으며, handoff는 미확인/미전달 상태입니다.
