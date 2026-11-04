# PR Preview 조회 복구와 적용 경계

PR #348의 빌드는 존재했지만 실행 중인 Web 목록에서 조회하지 못했습니다. 2026-11-05 확인한 기존 원격 HEAD `5d9c85fca75274a6026e1beb42bd577a690e3143`의 run `41835195075`, artifact `13325376218`은 성공·미만료 상태였습니다. 운영 Web `0.3.6-dev.66`의 조회는 GitHub 제한/접근 오류로 실패했고, 기존 메타데이터 Client는 지정 환경 토큰만 사용했습니다. 또한 trusted Dev workflow의 인용되지 않은 `run-name: Preview PR #...`에서 `#` 뒤가 YAML 주석이 되어 실제 제목이 `Preview PR`로 잘렸습니다. **빌드 성공, 수정 Client의 검증 성공, 운영 Web의 이용 가능은 별도 상태입니다.**

## 수정 범위

메타데이터 조회는 명시적 `TASK_MECCA_GITHUB_TOKEN`을 우선합니다. 없을 때 이미 로그인된 gh 인증을 Client 메모리에서 한 번 재사용합니다. 값은 명령줄 인자·로그·파일·Preview 자식 환경에 전달하지 않습니다. gh 미설치/미인증이면 공개 조회가 가능하며, 명시적 인증이 잘못되었다고 다른 계정 인증으로 자동 교체하지 않습니다. `RequestBuild`의 기존 명시적 Actions:write credential 요구는 그대로입니다. 조회·다운로드가 자동 dispatch나 전환 권한을 부여하지 않습니다.

HTTP 401은 인증, 429 또는 403의 남은 요청 수 0/Retry-After는 한도, 그 외 403은 접근 거부로 분류합니다. 공개 오류는 GitHub 응답 본문, 네트워크 오류의 signed download URL, 자격증명 값을 반영하지 않습니다. UI는 실패를 빈 목록으로 표시하지 않고 빌드 유무를 확인할 수 없다고 알립니다. 기존 목록은 마지막 조회 결과로 표시하고 재조회 성공 전에는 전환·재빌드 버튼을 비활성화합니다.

브랜치의 workflow 제목을 인용했습니다. 이는 현재 trusted Dev workflow를 바꾸거나 과거 run 제목을 고치지 않습니다. 과거 제목 `Preview PR`의 복구는 다음을 모두 요구합니다.

- 정확한 `go-artifacts.yml` workflow ID/path/active metadata, same-repository run·head repository 이름/ID.
- `workflow_dispatch`, `dev`, 성공 완료, 유효한 Dev run SHA, 알려진 bare 제목만 후보로 허용.
- 현재 same-repository PR의 정확한 HEAD SHA가 들어간 artifact 이름, 해당 run ID·Dev SHA·branch·repository ID와 artifact의 `workflow_run` 관계 일치.
- 미만료 artifact. 다운로드 시 기존 ZIP provenance의 repository/PR HEAD SHA/run ID/workflow/schema, checksum, 복귀 protocol gate를 그대로 통과.

Dispatch의 run SHA는 Dev workflow SHA입니다. PR 소스 SHA는 artifact 이름과 ZIP manifest에서 별도로 검증합니다. 제목만 느슨하게 매칭하거나 fork·다른 workflow/run/SHA·만료·변조·비지원 protocol을 허용하지 않습니다. 한 번의 목록 요청에서는 run별 artifact metadata를 재사용해 PR마다 같은 요청을 반복하지 않습니다. `enter-pr`의 `ListBuilds → DownloadVerified(protocol=true) → ListBuilds 재확인 → 설치/전환` 순서는 변경하지 않았습니다.

## 실행·설치 없는 실제 GitHub 검사

`internal/preview/live_readonly_test.go`의 opt-in 검사만 사용합니다. 일반 CI에서는 skip합니다. 현재 PR의 정확한 SHA를 명시해야 하며 이 명령은 artifact를 검증된 비실행 cache로 내려받을 뿐 설치·실행·dispatch·환경 전환·서비스 재시작을 하지 않습니다. 운영 Web port도 사용하지 않습니다. 기존 gh 로그인을 사용하며 토큰 값을 터미널에 출력하거나 새 파일에 저장할 필요가 없습니다.

