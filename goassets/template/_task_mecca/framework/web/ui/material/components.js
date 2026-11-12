// Shared material primitives. The review and future core consume the same DOM/CSS contract.
export const escapeHTML = (value = "") => String(value).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
const paths = {
  home: "M3 10 12 3l9 7v10H3z M9 20v-7h6v7",
  storage: "M4 4h16v6H4z M4 14h16v6H4z M7 7h.01M7 17h.01",
  project: "M3 7h7l2-3h9v16H3z",
  workload: "M4 19V9m5 10V4m6 15v-8m5 8V6",
  bell: "M6 9a6 6 0 0 1 12 0v6l2 3H4l2-3z M10 21h4",
  terminal: "m4 6 5 6-5 6 M12 18h8",
  book: "M12 5C8 2 4 3 2 4v16c3-2 6-2 10 0 4-2 7-2 10 0V4c-2-1-6-2-10 1Zm0 0v15",
  history: "M3 5v5h5 M3 10a9 9 0 1 1 1 7 M12 7v5l3 2",
  issue: "M12 8v5m0 4h.01 M10 3h4l8 15-2 3H4l-2-3z",
  plus: "M12 5v14M5 12h14",
  close: "m6 6 12 12M18 6 6 18",
  menu: "M4 6h16M4 12h16M4 18h16",
  refresh: "M20 8a8 8 0 1 0 0 8M20 3v5h-5",
  sun: "M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  check: "m5 12 4 4L19 6",
};
export const materialIcon = name => `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="${paths[name] || paths.project}"/></svg>`;
const navButton = (id, label, icon, unavailable = false) => `<button type="button" class="material-nav-item" data-destination="${id}" ${unavailable ? 'aria-disabled="true"' : ''} title="${escapeHTML(label + (unavailable ? ' · 미연결' : ''))}"><span class="material-nav-icon">${materialIcon(icon)}</span><span class="material-nav-label">${escapeHTML(label)}</span>${unavailable ? '<span class="material-unconnected" aria-hidden="true">미연결</span>' : ''}</button>`;

export function materialSidebar() {
  return `<aside class="material-rail material-surface" data-light-surface id="material-rail" aria-label="주 탐색">
    <div class="material-brand-row"><a class="material-brand" href="/ui/material-review.html" aria-label="Task Mecca 재질 검토 홈"><img src="/logo.svg" alt=""><strong>Task Mecca</strong></a><button type="button" class="material-icon-button material-mobile-toggle" aria-label="메뉴 열기" aria-expanded="false" aria-controls="material-rail-panel">${materialIcon('menu')}</button></div>
    <div class="material-rail-panel" id="material-rail-panel">
      <nav class="material-nav" aria-label="기존 메뉴 배치 시안">
        <section class="material-nav-group" aria-labelledby="material-system"><h2 id="material-system">SYSTEM</h2>${navButton('hub', 'Global Hub', 'home')}${navButton('storage', '저장소 관리', 'storage', true)}${navButton('preview', 'Preview Builds', 'project', true)}</section>
        <section class="material-nav-group" aria-labelledby="material-backlogs"><h2 id="material-backlogs">BACKLOGS</h2><div id="material-projects"><p class="material-nav-empty">프로젝트를 읽고 있습니다.</p></div><div class="material-project-opener"><button type="button" class="material-nav-item" id="material-open-project" aria-expanded="false" aria-controls="material-closed-projects"><span class="material-nav-icon">${materialIcon('plus')}</span><span class="material-nav-label">프로젝트 열기</span></button><div id="material-closed-projects" hidden></div></div></section>
        <section class="material-nav-group" aria-labelledby="material-operations"><h2 id="material-operations">OPERATIONS</h2>${navButton('workload', 'Subagent Workload', 'workload', true)}${navButton('notifications', '알림 센터', 'bell', true)}${navButton('terminal', '터미널', 'terminal', true)}</section>
        <section class="material-nav-group" aria-labelledby="material-help"><h2 id="material-help">HELP</h2>${navButton('manual', '사용자 매뉴얼', 'book', true)}${navButton('release', '업데이트 내역', 'history', true)}${navButton('issues', '진단 및 이슈', 'issue', true)}</section>
      </nav>
      <footer class="material-rail-footer"><div class="material-mode-control" role="group" aria-label="사이드바 표시 방식">${[['auto','자동'],['expanded','펼침'],['compact','최소']].map(([value,label]) => `<button type="button" data-rail-mode="${value}" aria-pressed="false" title="${label}">${label}</button>`).join('')}</div><p class="material-rail-note">재질 검토 · 하위 화면 미연결</p></footer>
    </div>
  </aside>`;
}

