# Agent CLI Operations Playbook

이 문서는 Task Mecca Framework를 읽는 **Root / Registrar / Controller / Worker**의 기본 CLI 운영 규약이다. 단순 사용 설명서가 아니라 **명령 선택, 권한, 승인, 검증, 실패 재개를 연결하는 실행 계약**이다.

## Context Lifecycle Manager — 통합 운영

컨텍스트 수명 조회 및 안전한 세션 교체에는 다음을 사용한다. `context status`·`policy show`·`rotation recover`는 읽기 전용이다. `event --apply`와 `rotation prepare/acknowledge/commit`은 durable 상태를 변경하므로 **실제 원본 프로젝트와 exact attempt**를 확인해야 한다.

```bash
task-mecca context status --role controller --project PROJECT --json
task-mecca context policy show --role controller --project PROJECT --json
task-mecca context event --role controller --event controller_resume --project PROJECT --json
task-mecca context event --role controller --event controller_resume --attempt <EXACT_OLD_ATTEMPT> --apply --project PROJECT --json
task-mecca context rotation acknowledge --role controller --id <ROTATION_ID> --successor <NEW_ATTEMPT> --checkpoint <SHA256> --project PROJECT --json
task-mecca context rotation commit --role controller --id <ROTATION_ID> --project PROJECT --json
task-mecca context rotation recover --role controller --project PROJECT --json
```

- 기본 경계는 65/80/90%이다. 수동 `observe`는 `reported_unverified`, Claude 구조화된 statusline은 `provider_structured`이고 강제 교체에 사용할 수 없다. 실측이 없는 경우 `unknown`을 반환한다.
- Claude Code가 실제로 제공하는 statusline JSON을 정확한 `session_id`와 bound attempt에 맞추어 전달할 수 있는 환경에서는 `task-mecca context observe --role controller --attempt <EXACT> --format claude-statusline --project PROJECT --json < statusline.json`으로 관측을 기록할 수 있다. statusline JSON은 암호학적 인증 수단이 아니므로 `provider_structured`로 표시하며 컨텍스트 사용률 65% 이상은 **안전한 준비만 권고**한다. null/세션 불일치는 실패이며 0으로 대체하지 않는다.

- 7일 이상 장기 세션 및 단건 Registrar 등록 완료 이벤트가 교체 준비를 권고한다. 실제 native successor 생성은 제공된 협업 도구가 수행하며, Task Mecca CLI만으로 생성되지는 않는다.
- 준비는 `doing`과 미처리 관련 Handoff가 없어야 하며, 인수는 별개 native attempt와 정확한 체크포인트 ACK를 필요로 한다.
- 이전 owner가 terminal로 확인되기 전에는 generation을 넘기지 않는다. 이 시점에도 legacy 직접 파일 편집 등 모든 운영 쓰기가 fenced된 것은 아니다.
- Windows/macOS에서 소유권 상태는 파일 잠금·원자 교체를 사용한다. 오류나 미확정 상태는 성공으로 대체하지 않는다.


## 0. 시작 시 확인 (모든 에이전트)

1. `task-mecca --help` 및 `task-mecca help contract`, `task-mecca help runtime`으로 **실제 설치판** 지원 여부를 확인한다. 문서가 최신이라고 CLI 기능도 설치됐다고 가정하지 않는다.
2. 명령을 호출하기 전에 현재 **project root, role, canonical ledger, user approval, external evidence, exact runtime identity**를 확인한다.
3. 내장 CLI 지원 항목에 Python/JavaScript/직접 JSONL 수정/Markdown 수동 패치를 추가하지 않는다. 비지원 항목은 `unsupported` 근거와 필요한 외부 동작을 적는다.
4. 출력의 `status` 및 `findings`, exit code를 확인한다. 코드 0 또는 텍스트 일부만 보고 전체 배치 성공이라 보고하지 않는다.
5. 불변 데이터 원칙: 사용자 요건 원문·출처·snapshot·lifecycle 이력·Git branch/commit은 원본 변경 없이 유지한다.

