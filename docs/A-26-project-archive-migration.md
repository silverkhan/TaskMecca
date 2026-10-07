# A26 프로젝트 보관·감시 관리와 선택형 마이그레이션

사람의 선택을 적용할 때는 응답 `plan_digest`를 `--expect-plan DIGEST`로 전달합니다. 선택 대기 중 manifest나 파일이 바뀌면 다시 `choice_required`를 반환하며 갱신/백업하지 않습니다. Web과 interactive CLI는 이 digest를 자동 전달합니다.

보관하기는 파일 변경 없이 활성 목록을 보관 목록으로 옮깁니다. 다시 편입은 감시를 복원하고 목록 제거는 기록만 지우며 기존 lexical/canonical 감시 차단 경계를 보존합니다. Web 열기·재시작은 등록하지 않으며 최초 Go `init`, 명시 `projects add`, 다시 편입만 등록을 생성합니다. Python 호환 CLI는 기존 framework-only 설치 역할을 유지합니다.

`migrate --json`은 수정된 관리 파일에서 exit 3 / `status: choice_required` / `modified_files` / `choices`를 반환합니다. 선택 전에는 쓰기가 없습니다. `--choice overwrite|backup|cancel`은 각각 새 framework 사용, 수정 파일 백업 성공 후 갱신, 무변경 취소입니다. Web은 키보드·Escape·포커스 복원 지원 확인창을 사용합니다. 비TTY는 즉시 반환합니다.

백로그, project config, Telegram 설정, 인증정보, `.runtime`, backups는 관리 대상으로 허용하지 않습니다. manifest 경로를 제한하고 symlink 파일/상위 framework 경계와 읽기 실패를 거부합니다. 기존 파일이 사라진 경우만 안전한 재설치 대상으로 분류합니다. 소유권을 확인할 수 없는 manifest 없는 레거시 파일은 보존하고 incoming framework 파일만 갱신합니다.

휴지통 이동·잔존 확인·폴더 삭제/복원 구현과 실제 OS Trash CI를 제거했습니다. registry는 `archived_projects`, Web/API는 `archives`를 사용합니다. 옛 `removal_history`는 표시하지 않으며 guard에서만 구식 writer의 차단 경계를 유지하기 위해 읽습니다. 옛 Trash 결과·복구 경로·잔존 상태를 표시하지 않습니다. 실제 파일은 사용자가 Finder·탐색기·터미널에서 관리합니다.

운영 원장·실제 프로젝트·운영 설치·Telegram delivery gate는 이번 Worker의 변경 대상이 아닙니다. Controller가 PR/dev 통합·canonical 완료·worktree 보존/정리를 담당합니다. 별도 health-probe 이슈와 A14 메시지 렌더링은 범위 밖입니다.