export function materialProjectRows(projects, opened, selected) {
  return projects.filter(p => opened.includes(p.path)).map(p => `<div class="material-project-row ${selected === p.path ? 'is-current' : ''}"><button type="button" class="material-nav-item material-project-open" data-project="${escapeHTML(p.path)}" ${selected === p.path ? 'aria-current="page"' : ''} title="${escapeHTML(p.name + ' · ' + p.path)}"><span class="material-nav-icon material-project-initial" aria-hidden="true">${escapeHTML(p.name.slice(0, 1).toUpperCase())}</span><span class="material-nav-label">${escapeHTML(p.name)}</span></button><button type="button" class="material-project-close" data-close-project="${escapeHTML(p.path)}" aria-label="${escapeHTML(p.name)} 닫기" title="프로젝트 닫기">${materialIcon('close')}</button></div>`).join('') || '<p class="material-nav-empty">열린 프로젝트가 없습니다.</p>';
}

export function materialToolbar() {
  return `<div class="material-toolbar material-surface" data-light-surface role="toolbar" aria-label="검토 화면 도구"><span class="material-toolbar-title">${materialIcon('sun')}<span>화면 모드</span></span><div class="material-theme-control" role="group" aria-label="화면 모드">${[['light','Light'],['dark','Dark'],['system','System']].map(([value,label]) => `<button type="button" data-material-mode="${value}" aria-pressed="false">${label}</button>`).join('')}</div><span class="material-tool-divider" aria-hidden="true"></span><button type="button" class="material-icon-button" id="material-refresh" aria-label="합성 작업 다시 읽기" title="다시 읽기">${materialIcon('refresh')}</button></div>`;
}

export function materialTaskCard(task, project) {
  const title = task?.title || '기록된 작업이 없습니다';
  const fileState = ({todo:'대기',doing:'진행',hold:'보류',done:'완료'})[task?.file_state] || '미확인';
  return `<article class="material-card material-surface" data-light-surface id="material-task-card" aria-labelledby="material-card-title"><header class="material-card-header"><span class="material-card-identity">${escapeHTML(task?.id || '—')} · ${escapeHTML(project?.name || '프로젝트')}</span><span class="material-status">파일: ${fileState}</span></header><h2 id="material-card-title">${escapeHTML(title)}</h2><p class="material-card-description">열린 프로젝트의 합성 작업입니다. 표면의 곡률과 반사광, 선택·포커스의 깊이를 확인해 주세요.</p><dl class="material-card-facts"><div><dt>프로젝트</dt><dd>${escapeHTML(project?.name || '미확인')}</dd></div><div><dt>담당</dt><dd>${escapeHTML(task?.agent || task?.assignee || '미확인')}</dd></div></dl><div class="material-card-actions"><button type="button" class="material-capsule material-select-card" aria-pressed="false">${materialIcon('check')}<span>카드 선택</span></button><button type="button" class="material-text-button" id="material-show-source" aria-expanded="false" aria-controls="material-source" ${task?.id ? '' : 'disabled'}>원문 보기</button></div><div class="material-source" id="material-source" hidden></div><footer class="material-card-footer">합성 데이터 · 실제 서버에서 읽음</footer></article>`;
}
