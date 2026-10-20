# Worktree 보존·검증·정확 제거 — AID-210

이 기능은 **Git worktree만** 정리하며 branch/commit을 삭제하지 않는다. 모든 정리 단계의 기본은 fail-closed다.

## 1. 보존형 계획

```bash
task-mecca worktree plan --project /PROJECT --into main --json
```

`worktrees[].protection_reasons`, `tracked_changed`, `untracked`, `ignored`, `nested_repositories`, `symlinks`, `usage=unknown`, `merged` 확인. 이후 `plan_digest`를 보관한다.

## 2. SHA 기반 자료 보존

```bash
task-mecca worktree preserve --project /PROJECT --into main --target /EXACT/WORKTREE \
  --archive /SAFE/EXTERNAL/ARCHIVE --expect-plan PLAN_DIGEST --approve --json

task-mecca worktree verify --archive /SAFE/EXTERNAL/ARCHIVE --json
```

- archive는 **모든 Git worktree 외부**의 실제 보존 폴더여야 한다. 원본과 archive를 다른 디스크/볼륨에 저장하면 디스크 장애에도 유리하다.
- tracked/untracked/ignored 자료를 포함해 각 일반 파일의 원본·복사본 SHA-256/bytes를 검증한다. root Git `.git` 포인터는 제외하고 Git branch/HEAD identity를 별도 기록한다.
- 원본/복제본 해시가 다르거나 symlink/중첩 Git/non-regular payload/검사 중 변경이 있으면 완료 불가. 절대 심볼릭 링크를 따라 외부 파일을 회수하지 않는다.
- 중간 종료 시 같은 대상/plan/archive로 재시도하고 이미 보존된 파일 해시가 맞으면 재복제하지 않는다. 다른 내용을 같은 archive에 덮어쓰지 않는다.
- 완료 시 `archive/manifest.json`과 `payload/**`를 생성한다. **archive는 백업이지 운영원장이 아니며, 자료 삭제 전에 사용자가 위치/용량을 확인해야 한다.**

## 3. 정상 Git 제거 — 조건 모두 만족해야 함

```bash
task-mecca worktree remove --project /PROJECT --into main --target /EXACT/WORKTREE \
  --archive /SAFE/EXTERNAL/ARCHIVE --expect-plan PLAN_DIGEST \
  --approve --confirm-unused --json
```

제거는 다음 조건에서만 허용된다.

- 사용자가 정확한 대상 worktree가 **어떤 Agent/사용자 프로세스에서도 사용 중이 아님**을 실제 확인한 뒤 `--confirm-unused`로 선언
- 보존 원본·archive 해시 및 파일 inventory 재검증 완료, SHA manifest 일치
- HEAD가 병합되었고 detached·locked·primary가 아니며, 추적 수정·untracked·ignored·symlink·중첩 Git 없음
- 직전 `worktree plan`의 digest 동일, 제거 직전 새 Git 상태 재조회 동일
- 오직 `git worktree remove -- EXACT_PATH`로 정상 제거. `--force`, `rm -rf`, branch delete, reset, commit 삭제를 사용하지 않음

**중요:** 자료를 백업했더라도 미추적/ignored/수정된 worktree는 자동 제거하지 않는다. 정리 수용 기준상 임시 삭제 스크립트 대체를 위해 유실 위험을 감수하지 않는다. dirty 대상은 원본 보존 후 수동 충돌 해소·정상 Git 상태 확인이 필요하다.

- 성공 결과 `status=removed`와 `archive/removal.json`에 exact target/HEAD/자료 digest/수행 방식 기록.
- 완료 후 동일 요청 재시도 시 보존 파일의 무결성을 재검증해 `already_removed` 반환. 다시 삭제 시도하지 않는다.
- `blocked/partial/unknown`은 제거 완료가 아니다. 확인할 사유와 다음 조치를 JSON으로 보고.
- CLI는 Git 이외 Provider 세션 사용 여부를 독자적으로 확증할 수 없으므로 **사용자 실제 확인**이 중요하다.

## 운영 검증

기존 작업 중인 Worktree를 대상으로 바로 제거하지 않는다. Dev 환경에서 격리된 테스트 저장소 2~3개로 clean/dirty, unique ignored, symlink, detached, failure/retry를 시험한 뒤 운영자가 사용 상태를 확인하여 진행한다.
