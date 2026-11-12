import { materialSidebar, materialToolbar, materialProjectRows, materialTaskCard, escapeHTML } from './components.js';
import { attachMaterialProximity } from './proximity.js';

const root = document.querySelector('#material-review');
const small = matchMedia('(max-width:760px)'), dark = matchMedia('(prefers-color-scheme:dark)');
const params = new URLSearchParams(location.search);
const read = key => { try { return localStorage.getItem(key); } catch { return null; } };
const save = (key, value) => { try { localStorage.setItem(key, value); } catch { /* optional preference */ } };
const valid = (v, choices, fallback) => choices.includes(v) ? v : fallback;
let mode = valid(params.get('theme') || read('task-mecca-material-review-mode'), ['light','dark','system'], 'dark');
let railMode = valid(params.get('rail') || read('task-mecca-material-review-rail'), ['auto','expanded','compact'], 'expanded');
let projects = [], opened = [], selected = '', mobileOpen = false, request = null, token = 0, currentTask = null, currentProject = null, initializedProjects = false;

root.innerHTML = `<div class="material-layout" data-rail-mode="${railMode}">${materialSidebar()}${materialToolbar()}<main class="material-main" id="material-main" tabindex="-1"><header class="material-page-header"><h1 id="material-view-title">Global Hub</h1><p>프로젝트를 바로 열고, 필요한 근거부터.</p></header><div id="material-card-slot" aria-busy="true"><article class="material-card material-surface" data-light-surface><h2>작업을 읽고 있습니다.</h2><p class="material-card-description">격리된 합성 프로젝트의 기록을 확인합니다.</p></article></div><div class="material-feedback" id="material-feedback" role="status" aria-live="polite"></div><div class="material-review-notes"><p><strong>재질·탐색 동작 시안</strong> · 사이드바, 툴바, 카드 한 장을 먼저 확인하는 화면입니다.</p><p>프로젝트 선택·닫기·다시 열기, 화면 모드와 카드 선택은 작동합니다. 작업 목록과 상세 화면, SYSTEM의 관리 화면 및 OPERATIONS·HELP의 하위 화면은 아직 연결하지 않았습니다.</p><p>마우스가 가까워지면 빛이 부드럽게 반응합니다. 터치 화면과 모션 감소 환경에는 정적 반사를 제공합니다.</p></div></main></div>`;
const layout = root.querySelector('.material-layout');
const lighting = attachMaterialProximity(root);
const announce = message => { root.querySelector('#material-feedback').textContent = message; };
function applyTheme() {
  document.documentElement.dataset.materialTheme = mode === 'system' ? (dark.matches ? 'dark' : 'light') : mode;
  root.querySelectorAll('[data-material-mode]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.materialMode === mode)));
}
function applyRail() {
  layout.dataset.railMode = railMode;
  root.querySelectorAll('[data-rail-mode]').forEach(b => { if (b.tagName === 'BUTTON') b.setAttribute('aria-pressed', String(b.dataset.railMode === railMode)); });
  const toggle = root.querySelector('.material-mobile-toggle'), panel = root.querySelector('#material-rail-panel');
  panel.hidden = small.matches && !mobileOpen;
  toggle.setAttribute('aria-expanded', String(mobileOpen));
  toggle.setAttribute('aria-label', mobileOpen ? '메뉴 닫기' : '메뉴 열기');
  lighting.refresh();
}
function renderProjects() {
  root.querySelector('#material-projects').innerHTML = materialProjectRows(projects, opened, selected);
  root.querySelector('[data-destination="hub"]').toggleAttribute('aria-current', !selected);
  if (!selected) root.querySelector('[data-destination="hub"]').setAttribute('aria-current', 'page');
  const closed = projects.filter(p => !opened.includes(p.path));
  root.querySelector('#material-closed-projects').innerHTML = closed.length ? closed.map(p => `<button type="button" data-reopen-project="${escapeHTML(p.path)}">${escapeHTML(p.name)}</button>`).join('') : '<p class="material-nav-empty">닫힌 프로젝트가 없습니다.</p>';
}
async function json(url, signal) {
  const response = await fetch(url, { signal, cache: 'no-store' });
  if (!response.ok) throw Error(`조회 실패 (${response.status})`);
  return response.json();
}
async function showProject(path = '') {
  selected = path; currentProject = projects.find(p => p.path === path) || projects[0];
  root.querySelector('#material-view-title').textContent = selected ? currentProject?.name || '프로젝트' : 'Global Hub';
  renderProjects();
  if (!currentProject) return;
  request?.abort(); request = new AbortController(); const generation = ++token, signal = request.signal;
  const slot = root.querySelector('#material-card-slot'); slot.setAttribute('aria-busy', 'true');
  try {
    const query = new URLSearchParams({ project: currentProject.path, sort: 'id_asc', page_size: '10', page: '1' });
    const data = await json('/api/backlog/tasks?' + query, signal);
    if (generation !== token) return;
    currentTask = data.items?.[0] || null;
    slot.innerHTML = materialTaskCard(currentTask, currentProject); lighting.refresh();
    announce(selected ? `${currentProject.name}의 합성 작업을 열었습니다. 전체 작업 목록은 이 시안에 연결하지 않았습니다.` : '합성 작업을 읽었습니다.');
  } catch (error) {
    if (error.name !== 'AbortError' && generation === token) {
      slot.innerHTML = `<article class="material-card material-surface" data-light-surface role="alert"><h2>작업을 읽지 못했습니다.</h2><p class="material-card-description">${escapeHTML(error.message)}</p><button type="button" class="material-capsule" data-retry>다시 읽기</button></article>`; lighting.refresh();
    }
  } finally { if (generation === token) slot.setAttribute('aria-busy', 'false'); }
}
async function load() {
  const button = root.querySelector('#material-refresh'); button.disabled = true;
  try {
    let data, waiting = false;
    for (let i = 0; i < 8; i++) {
      data = await json('/api/overview?page_size=4');
      const section = data.sections?.projects;
      waiting = Boolean(section?.loading || section?.pending || data.loading || data.pending);
      if (!waiting) break;
      if (i < 7) await new Promise(resolve => setTimeout(resolve, Math.min(500, Math.max(100, Number(data.retry_after_ms) || 180))));
    }
    projects = (data.sections?.projects?.items || []).map(p => ({ name: p.name || p.path?.split('/').pop() || '프로젝트', path: p.path || p.project })).filter(p => p.path);
    if (!initializedProjects) { opened = projects.map(p => p.path); initializedProjects = projects.length > 0 || data.sections?.projects?.coverage?.complete === true; }
    if (selected && !projects.some(p => p.path === selected)) selected = '';
    renderProjects();
    if (projects.length) await showProject(selected);
    else {
      const complete = data.sections?.projects?.coverage?.complete === true && !waiting;
      root.querySelector('#material-card-slot').innerHTML = `<article class="material-card material-surface" data-light-surface data-availability="${complete ? 'empty' : 'unknown'}"><h2>${complete ? '등록된 프로젝트가 없습니다.' : '프로젝트 근거를 아직 확인하지 못했습니다.'}</h2><p class="material-card-description">${complete ? '완전한 조회 결과입니다. 검토용 합성 프로젝트를 등록한 서버에서 확인해 주세요.' : '로딩 중이거나 조회가 불완전합니다. 이 상태를 프로젝트 0개로 처리하지 않습니다.'}</p><button type="button" class="material-capsule" data-retry>다시 읽기</button></article>`;
      root.querySelector('#material-card-slot').setAttribute('aria-busy', 'false'); lighting.refresh(); announce(complete ? '완전한 조회 결과: 등록된 프로젝트가 없습니다.' : '최대 8회의 유한 조회를 마쳤습니다. 다시 읽기로 재시도할 수 있습니다.');
    }
  } catch (error) { announce(error.message + ' · 다시 읽기를 눌러 주세요.'); }
  finally { button.disabled = false; }
}

root.addEventListener('click', async event => {
  const button = event.target.closest('button'); if (!button) return;
  if (button.dataset.materialMode) { mode = button.dataset.materialMode; save('task-mecca-material-review-mode', mode); applyTheme(); }
  else if (button.dataset.railMode) { railMode = button.dataset.railMode; save('task-mecca-material-review-rail', railMode); applyRail(); }
  else if (button.matches('.material-mobile-toggle')) { mobileOpen = !mobileOpen; applyRail(); }
  else if (button.dataset.destination) {
    if (button.getAttribute('aria-disabled') === 'true') { event.preventDefault(); announce(`${button.querySelector('.material-nav-label').textContent} 화면은 아직 연결하지 않았습니다.`); }
    else { await showProject(''); if (small.matches) { mobileOpen = false; applyRail(); root.querySelector('#material-main').focus(); } }
  }
  else if (button.dataset.project) { await showProject(button.dataset.project); if (small.matches) { mobileOpen = false; applyRail(); root.querySelector('#material-main').focus(); } }
  else if (button.dataset.closeProject) {
    const path = button.dataset.closeProject, project = projects.find(p => p.path === path);
    opened = opened.filter(p => p !== path); if (selected === path) await showProject(''); else renderProjects();
    root.querySelector('#material-open-project').focus(); announce(`${project?.name || '프로젝트'}를 사이드바에서 닫았습니다. 서버의 등록 상태는 바꾸지 않습니다.`);
  }
  else if (button.id === 'material-open-project') {
    const menu = root.querySelector('#material-closed-projects'); menu.hidden = !menu.hidden; button.setAttribute('aria-expanded', String(!menu.hidden));
  }
  else if (button.dataset.reopenProject) {
    opened.push(button.dataset.reopenProject); root.querySelector('#material-closed-projects').hidden = true; root.querySelector('#material-open-project').setAttribute('aria-expanded', 'false');
    await showProject(button.dataset.reopenProject); root.querySelector('[data-project="' + CSS.escape(button.dataset.reopenProject) + '"]')?.focus();
  }
  else if (button.matches('.material-select-card')) {
    const selectedCard = button.getAttribute('aria-pressed') !== 'true'; button.setAttribute('aria-pressed', String(selectedCard)); button.querySelector('span').textContent = selectedCard ? '선택됨' : '카드 선택'; button.closest('.material-card').classList.toggle('is-selected', selectedCard);
  }
  else if (button.id === 'material-show-source') {
    const source = root.querySelector('#material-source'), opening = source.hidden; source.hidden = !opening; button.setAttribute('aria-expanded', String(opening));
    if (opening && !source.textContent && currentTask) {
      source.textContent = '원문을 읽고 있습니다.';
      const captured = currentTask, project = currentProject;
      try { const data = await json(`/api/tasks/${encodeURIComponent(captured.id)}?` + new URLSearchParams({ project: project.path, projection: 'content' })); if (source.isConnected) source.textContent = data.raw_markdown || '원문이 기록되지 않았습니다.'; }
      catch (error) { if (source.isConnected) source.textContent = error.message; }
    }
  }
  else if (button.id === 'material-refresh' || button.hasAttribute('data-retry')) await load();
});
root.addEventListener('keydown', event => {
  if (event.key === 'Escape' && small.matches && mobileOpen) { mobileOpen = false; applyRail(); root.querySelector('.material-mobile-toggle').focus(); }
});
small.addEventListener('change', applyRail); dark.addEventListener('change', applyTheme);
applyTheme(); applyRail(); load();
window.addEventListener('pagehide', () => { lighting.dispose(); request?.abort(); }, { once: true });
// Review instrumentation observes the real shared module; it does not replace its implementation.
window.taskMeccaMaterialReview = Object.freeze({ inspect: () => lighting.inspect(), disableProximity: () => lighting.dispose() });
