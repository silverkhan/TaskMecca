# A-65 기존 UX / 승인 재질 확대 대응표

비교 원본은 `origin/dev`의 `f0a732e3dd8234a663dccf3c6937fc9dc674917b`와 그 `goassets/template/_task_mecca/framework/web/app.js`다. 이 파일은 원본 legacy application 역할이며 이번 후보에서는 실행하지 않는다. 함수 목록과 SHA-256을 `handoff-evidence/accepted-expansion/legacy-functions.txt`, `legacy-reference.sha256`에 보존했다. 과거 aa5의 5개 메뉴 제안은 현재 정보구조가 아니다. 전체 수용 판단은 Controller가 실제 후보와 아래 잔여 검증을 함께 확인한다.

| 기존 완결 행동 | native module 목적지 / 구현 | 검증 경계 |
|---|---|---|
| Global Hub 첫 화면, 여러 프로젝트 관제, 사용자 조치·진행·최근 활동·전체 프로젝트 page | `home.js`, `data.js`의 overview 및 cursor 복구 유지 | 기존 core 검사, 합성 실제 Go API |
| SYSTEM/BACKLOGS/OPERATIONS/HELP 위치 | `shared.js` material rail, Preview Builds는 SYSTEM | PC/mobile 메뉴 실제 조작 |
| 프로젝트 직접 진입, 닫기·다시 열기, 저장된 열린 프로젝트 보존 | `expansion.js`, 기존 `task-mecca-open-projects` 읽기·쓰기 및 경로 fallback | 새 browser 계약 검사 |
| 자동/펼침/최소 sidebar, mobile 열고 닫기 | `expansion.js`, `expansion.css`, 기존 compact 값 호환 | hover/focus/ESC, mobile 메뉴 |
| Backlog 검색·복수 상태·전체 해제·정렬·page·태그·namespace·위치 | `backlog.js`, `main.js`, `data.js`, `state.js`; 네이티브 change 모델 위 공통 combo | core / 새 복수 상태 검사, catalog/Auto는 별도 확대 |
| 행 키보드, 전용 상세, back/refresh/deepURL·선택·scroll 복원 | 기존 `main.js`·`detail.js` 유지 | core 복귀·문서 우선 읽기 검사 |
| slim lifecycle, connected intervals, individual tooltip/pinned reader, zoom/fit/wheel/pinch | 기존 `workflow.js`, `workflow.css` 그대로 | 기존 8개 native workflow 검사, fixture 측정은 실제 사용자 수행 근거 아님 |
| Workload, 현재 Root, 이전 Root page·정리계획, 실행 이력 필터/page | `management.js`, `/api/workload`, `/api/runtime/{history,root-sessions}` | 실제 Go 읽기 + 격리 browser 동작; provider 로컬 기록 범위만 |
| Hook 상태/범위별 설정 | `management.js`, 기존 hooks API의 device/project/global 의미 유지 | 쓰기 fixture만, 운영 Hook 변경 0 |
| 운영 감시·단계·사건·복구 근거 | `management.js`, `/api/operations` | 읽기 API. 원장 writer 권한을 UI로 확장하지 않음 |
| 알림센터 현재 사용자 조치·전송 이력·프로젝트/종류 설정 | `notifications.js`, 기존 canonical reason 필터와 API | 다중 프로젝트 history·unknown/sent 구별, 실패 시 설정 복원 |
| 웹 알림·Push 연결/해제/시험·Telegram 공통/개별 연결/수신자검색/종류/비활성 | `notifications.js`, 기존 same-origin action API; canonical event claim/ack/fail | 격리 API 계약 검사. 외부 Telegram/Push 기기 전달 미실행 |
| 프로젝트 등록/감시 중지·재개/보관/복원/이력삭제/지침갱신 | `management.js`, `/api/hub/projects`, `/api/migrate` | modal 확인, 409 plan_digest 시 backup 선택; 쓰기 fixture만 |
| 로그/실행 저장소 다중 프로젝트/전역 중복제외/보호/정리 | `management.js`, storage API | 전역 log action 1회, protected 삭제버튼 없음, 403 오류 검사 |
| 사용자 매뉴얼·세션/Root prompt 읽기·복사 | `management.js`, `/api/manual?lang` | source Markdown renderer 유지 |
| 업데이트 내역·page·버전별 detail/읽음 | `management.js`, release-notes API | 로컬 release 자료 읽기, 사용자가 펼친 버전만 기록 |
| 버전조회/upgrade/channel/restart/migrate | `management.js` settings | 요청 응답과 실제 재시작 완료를 구분. 실설치·채널전환 미실행 |
| Preview 빌드/검색/필터/재빌드/인스턴스중지/실행 전환 | `preview.js`, SHA-pinned 기존 API | local process와 remote build 별도, fresh server identity 조건·확인 후 실행, 격리 fixture만 |
| actual PR identity·Dev 복귀 false-success 방지 | 기존 `environment.js` 전부 유지, mobile unknown summary 표시 수정 | P0 unit 및 browser positive/denied/mismatch/timeout |
| 터미널/security/provider 기능 | 기존 `/terminal`을 실제 iframe으로 마운트, 원래 terminal runtime 사용 | URL placeholder가 아닌 동작 화면; 실제 shell 명령 실행 미실행 |
| Markdown 코드 복사 / Mermaid 원문·diagram | `document-tools.js`, 기존 optional local classic bundle 및 exact-pinned 11.10.0 CDN fallback, strict renderer | 코드 원문 보존, 실패 시 원문 제공; 외부 runtime는 지원 범위와 실측을 구별 |
| 백로그 진단/복원 요청 | `management.js`, `/api/issues` health 그룹·diagnostics 및 Agent 복원요청 복사 | 진단 자료 읽기. Agent에 자동 전송하지 않음 |
| 원래 로고·한영·Light/Dark/System·고정 배경·image/solid | 기존 boot/state, `expansion.js`, accepted material assets | 선택 복원 및 기능 검사 |
| input/select/combo/table/header/menu/popover/modal 공통 재질 | accepted material CSS 및 `expansion.css`, 기존 `select-ui.js` 부분 기능 모듈 재사용 | text/icon은 filter 밖, 행 blur/canvas/CPU optics/continuous rAF 없음 |

필요한 관리 동작은 API가 가진 권한과 확인을 그대로 사용한다. 검토 환경/운영 설치 권한을 이번 구현자가 획득하거나 실행하지 않는다. 미실행 물리기기·Safari/Firefox·기기 전달·전체 설치 전환과 최종 성능 gate는 보고서에서 별도로 기록한다. 이 대응표 자체는 AC 체크나 A-65 done 선언이 아니다.
