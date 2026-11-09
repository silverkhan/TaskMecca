import { state, routeURL } from '../state.js';
import { esc, text, count, date, markdown } from '../format.js';
import { get } from '../data.js';
import { heading, empty, link, taskRows } from '../components/shared.js';
import { refreshSidebar, openProject, setBackground } from '../expansion.js';
import { notifications, bindNotifications } from './notifications.js';
import { preview, bindPreview } from './preview.js';
export const managedViews = ['projects','storage','workload','operations','notifications','terminal','manual','release-notes','preview','issues','settings'];
let model, scope='', page=1, runtimePage=1;
const unknown = () => text('미확인','Unknown');
const val = v => v == null || v === '' ? unknown() : typeof v === 'boolean' ? text(v?'예':'아니요', v?'Yes':'No') : typeof v === 'object' ? JSON.stringify(v) : String(v);
export const panel = (title,body) => `<section class="management-sheet"><h2>${esc(title)}</h2>${body}</section>`;
const factLabels={current:'현재 버전',latest:'최신 버전',channel:'채널',update_available:'업데이트 가능',framework_version:'프로젝트 지침 버전',migration_available:'지침 갱신 가능',project_path:'프로젝트 경로',snapshot_at:'관측 시각',active:'활성',terminal:'종료',total:'전체',hidden:'숨김',needs_check:'확인 필요',cleanup_eligible:'정리 가능',telegram_transport_disabled:'Telegram 전송 비활성',last_scan_at:'최근 검사',generated_at:'생성 시각',os:'운영체제'};
export const facts = entries => `<dl>${entries.map(([k,v])=>`<div><dt>${esc(state.language==='ko'?(factLabels[k]||k):k)}</dt><dd>${esc(val(v))}</dd></div>`).join('')}</dl>`;
export const evidence = data => `<details><summary>${text('상세 근거','Evidence details')}</summary><pre>${esc(JSON.stringify(data,null,2))}</pre></details>`;
export const table = (rows,columns) => !rows?.length ? empty(text('표시할 기록이 없습니다','No records')) : `<div class="table-scroll"><table><thead><tr>${columns.map(([title])=>`<th scope="col">${esc(title)}</th>`).join('')}</tr></thead><tbody>${rows.map(row=>`<tr>${columns.map(([,key,render])=>`<td>${render?render(row):esc(val(row[key]))}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
export const button = (label,action,attrs='') => `<button type="button" data-manage="${action}" ${attrs}>${esc(label)}</button>`;
export const scoped = (path,project=scope,extra={}) => path+'?'+new URLSearchParams({...project?{project}:{},...extra});
export async function post(path,body,signal) {
  const response = await fetch(path,{method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify(body),cache:'no-store',signal:signal||AbortSignal.timeout(120000)});
  let data; try { data=await response.json(); } catch { throw Error(text('서버 응답 형식이 올바르지 않습니다','Invalid server response')); }
  if (!response.ok || data.error || data.ok===false) { const error=Error(data.error||'HTTP '+response.status);error.status=response.status;error.body=data;throw error; }
  return data;
}
export function feedback(message,error=false) { const el=document.getElementById('management-feedback');if(el){el.textContent=message;el.dataset.error=String(error);el.focus({preventScroll:true});} }
// A native modal keeps focus and Escape handling inside the confirmation step.
export async function confirmAction(title,description,extra='') {
  const previous=document.activeElement, dialog=document.createElement('dialog');
  dialog.innerHTML=`<form method="dialog"><h2>${esc(title)}</h2><p>${esc(description)}</p>${extra}<div class="management-actions"><button value="cancel">${text('취소','Cancel')}</button><button value="confirm">${text('진행','Continue')}</button></div></form>`;
  document.body.append(dialog);dialog.showModal();dialog.querySelector('button').focus();
  return new Promise(resolve=>dialog.addEventListener('close',()=>{const result=dialog.returnValue==='confirm';dialog.remove();previous?.isConnected&&previous.focus();resolve(result);},{once:true}));
}
function selector(projects,all=false,scope="") {
  return `<div class="management-fields"><label>${text('조회 프로젝트','Project scope')}<select id="management-scope">${all?`<option value="">${text('모든 등록 프로젝트','All registered projects')}</option>`:''}${projects.map(p=>`<option value="${esc(p.path)}" ${scope===p.path?'selected':''}>${esc(p.name||p.path)}</option>`).join('')}</select></label></div>`;
}
function pager(data,kind='history') {const current=Number(data.page)||1,total=data.total_pages??data.pages;return `<div class="pagination"><span>${text('페이지','Page')} ${current} / ${total==null?unknown():count(total)} · ${text('기록','Records')} ${count(data.total)}</span><div>${button(text('이전','Previous'),kind+'-prev',current<=1?'disabled':'')}${button(text('다음','Next'),kind+'-next',total!=null?current>=total?'disabled':'':data.has_more===true?'':'disabled')}</div></div>`;}
async function settledReads(reads,signal) {
  const entries=Object.entries(reads),results=await Promise.allSettled(entries.map(([,url])=>get(url,signal)));
  return Object.fromEntries(entries.map(([name],i)=>[name,results[i].status==='fulfilled'?results[i].value:{load_error:results[i].reason.message}]));
}
function readError(data) { return data?.load_error ? `<p class="notice" role="alert">${esc(data.load_error)}</p>` : ''; }
const titleMap = {projects:['프로젝트 관리','Manage projects'],storage:['저장소 관리','Storage'],workload:['Subagent Workload','Subagent Workload'],operations:['운영 상태','Operations'],notifications:['알림센터','Notifications'],terminal:['터미널','Terminal'],manual:['사용자 매뉴얼','User manual'],'release-notes':['업데이트 내역','Updates'],preview:['Preview 관리','Preview management'],issues:['백로그 진단','Backlog diagnostics'],settings:['설정','Settings']};
export async function management(signal) {
  const view=state.view,params=new URLSearchParams(location.search);
  let page=Math.max(1,Number(params.get('op_page'))||1);let runtimePage=Math.max(1,Number(params.get('runtime_page'))||1);
  let inventory={projects:[],archives:[]};
  if(!['manual','release-notes','preview','operations'].includes(view)) inventory=await get('/api/hub/projects',signal);
  let scope=params.get('scope')??(['notifications','storage'].includes(view)?'':state.project)??'';
  if(!Array.isArray(inventory.projects)) throw Error(text('프로젝트 목록 응답이 올바르지 않습니다','Invalid project inventory response'));
  if(!scope&&inventory.projects.length&&view!=='storage'&&view!=='notifications')scope=inventory.projects[0].path;
  // Compatibility APIs fall back to the service project on an unknown path.
  // Refuse that path here before issuing any scoped read or write.
  if(scope&&!inventory.projects.some(p=>p.path===scope)&&!['manual','release-notes','preview','operations'].includes(view)) throw Error(text('등록되지 않은 프로젝트입니다. Global Hub에서 경로를 확인하세요.','Unregistered project. Verify the path in Global Hub.'));
  const route=location.href; const model={inventory,route};let body='';
  if(view==='projects') body=projectView(inventory);
  if(view==='settings') {
    model.settings=await settledReads({version:scoped('/api/version',scope),hooks:scoped('/api/runtime/hooks',scope),channels:'/api/channel-options'},signal);
    body=selector(inventory.projects,false,scope)+panel(text('화면과 언어','Appearance and language'),`<p>${text('상단의 화면 모드와 언어 선택이 모든 화면에 저장됩니다.','Appearance and language in the header are saved across screens.')}</p><div class="management-actions">${button(text('사진 배경','Photo background'),'background-image')}${button(text('단색 배경','Solid background'),'background-solid')}</div>`)+hookView(model.settings.hooks)+panel(text('버전 및 채널','Version and channel'),readError(model.settings.version)+facts(Object.entries(model.settings.version.cli||{}).filter(([,v])=>typeof v!=='object'))+`<div class="management-actions">${button(text('업데이트 확인','Check updates'),'check-update')}${button(text('업데이트 설치','Install update'),'upgrade')}${button(text('프로젝트 지침 갱신','Migrate project'),'migrate',`data-project="${esc(scope)}"`)}${button(text('Web 재시작','Restart Web'),'restart')}</div>`+readError(model.settings.channels)+`<label>${text('채널','Channel')}<select id="channel-target"><option value="stable">Stable</option><option value="dev">Dev</option></select></label>${button(text('채널 전환','Switch channel'),'channel-switch')}`+evidence(model.settings.channels));
  }
  if(view==='storage') {
    const selected=scope?[{path:scope}]:inventory.projects;
    const outcomes=await Promise.all(selected.map(async p=>({project:p.path,...await settledReads({logs:scoped('/api/storage/logs',p.path),runtime:scoped('/api/runtime/storage',p.path)},signal)})));
    model.storage=outcomes;const globalIDs=new Set();
    body=selector(inventory.projects,true,scope)+outcomes.map(row=>panel(inventory.projects.find(p=>p.path===row.project)?.name||row.project,readError(row.logs)+readError(row.runtime)+facts([[text('실행 데이터','Runtime bytes'),row.runtime.total_bytes],[text('즉시 회수 가능','Reclaimable bytes'),row.runtime.cleanup?.reclaimable_bytes]])+table((row.logs.items||[]).filter(item=>{if(item.scope!=='global')return true;if(globalIDs.has(item.id))return false;globalIDs.add(item.id);return true;}),[[text('종류','Category'),'label'],['Bytes','size_bytes'],[text('보존 조건','Retention'),'release_condition'],[text('작업','Action'),'',r=>r.can_clear?button(text('로그 비우기','Clear log'),'clear-log',`data-project="${esc(row.project)}" data-id="${esc(r.id)}"`):esc(text('보호됨','Protected'))]])+`<div class="management-actions">${button(text('안전한 실행 기록 정리','Clean eligible runtime'),'cleanup-runtime',`data-project="${esc(row.project)}" ${row.runtime.cleanup?.reclaimable_bytes>0?'':'disabled'}`)}</div>`+evidence(row.runtime))).join('');
    if(!outcomes.length)body+=empty(text('등록된 프로젝트가 없습니다','No registered projects'));
  }
  if(view==='workload') {
    if(!scope)body=empty(text('등록된 프로젝트가 없습니다','No registered projects'));
    else {
      model.workload=await settledReads({workload:scoped('/api/workload',scope,scope===state.project&&state.backlog?{backlog:state.backlog}:{}),history:scoped('/api/runtime/history',scope,{page,page_size:20,provider:params.get('provider')||'',state:params.get('runtime_state')||''}),roots:scoped('/api/runtime/root-sessions',scope,{page:runtimePage,page_size:10,status:'previous',include_storage:1}),hooks:scoped('/api/runtime/hooks',scope)},signal);
      const w=model.workload.workload,h=model.workload.history,r=model.workload.roots;
      const agents=w.workload?.agents||w.agents||w.workers||[];
      body=selector(inventory.projects,false,scope)+panel(text('현재 워크로드','Current workload'),readError(w)+facts(Object.entries(w).filter(([,v])=>typeof v!=='object'))+table(Array.isArray(agents)?agents:Object.values(agents),[[text('담당','Agent'),'agent'],[text('역할','Role'),'role'],[text('진행 작업','Working tasks'),'',a=>(a.doing||[]).map(id=>link(esc(id),{view:'backlog',project:scope,detail:id,backlog:''})).join(' ')],[text('막힘','Blocking'),'blocking_count'],[text('준비 후보','Ready candidates'),'ready_candidate_count'],[text('근거','Evidence'),'',evidence]])+evidence(w))+panel(text('실행 attempt','Runtime attempts'),table(w.runtime_observability?.attempts,[[text('담당','Agent'),'agent_path'],[text('제공자','Provider'),'provider'],[text('상태','State'),'state'],[text('작업','Task'),'',r=>r.task_id?link(esc(r.task_id),{view:'backlog',project:scope,detail:r.task_id,backlog:''}):esc(text('작업 연결 없음','No task binding'))],[text('관측','Observed'),'last_observed_at'],[text('근거','Evidence'),'',evidence]]))+panel(text('현재 Root 세션','Current Root sessions'),facts(Object.entries(w.runtime_observability?.root_session_counts||{}))+table(w.runtime_observability?.root_sessions,[[text('세션','Session'),'display_name'],[text('상태','State'),'status'],[text('마지막 활동','Last activity'),'last_activity_at'],[text('현재 실행','Current attempts'),'current_count'],[text('확인 필요','Needs check'),'needs_check_count'],[text('근거','Evidence'),'',evidence]]))+panel(text('담당 미지정 및 보류 작업','Unassigned and held work'),`<ul class="task-list">${taskRows([...(w.workload?.unassigned_doing||[]).map(id=>w.all_items?.[id]||{id,title:id,state:'doing'}),...(w.workload?.released_holds||[]).map(t=>({...w.all_items?.[t.id],...t,state:'hold'}))].map(t=>({...t,project:scope})))}</ul>`)+hookView(model.workload.hooks)+panel(text('실행 이력','Execution history'),readError(h)+`<div class="management-fields"><label>${text('제공자','Provider')}<select id="runtime-provider">${['','codex','claude'].map(v=>`<option value="${v}" ${v===(params.get('provider')||'')?'selected':''}>${v||text('전체','All')}</option>`).join('')}</select></label><label>${text('상태','State')}<select id="runtime-state">${['','running','completed','interrupted','failed','unknown'].map(v=>`<option ${v===(params.get('runtime_state')||'')?'selected':''} value="${v}">${v||text('전체','All')}</option>`).join('')}</select></label></div>`+table(h.items,[[text('담당','Agent'),'agent_path'],[text('상태','State'),'state'],[text('작업','Task'),'task_id'],[text('종료','Ended'),'ended_at'],[text('근거','Evidence'),'',evidence]])+pager(h))+panel(text('이전 Root 세션','Previous Root sessions'),readError(r)+table(r.items,[[text('세션','Session'),'display_name'],[text('상태','State'),'status'],[text('마지막 활동','Last activity'),'last_activity_at'],['Bytes','storage_bytes'],[text('작업','Action'),'',row=>button(text('정리 계획 확인','Review cleanup'),'cleanup-root',`data-id="${esc(row.root_session_id)}" ${row.cleanup_eligible?'':'disabled'}`)]])+pager(r,'roots'));
    }
  }
  if(view==='operations') {model.operations=await get('/api/operations',signal);const o=model.operations;body=panel(text('감시 상태','Monitoring status'),facts(Object.entries(o).filter(([,v])=>typeof v!=='object'))+evidence(o))+panel(text('진행 단계','Active stages'),table(o.stages||o.active_stages,[[text('프로젝트','Project'),'project'],[text('작업','Task'),'task_id'],[text('단계','Stage'),'stage'],[text('근거','Evidence'),'',evidence]]))+panel(text('감지 및 복구 기록','Incidents and recovery'),table((o.active||[]).filter(r=>!r.recovered_at&&!r.resolution&&!r.recovery_evidence&&!r.policy_resolution&&r.kind!=='normal_exit'&&!r.history&&r.active!==false),[[text('종류','Kind'),'kind'],[text('작업','Task'),'task_id'],[text('상태','State'),'quality'],[text('근거','Evidence'),'evidence'],[text('조치','Action'),'action']])+evidence(o.active))+panel(text('최근 사건·복구 이력','Recent incidents and recovery'),table(o.recent,[[text('종류','Kind'),'kind'],[text('작업','Task'),'task_id'],[text('감지','Detected'),'detected_at'],[text('복구','Recovered'),'recovered_at'],[text('근거','Evidence'),'',evidence]]))+panel(text('해소된 관측','Resolved observations'),table(o.resolved_observations,[[text('작업','Task'),'task_id'],[text('복구','Recovered'),'recovered_at'],[text('근거','Evidence'),'',evidence]]))+panel(text('프로젝트 감시 범위','Project monitoring coverage'),table(o.projects,[[text('프로젝트 경로','Project path'),'path'],[text('마지막 검사','Last scan'),'last_scan_at'],[text('오류','Error'),'error']]));}
  if(view==='notifications') body=selector(inventory.projects,true,scope)+await notifications(inventory.projects,scope,signal);
  if(view==='preview') body=await preview(signal);
  if(view==='terminal') body=panel(text('터미널','Terminal'),`<iframe title="${text('Task Mecca 터미널','Task Mecca terminal')}" src="/terminal?embedded=1&lang=${state.language}&project=${encodeURIComponent(scope)}"></iframe>`);
  if(view==='manual'){model.manual=await get('/api/manual?lang='+state.language,signal);body=Object.entries(model.manual).filter(([k,v])=>['readme','session_guide','root_prompt'].includes(k)&&typeof v==='string').map(([k,v])=>panel(({readme:text('사용 안내','User guide'),session_guide:text('세션 가이드','Session guide'),root_prompt:'Root prompt'})[k],`<div class="markdown">${markdown(v)}</div><div class="management-actions">${button(text('복사','Copy'),'copy-manual',`data-key="${k}"`)}</div>`)).join('');}
  if(view==='issues') {
    if(!scope)body=empty(text('프로젝트를 선택하세요','Choose a project'));
    else {
      model.issues=await get(scoped('/api/issues',scope,scope===state.project&&state.backlog?{backlog:state.backlog}:{}),signal);
      model.issueEntries=[];
      for(const [type,value]of Object.entries(model.issues.health||{})){
        if(Array.isArray(value))model.issueEntries.push(...value.map(item=>({type,item})));
        else if(value&&typeof value==='object'&&Object.keys(value).length)model.issueEntries.push({type,item:value});
      }
      model.issueEntries.push(...(model.issues.diagnostics||[]).map(item=>({type:item.component||'doctor',item})));
      body=selector(inventory.projects,false,scope)+panel(text('진단 결과','Diagnostic results'),table(model.issueEntries,[[text('종류','Kind'),'type'],[text('현재 근거','Current evidence'),'',r=>`<pre>${esc(JSON.stringify(r.item,null,2))}</pre>`],[text('복구','Recovery'),'',r=>button(text('Agent 복원 요청 복사','Copy Agent recovery request'),'copy-diagnostic',`data-index="${model.issueEntries.indexOf(r)}"`)]]));
    }
  }
  if(view==='release-notes'){model.releases=await get('/api/release-notes?offset='+((page-1)*20)+'&limit=20',signal);const r=model.releases;body=panel(text('업데이트 내역','Release history'),(r.items||r.releases||[]).map(row=>`<details class="release-entry" data-version="${esc(row.version)}"><summary><strong>v${esc(row.version)}</strong> · ${esc(row.date)} · ${esc(localized(row.summary))}</summary><div class="release-content" role="status">${text('불러오는 중…','Loading…')}</div></details>`).join('')||empty(text('업데이트 내역이 없습니다','No release notes')))+`<div class="pagination">${button(text('이전','Previous'),'history-prev',page<=1?'disabled':'')}${button(text('다음','Next'),'history-next',r.has_more?'':'disabled')}</div>`;}
  if(signal?.aborted) throw new DOMException('Aborted','AbortError');
  activate(model,scope,page,runtimePage);
  return heading(text(...titleMap[view]),'')+`<div id="management-feedback" class="management-feedback" role="status" tabindex="-1"></div>`+body;
}
function activate(next,nextScope,nextPage,nextRuntimePage){model=next;scope=nextScope;page=nextPage;runtimePage=nextRuntimePage;}
function localized(v) {return typeof v==='object'?(v?.[state.language]||v?.en||v?.ko||''):v||'';}
function projectView(inventory){return panel(text('프로젝트 등록','Register project'),`<form id="add-project" class="management-fields"><label>${text('프로젝트 절대 경로','Absolute project path')}<input name="path" required placeholder="/path/to/project"></label><button>${text('등록','Register')}</button></form>`)+panel(text('등록된 프로젝트','Registered projects'),table(inventory.projects,[[text('프로젝트','Project'),'name'],[text('경로','Path'),'path'],[text('상태','State'),'status'],[text('작업','Actions'),'',p=>`<div class="management-actions">${link(text('백로그 열기','Open backlog'),{view:'backlog',project:p.path,backlog:'',detail:null,page:1},'button')}${button(text('감시 중지','Pause monitoring'),'project-unregister',`data-project="${esc(p.path)}"`)}${button(text('감시 재개','Resume monitoring'),'project-register',`data-project="${esc(p.path)}"`)}${button(text('보관','Archive'),'project-archive',`data-project="${esc(p.path)}"`)}${button(text('지침 갱신','Migrate'),'migrate',`data-project="${esc(p.path)}"`)}</div>`]]))+panel(text('보관 이력','Archived projects'),table(inventory.archives,[[text('프로젝트','Project'),'name'],[text('경로','Path'),'path'],[text('작업','Actions'),'',p=>`<div class="management-actions">${button(text('복원','Restore'),'project-restore',`data-id="${esc(p.id)}" data-project="${esc(p.path)}"`)}${button(text('이력 삭제','Forget archive'),'project-forget',`data-id="${esc(p.id)}" data-project="${esc(p.path)}"`)}</div>`]]));}
function hookView(h) {const hooks=h.hooks||h.providers||[];return panel(text('런타임 Hook','Runtime hooks'),readError(h)+evidence(h)+`<div class="management-fields"><label>${text('제공자','Provider')}<select id="hook-provider"><option>codex</option><option>claude</option><option>all</option></select></label><label>${text('적용 범위','Scope')}<select id="hook-scope"><option value="device">${text('현재 기기','Device')}</option><option value="project">${text('프로젝트','Project')}</option><option value="global">${text('전역','Global')}</option></select></label></div><div class="management-actions">${button(text('Hook 연결','Enable hooks'),'hook-enable')}${button(text('Hook 해제','Disable hooks'),'hook-disable')}</div>`);}
function updateQuery(values,reload){const u=new URL(location.href);for(const [k,v]of Object.entries(values)){if(v==null||v==='')u.searchParams.delete(k);else u.searchParams.set(k,v);}history.replaceState(history.state,'',u);reload({preserve:true});}
async function migrate(project){let result;try{result=await post('/api/migrate',{project});}catch(error){if(error.status!==409||!error.body?.plan_digest)throw error;const plan=error.body;if(!await confirmAction(text('변경된 지침 백업 후 갱신','Back up modified instructions and migrate'),project,`<pre>${esc(JSON.stringify(plan,null,2))}</pre>`))return;result=await post('/api/migrate',{project,choice:'backup',expected_plan:plan.plan_digest});}return result;}
export function bindManagement(reload) {
  document.getElementById('management-scope')?.addEventListener('change',e=>updateQuery({scope:e.target.value,project:e.target.value,backlog:null,op_page:1,runtime_page:1},reload));
  for(const [id,key]of [['runtime-provider','provider'],['runtime-state','runtime_state']])document.getElementById(id)?.addEventListener('change',e=>updateQuery({[key]:e.target.value,op_page:1},reload));
  document.getElementById('add-project')?.addEventListener('submit',async e=>{e.preventDefault();const path=new FormData(e.target).get('path').trim();await run(e.submitter,async()=>{const result=await post('/api/hub/projects',{action:'add',path});openProject(path);await refreshSidebar();await reload({preserve:true});return result;});});
  document.querySelectorAll('[data-manage]:not([data-manage^=telegram-]):not([data-manage^=push-]):not([data-manage^=notifications-]):not([data-manage^=preview-]):not([data-manage=web-permission])').forEach(b=>b.addEventListener('click',()=>run(b,async()=>{
    if(model.route!==location.href)throw Error(text('화면 문맥이 바뀌었습니다. 새로고침하세요.','Screen context changed. Refresh.'));
    const action=b.dataset.manage,project=b.dataset.project||scope,id=b.dataset.id;
    if(action==='background-image'||action==='background-solid'){setBackground(action.split('-')[1]);return;}
    if(action==='history-next'||action==='history-prev'){updateQuery({op_page:Math.max(1,page+(action.endsWith('next')?1:-1))},reload);return;}
    if(action==='roots-next'||action==='roots-prev'){updateQuery({runtime_page:Math.max(1,runtimePage+(action.endsWith('next')?1:-1))},reload);return;}
    if(action==='copy-diagnostic'){const entry=model.issueEntries[Number(b.dataset.index)];if(!entry)throw Error(text('진단 근거가 바뀌었습니다','Diagnostic evidence changed'));const request=['Task Mecca 백로그 복원 요청','프로젝트: '+scope,'백로그: '+(state.backlog||'자동 선택'),'진단 유형: '+entry.type,'문제 파일·항목·근거: '+JSON.stringify(entry.item,null,2),'요청: 현재 근거를 먼저 확인하고 원본 계약과 사용자 변경을 보존하는 복원 방안을 제시해 주세요. 이 복사는 Agent 실행·자동 복원·작업 재개를 승인하거나 수행하지 않습니다.'].join('\n');await navigator.clipboard.writeText(request);feedback(text('복원 요청을 복사했습니다. Agent에게 직접 전달하세요.','Recovery request copied. Send it to an Agent yourself.'));return;}
    if(action==='copy-manual'){await navigator.clipboard.writeText(model.manual[b.dataset.key]);feedback(text('복사했습니다','Copied'));return;}
    if(action==='check-update'){const result=await get(scoped('/api/version',scope,{refresh:1}));feedback(JSON.stringify(result.cli));return;}
    if(!await confirmAction(b.textContent,project||text('현재 설치에 적용합니다','Applies to this installation')))return;
    let result;
    if(action.startsWith('project-')){const operation=action.slice(8);result=await post('/api/hub/projects',{action:operation,path:project,archive_id:id||'',...['archive','forget'].includes(operation)?{confirm_path:project}:{}});await refreshSidebar();}
    else if(action==='migrate')result=await migrate(project);
    else if(action==='clear-log')result=await post(scoped('/api/storage/logs',project),{id});
    else if(action==='cleanup-runtime')result=await post(scoped('/api/runtime/storage',project),{action:'cleanup'});
    else if(action==='cleanup-root'){const plan=await get(scoped('/api/runtime/root-sessions',project,{root_session_id:id}));if(!plan.cleanup?.eligible)throw Error(plan.cleanup?.reason||text('정리할 수 없는 세션입니다','Session is not eligible for cleanup'));if(!await confirmAction(text('계획대로 세션 정리','Clean session with this plan'),id,`<pre>${esc(JSON.stringify(plan.cleanup,null,2))}</pre>`))return;result=await post(scoped('/api/runtime/root-sessions',project),{action:'cleanup',root_session_id:id});}
    else if(action.startsWith('hook-'))result=await post(scoped('/api/runtime/hooks',project),{action:action.slice(5),provider:document.getElementById('hook-provider').value,scope:document.getElementById('hook-scope').value});
    else if(['upgrade','restart','channel-switch'].includes(action)){
      const before=await get('/api/health');if(before.ok!==true||!before.boot_id||!before.instance_id||before.preview_mode!==false)throw Error(text('관리 서버 identity를 확인하지 못했습니다','Management server identity unverified'));
      let expected=before.version;
      if(action==='upgrade'){const target=model.settings?.version?.cli;expected=target?.latest_version||target?.latest||'';if(!expected)throw Error(text('목표 버전을 다시 확인하세요','Recheck the target version'));result=await post('/api/upgrade',{expected_version:expected});}
      else if(action==='restart')result=await post('/api/admin/restart',{});
      else {result=await post('/api/channel-switch',{channel:document.getElementById('channel-target').value});expected=result.to||'';}
      feedback(text('요청 응답을 받았습니다. 실제 실행 서버와 버전을 확인하고 있습니다…','Response received. Verifying the actual restarted server and version…'));
      if(result.restart_required===false){const current=await get('/api/health');if(expected&&current.version!==expected)throw Error(text('목표 버전과 실행 응답이 다릅니다','Running version differs from the target'));feedback(text('서버가 변경 결과를 확인했습니다','Server confirmed the resulting state'));return;}
      await verifyRestart(before,expected);return;
    }
    if(result){await reload({preserve:true});feedback(text('서버 처리 결과','Server result')+': '+JSON.stringify(result));}
  })));
  document.querySelectorAll('.release-entry').forEach(entry=>entry.addEventListener('toggle',async()=>{if(!entry.open||entry.dataset.loaded)return;try{const r=await get('/api/release-notes/'+encodeURIComponent(entry.dataset.version));if(!entry.isConnected)return;entry.dataset.loaded='true';entry.querySelector('.release-content').innerHTML=`<div class="markdown">${markdown(localized(r.body||r.markdown||r.summary))}</div>`+(r.sections||[]).map(s=>`<h3>${esc(localized(s.title)||s.type)}</h3><div class="markdown">${markdown(localized(s.body)||s.items?.map(localized).join('\n')||'')}</div>`).join('')+evidence(r);localStorage.setItem('task-mecca-release-notes-seen-'+entry.dataset.version,'1');}catch(e){entry.querySelector('.release-content').textContent=e.message;}}));
  if(state.view==='notifications')bindNotifications(reload);
  if(state.view==='preview')bindPreview(reload);
}
async function verifyRestart(before,expected){
  const deadline=Date.now()+45000;
  while(Date.now()<deadline){
    try{const current=await get('/api/health');if(current.ok===true&&current.preview_mode===false&&current.boot_id&&current.boot_id!==before.boot_id&&current.instance_id&&current.version&&(!expected||current.version===expected)){location.reload();return;}}catch{}
    await new Promise(resolve=>setTimeout(resolve,600));
  }
  throw Error(text('새 서버의 실행 identity와 목표 버전을 확인하지 못했습니다. 성공으로 처리하지 않습니다. 새로고침 후 확인하세요.','The restarted identity and target version remain unverified. Success is unconfirmed. Refresh to check.'));
}
async function run(button,action) {if(button.disabled)return;button.disabled=true;button.setAttribute('aria-busy','true');try{await action();}catch(error){feedback(error.message,true);}finally{if(button.isConnected){button.disabled=false;button.removeAttribute('aria-busy');}}}