## 1. 요청 종류별 명령 선택

| 사용자 요청 / 상황 | 담당 역할 | 먼저 수행할 명령 | 성공 이후 | 금지 |
|---|---|---|---|---|
| 신규 외부 이슈 1~N건 등록 | Root→Registrar | `contract validate` → `contract plan` | `contract apply` → `contract status` → `inspect` | 중복 ID·독자 lifecycle·임시 manifest 수선 |
| 기존 backlog 세부 확인 | Root/Controller | `inspect ID --json` / `search` | 근거 확인 | 조회를 등록/실행 승인으로 취급 |
| 실행 여부 승인·정책 확정·철회 | Root (증거)→권한 있는 기록 담당 | `contract decision show` | 명시적 사용자 결정 `record` 후 재조회 | `--user-confirmed`를 승인 추정 수단으로 사용 |
| 실행 준비 및 인계 | Registrar/Controller | `decision show` → 기존 `preflight` / `handoff prepare` | 지원되는 native 전달·결과 확인 후 `handoff mark` | 과거 completed attempt를 live로 재사용 |
| 현재 Controller/Worker/세션 조회 | Root/Controller | `runtime query` | 필터링된 attempt/evidence 확인 | 전체 JSON Python 후처리 |
| Hook 설정 vs 관측 장애 | Root/Controller | `runtime diagnose` | 근거/복구 단계 확인 | 합성 Hook 시작 이벤트 주입 |
| worktree 정리 | Controller/운영자 | `worktree plan → preserve → verify` | clean/merged·unused 확인 뒤 `remove` | plan만 보고 삭제·force cleanup |
| 업데이트 | 운영자 | `protect snapshot → upgrade/migrate → protect verify` | `protect versions`로 CLI·Framework·Web 확인 | 설치 성공=Framework/Web 적용 성공으로 취급 |

## 2. Registrar: 구조화 등록 기본 경로

Root는 요건을 해석해 manifest에 `key/title/kind/goal/acceptance_criteria` 및 원문·출처·태그·선행키를 채운다. **Registrar는 내용을 재해석하거나 생략하지 않는다.**

```bash
task-mecca contract validate --input tasks.json --json
task-mecca contract preview --input tasks.json --json
task-mecca contract plan --project PROJECT --input tasks.json --request-id SOURCE_BATCH_KEY --json
# plan_digest를 JSON에서 추출하고 명시적 등록 승인을 확인한 뒤
task-mecca contract apply --project PROJECT --input tasks.json --request-id SOURCE_BATCH_KEY --expect-plan PLAN_DIGEST --approve --json
task-mecca contract status --project PROJECT --request-id SOURCE_BATCH_KEY --json
```

- `validate/preview/plan`은 읽기 전용이고 ID를 **예약하지 않는다**. `apply`만 등록 파일 및 `registered` lifecycle를 생성한다.
- `request-id`는 **같은 등록 요청의 모든 재시도에서 절대로 변경하지 않는다.** 값이나 입력이 달라지면 신규 요청을 조용히 만들지 않고 충돌로 보고한다.
- 외부 `depends_on` 참조는 현재 **동일 manifest 안의 key**만 자동 매핑한다. 미해석 선행키·순환은 차단한다. 기존 B-ID 선행 추가는 현재 `apply`가 지원한다고 가정하지 않는다.
- tags는 프로젝트 active canonical taxonomy로 확인한다. 신규 태그 정의는 Root/Registrar가 기존 `tags` 명령에 따라 별도 승인/정의한다.
- `apply` 성공은 **등록 완료**이며 실행 승인·배정·착수·구현 완료가 아니다.
- **레거시 단건 절차**(`ensure-backlog` → `next-id` → Markdown → `lifecycle record registered`)는 현 CLI에서 구조화 입력을 지원하지 않거나 legacy ledger migration에 사용 불가능할 때만 선택한다. 기존 수동 경로도 Registrar만 수행한다.
- 현재 CLI가 자동으로 생성하는 계약 문서에 과거 사용자 수동 변경을 강제로 덮어쓰면 안 된다.

