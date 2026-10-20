# 보존 우선 Git worktree 정리 계획 (AID-204)

```bash
task-mecca worktree plan --project /PROJECT --into origin/dev --json
task-mecca worktree plan --project /PROJECT --into main --max-files 200000 --json
```

이 기능은 **읽기 전용**이고 Git worktree를 제거·잠금 해제·브랜치 삭제·정리하지 않는다. Git의 `--porcelain -z` 목록, 대상 HEAD/branch/detached 상태, 통합 브랜치 ancestry, tracked/untracked/ignored 파일, 전체 파일 수·용량·symlink·중첩 Git 저장소 여부를 분류한다.

- `primary_worktree`, `detached_head`, `locked_worktree`, `unmerged_head`, `tracked_modified`, `untracked_payload`, `ignored_payload`, `symlink_payload`, `nested_repository` 등은 보존 사유다.
- `usage_not_verified`는 Git만으로 현재 Agent가 사용하는지 판정할 수 없다는 뜻이다. 따라서 깨끗하고 병합된 worktree라도 이 CLI만으로 삭제를 승인하지 않는다.
- `ScanComplete=false` 또는 `Merged=unknown`이면 보존을 유지하고 근거 부족 사유를 표시한다.
- `PlanDigest`는 Git 상태와 파일 inventory 요약에 대해 결정적으로 산출하며 후속 보존·정리 단계에서 대상 재검증 기준이 된다.
- 실제 자료 회수/원본·복사본 SHA-256 검증/중단 후 재개/정확한 `git worktree remove`는 **AID-210**에서 별도 검증과 승인 후 구현한다. `worktree plan`의 출력만으로 삭제하지 않는다.
- `GIT_OPTIONAL_LOCKS=0`을 적용해 Git 조회가 인덱스 갱신을 피하도록 한다. CLI는 쉘 문자열이 아닌 Git 인자 배열을 사용한다.
