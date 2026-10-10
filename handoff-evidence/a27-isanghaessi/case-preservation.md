# A27 읽기 전용 실제 사례 경계

실제 A19/A24/A25/B457/B458에서 제목·담당자·Dispatch 상태 및 해당 task_id로 필터링한 배정/인계 event 종류·시각만 읽었다. 원문 자격증명, 알림 설정 또는 원장 payload는 출력하지 않았다. 실제 EMPFUND/TaskMecca 운영원장·완료 사실을 수정하지 않았다. 테스트는 Go t.TempDir 및 별도 paused UI fixture만 사용한다.

원본 완료 문서 SHA256을 read-only 점검 전후에 대조했고 아래 값은 모두 불변이다. 이 해시는 완료 문서 보존 증거이며, 계속 동작하는 운영 monitor 전체 원장의 불변을 주장하지 않는다.

| 사례 | SHA256 before = after |
| --- | --- |
| A-19 | 421b78b61f0f41e218bae28ed657d8457c59310ae1b5bad642b9acdb1c11e44b |
| A-24 | 58cc51d42758bbe923d02f572759f03fbf237e56067dd9fa96652392503314be |
| A-25 | dd559b58a5922f529b82e57bd77f63e1af4e832397fe00218e205674ca825b6e |
| B-457 | 95e07214b08d1519c061812c60adbad1c6101647bc06da10e21f8deca87bfca6 |
| B-458 | 7d0764526f9891b70b78de9917b851d4dfd3ef57e20767f8806864360c1f706c |

실제 metadata: A19는 이전 kkobugi 배정 후 raichyu 재배정들이 있고 worker_done 시각이 최신 배정 이후이다. A24는 단일 최신 배정 및 worker_done 보고가 있다. A25는 초기 구현과 fixture 안전 follow-up의 별도 배정과 여러 보고가 있어 최신 assignment/attempt와의 정확한 대조가 필요하다. B457/B458은 완료 파일과 pikachyu 배정이 확인된다. done 파일명만으로 실제 오류를 해소하지 않는다.

대응 synthetic regression: A19 고정 handoff/claim/acceptance grace 및 오류 보존, A24 stage scan 전이와 doing 보존, A25 미확정 terminal 오류 유예 제외, B457 최신 observed assignment로 ID 없는 옛 배정 경고 해소, B458 재배정 이후 old source 재출현 방지 및 새 actual errored 유지. controller_verified completion gate와 runtime 불변은 별도 completion fixture로 검증했다.