## 3. 실행 승인과 직접 handoff

```bash
task-mecca contract decision show --project PROJECT --task B-508 --json
# 사용자 승인 원본이 있을 때만 Root/승인 기록 담당이 record 한다.
task-mecca contract decision record --project PROJECT --task B-508 --kind execution_approved --event-id APPROVAL_EVENT_ID --actor /root --evidence USER_DECISION_REF --user-confirmed --json
task-mecca contract decision show --project PROJECT --task B-508 --json
```

- `execution_authorized=true`와 `status=execution_authorized`를 재조회하고 정책 미확정/불일치를 확인한다. `status=unknown`은 허용 아님.
- **Root의 승인 해석과 권한 책임은 그대로 유지**한다. Registrar는 등록만으로 승인을 부여하지 않는다.
- Registrar는 기존 규약대로 target Controller identity와 fresh preflight를 확인한 후 `task-mecca handoff prepare ... --execution-authorized --json` 실행 → 반환 action이 **지원되는 실제 native 도구**로 전송 → 실제 transport 증거가 있을 때만 `handoff mark` 기록한다.
- `hold`/미지원 provider native 호출/미확인 실행은 성공으로 표시하지 않는다. Root는 등록·인계 확인 뒤 복귀하며 Controller 완료를 기다리거나 중계하지 않는다.
- 보충·정책 확정이 기존 approval의 effective scope를 바꾸면 **새 사용자 확인** 필요. 원본 snapshot은 수정하지 않는다.

### Controller 인계 조회 표준 (AID-219)

Controller는 인계 복구를 시작할 때 전체 JSON을 임시 Python으로 제거하거나 설치 바이너리에 `strings`를 실행하지 않는다. 정확한 옵션은 `task-mecca help handoff`에서 확인한다.

```bash
# 요약 및 진단만 조회 (handoffs 상세/저널 경로 미출력)
task-mecca handoff reconcile --project PROJECT --summary --json
# 원하는 최상위 집계 필드만 조회
task-mecca handoff reconcile --project PROJECT --summary --fields total_handoffs,claimed_pending_count,unclaimed_pending_count,hold_count,findings_count --json
# 특정 인계는 정확한 handoff ID로 직접 확인
task-mecca handoff inspect HANDOFF_ID --project PROJECT --json
# 정규화된 기존 백로그 상태 조회
task-mecca inspect B-525 --project PROJECT --json
```

- `--summary`와 `--fields`는 **handoff reconcile 전용**이다. `--fields`는 출력 객체의 **최상위 필드만** 선택하며 오타나 중복 필드 요청은 exit code 2로 실패한다. 집계는 journal 증거이지 Provider 전송 성공·실제 Agent liveness의 증거가 아니다.
- 동일 workflow에 속한다는 정확한 근거 없이 서로 다른 B-ID(예: B-525와 B-526)를 연관 지어 판단하지 않는다. `claim`은 현재 exact Controller attempt로, `mark dispatch=ok`는 실제 native 전송 증거가 있을 때만 실행한다.
- shell에서 파일 저장/후처리가 불가피하면 `;` 대신 명령 exit status와 JSON `findings`를 확인하여 실패를 성공으로 취급하지 않는다. 본 범위의 집계/필드 조회는 CLI 자체로 해결한다.

### 이벤트 기반 기존 미아 백로그 점검 (AID-223 1단계)

신규 백로그 등록 시 Root는 Registrar의 exact ID를 받은 후, 사용자에게 등록 결과를 알리기 전에 다음을 **한 번** 조회한다.

```bash
task-mecca recovery event --event registration --task B-550 --project PROJECT --json
```

Controller는 `registration_ready` 인수, 신규 시작/재개, Worker DONE/BLOCKED 보고 시 전체 `coordinate.recovery_queue`를 이미 확인했다면 재조회할 필요가 없다. 요약 전용 조회가 필요하면 각각 `--event controller_start`, `--event controller_resume`, `--event worker_report --task ID`로 호출한다.

