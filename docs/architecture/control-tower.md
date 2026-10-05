# Canonical Control Tower

Task Mecca의 runtime 관제는 **복수 센서, 단일 해석자, 다중 projection** 원칙을 따른다.

## 데이터 흐름

```text
Backlog / Hold review ─┐
Runtime hooks ─────────┤
Execution ledger ──────┼─> reconcileControlTower
Heartbeat registry ────┤        │
Git lifecycle ─────────┘        ├─ Canonical lifecycle
                                ├─ Canonical operational state
                                ├─ Attention reason
                                └─ Notification condition
                                         │
                     ┌───────────────────┼───────────────────┐
                     v                   v                   v
                  Web/UI             Dashboard       Notification journal
                                                             │
                                                             v
                                                      Channel dispatcher
                                                        (Telegram)
```

## 소유권

### Sensors
센서는 관측 사실만 제공한다. 상태를 사용자-facing 의미로 확정하지 않는다.

- backlog 파일 상태 및 hold metadata
- provider hook
- execution attempt ledger / binding
- runtime heartbeat
- Git lifecycle history

### Control Tower
`reconcileControlTower`가 센서 증거를 한 번만 통합한다.
`canonicalOperationalState`가 approval, intervention, stalled, interrupted,
runtime_unknown, finalize의 의미를 단 한 곳에서 결정한다.

Lifecycle의 Started는 canonical execution evidence가 있는 `lifecycleTimings`가 소유한다.
단순 `doing` 파일 이동은 observable runtime이 있는 경우 Started의 증거가 아니다.

### Consumers
Web, Dashboard, Attention, notification journal, Telegram은 canonical 결과를
projection하거나 전달할 뿐 runtime/backlog evidence를 다시 해석하지 않는다.

## 불변조건

1. Task별 canonical lifecycle은 하나다.
2. Task별 현재 operational interpretation은 하나다.
3. Started/Completed notification은 canonical lifecycle만 소비한다.
4. Operational notification은 canonical `notification_condition`만 소비한다.
5. 동일 execution attempt + 동일 condition은 한 episode로 취급한다.
6. 새 execution attempt는 동일 종류 condition의 새 episode가 될 수 있다.
7. notification journal은 의미를 판정하지 않는다.
8. channel dispatcher는 event를 필터링·전송·dedupe할 뿐 상태를 판정하지 않는다.

## 우선순위

runtime의 명시적 상태가 backlog hold 추정보다 우선한다. 예를 들어 hold가 사용자
입력을 가리키더라도 bound execution attempt가 completed라면 canonical 상태는
`finalize`다. 승인 대기는 `approval`이라는 first-class condition으로 유지한다.

## 변경 규칙

새 센서나 새 상태를 추가할 때 consumer에 switch/if를 추가하지 않는다.
센서 정규화와 Control Tower 판정만 확장하고, consumer는 canonical schema를
그대로 소비해야 한다.
