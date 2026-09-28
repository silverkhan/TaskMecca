# Task Mecca — project `AGENTS.md` integration snippet

Task Mecca를 설치한 프로젝트의 **최상위 `AGENTS.md`**에 아래 취지의 지시를 한 번 통합한다. 기존 프로젝트 규칙을
덮어쓰지 말고 함께 유지한다.

```markdown
## Task Mecca

사용자가 코드 수정, 파일 생성, 테스트, 리팩터링 등 실행형 작업을 요청하면 별도 bootstrap 명령을 요구하지 말고
`_task_mecca/SESSION_GUIDE.md`를 먼저 읽어 Task Mecca 흐름을 적용한다.
새 세션의 첫 실행형 요청에서는 SESSION_GUIDE의 effective Full Access preflight를 subagent 생성보다 먼저 수행한다.
단순 질의나 설계 대화만 하는 경우에는 Task Mecca orchestration을 강제하지 않는다.
```

이 연결은 사용자 편의 기능이 아니라 **즉시 투입을 위한 진입점 계약**이다. Root가 `SESSION_GUIDE.md`를 자동 발견하지
못하는 설치 상태는 Task Mecca 배포가 완전히 끝난 상태로 보지 않는다.
