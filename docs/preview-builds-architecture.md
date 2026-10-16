# Task Mecca PR Preview Builds — 격리·신뢰·실행 계약 (AID-145)

## 목표 및 범위

Dev Web 사이드바에서 PR 목록을 최신 **완료된 빌드 시각** 순으로 보고 특정 PR/커밋의 Preview를 별도로 실행/열기/종료한다. Stable/Dev 서비스 자체의 업데이트나 프로젝트 마이그레이션 기능과 분리한다. 최초 구현은 단일 신뢰 저장소 `silverkhan/TaskMecca`의 PR에 한정한다.

## 가장 중요한 보안 판단

**PR 바이너리는 현재 사용자의 권한으로 실행하는 코드다.** Docker 없는 네이티브 프로세스 방식은 포트·HOME·테스트 프로젝트와 운영 파일의 *정상 동작 중 충돌*을 분리하지만, 호스트 파일·네트워크 접근을 막는 OS 샌드박스가 아니다. 자체 저장소에서 신뢰하는 코드만 명시적인 사용자 동의를 받아 실행한다.

1. **출처 검증:** 저장소가 정확히 일치하는 same-repo PR만 후보로 삼고, head SHA, PR 번호, 아티팩트 SHA, 성공한 지정 Actions 워크플로 실행을 교차 검증한다. Fork/외부 PR은 자동 실행 후보에서 제외.
2. **사용자 승인:** Preview는 조회·다운로드가 가능해도 **명시적인 [실행] 조작** 없이는 실행되지 않는다. 코드 신뢰 정책 위반 시 버튼 대신 사유를 표시한다.
3. **운영 격리:** 부모 Dev/Stable의 설치 위치·HOME·프로젝트·백로그 원장·Hook/알림 설정·토큰을 자식에 전달하지 않는다. Preview 테스트 프로젝트는 독립된 fixture로만 생성하고, 원본 운영 경로를 넘기지 않는다. 실행 파일을 부모 설치 파일로 교체하지 않는다.
4. **런타임 차단:** Preview 자식에 Telegram 전송, Web Push, 원격/Tailscale 서버, 시스템 터미널, 프로젝트 마이그레이션/업그레이드 및 쓰기 API를 허용하지 않는다. UI 숨김뿐 아니라 자식 프로세스의 라우팅/모드 검사로 강제한다.
5. **호스트 격리의 한계:** 사용자의 결정에 따라 Docker 없이 `os/exec`로 실행한다. 실행 전에 동일 OS 사용자 권한을 명시한 확인창을 표시한다. **임의 외부/Fork PR은 실행을 금지**한다. 악의적인 PR 코드를 안전하게 실행할 수 있다고 주장하지 않는다.
6. **제어 평면 보안:** Preview 관리 API는 Dev 모드 확인, loopback 또는 인증된 원격 접근 통제, 요청 오리진 검증, 명시적 mutation 헤더, 정확한 부모 인스턴스/프로세스 소유권 검증을 적용한다. 자식 Preview 프로세스는 부모의 비밀 관리자 토큰을 받지 않는다.

## 빌드 프로비넌스

기존 `.github/workflows/go-artifacts.yml`은 `pull_request` 이벤트에서 기본 checkout을 사용해 임시 **merge commit**을 빌드할 수 있고, 아티팩트를 `github.sha` 기준으로 기록한다. 이는 PR **head SHA**를 식별하려는 Preview 요구와 다르다.

- 워크플로는 PR에서 정확히 `github.event.pull_request.head.sha`를 checkout하여 head SHA 빌드만 산출한다.
- `workflow_dispatch`는 검증된 repo ref를 별도 경로로 다루며 Preview 자동 실행 대상으로 간주하지 않는다.
- manifest(`preview-build.json`)에 `repository`, `pr_number`, `head_sha`, `run_id`, `workflow`, `platforms` 등을 기록한다.
- 서버는 최신 PR head SHA와 빌드 manifest SHA가 **완전히 같고** 해당 workflow run이 성공하고 유효한 아티팩트인 경우에만 준비 완료라고 표시한다. zip 자체와 그 안의 바이너리·`SHA256SUMS.txt`의 무결성을 검사한다.
- 아티팩트 만료/CI 중지/조회 실패/불일치 상태는 성공으로 추정하지 않는다.

## 상태 전이 및 UI 계약

두 축으로 모델링한다.

