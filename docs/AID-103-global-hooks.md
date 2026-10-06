# 전역 Hook 설정과 신뢰 확인

Task Mecca Web의 워크로드 → Hook 상태에서 Codex와 Claude Code 각각에 대해 `현재 프로젝트`와 `이 기기의 전체 프로젝트`를 별도로 설정·해제할 수 있다. 전역 설정은 사용자 홈의 `~/.codex/hooks.json` 또는 `~/.claude/settings.json`에 Task Mecca 명령만 병합한다. 기존 설정과 다른 Hook은 유지한다.

전역 Hook은 다른 프로젝트에서도 로드될 수 있다. 그러나 `task-mecca runtime observe`는 현재 작업 디렉터리에서 `_task_mecca`가 있는 프로젝트를 찾지 못하면 기록하지 않는다. 전역·프로젝트 Hook이 함께 실행되어 같은 공급자 이벤트를 두 번 전달하면 짧은 동시 실행 구간의 동일 이벤트를 한 번만 원장에 추가한다. 서로 다른 시각에 재사용된 Worker의 정상 이벤트는 별개로 보존한다.

Web의 `설정됨`은 파일에 규칙이 있다는 뜻이다. Codex의 신뢰 승인이나 Claude Code의 workspace trust를 대신하지 않는다. `승인 진행`은 공급자별 정확한 절차와 공식 문서를 같은 화면에 표시하지만 승인을 자동으로 누르지 않는다. 설정 파일의 현재 수정 시각 이후 실제 Hook 이벤트가 도착해야 `관측 확인됨`으로 바뀐다. 기존 세션에서 발생했을 Start/Stop을 추정하지 않는다. 두 범위가 함께 설정되어 있으면 이벤트만으로 어느 설정이 실행됐는지 알 수 없어 개별 범위는 `실행 경로 미확인`으로 표시한다.

- [Codex Hooks — Review and trust hooks](https://learn.chatgpt.com/docs/hooks#review-and-trust-hooks): CLI `/hooks`에서 새로 추가되거나 변경된 Hook을 검토하고 신뢰한다. 지원이 확인되지 않은 Desktop deep link는 제공하지 않는다.
- [Claude Code Hooks — Workspace trust](https://code.claude.com/docs/en/hooks#workspace-trust): interactive 세션의 workspace trust를 수락하고 `/hooks`에서 설정 출처를 확인한다. `/hooks` 자체가 신뢰 승인은 아니다.

검증: `go test ./internal/runtimeobs ./internal/backlog ./internal/webui`, `node --check goassets/template/_task_mecca/framework/web/app.js`. 실제 Codex·Claude 승인과 실제 세션 이벤트는 자동 테스트가 대신하지 않는다.