- `candidates`는 trigger task와 무관한 과거 `doing` 작업도 포함한다. registration 시 현재 신규 ID 자체는 제외한다.
- `report_to_root=true`이면 Root는 기존 미완결 후보 ID·관측 상태와 조치 주체를 등록 결과에 함께 보여준다. `requires_user`가 확정된 항목만 결정을 요청한다.
- `incident_key` / `event_fingerprint`는 관측 동일성 비교용이다. CLI 자체는 알림을 전송하지 않으며 실제 수신 보장을 뜻하지 않는다.
- 조회는 **read-only**다. 실제 Worker 재개·재배정·완료는 Controller가 별도 native 상태 확인, 계약·승인·fresh preflight·미커밋 결과 보존 후 수행한다.
- `no_confirmed_candidates`는 관측 가능한 `coordinate.recovery_queue`가 비었다는 뜻이며 미아 백로그가 절대 없다는 증명은 아니다. 조회 실패를 빈 목록으로 대체하지 않는다.

## 4. 실행 조회·진단

```bash
task-mecca runtime query --agent-id AGENT_UUID --limit 2 --fields attempt_id,binding_state,current_state,runtime_agent_id,session_id,turn_id --json
task-mecca runtime query --role controller --state open --json
task-mecca runtime diagnose --provider codex --agent-id AGENT_UUID --json
```

- `not_found` = **일치하는 관측 증거가 없음**(실제 미실행 단정 금지).
- `open/observed_nonterminal` = **비종료 기록 존재**(살아 있는 프로세스 인증 아님).
- `stale/unknown/unbound/ambiguous`는 근거 부족·연결 문제이며 실제 Provider 이벤트 확인 없이는 시작/종료를 합성하지 않는다.
- Codex 세션 ID가 있다면 `runtime diagnose --provider codex --session-id ID --json`이 직접 해당 native JSONL의 `session_meta/task_started/turn_context` **건수만** 교차검증한다. 원문을 직접 검색하거나 transcript 내용을 보고에 복사하지 않는다. native started ≠ Hook observation 이면 `native_hook_observation_gap`으로 표시한다.
- `terminal_history`를 현재 실행으로 재사용하지 않는다. 필요한 외부 동작은 native capability와 허용된 실제 Agent 도구 사용을 명시한다.

### Controller 종료 전 읽기 전용 Worktree 정리 판단 (Decision P1)

사용 가능한 설치판에서는 `task-mecca flow next --role controller --event final_requested --project REPO --task B-541 --into origin/dev --worktree EXACT_PATH --archive EXTERNAL_ARCHIVE --json`을 실행한다. **증거가 없는 `완료` 텍스트는 상태가 아니다.** CLI는 canonical done, Worktree Git 목록/HEAD 병합 ancestry/ignored/untracked, 외부 archive SHA, 정상 Git 제거 `removal.json`을 확인해 다음 행동을 반환한다. 기록이 없거나 오래됐다면 `EXIT_BLOCKED`로 보고하며 어떠한 파일도 수정하지 않는다. 정리 증거가 있더라도 다른 종료 검증은 계속 필요하고, 강제 git 삭제나 cherry-pick을 병합 ancestry로 간주하는 우회는 하지 않는다.

기존 계획 도구와 연결: `worktree plan → preserve → verify → remove`는 별도의 명시 실행·미사용 확인을 요구한다. **blocked/retained 상태의 canonical backlog는 실제 작업 완료로 둘 수 있으나 Controller가 후속 담당·정리 사유·재개조건을 기록해야 한다.** 현 버전에서 `flow next`는 기록/exit intercept를 자동 시행하지 않는 읽기 전용 초기 단계이므로 저장·독립 복구·역할별 정식 gate는 AID-222/223 후속 구현 범위다.

## 5. Worktree/업데이트 안전 경계

