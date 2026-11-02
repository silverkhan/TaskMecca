# AID-246 · 감시 비용 진단 (Dev)

## 목적과 제한
macOS에서 관측된 Task Mecca CPU 75.8%는 특정 순간의 프로세스 CPU 점유율이며 배터리 방전률의 직접 측정값이 아니다. **프로파일링 전에는 원인을 확정하지 않는다.** 이 단계는 감시 알고리즘이나 알림 정책을 변경하지 않고 관측만 추가한다.

## 진단 활성화
웹 서비스 실행 환경에 `TASK_MECCA_MONITOR_PERF=1`을 설정하고 서비스를 재시작한다. 기본값은 꺼짐. 활성화 시 약 60초마다 로그에 집계치가 출력된다. 설정은 실행 중에 재시작 없이 변경할 수 없다.

- `attention_refresh`: 실제 실행한 AttentionSnapshot / 이벤트 전달 처리 시간, 주의 대상 수 및 알림 이벤트 수, payload revision 변화 횟수.
- `attention_skipped`: 프로젝트 monitoring guard 때문에 실행하지 못한 refresh 횟수.
- `operation_scan`: 프로젝트 운영 감시 전체 소요 시간, 생성된 stage 및 incident 수. 여기서 `changed`는 측정하지 않는다.
- `operation_skipped`: 운영 스캔이 monitoring guard에서 거부된 횟수.

계측 결과의 `total`은 CPU 시간 아니라 **경과 시간(wall time)** 합계다. 잠금 대기, 디스크 I/O, Telegram 전송도 포함될 수 있다. `items`는 현재 **주의 대상 또는 stage**의 수이지 전체 백로그나 검사 실행 수가 아니다. 상세 대상 카운트는 후속 계측 후보로 보존한다. 로그에는 프로젝트 경로·에이전트·백로그 ID·이벤트 본문을 출력하지 않는다.

## 실제 CPU 표본 (macOS)
웹 UI를 모두 닫아도 발생하는지부터 비교한다. 아래 명령으로 실행 PID를 확인하고, **높은 CPU가 재현되는 순간** 샘플링한다.

```bash
ps -axo pid,ppid,%cpu,%mem,etime,command | grep '[t]ask-mecca'
sudo sample PID 10 -file ~/Desktop/task-mecca-sample.txt
```

PID를 실제 숫자로 교체한다. sample 결과의 hot stack에서 `AttentionSnapshot`, `BuildLedger`, `CachedCatalog`, `Discover`, `NotificationEvents`, GC, runtime lock 및 웹 요청 경로를 비교한다.

## 비교 시나리오
1. 유휴 상태, 웹 UI 종료: 2~3분간 CPU/로그 관찰
2. 웹 UI 단일 탭: 동일하게 관찰
3. 프로젝트 여러 개와 탭 전환 반복 후: 피드 개수 증가와 CPU 변동 비교
4. Worker 실행 및 handoff/hold/완료 이벤트 중: 정확한 알림·무신호 감시 유지 확인
5. 프로젝트 감시 pause/remove/re-enable: 추가 계산 및 누락 알림 확인

## 경로별 대상 분류 (코드에서 확인됨)
- `operationProjects`: 등록·존재·monitoring=true 관리 프로젝트, 일부 미등록 primary fallback. 제거/중지 이력은 무조건 대상으로 복귀하지 않도록 필터됨.
- `ensureOperationNotificationFeed`: 등록 활성 프로젝트별 브라우저가 없어도 이벤트 감시 피드 생성; 이것을 일률 중지하면 다중 프로젝트 알림 회귀.
- `attentionFeed`: 한 project/root마다 3초 ticker; SSE 구독 해제는 subscriber를 제거하지만 피드 종료 조건과는 별개.
- `AttentionSnapshotFromRows`: 캐시된 카탈로그의 preferred rows를 순회하며 active doing/hold와 canonical 알림 상태를 구성.
- `scanOperationProjectsWithPriority`: 15초 ticker, 프로젝트별 BuildLedger 및 운영 상황 판정.
- `cachedBacklogDiscover`: 5초 TTL 만료 후 다음 호출에서 비동기 디렉터리 탐색 가능; 매 5초 무조건 실행 아님.

## 단계 완료 기준과 체감 평가
Dev 빌드/자동 회귀가 통과하고 CPU 프로파일·시나리오별 계측 자료가 모여야 P0 진단을 완료한다. 계측이 추가됐다는 사실만으로 성능 문제가 해결된 것은 아니다. 실제 개선의 수용은 사용자의 Dev 실사용 **체감**을 기준으로 후속 P1~P4 단계에서 순차 확인한다.