```sh
TASK_MECCA_LIVE_PREVIEW=1 \
TASK_MECCA_LIVE_PREVIEW_PR=348 \
TASK_MECCA_LIVE_PREVIEW_SHA='<현재 PR의 정확한 40자 HEAD SHA>' \
TASK_MECCA_LIVE_PREVIEW_CACHE="$PWD/handoff-evidence/pr348-preview-recovery/artifact-cache" \
go test ./internal/preview -run '^TestLivePreviewReadOnlyProbe$' -count=1 -v
```

`CACHE`를 생략하면 테스트 전용 임시 디렉터리를 사용합니다. 출력 JSON은 PR/SHA/run/artifact/platform/binary bytes/SHA256, `protocol_verified`, `metadata_rechecked`, `installed=false`, `executed=false`, `transitioned=false`입니다. 토큰·signed URL은 출력하지 않습니다. 새 최종 commit을 push하고 CI/Preview가 완료되면 **그 새 원격 HEAD SHA로 다시 실행**해야 합니다.

이번 기존 원격 `5d9c85fc` 대상 검사에서는 darwin/arm64 binary 12,308,242 B, SHA256 `a6ad5e542bad94ff1d3db9605b0cc67d0d60366c1dc22cc501cd82802069efc0`, protocol·checksum·다운로드 뒤 metadata 일치를 확인했습니다. 이는 새 수정 commit의 원격 artifact 검증이나 운영 Web 적용 성공을 의미하지 않습니다.

## 현재 운영 적용은 별도 판단

현재 `.66` Web은 수정 소스로 자동 교체되지 않습니다. 기존 Web의 catalog가 실패하는 상황에서 `enter-pr`로 수정 catalog 자체를 설치하려 하면 같은 bootstrap 경계에 걸립니다. trusted Dev workflow의 인용 수정도 별도 승인된 Dev 반영 전에는 현재 dispatch에 적용되지 않습니다. `TOKEN=... task-mecca web restart`로 launchd 인증 전달이 해결된다고 안내하지 않습니다.

이 작업은 설치본 교체·재시작·운영 환경 전환을 승인받지 않았습니다. 실제 적용은 다음 사항을 Root/사용자와 확정해야 합니다.

1. 수정 catalog가 포함된 **지원되는 Dev 배포/설치 대상의 정확한 버전·SHA·checksum·설치 경로**, 현재 설치본의 보존·복구 방법.
2. 운영 Web을 해당 실행본으로 갱신하는 지원 절차와 영향(연결 종료/재시작), 승인된 시간 및 실패 시 복구 기준.
3. 갱신 후 `GET /api/preview/builds`가 현재 PR/SHA를 반환하는지 확인한 뒤 제품의 PR 환경 전환 경로를 사용. 기존 Dev 복귀/rollback 정보와 `return-dev` 지원도 사전 확인.

해당 배포 대상과 적용 명령이 확정되지 않은 상태에서는 존재하지 않는 exact-version 설치/`enter-pr` CLI 명령을 만들어 제시하지 않습니다. 현재 제품의 `/api/environment/enter-pr`·`return-dev`는 변이 API이며 이 검사에서 호출하지 않았습니다. trust/target/manifest 위조, legacy `/api/preview/start` 410 우회, unmanaged `Native.Start`로 bootstrap을 피하지 않습니다. 현재 운영 Web 가용성은 **미적용/미확인**으로 유지합니다.

## 회귀 범위

담당 Preview catalog/auth/artifact/checksum/protocol/rebuild 20개 top-level named 검사를 일반·race 모드로 실행했고 Preview vet, named Web 2개, Node Preview 10개를 통과했습니다. 부정 사례에는 잘못된 workflow/path/repository/run/Dev SHA/PR SHA, fork, 만료, metadata 오류, ZIP/checksum 변조와 protocol 미지원, raw credential-bearing network 오류가 포함됩니다.

로컬 전체 Preview package의 기존 manager 테스트는 Linux 전용 ZIP fixture를 macOS runtime asset으로 요청해 실패했습니다. 이 기존 fixture는 수정하지 않았고 해당 실패와 담당 named 검사 성공을 구분했습니다. 운영 서비스에 영향을 줄 수 있는 무보호 `go test ./...`는 실행하지 않았습니다. 전체 Go/새 최종 SHA CI·Preview는 원격 CI에서 별도로 확인합니다.
