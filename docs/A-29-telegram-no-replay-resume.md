# A29 Telegram 유지보수 후 과거 알림 없이 재개

## 필요한 별도 경계

`TASK_MECCA_TELEGRAM_TRANSPORT=disabled`는 네트워크·설정·delivery ledger 갱신 없이 즉시 반환한다. 단순 gate 해제/재시작은 유지보수 중 발견된 이벤트와 기존 failed/sending 재시도를 재생할 수 있다. 기존 활성화/프로젝트 enable 토글은 설정을 변경하고 원래 시도 근거를 독립 cutover로 보존하지 않으므로 A29에는 사용하지 않는다.

`notifications suppress-before-resume`는 configuration/token/chat/Enabled/recipient/ActivatedAt/Delivered를 읽거나 쓰지 않는다. 별도 `resume_boundary` 및 `suppressed_before_resume`를 notification delivery ledger에 커밋한다. suppression은 sent가 아니며 기존 attempts/unknown success/duplicate_possible/error/retry deadline/attempt history를 지우지 않는다. 기존 sent/consumed/suppressed와 미지의 원장 확장 사실도 보존한다. 상태 진단에는 opaque 확장 사실이나 recipient 정보가 노출되지 않는다.

## Controller 전용 안전 절차

Worker는 다음 운영 절차를 실행하지 않는다. 사용자 승인 및 A28 실제 경고 보정 검증을 마친 Controller만 수행한다.

1. 실제 대상 프로젝트 목록을 확정한다. registry 등록/해제는 하지 않는다. missing/archived/차단된 프로젝트에 임의 파일을 생성하거나 guard를 우회하지 않는다.
2. 기존 Web/CLI/다른 writer의 process transport gate를 disabled로 유지하고 quiesce한다. cutoff 이후 gate를 자동 해제하는 명령은 없다. secret 포함 configuration/원장/서비스 backup은 repository 밖 0700 디렉터리와 0600 파일로 Controller가 별도 보존한다.
3. A29 구현이 포함된 새 binary의 exact source/artifact SHA·hash·version을 확인한다. 이전 binary는 resume 경계를 알지 못하므로 이전 writer가 gate 해제 후 실행되지 않도록 한다.
4. A28 검증 후 단일 승인 UTC RFC3339 cutoff를 선택하고 모든 enabled target에 같은 cutoff로 명시 커밋한다. cutoff는 미래로 설정할 수 없고 기존 boundary보다 뒤로 이동할 수 없다. 다음은 placeholder이며 실제 경로/시각은 Controller가 확정한다.

```sh
TASK_MECCA_TELEGRAM_TRANSPORT=disabled task-mecca notifications suppress-before-resume \
  --project /ABSOLUTE/APPROVED/PROJECT \
  --confirm-project /ABSOLUTE/APPROVED/PROJECT \
  --cutoff 2026-10-07T00:00:00Z --json
task-mecca notifications resume-status --project /ABSOLUTE/APPROVED/PROJECT --json
```

5. 각 명령 exit0과 resume-status의 exact cutoff/evidence ID, suppression count, sent-preserved count 및 백업 SHA256을 확인한다. 하나라도 실패·corrupt snapshot·unknown 결과면 **전체 gate를 disabled로 유지**하고 근거를 조사한다. output 실패도 commit 여부가 불확실하므로 gate를 유지한 채 resume-status로 확인한다. command는 gate를 해제하거나 서비스/registry/설정을 변경하지 않는다.
6. Controller만 supported Web restart로 process gate를 해제하고 새 binary/service/API hash/version 및 운영 UI 차단 표시를 확인한다. user settings를 초기화하거나 token/chat을 재설정하지 않는다.
7. cutoff 이후 생성된 정상 신규 검증 이벤트 하나만 최소 발송하여 실제 수신·sent record·중복 방지를 확인한다. 과거 이벤트를 test message로 재사용하지 않는다. 신규 실패는 truthful failed/uncertain으로 남고 기존 retry 경로를 따른다. 실제 발송 성공/수신은 Worker 합성 회귀로 대신 주장하지 않는다.

## 경계와 실패 의미

- 최초/전진 cutover의 **기존 unsettled records 전체**는 당시 pending backlog로 억제한다. future/invalid/unknown/sending도 sent로 위장하지 않는다.
- snapshot 이벤트 및 이후 늦게 발견된 이벤트는 `event_at <= cutoff`, 누락·invalid·미래 시각, 기존 event/created/attempt evidence가 cutoff 이전인 경우 억제한다. 알려진 event ID 시각을 바꿔도 재활성화하지 않는다.
- 동일 cutoff 재실행은 무변경으로 idempotent하며 cutoff 이후 정상 failed retry는 소비하지 않는다. 전진 cutoff는 이전 boundary를 resume_history에 보존한다.
- saved configuration는 command 전후 byte-for-byte 동일하다. existing sent 기록·attempt details·opaque evidence를 보존하고 성공 횟수를 조작하지 않는다.
- process mutex + 동일 cross-process delivery file lock 아래에서 snapshot/backup/atomic ledger rename을 직렬화한다. late event journal 관측은 persisted cutoff로 커버한다. event journal을 쓰지 않는다.
- 기존 delivery ledger를 0600 byte-exact backup + SHA256으로 보존하고 backup 성공 후 fsync/atomic ledger persist를 수행한다. backup/commit 실패는 기존 ledger/config/boundary를 그대로 두고 gate는 disabled다. partial backup도 삭제하지 않아 forensic 자료를 보존한다.
- corrupt ledger/boundary/snapshot 및 symlinked ledger/backup ancestor는 fail-closed다. API는 network 호출을 하지 않는다. 일반 Deliver는 malformed boundary에서 네트워크 이전에 실패한다.
- ledger backup 위치는 해당 project의 `_task_mecca/.runtime/notifications/resume-backups/`이며 credentials/config는 포함하지 않는다. Controller의 secret configuration backup과 구분한다. 토큰/수신처를 출력하는 raw config 진단을 사용하지 않는다.

## 변경 범위

notify delivery ledger/evidence/resume API와 CLI·회귀·문서만 변경했다. main.go는 별도 notifications dispatcher 한 줄만 추가했다. A28 operation projection, A14 held renderer, Web/server/UI는 변경하지 않았다. dev 통합, 운영 적용, Linear/canonical 및 cleanup은 Controller 소유다.
