# 업데이트·마이그레이션 보호 검증 (AID-205)

프로젝트 파일 보존 여부는 **실제 해시 대조 결과만** 근거로 보고한다. 기존 migration의 managed framework 파일 변경 목록과 사용자 소유 데이터 보호 증명은 별도다.

## 1. 업데이트 전 보호 스냅샷 생성

```bash
task-mecca protect snapshot --project /PROJECT --output /SAFE_LOCATION/project-protected-before.json --json
```

`_task_mecca/data/**`, `_task_mecca/backlog*/**`, `_task_mecca/config.toml`을 SHA-256/bytes/상대경로로 기록한다. 파일을 수정하지 않고, 출력 manifest를 보호 대상 외부에 **독점 신규 파일**로 기록한다. 자료 원문은 복사하지 않는다. 대규모 파일은 streaming SHA-256으로 처리한다.

누락/읽기 실패/보호 영역 내 symlink가 있으면 snapshot 생성이 실패하며 보존 확인으로 표시하지 않는다.

## 2. Framework migration

```bash
task-mecca migrate --project /PROJECT --json
# 사용자 수정 framework 파일에 대한 choice_required가 발생한 경우,
# modified_files, plan_digest, 사용자 선택(backup/overwrite/cancel)을 확인한 후 재호출
```

`migrate`는 기존 수정 파일 및 plan-digest 승인 정책을 유지하고, 수정 직전/직후 위 범위를 다시 SHA-256으로 검사한다.

- `protected_files.status=verified`: 보호 범위에서 파일 추가·삭제·내용 변경 없이 전후 해시 확인.
- `protected_files.status=changed`: `missing/changed/added` 상대경로가 보고됨. migration 수행과 보호 증명 성공은 다른 결과.
- `protected_files.status=unknown`: 검증 범위 접근 불가, symlink, 파일 읽기 실패 등. 이때 성공으로 추정 금지.
- `protected_files.status=not_run`: 취소/선택 대기 등 보호 비교가 수행되지 않은 경우.
- 기존 `modified_files`, `backup_path`, `to_version`은 framework 관리 파일 변경 및 백업의 증거이며 보호 해시 증명과 혼동하지 않는다.
- 보호 사전 스냅샷을 확보할 수 없으면 기본적으로 migration을 중단한다.

## 3. CLI 자체 upgrade 전/후 비교

```bash
task-mecca protect versions --project /PROJECT --json
task-mecca upgrade
task-mecca protect verify --project /PROJECT --snapshot /SAFE_LOCATION/project-protected-before.json --json
task-mecca protect versions --project /PROJECT --json
```

`upgrade`는 CLI 바이너리 업데이트이며 각 프로젝트의 framework migration을 자동으로 증명하지 않는다. `versions` 결과의 `cli_version`, `framework_version`, `web_version`, `web_running`, `web_applied`를 **별도로** 비교한다. 실제 Web 프로세스가 종료·재시작되었는지 status가 확인되지 않으면 최신 CLI 버전만으로 Web 적용 완료라고 보고하지 않는다.

### 범위의 한계

이 보호 증명은 위 명시 범위만 검증한다. `_task_mecca/.runtime/**`, 외부 Git 저장소, 사용자 홈과 다른 프로젝트는 범위 밖이므로 **not_run/미검증**이다. 백업은 migration이 실제 생성한 `backup_path`로만 확인한다. 보호 manifest 자체가 백업 파일의 사본은 아니다.

모든 Dev 직접 검증은 기존 사용자의 운영 데이터를 훼손하지 않는 샌드박스에서 우선 수행한다.
