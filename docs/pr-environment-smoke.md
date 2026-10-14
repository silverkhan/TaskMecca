# PR 환경 전환 실기기 검증용

이 Pull Request는 Task Mecca **Dev → PR → Dev** 전환의 설치·복귀·관제 E2E를 검증하기 위한, 의도적으로 문서 파일만 변경한 테스트 브랜치입니다.

- 기준 Dev: `4fb78c2ae0ecd7423087b16e36326b643ce4bef4`
- 애플리케이션 실행 코드, Hook, 상태 원장 및 프로젝트 프레임워크: 변경 없음
- 검증 포인트: 정확한 PR Head SHA 빌드, 기존 프로젝트 UI, Root Session/Workload, Telegram 알림 정합성, 진입 직전 Dev 바이너리 복원
- 별도 Preview 프로세스가 아닌 현 웹 서비스 전환
- 테스트를 마치면 PR을 Dev로 병합할 필요 없이 종료할 수 있음

**주의:** OS 사용자 권한에서 실제 실행되므로 검증 전 프로젝트 데이터의 안전성을 확인합니다. 사용자 환경의 파일/원장 상태 검증은 별도 [AID-154](https://linear.app/aiden7/issue/AID-154)에서 수행합니다.