- `worktree plan`은 **읽기 전용**이고 `preserve`는 Git worktree 외부에 tracked/untracked/ignored payload를 SHA-256 보존한다. `verify`는 manifest와 복사본 해시를 확인한다. `remove`는 **사용자 실제 미사용 확인(`--confirm-unused`)** 및 clean·merged·non-detached·non-primary·sha 검증을 모두 만족할 때만 강제하지 않는 `git worktree remove`를 실행한다. dirty/ignored/untracked 자료가 남으면 백업 후에도 삭제하지 않는다. 관련 사용법: `task-mecca help worktree`.
- Git commit/branch·작업 중 자료, ignored/untracked/중첩 저장소/외부 symlink를 보존한다. SHA 증거 없이는 데이터 회수가 끝났다고 주장하지 않는다.
- `upgrade`는 설치 CLI 갱신, `migrate`는 프로젝트 Framework 갱신, `web restart`는 실제 서비스 프로세스 갱신이다. 각 적용 상태는 별도로 보고한다.
- `task-mecca protect snapshot/verify/versions`와 migration의 `protected_files`가 명시하는 범위만 확인한다. 보호 비교는 `_task_mecca/data/**`, `_task_mecca/backlog*/**`, `_task_mecca/config.toml`이며 `.runtime`·외부 저장소는 범위 밖이다. 검증하지 않은 경로는 `not_run/unknown`, 변경 확인이 있으면 변경·누락·추가 목록을 제시한다.

## 6. 오류 및 복구 결정표

| 조건 | 에이전트 처리 |
|---|---|
| `INVALID_JSON/REQUIRED_FIELD/UNKNOWN_DEPENDENCY` | 입력·원문·출처 누락을 보완하고 **plan을 다시 만들되 실제 apply는 금지** |
| `STALE_PLAN` | 기존 plan 무효; current catalog를 재조회해 계획 재검토. **실제 반영 시작 전**에는 기존 요청 식별 유지 |
| `IDEMPOTENCY_CONFLICT` | 같은 요청 key를 다른 payload로 재사용하지 말 것; 원본 요청과 이력 확인 후 사용자 판단 |
| `partial` | `contract status`로 완료 파일/사건/남은 작업 확인, **동일 input/request-id/plan digest**로 재개 |
| `unknown` / 검증 불가 | 원인·근거·남은 확인 조건 기록; 성공으로 승격 금지 |
| 실행 승인 미확인 | registered-only 유지, Controller 깨우지 않음 |
| 보호 해시 불일치/사용 중 worktree | 삭제 중단, 원본 보존 및 명시적 확인 |
| unsupported native operation | 실제 사용 가능한 Provider 도구와 권한 확인, 지원 확인 전 성공 추정 금지 |

**JSON 계약:** 계약 검증 및 일부 등록/진단 명령은 구조화 `status`/findings를 제공한다. 그러나 **모든 오류 경로가 동일 JSON envelope을 제공하는 것은 아직 아니다.** 에이전트는 nonzero exit와 stderr 오류를 함께 확인해야 하며, 성공 JSON이 없으면 성공으로 추측하지 않는다. 전 명령 machine-readable 오류 통일은 AID-213의 후속 검증 항목이다.

## 7. 역할별 보고 의무

- **Root:** 사용자 결정의 정확한 범위, source links, 등록/실행 승인 구분, 새 확인 조건.
- **Registrar:** source key → B-ID mapping, actual canonical file and registered event, 성공/partial, 재시도 키, 기존/외부 참조 차단.
- **Controller:** 현재 effective approval 검증, native transport 증거, Worker 배정/착수/종료 사실, 잔여 hold, 정리/업데이트 보호 증명.
- **Worker:** 코드·테스트·실측 결과와 미확인 항목을 Controller에 보고. canonical 원장 직접 수정 금지.

**유지보수:** 이 문서와 `task-mecca help`, `docs/cli-*.md`, 기존 role 문서, README의 명령은 함께 갱신하고 테스트한다. 문서만 먼저 수정하여 미구현 기능을 구현된 것처럼 광고하지 않는다.
