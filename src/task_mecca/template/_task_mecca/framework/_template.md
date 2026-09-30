# 백로그 작성 양식

신규 백로그는 **Simple Task**와 **Defined Task** 중 하나의 contract 형식을 사용한다. 두 형식을 한 항목에 동시에 넣지 않는다.
공통 metadata / 핵심 요약 / 실행 정보 / 작업 노트 / 결과 / 검증 형식은 동일하다.

제목은 Agent 내부 용어보다 사람이 이해할 수 있는 **대상 + 변경/결과**를 우선한다.
예: `현행 결과 기준선 및 재현 비교 계약 확정`보다 `실험 결과 재현 여부를 자동으로 비교하는 기준 마련`.

`## 핵심 요약`은 첫 화면용 projection이다. canonical contract를 대체하지 않으며 상태 변화 때 함께 갱신한다.
기본 3~5개 짧은 항목을 가이드로 삼되 의미 없는 항목을 억지로 채우지 않는다.
차단·실패·사용자 승인 요청·중요한 미검증 사항은 상세에만 숨기지 않고 `확인·후속`에 드러낸다.

---

## A. Simple Task

추가 해석 없이 바로 실행·검증 가능한 bounded 작업에 사용한다. 별도 요건정의서와 사용자 재확인을 요구하지 않는다.

```markdown
# <ID> <제목>

## 작업 개요

- 등록자: -
- Agent: -
- 변경범위: -
- 선행: -
- 연관: -
- Tags: -

## 핵심 요약

- 목적: -
- 핵심 변경: -
- 상태·결과: -
- 확인·후속: -

## 작업 정의

### 목표

-

### 수용 기준

- [ ] -

## 실행 정보

- RuntimeProvider: unknown
- Dispatch상태: unknown
- 실행근거: unknown
- Fallback근거: -

## 작업 노트

- 대기: -
- 대기유형: -
- 재개조건: -
- 대기근거: -
- 메모: -

## 결과

-

## 검증

-
```

---

## B. Defined Task

요구사항 정제, 범위 합의, 설계 선택, 작업 분해 또는 명시적인 사용자 확인이 필요한 작업에 사용한다.

```markdown
# <ID> <제목>

## 작업 개요

- 등록자: -
- Agent: -
- 변경범위: -
- 선행: -
- 연관: -
- Tags: -

## 핵심 요약

- 목적: -
- 핵심 변경: -
- 상태·결과: -
- 확인·후속: -

## 요건 정의서

### 배경 및 문제

-

### 목표

-

### 요구사항

-

비교·매핑·상태별 동작처럼 표가 더 명확한 경우 Markdown table을 사용한다.

### 범위

#### 포함

-

#### 제외

-

### 수용 기준

- [ ] -

### 제약 및 보존 조건

-

## 실행 정보

- RuntimeProvider: unknown
- Dispatch상태: unknown
- 실행근거: unknown
- Fallback근거: -

## 작업 노트

- 대기: -
- 대기유형: -
- 재개조건: -
- 대기근거: -
- 메모: -

## 결과

-

## 검증

-
```

---

`Tags`는 선택형 project taxonomy다. 쉼표로 구분한 `namespace:name` 형식을 사용하며 예시는
`area:backend, type:bug, concern:data-integrity`다. 상태(todo/doing/hold/done), 선행관계, Agent assignment처럼
이미 구조화된 metadata는 태그로 중복하지 않는다. 신규 태그를 만들기 전에는 `task-mecca tags resolve/search`로
기존 taxonomy와 alias를 먼저 확인한다. 상세 정책은 `TAGS.md`를 따른다.

`선행`에는 이 항목의 원래 수용을 직접 막는 ID만 적고, 비차단 후속·맥락은 `연관`에 적는다.
다른 항목의 잔여 작업이나 전체 queue 종료를 이 항목의 완료 기준으로 삼지 않는다.

대기가 없으면 네 대기 칸은 `-`다. 실제 hold에서는 `대기`에 직접 사유, `재개조건`에 해제 사건,
`대기근거`에 현재 판단을 뒷받침하는 기록/링크를 적는다. `대기유형`은 `external`, `user`,
`dependency`, `internal` 중 하나다. 내부에서 실행 가능한 구현·통합·검증이 남으면 doing이다.

`## 작업 정의`와 `## 요건 정의서`는 각각 해당 task의 canonical contract다. `## 핵심 요약`은 사람용 첫 화면이며
Registrar·Controller·Worker가 contract를 요약으로 대체해서는 안 된다. 등록 시에는 목적·예정 변경·완료 조건을,
진행 중에는 현재 단계·남은 일·차단 요인을, 완료 시에는 실제 변경·검증 결과·남은 후속을 우선해 요약을 갱신한다.
Simple Task가 실행 중 사용자 판단이 필요한 범위로 커지면 Root에 재정의를 요청한다.

상세 작성 및 상태별 요약 예시는 `HUMAN_READABLE_BACKLOG.md`를 따른다.

legacy flat 원장은 그대로 읽으며 신규 항목만 이 dual-lane schema를 사용한다. 파일명 상태와 Git lifecycle이 상태·시점의
영속 원장이고 별도 완료 상태나 수동 timestamp 원장을 만들지 않는다.
