# 계약 승인·정책·보충 이벤트 (AID-208)

CLI는 사용자 결정을 **추정하지 않는다**. Root는 실제 대화에서 승인된 내용과 출처를 정확히 기입한다. 계약 원문은 덮어쓰지 않고, 별도 append-only 이벤트를 남긴다.

## 기본 사용

```bash
task-mecca contract decision show --project /PROJECT --task B-508 --json

task-mecca contract decision record --project /PROJECT --task B-508 \
  --event-id approval-20261016-b508 --kind execution_approved \
  --actor /root --evidence "사용자가 명시적으로 승인한 대화 참조" \
  --user-confirmed --json

task-mecca contract decision record --project /PROJECT --task B-508 \
  --event-id policy-pending-01 --kind policy_pending --policy-key universe \
  --actor /root --evidence "확인이 필요한 정책 논의 참조" --json

task-mecca contract decision record --project /PROJECT --task B-508 \
  --event-id policy-confirmed-01 --kind policy_confirmed --policy-key universe \
  --actor /root --evidence "사용자가 정책을 확정한 대화 참조" --user-confirmed --json

task-mecca contract decision record --project /PROJECT --task B-508 \
  --event-id supplement-01 --kind supplement --body-file amendment.md \
  --actor /root --evidence "사용자가 보충 계약을 확정한 대화 참조" \
  --user-confirmed --json

task-mecca contract decision record --project /PROJECT --task B-508 \
  --event-id revoke-01 --kind execution_revoked \
  --actor /root --evidence "사용자가 실행 승인을 철회한 대화 참조" \
  --user-confirmed --json
```

### 불변 조건

- `execution_approved`는 **사용자가 실제로 실행을 승인한 증거**를 전달해야 한다. `--user-confirmed`는 CLI가 사용자의 실제 결정을 판독했다는 뜻이 아니라, 호출자가 그 명시적 결정과 근거를 제공한다는 선언이다.
- 등록된 상태와 실행 허용 상태는 별개다. `contract apply`는 등록만 하며 `execution_approved` 이벤트는 만들지 않는다.
- 과거 snapshot의 `execution_authorized=false` 또는 과거 승인 이벤트를 수정하지 않는다. `show`는 시점순 결정 이력과 현재 `execution_authorized`를 각각 반환한다.
- 보충 또는 정책 변경 뒤 계약의 effective SHA가 변하면 과거 승인만으로 실행할 수 없다. **새로운 명시적 승인**을 기록한다.
- 런타임의 `handoff prepare`는 별도 협업 전송 절차다. 이 명령만으로 native Agent 전송·상태 변경을 했다고 주장하지 않는다.
- `status=unknown`은 이벤트 손상 등 확인 불가 상태이며 실행 허용이 아니다. 사용자 정책 미확정은 `policy_pending`으로 표시한다.
- 동일 event ID/동일 내용 반복 호출은 기존 결과를 반환하고, 서로 다른 내용으로 재사용하면 `DECISION_ID_CONFLICT`로 거부한다.
