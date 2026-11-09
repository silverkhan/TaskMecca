import '../select-ui.js';
import { refreshOptics } from './optics.js';
import { attachMaterialProximity } from './material/proximity.js';
import { state, routeURL, persist } from './state.js';
import { esc, text } from './format.js';
import { get } from './data.js';
const read = (key, fallback) => { try { return JSON.parse(localStorage.getItem(key)) ?? fallback; } catch { return fallback; } };
const readString=(key,fallback='')=>{try{return localStorage.getItem(key)||fallback;}catch{return fallback;}};
let backgroundMode=readString('task-mecca-material-background-v1',readString('task-mecca-material-review-background','image'));
let projects = [], hidden = read('task-mecca-closed-projects-v1', []), sidebarMode = readString('task-mecca-sidebar-mode','auto');
if(sidebarMode==='compact') sidebarMode='collapsed';
let opened=read('task-mecca-open-projects',null);
let inventory;
export async function refreshSidebar(signal) {
  inventory = get('/api/hub/projects', signal).then(data => {
    projects = data.projects || [];
    if(opened) { for(const path of opened) if(!projects.some(p=>p.path===path)) projects.push({path,name:path.split('/').pop()}); hidden=projects.filter(p=>!opened.includes(p.path)).map(p=>p.path); }
    paintProjects(); return data;
  }).catch(error => {
    const slot = document.getElementById('sidebar-projects');
    if (slot && error.name !== 'AbortError') slot.innerHTML = `<p class="material-nav-empty" role="status">${text('프로젝트 목록 미확인','Project inventory unavailable')}</p>`;
  });
  return inventory;
}
export function sidebarProjects(){return [...projects];}
export function paintProjects() {
  void syncProjectDiagnostics();
  const slot = document.getElementById('sidebar-projects'); if (!slot) return;
  // A deep link opens its selected project; closed tabs are independent of registry membership.
  if (state.project && hidden.includes(state.project)) { hidden = hidden.filter(p => p !== state.project); persist('task-mecca-closed-projects-v1', JSON.stringify(hidden)); }
  const rows = projects.filter(p => !hidden.includes(p.path) && p.open !== false);
  slot.innerHTML = rows.map(p => `<div class="material-project-row"><a class="rail-link material-nav-item" data-route href="${esc(routeURL({view:'backlog',project:p.path,backlog:'',detail:null,page:1,query:'',tags:[],status:'all'}))}" title="${esc(p.path)}" ${state.project===p.path?'aria-current="page"':''}><span class="material-nav-icon material-project-initial">${esc((p.name||'P').slice(0,1))}</span><span class="material-nav-label">${esc(p.name||p.path.split('/').pop())}</span></a><button class="material-project-close" data-close-project="${esc(p.path)}" aria-label="${esc(p.name)} ${text('닫기','Close')}">×</button></div>`).join('') || `<p class="material-nav-empty">${text('열린 프로젝트가 없습니다','No open projects')}</p>`;
  const closed=projects.filter(p=>hidden.includes(p.path));
  slot.innerHTML += `<details class="project-opener"><summary>${text('프로젝트 열기','Open project')}</summary>${closed.map(p=>`<a data-route href="${esc(routeURL({view:'backlog',project:p.path,backlog:'',detail:null,page:1,query:'',tags:[],status:'all'}))}" class="material-nav-item" data-reopen-project="${esc(p.path)}">${esc(p.name||p.path)}</a>`).join('')||`<p class="material-nav-empty">${text('닫힌 프로젝트가 없습니다','No closed projects')}</p>`}</details>`;
  slot.querySelectorAll('[data-close-project]').forEach(button => button.onclick = () => { hidden.push(button.dataset.closeProject); opened=projects.filter(p=>!hidden.includes(p.path)).map(p=>p.path); persist('task-mecca-open-projects',JSON.stringify(opened)); persist('task-mecca-closed-projects-v1',JSON.stringify(hidden)); paintProjects(); document.getElementById('sidebar-toggle')?.focus(); });
}
export function openProject(path) { if(opened&&!opened.includes(path))opened.push(path); if(opened)persist('task-mecca-open-projects',JSON.stringify(opened)); hidden = hidden.filter(p => p !== path); persist('task-mecca-closed-projects-v1', JSON.stringify(hidden)); }
function syncMode() {
  document.documentElement.dataset.sidebar = sidebarMode;
  document.querySelectorAll('[data-sidebar-mode]').forEach(b => b.setAttribute('aria-pressed',String(b.dataset.sidebarMode===sidebarMode)));
}
export function bindExpansionShell() {
  paintProjects(); syncMode();
  document.querySelectorAll('[data-sidebar-mode]').forEach(b => b.onclick = () => { sidebarMode=b.dataset.sidebarMode; persist('task-mecca-sidebar-mode',sidebarMode); syncMode(); });
  const toggle = document.getElementById('sidebar-toggle');
  if (toggle) toggle.onclick = () => { const open=document.documentElement.toggleAttribute('data-menu-open'); toggle.setAttribute('aria-expanded',String(open)); };
}
const root = document.documentElement;
function syncTheme() { root.dataset.materialTheme = root.dataset.theme || 'light'; root.dataset.materialBackground = backgroundMode; }
new MutationObserver(syncTheme).observe(root,{attributes:true,attributeFilter:['data-theme']}); syncTheme();
export function setBackground(mode) { if (!['image','solid'].includes(mode)) return; backgroundMode=mode; persist('task-mecca-material-background-v1',mode); syncTheme(); }
const host = document.getElementById('material-review');
const light = attachMaterialProximity(document.body);
const surfaceSelector = '.tm-combo-popup,dialog[open],.material-reading-plane';
function surfaces() {
  document.querySelectorAll(surfaceSelector).forEach(el => {
    if (!el.classList.contains('material-surface')) { el.classList.add('material-surface'); el.setAttribute('data-light-surface',''); }
  });
  document.querySelectorAll(".workflow-floating").forEach(el=>el.setAttribute("data-light-surface",""));
  refreshOptics();
  light.refresh();
}
// Text/log/option updates and our own decorative layer insertion do not change
// the surface registry. Readonly screens still enter through this same selector.
const changesSurfaces = node => node.nodeType === 1 &&
  (node.matches(surfaceSelector + ',[data-light-surface],.workflow-floating') ||
   node.querySelector(surfaceSelector + ',[data-light-surface],.workflow-floating'));
