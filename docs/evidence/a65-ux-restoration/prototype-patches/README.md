# 기각한 모바일 경로의 소스 보존

이 패치는 제품에 적용된 코드가 아니다. base8d28d7a2의 web 자산을 별도의 **asset-only 디렉터리**로 복사한 뒤 `-p1`로 적용하면 해당 실험 소스가 된다. 새 repository checkout/worktree를 만들지 않는다. 전체 asset snapshot과 모든 gzip trace는 canonical `ux-restoration-20261110-8728`에 있다.

- photo-css-web: viewport 고정 사진 + live CSS SVG filter
- photo-static-web: 전체 면 SVG image
- photo-edge-web: photo edge SVG image strips
- photo-layer-web: edge image + foreground layer promotion

일부 대체 조건에서 CPU 비용이 증가해 모두 기각했다. 최종 source8728은 **native7px blur + 제한된 live SVG edge band**다. 이미지 인코딩/생성·canvas 경로를 제품에 넣지 않았다. 각 실험의 raw 결과를 다음 실험의 성공으로 재분류하지 않는다.

측정은 해당 Mac/Chrome 버전의 scoped 진단이며, 삼성 실기나 통계적 성능 gate를 대체하지 않는다. 이전 Chrome156과 이후157 결과는 같은 환경이라고 합치지 않는다. 검증 output은 덮어쓰지 않도록 immutable guard가 있으므로 재실행은 새 output fixture 디렉터리에서 한다. 원래48그룹하네스는 바꾸지 않는다.
