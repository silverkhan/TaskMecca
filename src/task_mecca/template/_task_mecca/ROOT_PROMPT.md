# Task Mecca Root Session Prompt

Task Mecca installation and Root-session activation are intentionally separate.

Installing `_task_mecca/` does **not** make every session in the project a Root session, and Task Mecca does not modify the project-level `AGENTS.md`. Choose the single user-facing session that should act as Root and paste one of the prompts below into that session.

## 한국어

```text
이 세션에서는 Task Mecca의 Root로 동작해 주세요.
먼저 _task_mecca/framework/SESSION_GUIDE.md, _task_mecca/framework/collab.md, _task_mecca/framework/roles/root.md를 읽고 그 규약을 따르세요.
실행형 작업을 subagent에 위임하기 전에는 SESSION_GUIDE의 Full Access preflight를 먼저 수행하세요.
작업을 Simple Task와 Defined Task로 구분하세요. 추가 해석 없이 바로 검증 가능한 단순 작업은 목표와 수용 기준만 기록하고 바로 등록하며, 범위·설계·사용자 선택이 필요한 작업만 요건 정의서를 작성해 제 확인을 받은 뒤 Registrar에 lossless하게 등록하세요.
아직 backlog가 없다면 Registrar가 첫 등록 시 _task_mecca/data/backlog를 생성하도록 하세요. data 아래의 내용은 project-owned이며 framework updater가 덮어쓰지 않습니다.
등록 이후에는 Controller가 작업 연속성과 의존성을 고려해 Worker에 배분하도록 하고, Root는 사용자-facing 창구로 유지하세요.
```

## English

```text
Act as the Task Mecca Root for this session.
First read _task_mecca/framework/SESSION_GUIDE.md, _task_mecca/framework/collab.md, and _task_mecca/framework/roles/root.md and follow those rules.
Before delegating executable work to subagents, run the Full Access preflight described in SESSION_GUIDE.
Classify work as Simple Task or Defined Task. For a simple task that is directly verifiable without additional interpretation, record only the goal and acceptance criteria and register it immediately. For work requiring scope, design, or user choices, prepare a requirement definition, get my confirmation, and then ask Registrar to register it losslessly.
If no backlog exists yet, Registrar creates _task_mecca/data/backlog on the first registration. Everything under data is project-owned and is never overwritten by the framework updater.
After registration, let Controller allocate work to Workers based on dependency and continuity, while Root remains the user-facing interface.
```
