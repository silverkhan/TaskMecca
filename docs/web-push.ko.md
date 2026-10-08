# Task Mecca Web Push (AID-120)

## 전제 및 사용 방법

Task Mecca Web의 **알림센터 → 설정 → Web Push · 백그라운드 알림**에서 **백그라운드 알림 켜기**를 누릅니다. 운영체제 알림 권한을 허용한 후 **테스트**로 Push 서비스 접수를 확인합니다. 테스트 응답의 성공은 운영체제의 실제 알림 표시나 사용자의 읽음 확인을 뜻하지 않습니다.

Push 지원을 위해서는 사용 브라우저가 Task Mecca 서버에 HTTPS(또는 localhost 보안 문맥)로 연결되어 있어야 하고, **서버**가 해당 브라우저의 Push 서비스(FCM, Apple, Mozilla 또는 Microsoft)에 인터넷으로 HTTPS 요청을 보낼 수 있어야 합니다. Tailscale Serve는 브라우저 접속을 보호하지만, Push 서비스에 대한 서버 아웃바운드 네트워크는 별도로 필요합니다. 서버가 실행 중이어야 합니다.

| 환경 | 확인할 사항 |
| --- | --- |
| Windows/macOS Chrome·Edge | 브라우저 및 운영체제 알림 권한, Push API, 보안 문맥 |
| macOS Safari | 최신 Safari의 Web Push 지원, macOS 알림 권한, HTTPS |
| Android Chrome·삼성 인터넷 | 사용 버전의 PushManager 지원, Android 알림 권한, 백그라운드 제한 |
| iOS/iPadOS Safari | iOS/iPadOS 16.4 이상에서 홈 화면에 추가된 웹앱(PWA)으로 실행해야 Web Push 사용 가능. 일반 Safari 탭에서는 불가 |
| 브라우저나 기기가 오프라인 | Web Push 서비스의 전송/보관 정책 및 OS 제약에 따라 전달 지연·누락 가능 |

## 중복 방지·보안 원칙

- 서버에서 생성한 canonical notification event ID만 Web Push 대상이 됩니다. 완료 등 과거 이벤트는 AID-119 서버 원장의 cutover와 TTL 정책을 그대로 적용해 새 기기 연결 시 소급하지 않습니다.
- 전경 탭 알림과 백그라운드 Push 모두 같은 원자적 전달권을 사용합니다. 동일 사건의 서로 다른 기기 중복 발송을 방지합니다. Push 서비스 접수(`push_accepted`)는 운영체제 표시나 사용자의 읽음과 구분하여 표시합니다.
- VAPID 비밀키는 설치별 공통 경로 `~/.task-mecca/webpush-vapid.json`(또는 `TASK_MECCA_HOME`)에, 브라우저 구독의 엔드포인트·키는 각 프로젝트의 `_task_mecca/.runtime/notifications/push_subscriptions.json`에 0600 권한으로 보관합니다. 두 파일은 백업 및 접근 통제가 필요합니다. 비밀값은 API에 응답하지 않습니다.
- Push 송신 대상은 공개 Push 서비스의 승인된 HTTPS 호스트로 제한합니다. 리디렉션을 허용하지 않으며 POST에는 관리 헤더와 동일 출처 확인을 적용합니다.
- **등록**은 선택한 브라우저에서 명시적인 사용자 제스처에 의해 시작되며, 백그라운드 구독을 해제하면 브라우저 구독과 서버의 관련 프로젝트 구독을 함께 제거합니다.
- 배포 이후 macOS Safari, Windows Chrome, Android 삼성 인터넷 및 지원되는 iOS 설치형 웹앱의 실제 OS 알림 수신을 재현 검증해야 합니다. CI의 Go/Node 테스트만으로 실기기 동작이 보장되는 것은 아닙니다.

## 장애 진단

1. 알림센터의 권한 상태, HTTPS 주소, Web Push 지원 여부를 확인합니다.
2. **테스트** 결과가 Push 서비스 접수에 실패하면 Task Mecca 서버의 외부 HTTPS 연결, 프록시/방화벽, VAPID 키 일관성과 구독 만료를 확인합니다.
3. 서비스에서 접수했다고 표시되지만 기기 알림이 보이지 않으면 운영체제·브라우저 알림 권한, 절전/집중 모드, 서비스워커 등록 및 브라우저 제공자별 제약을 점검합니다.
4. 다른 기기에 이미 전달된 사건이 다시 Push되지 않아야 합니다. 현재 해소되지 않은 사용자 확인 필요 사건은 알림센터에서 확인할 수 있으며 과거 이벤트를 푸시로 재생하지 않습니다.

대상: `dev` 우선. Stable 승격은 별도 승인.
