# 라이프사이클 조회와 관제 계산의 분리

[AID-237](https://linear.app/aiden7/issue/AID-237)의 개발 범위는 조회 경로 분리, 정합성·성능 회귀 검증 및 Dev 반영이다. 실사용 체감 확인은 Dev 운영에서 별도로 수행하고 Stable 반영은 별도 승인한다.

## 요청과 계산의 경계

`GET /api/tasks/:id?projection=lifecycle`는 Canonical Control Tower가 계산한 작업별 결과만 읽는다. 이 요청에서는 catalog, Git 이력, Runtime/Handoff 원장을 스캔하거나 전역 reconciliation을 실행하지 않고, 프로젝트 monitoring mutex를 기다리지 않는다. 기존 전체 상세·목록·관제 경로의 판정 엔진을 복제하지 않는다. `projection=content`는 본문을 먼저 표시하며 이후의 최초 라이프사이클 조회도 동일한 읽기 API를 사용한다.

조회 결과가 아직 없으면 HTTP 202와 `snapshot.trust=pending`을 반환한다. 빈 사건 목록을 착수·완료의 근거로 해석하면 안 된다. 조회 결과를 만드는 계산은 별도 작업으로 합쳐서 실행하며, 기존 프로젝트 monitoring guard를 통과해야 한다. 중지·미등록 프로젝트의 조회는 기존과 동일하게 거부한다. 이미 결과가 있는 경우에는 다른 계산이 오래 걸려도 이전 결과와 그 신뢰 상태를 즉시 반환한다. 확인된 결과에서 작업이 없으면 404이며, 검증 중인 누락은 202로 구분한다.

응답의 `snapshot` 필드:

| 필드 | 의미 |
| --- | --- |
| `version` | 이 프로세스에서 게시한 결과의 식별자. 사건 ID나 실행 attempt ID가 아니다. |
| `computed_at` | 단일 판정 엔진으로 결과를 계산한 시각 |
| `validated_at` | 입력의 변경 여부를 마지막으로 검사한 시각 |
| `source_revision` | 백그라운드 검사에서 얻은 입력 식별값 |
| `trust` | `pending`, `verified`, `stale`, `error` |
| `refreshing` | 별도 계산 또는 monitoring guard 대기 여부 |
| `refresh_timings_ms` | 최근 별도 계산의 `monitor_wait`, `catalog`, `control`, `runtime`, `lifecycle_git`, `activity_handoff`, `total` 비용 |

`verified`는 기록된 확인 시점의 결과라는 의미이며 현재 시각까지 변화가 없다는 보장이 아니다. 알려진 변경은 즉시 오래된 상태로 표시하고, 계산·검증이 30초를 넘긴 결과도 오래된 상태로 표시한다. 시간에 따라 달라지는 관제 판정 역시 이 재계산 범위 안에서 갱신한다. `stale`이나 `error`에서도 과거 사건의 정체성과 시각을 보존하고, UI가 이전 확인 결과임을 알리며 재시도를 제공한다. 사건·착수·완료·검수 판정은 기존 Canonical Control Tower만 수행한다.

## 변경 감지와 복구

같은 프로세스의 Runtime event, assignment, lifecycle 전환, Handoff 기록은 메모리 내 무효화 신호를 보낸다. 신호 자체는 사건·실행 근거가 아니며 새 판정을 만들지 않는다. 변경이 감지된 프로젝트의 결과는 오래된 상태로 표시하고 별도 계산에서 갱신한다.

다른 CLI 프로세스, 파일 수정, 놓친 신호는 조회가 이어지는 동안 최대 초당 한 번의 별도 입력 검사로 복구한다. backlog 파일, durable lifecycle/assignment, Runtime 실행 이력·관측, Handoff, Git HEAD를 검사한다. 일반 요청과 입력 검사는 프로젝트별로 합쳐 실행하므로 동시 요청 수만큼 전역 계산을 반복하지 않는다. 다른 프로젝트의 입력은 해당 결과를 무효화하지 않는다.

변경 시에는 기존 단일 엔진으로 프로젝트 결과를 보수적으로 다시 계산한다. 입력 검사를 생략하고 개별 사건에서 새로운 판정을 합성하지 않는다. 파일 시각·크기를 보존한 변경도 복구하도록 조회가 이어질 때 30초마다 전체 재구성과 parsed catalog 무효화를 수행한다. 조회가 없을 때는 반복 감시 루프를 유지하지 않는다. 활성 상세 화면은 신호 변경 여부와 별개로 최대 10초 간격으로 라이프사이클을 재확인하므로 Git 변경이나 놓친 신호도 복구할 수 있다. 오래된 결과나 재시작 대기에는 횟수가 제한된 추가 재확인을 수행한다. Web 재시작 뒤 저장된 조회 결과를 신뢰하지 않고 durable 근거로 다시 계산한다.

입력을 계산 전후로 확인해 동시 변경이나 계산 중 근거 복구가 있으면 최신성을 확정하지 않고 다음 계산에서 재확인한다. 잘못된 Runtime/Lifecycle/Handoff 기록이나 읽기 실패를 성공한 빈 목록으로 숨기지 않는다. 이전 사건 목록은 유지하고 오류를 표시한다.

## 서버와 브라우저 계측

읽기 API의 `Server-Timing`은 `monitor_wait`, `selection`, `catalog`, `control`, `projection`, `detail`을 구분한다. lifecycle 읽기의 `monitor_wait`, `catalog`, `control`은 0이며 별도 계산의 비용은 `snapshot.refresh_timings_ms`에서 확인한다. `lifecycle_git`은 durable lifecycle·Git 처리 묶음이고 `activity_handoff`는 활동·검수·Handoff 판정 묶음이다. 이 묶음을 개별 함수의 확정 병목으로 단정하지 않는다.

브라우저는 최근 50개 계측을 `state.taskLoadTimings`에 보존한다. 개발자 도구에서 다음처럼 확인할 수 있다.

```javascript
console.table(state.taskLoadTimings)
```

`content:load`와 본문 렌더 후 `content:first`, 최초 라이프사이클 `lifecycle:first`, 독립 갱신 `lifecycle:refresh`를 구별한다. 각 항목은 요청 URL, `browser_ms`, Resource Timing에서 얻은 `ttfb_ms`·`transfer_ms`, 응답의 `server_timing`, 접속 경로를 담는다. Performance API가 제공되지 않으면 해당 값은 `null`이며 0으로 추정하지 않는다. `browser_ms`는 요청·JSON 처리와 해당 UI 갱신을 포함하지만 사용자 체감 전체를 대변하지 않는다. 본문 최초 표시는 가능한 경우 다음 animation frame에서 기록한다.

`route=localhost`와 `route=tailscale` 결과를 별도로 모은다. 다른 호스트나 Tailscale IP 직접 접속은 `other`로 남기므로 측정자가 실제 경로를 기록해야 한다. TTFB에는 서버와 네트워크가 함께 포함된다. 서버 헤더 및 브라우저 전체 시간과 함께 비교하고, 로컬 HTTP 측정으로 원격 Tailscale이나 실제 모바일 브라우저의 속도를 추정하지 않는다.

라이프사이클 갱신은 해당 섹션만 교체한다. 본문 DOM·스크롤·다른 펼침 섹션·집중된 본문 요소를 다시 만들지 않는다. 최초 준비 중에는 횟수가 제한된 재확인을 수행하고, 실패 시 본문과 마지막 사건 목록을 유지하며 다시 확인 버튼을 제공한다.

## 자동 회귀 검증

```bash
go test ./internal/backlog ./internal/webui -run 'TestLifecycleProjection' -count=1
go test -race ./internal/backlog ./internal/webui -run 'TestLifecycleProjection' -count=1
node --test tests/web_detail_lifecycle_live.test.cjs tests/web_task_detail_prefetch.test.cjs
TASK_MECCA_LIFECYCLE_PERF=1 TASK_MECCA_LIFECYCLE_PERF_REPORT=/tmp/lifecycle-performance.json go test ./internal/backlog -run '^TestLifecycleProjectionPerformanceRegression$' -count=1 -v
TASK_MECCA_LIFECYCLE_HTTP_REPORT=/tmp/lifecycle-http.json go test ./internal/webui -run '^TestLifecycleProjectionLocalhostTiming$' -count=1 -v
```

100/500/1,000개 backlog와 같은 수의 Runtime/Handoff 사건으로 최초 구성·변경 없는 별도 검사·반복 읽기·동시 읽기를 측정한다. 읽기 요청의 전역 계산 및 입력 검사 횟수가 0인지, catalog lock을 잡아도 읽을 수 있는지가 우선 기준이다. 시간은 동일 fixture의 기존 전체 상세 경로와 같은 실행의 100건 분포를 기준선으로 자동 비교한다. 환경마다 달라지는 고정 밀리초를 적용하지 않는다. 큰 입력의 읽기 중앙값에는 같은 실행의 100건 p95 대비 4배까지의 잡음 허용 폭을 두고, 결정적인 작업량 기준과 함께 사용한다. 이 폭은 성능 목표나 실사용 개선 배수가 아니다.

PR CI와 Dev 배포 모두 이 검증을 수행하고 성능 JSON을 artifact로 남긴다. 보고서는 Go 버전·OS·아키텍처·CPU 수와 측정 시각을 포함한다. 새 환경의 값을 이전 환경의 절대 지연 수치와 섞어 비교하지 않는다.

실제 브라우저·모바일 Tailscale의 체감 개선은 Dev 반영 후 위 계측으로 별도 확인해야 한다. 개발 중 in-process와 localhost TCP/HTTP 결과가 통과해도 측정하지 않은 환경의 개선 효과나 확정 원인은 주장하지 않는다.