new MutationObserver(records => {
  if (records.some(record => [...record.addedNodes,...record.removedNodes].some(changesSurfaces))) surfaces();
}).observe(document.body,{childList:true,subtree:true});
addEventListener('pagehide',()=>light.dispose(),{once:true});
document.addEventListener('click',e=> { if(e.target.closest('a[data-route]')) { const path=e.target.closest('[data-reopen-project]')?.dataset.reopenProject; if(path)openProject(path); root.removeAttribute('data-menu-open');document.getElementById('sidebar-toggle')?.setAttribute('aria-expanded','false');} });
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&root.hasAttribute('data-menu-open')){root.removeAttribute('data-menu-open');document.getElementById('sidebar-toggle')?.setAttribute('aria-expanded','false');document.getElementById('sidebar-toggle')?.focus();}});

let diagnosticProject='',diagnosticSequence=0;
export async function syncProjectDiagnostics(force=false){
  const node=document.querySelector('.rail-link[href*="view=issues"]');if(!node)return;
  if(!state.project){node.hidden=true;diagnosticProject='';return;}
  if(diagnosticProject===state.project&&!force)return;
  const project=state.project,sequence=++diagnosticSequence;diagnosticProject=project;node.hidden=false;
  try{const data=await get('/api/issues?'+new URLSearchParams({project,...state.backlog?{backlog:state.backlog}:{}}));if(sequence!==diagnosticSequence||project!==state.project)return;
    const has=(data.diagnostics||[]).length||Object.values(data.health||{}).some(v=>Array.isArray(v)?v.length:v&&typeof v==='object'&&Object.keys(v).length);
    node.hidden=!has;node.title=text('백로그 진단','Backlog diagnostics');
  }catch{if(sequence===diagnosticSequence&&project===state.project){node.hidden=false;node.title=text('진단 조회 미확인','Diagnostic status unverified');}}
}
