# CLI 구조화 계약 사전 검증 — AID-201 / AID-202

> 개발 대상: `dev`. `contract validate/preview/plan`은 읽기 전용이며 `contract apply`만 승인된 배치의 **등록 파일·lifecycle**을 기록한다. **실행 승인은 부여하지 않는다.**

## 에이전트용 최소 호출

```bash
task-mecca contract validate --input tasks.json --json
task-mecca contract preview --input tasks.json --json
task-mecca contract help
```

- `--input -` (기본값)은 표준 입력에서 JSON manifest를 읽는다. `--input PATH`는 파일을 읽는다. stdin/파일 모두 최대 8 MiB.
- `validate`는 쓰기 없는 검증 결과만 반환한다. `preview`는 유효한 경우 `documents[].markdown`과 원천 내용의 SHA-256을 함께 반환한다.
- 정상은 exit 0 / `ok=true`, 검증 실패는 exit 2 / `ok=false`이며 오류는 `findings[].code/path/message`로 반환한다. `written`은 항상 `false`다.
- 내용을 판단·작성하고 `kind` (simple / defined)을 고르는 것은 Root/에이전트 책임이다. CLI는 내용의 **의미**를 자동 고치거나 요약하지 않는다.
- `source_text`는 JSON 문자열 그대로 입력한다. 출력 Markdown에서는 각 줄에 `> `를 붙여 Markdown heading으로 인한 계약 구조 오염을 방지하며 `source_sha256`은 입력 원문의 원본 바이트를 기준으로 계산한다.
- `depends_on`은 같은 manifest에 있는 `key`의 배열이다. 이 단계에서는 외부 키↔B-ID 매핑을 수행하지 않으며, preview의 `선행`은 **미연결 표시**다.
- 위험 입력(누락 goal/acceptance, 중복 key, 없는 dependency, cycle, 무효 URL, 외부 URL인데 source_text 누락, 원문과 충돌하는 canonical heading)은 계획 단계에서 거부한다.
- 이 문서의 **validate/preview만** 등록이나 승인을 수행하지 않는다. 실제 등록은 아래 `contract plan/apply/status`를 사용하며 실행 승인은 별도 `contract decision`으로 기록한다.

## JSON manifest 예시

```json
{
  "schema_version": 1,
  "tasks": [
    {
      "key": "linear:AID-508",
      "title": "가격 데이터 검증 조건 보완",
      "kind": "defined",
      "goal": "분석 전에 입력 품질을 검증한다.",
      "acceptance_criteria": [
        "누락 데이터를 검출한다.",
        "기존 원자료를 변경하지 않는다."
      ],
      "background": "데이터 검수 절차가 분리되어 있다.",
      "requirements": "검증 범위와 실패 사유를 남긴다.",
      "constraints": "원본 데이터 읽기 전용.",
      "source_label": "AID-508",
      "source_url": "https://linear.app/aiden7/issue/AID-508",
      "source_text": "원천 요구사항 전문을 여기에 원문 그대로 포함",
      "tags": ["area:backend"],
      "depends_on": []
    },
    {
      "key": "linear:AID-509",
      "title": "검증 결과 리포트 작성",
      "kind": "simple",
      "goal": "검증 결과를 요약한다.",
      "acceptance_criteria": ["선행 검증 결과를 참조한다."],
      "depends_on": ["linear:AID-508"]
    }
  ]
}
```

## 등록된 기존 CLI 기능과 격차(소스 확인 기준)

| 운영 작업 | 확인된 기존 기능 | 필요한 후속 기능 |
| --- | --- | --- |
| 단건 ID/출처/태그/lifecycle | `next-id`, `inspect`, `tags`, `lifecycle record` | AID-207: 다건 `contract plan/apply/status` 구현·dev 반영 |
| Simple/Defined 계약 | `_template.md`, parser의 `작업 정의/요건 정의서` | AID-202 구조화 preview·AID-207 등록 구현 |
| 실행 승인/보충 | `handoff prepare --execution-authorized` 등 | AID-208: `contract decision record/show`로 실행 승인과 이력 분리 구현 |
| Runtime 최신 실행 | `runtime list|reconcile --json`, provider / limit | AID-203 role/session/agent filter, AID-209 Hook 진단 |
| Worktree 보존형 정리 | Git/handoff 운영 절차, 일부 storage 정리 | AID-204/210 |
| 업데이트 검증 | `upgrade`, `migrate` (modified_files, plan_digest, choices) | AID-205 protect hash/result |
| CLI 자체 설명 | 현재 코드에 `version`, `--version` / README의 `upgrade` | AID-206 글로벌 help/unknown aliases |

**검증 범위:** 2026-10-16의 GitHub `main`/`dev` CLI 소스와 마이그레이션 정책. 실제 사용자가 설치한 바이너리, 모든 OS 및 0.3.1 재현 결과는 아직 **미확인**. AID-201 실측 작업 완료 전까지 기능 지원 확정으로 표기하지 않는다.

## 구현 규칙

1. 기존 canonical backlog/원장, taxonomy, handoff, provider runtime을 별도로 복제하지 않는다.
2. 이후 쓰기 단계는 `plan digest → 사용자 승인 → apply journal → verify/resume`와 idempotency-key 충돌 차단을 통과해야 한다.
3. `success`, `partial`, `failed`, `unknown`, `not_run`을 증거대로 구분한다.
4. 사용자/Worker가 독자적으로 canonical lifecycle을 쓰거나 근거 없는 runtime event를 합성하지 않는다.
5. Stable/main 승격은 Dev 검증 및 사용자 승인 이후 별도로 수행한다.

## AID-207: 다건 등록·재개

```bash
task-mecca contract plan --project /PROJECT --input tasks.json --request-id linear-20261016-1 --json
task-mecca contract apply --project /PROJECT --input tasks.json --request-id linear-20261016-1 --expect-plan PLAN_DIGEST --approve --json
task-mecca contract status --project /PROJECT --request-id linear-20261016-1 --json
```

- `plan`은 현재 백로그를 읽고 예정 **B-ID/선행관계/해시**를 계산하지만 원장에 아무것도 쓰지 않는다. ID는 아직 예약되지 않으므로 다른 등록으로 catalog가 바뀌면 `apply`가 `STALE_PLAN`을 반환한다.
- `apply`는 **등록 전용 승인**(`--approve`) 및 동일 계획의 `--expect-plan`을 모두 요구한다. `--request-id`는 동일 요청 재시도 시 **같은 값으로 유지**해야 한다. 중단 뒤 동일 요청·입력·digest로 재실행하면 완료한 파일과 등록 이벤트를 확인하고 재개한다.
- 같은 요청 ID에 다른 JSON 입력을 제공하면 `IDEMPOTENCY_CONFLICT`. 등록된 외부키와의 충돌, 미확인 선행키, 등록되지 않은 태그는 차단한다. 선행키 매핑은 현재 **동일 manifest 안의 작업**만 지원한다.
- `status`의 `success`는 파일 등록과 `registered` lifecycle 사건이 확인되었다는 의미이며 **착수·완료나 실행 승인**을 뜻하지 않는다. 오류 시 `partial/unknown`을 구분한다.
- 삭제·수정·승인/실행 전환을 하지 않는다. 등록 후 인계는 기존 Registrar/Controller 규약에 따라 별도 처리한다.