- Build: `none` → `queued` → `in_progress` → `success` / `failure` / `expired`. `blocked`는 보안 정책상 실행 제한, `unavailable`은 API 오류를 뜻한다.
- Local: `stopped` → `downloading` → `starting` → `running` → `stopping` → `stopped` 또는 `error`. 빌드와 로컬 상태는 섞지 않는다.
- 빌드 성공/미실행: **실행**; 실행 중: **열기 / 종료**; 빌드 중/실패/만료/차단: 오류·로그·재빌드 안내.
- 기본 PR 정렬은 **완료 빌드 시각 내림차순**. 빌드 없는 PR은 PR 업데이트 시각을 fallback으로 사용하고 빌드 성공인 것처럼 표시하지 않는다.
- 새 PR head SHA가 나타나도 실행 중인 구 SHA 프로세스는 자동 교체하지 않고 **새 빌드 사용 가능**으로만 표시한다.
- Preview Web의 모든 화면에 `PREVIEW · PR #번호 · SHA`를 표기하고, 원래 Dev 탭 복귀 및 부모 제어를 통한 Preview 종료 제공.
- 다중 PR은 프로세스·포트·HOME·데이터·로그 별로 구분하고, 버튼 중복으로 복수 프로세스를 만들지 않는다.

## 테스트 게이트

- 비 Dev/Stable 접근, 다른 저장소/Fork PR, SHA mismatch, failed/expired artifact, 아티팩트 변조, rate limit, 빈 목록.
- Dev/Stable의 실제 프로젝트·라이프사이클·알림 원장·포트·바이너리의 불변성 확인.
- 동시 실행/중복 클릭/자식 crash/부모 재시작/포트 충돌 및 stale PID 재활용 방지.
- macOS Apple Silicon·Intel, Windows, Linux, 모바일/데스크톱과 한국어·영어.
- 실제 사용자 검증과 Stable 승격은 **독립 승인** 필요.

## 단계

1. **M1:** 위 신뢰/격리 계약과 PR/Actions read-only 조회, 아티팩트 프로비넌스/검증. 실행 API는 제공하지 않는다.
2. **M2:** 자체 저장소 신뢰 PR만 허용하는 네이티브 프로세스 관리(추가 설치 없음) 및 Dev Web UI/Preview 배너. 현재 OS/arch 전용 바이너리로 실행하며 요청한 PR 번호·SHA·Preview 모드를 Web health에서 확인한 후에만 탭을 연다.
3. **M3:** 동시 Preview, 수명/로그/보존 정책, E2E + 사용자 승인.

Linear: AID-145~154 / P-AID-15.

## Native Preview 실행 계약 (Docker 미사용)

- CI에서 검증한 현재 OS/arch별 바이너리를 별도 Preview 전용 인스턴스 폴더로 복사해 실행한다. 다운로드 캐시를 직접 chmod하거나 기존 설치 파일을 교체하지 않는다.
- `exec.Command`를 사용해 HTTP 요청 수명이 끝나도 유지되며, 실행 인자는 `web --foreground --host 127.0.0.1 --port <자동 포트> --project <합성 프로젝트> --no-open`으로 고정한다. 셸 해석이나 임의 명령 입력은 없다.
- HOME/USERPROFILE/APPDATA/XDG/TASK_MECCA_HOME을 분리하고 인증정보·SSH·프록시 등의 환경변수를 자식에 전파하지 않는다. Telegram 전송 비활성 및 Preview Web 읽기 전용 API 차단 정책은 방어 심층화이다.
- 준비 확인 응답은 반드시 `ok=true`, `preview_mode=true`, 정확한 `preview_pr`와 `preview_sha`를 포함해야 한다. 해당 기능이 없는 과거 PR은 준비 확인에서 거절·종료한다.
- 종료는 현재 부모 프로세스가 직접 `exec.Cmd.Start()`로 생성하여 보유한 `*os.Process` 핸들에만 적용한다. PID만 저장했다가 임의 프로세스를 찾아 종료하지 않는다. 자식 프로세스 예기치 않은 종료 시 UI 상태는 `error`로 전환한다.
- **보안 경고:** 네이티브 자식은 사용자 파일과 네트워크에 접근할 수 있다. 실행 확인창이 보안 샌드박스가 되는 것은 아니므로, 신뢰되지 않는 코드·외부 PR은 실행하지 않아야 한다. OS 수준 격리가 필요한 사용자에게 이 모드를 권장하지 않는다.
- Docker·VM·추가 프로세스 감독 패키지 설치는 필수 조건이 아니다. 서비스 부모 재시작 후 잔존 자식 발견 및 복구·정리 정책은 AID-153에서 다룬다.
