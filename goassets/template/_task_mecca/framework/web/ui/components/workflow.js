import {state} from '../state.js';
import {esc,fmtSec} from '../format.js';
import {fetchActivity} from '../data.js';
import {detailLifecycleBody} from './lifecycle.js';
const activityDetailCache=new Map();
const activityDetailPending=new Map();
function activityDetailKey(id,project=state.project){return JSON.stringify([project,String(id||'').toUpperCase()]);}
function activityDuration(ms){if(!Number.isFinite(ms)||ms<0)return '—';return fmtSec(Math.round(ms/1000));}

const activityDetailViews=new Map();
let workflowTimelineEntry=null;
function syncWorkflowTimelineEntry(){
  const entry=state.view==='backlog'&&state.detail?JSON.stringify([state.project,state.backlog,String(state.detail).toUpperCase()]):null;
  if(entry!==workflowTimelineEntry&&entry){const prefs=activityDetailPrefs(state.detail);prefs.window=null;prefs.windowMode='fit';prefs.selection=null;}
  workflowTimelineEntry=entry;
}
const ACTIVITY_DETAIL_MAX_ROWS=450;
const ACTIVITY_DETAIL_MAX_LEDGER=120;
function activityDetailPrefs(id,project=state.project){
  const key=activityDetailKey(id,project);
  if(!activityDetailViews.has(key)){
    if(activityDetailViews.size>=24)activityDetailViews.delete(activityDetailViews.keys().next().value);
    activityDetailViews.set(key,{round:'all',mode:'timeline',zoom:1,focus:'manual',groupAgents:false,ledger:false,windowMode:'fit'});
  }
  return activityDetailViews.get(key);
}
function activityEventTime(value,ko){
  const stamp=Date.parse(value||'');
  return Number.isFinite(stamp)?new Date(stamp).toLocaleString(ko?'ko-KR':'en-US'):'—';
}
function activityRoundBreakdown(segments){
  const rounds=new Map();
  for(const seg of segments){
    if(!Number.isFinite(seg.duration_ms)||seg.duration_ms<0)continue;
    const round=Number(seg.round)||0;
    if(!rounds.has(round))rounds.set(round,new Map());
    const group=rounds.get(round);
    group.set(seg.type,(group.get(seg.type)||0)+seg.duration_ms);
  }
  return rounds;
}
function activityDisplayType(type,ko){
  const names={requirements:['요구사항 분석','Requirements'],research:['자료 조사','Research'],implementation:['구현','Implementation'],targeted_test:['테스트','Test'],full_regression:['테스트','Test'],build_check:['빌드 확인','Build check'],controller_review:['Controller 검토','Controller review'],controller_rereview:['Controller 재검토','Controller rereview'],handoff:['작업 인계','Handoff'],recovery:['복구 작업','Recovery'],other:['기타 활동','Other activity'],registration_authorization:['작업 실행 승인','Execution authorization']};
  return names[type]?(ko?names[type][0]:names[type][1]):String(type||'—');
}
function activityDisplayRole(role,ko){
  return ({registrar:'Registrar',controller:'Controller',sub_controller:'Sub-Controller',worker:'Worker'})[role]||workflowText(ko,'역할 미확인','Role unverified');
}
function activityDisplayWait(kind,ko){
  const names={registration_queue:['작업 배정 대기','Awaiting assignment'],dispatch_wait:['작업 착수 대기','Awaiting work start']};
  return names[kind]?(ko?names[kind][0]:names[kind][1]):String(kind||'—');
}
function activityDisplayDuration(ms,ko){
  if(!Number.isFinite(ms)||ms<0)return '—';
  if(ms>0&&ms<1000)return new Intl.NumberFormat(ko?'ko-KR':'en-US',{maximumFractionDigits:3}).format(ms)+(ko?'밀리초':'ms');
  const sec=Math.round(ms/1000),h=Math.floor(sec/3600),m=Math.floor(sec%3600/60),s=sec%60;
  return (ko?[h?h+'시간':null,m?m+'분':null,(!h&&!m||s)?s+'초':null]:[h?h+'h':null,m?m+'m':null,(!h&&!m||s)?s+'s':null]).filter(Boolean).join(' ');
}
function workflowText(ko,koText,enText){return ko?koText:enText;}
function workflowRoundLabel(round,ko){return round==null?workflowText(ko,'회차 미분류','Unclassified round'):Number(round)===0?workflowText(ko,'최초 작업','Initial work'):workflowText(ko,'보완'+round+'차','Correction '+round);}
function workflowAgentLabel(agent,ko){return workflowText(ko,'이름 미확인','Name unverified');}
function workflowActorLabel(row,ko){
  const role=activityDisplayRole(row.role,ko),name=row.display_name;
  return row.display_name_source==='bound_runtime_agent_path'&&name&&name.toLowerCase().replace(/[-_ ]/g,'')!==role.toLowerCase().replace(/[-_ ]/g,'')?role+' · '+name:role;
}
function workflowPhaseLabel(value,ko){
  const labels={queued:['배정 대기','Queued'],dispatch_wait:['착수 확인 대기','Awaiting start'],implementation:['구현 중','Implementing'],rework:['수정 중','Correcting'],rework_requested:['수정 요청됨','Changes requested'],controller_review:['Controller 검토','Controller review'],finalizing:['완료 처리 중','Finalizing'],completed:['완료','Completed'],external_ci_wait:['외부 검증 대기','External verification'],user_decision_wait:['사용자 판단 대기','User decision'],wait_unclassified:['대기 사유 미확인','Wait reason unverified'],recovery:['실행 근거 확인 필요','Execution needs verification'],unknown:['처리 단계 미확인','Phase unverified']};
  return labels[value]?.[ko?0:1]||labels.unknown[ko?0:1];
}
function workflowActionLabel(value,ko){
  const labels={assign_worker:['Worker 배정','Assign Worker'],confirm_worker_start:['실제 착수 확인','Confirm actual start'],implement_and_verify:['구현·검증 진행','Implement and verify'],review_report:['결과와 수용 기준 검토','Review result and criteria'],dispatch_correction:['수정 범위 인계','Hand off corrections'],finalize_backlog_and_external_result:['검증 결과·완료 기록 반영','Finalize verified result'],await_ci_result:['외부 검증 결과 확인','Check verification result'],user_decision:['사용자 판단 확인','Await user decision'],verify_wait_reason:['대기 근거 확인','Verify wait evidence'],verify_execution_evidence:['실행 근거 확인','Verify execution evidence'],none:['추가 실행 없음','No further execution']};
  return labels[value]?.[ko?0:1]||labels.verify_execution_evidence[ko?0:1];
}
function workflowMeasurementLabel(status,ko){return ({not_recorded:['활동 미측정','Activity unmeasured'],partial:['부분 관측','Partially observed'],observed:['관측됨','Observed'],conflict:['근거 충돌','Conflicting evidence']})[status]?.[ko?0:1]||workflowText(ko,'측정 상태 미확인','Measurement unverified');}
function workflowRoleClass(role){return ['worker','controller','registrar','sub_controller','wait'].includes(role)?role:'unknown';}
function workflowMetric(metric,ko){return metric&&Number.isFinite(metric.duration_ms)?activityDisplayDuration(metric.duration_ms,ko):workflowText(ko,'미측정','Unmeasured');}
function workflowIntervalType(interval,ko){
  if(interval.kind==='execution_report')return interval.end?workflowText(ko,'실행 보고 · 활동 미측정','Execution reported · activity unmeasured'):workflowText(ko,'시작만 관측 · 종료 미확인','Start observed · end unverified');
  if(interval.kind==='handoff_review')return workflowText(ko,'검토 인계 · 활동 미측정','Review claimed · activity unmeasured');
  return activityDisplayType(interval.type,ko)+(interval.kind==='activity_open'?workflowText(ko,' · 종료 미관측',' · end unobserved'):'');
}
function workflowOutcome(value,ko){return ({passed:['성공','Passed'],ok:['수용','Accepted'],accepted:['수용','Accepted'],failed:['실패','Failed'],rejected:['반려','Rejected'],needs_changes:['수정 요청','Changes requested'],cancelled:['취소','Cancelled']})[value]?.[ko?0:1]||'';}
function workflowViewWindow(wf,prefs){
  const start=Date.parse(wf.axis?.start||''),end=Date.parse(wf.axis?.end||'');
  if(!Number.isFinite(start)||!Number.isFinite(end)||end<start)return null;
  const boundEnd=Math.max(start+1,end),full=boundEnd-start;
  let view=prefs.window;
  if(prefs.windowMode==='fit'||!view||!Number.isFinite(view.start)||!Number.isFinite(view.end)){view={start,end:boundEnd};prefs.windowMode='fit';}
  const span=Math.max(1,Math.min(full,view.end-view.start));
  const left=Math.max(start,Math.min(boundEnd-span,view.start));
  return prefs.window={start:left,end:left===boundEnd-span?boundEnd:left+span};
}
function workflowSetWindow(wf,prefs,start,end){prefs.windowMode='manual';prefs.window={start,end};const view=workflowViewWindow(wf,prefs);if(view&&view.start===Date.parse(wf.axis.start)&&view.end===Math.max(Date.parse(wf.axis.start)+1,Date.parse(wf.axis.end)))prefs.windowMode='fit';return view;}
function workflowZoomWindow(wf,prefs,factor,anchor,ratio=.5){
  const view=workflowViewWindow(wf,prefs);if(!view)return;
  const span=Math.min(Math.max(1,Date.parse(wf.axis.end)-Date.parse(wf.axis.start)),Math.max(1,(view.end-view.start)*factor));
  const time=Number.isFinite(anchor)?anchor:(view.start+view.end)/2;
  return workflowSetWindow(wf,prefs,time-span*ratio,time+span*(1-ratio));
}
function workflowTimeLabel(time,view,ko){
  const span=view.end-view.start,milliseconds=span<1000,seconds=span<300000||view.start%60000!==0||view.end%60000!==0;
  const options={hour:'2-digit',minute:'2-digit',...(seconds?{second:'2-digit'}:{}),...(milliseconds?{fractionalSecondDigits:3}:{})};
  return new Date(time).toLocaleTimeString(ko?'ko-KR':'en-GB',options);
}
function workflowVisibleRange(view,ko,axis=null){
  const first=new Date(view.start),last=new Date(view.end),crossesDay=first.toDateString()!==last.toDateString();
  const multipleDays=axis&&new Date(axis.start).toDateString()!==new Date(axis.end).toDateString();
  const yearsDiffer=first.getFullYear()!==last.getFullYear()||(axis&&new Date(axis.start).getFullYear()!==new Date(axis.end).getFullYear());
  const date=time=>new Date(time).toLocaleDateString(ko?'ko-KR':'en-GB',{month:'2-digit',day:'2-digit',...(yearsDiffer?{year:'numeric'}:{})});
  return (date(view.start)+' · ')+workflowTimeLabel(view.start,view,ko)+' — '+(crossesDay?date(view.end)+' · ':'')+workflowTimeLabel(view.end,view,ko);
}
function workflowIntervalKey(row,interval){return JSON.stringify([row.id,interval.id,interval.kind]);}
// Exact timestamps decide continuity/overlap; CSS geometry uses relative offsets.
// This preserves positive sub-millisecond gaps instead of Date.parse rounding them into joins.
function workflowTimeNS(value){
  const match=String(value||'').match(/^(.*:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/);
  const millis=Date.parse(match?match[1]+match[3]:value||'');
  if(!Number.isFinite(millis))return null;
  return BigInt(Math.trunc(millis))*1000000n+(match?BigInt((match[2]||'').padEnd(9,'0')):0n);
}
function workflowIntervalBounds(interval,axisEnd){
  const from=Date.parse(interval.start||''),point=!interval.end&&['execution_report','handoff_review'].includes(interval.kind),open=!interval.end&&!point;
  return {from,to:interval.end?Date.parse(interval.end):point?from:Date.parse(axisEnd),point,open,fromNS:workflowTimeNS(interval.start),toNS:workflowTimeNS(interval.end|| (point?interval.start:axisEnd))};
}
function workflowActivityLayout(row,axisEnd){
  const entries=(Array.isArray(row.intervals)?row.intervals:[]).filter(i=>i.kind==='activity'||i.kind==='activity_open').map((interval,index)=>({interval,index,...workflowIntervalBounds(interval,axisEnd)})).filter(e=>e.fromNS!==null&&e.toNS!==null&&e.toNS>=e.fromNS);
  entries.sort((a,b)=>a.fromNS<b.fromNS?-1:a.fromNS>b.fromNS?1:a.index-b.index);
  const ends=[],runs=[],lastByLane=[];
  for(const entry of entries){let lane=0;while(lane<ends.length&&ends[lane]>entry.fromNS)lane++;ends[lane]=entry.toNS;entry.lane=lane;
    const previous=lastByLane[lane];
    if(previous&&previous.entries.at(-1).interval.end&&previous.toNS===entry.fromNS){previous.entries.push(entry);previous.toNS=entry.toNS;previous.to=entry.to;previous.open=entry.open;}
    else{const run={from:entry.from,to:entry.to,fromNS:entry.fromNS,toNS:entry.toNS,lane,open:entry.open,entries:[entry]};runs.push(run);lastByLane[lane]=run;}
  }
  return {entries,runs,lanes:Math.max(1,ends.length)};
}
function workflowEdgeState(wf,view){return {left:Boolean(view&&view.start>Date.parse(wf.axis?.start)),right:Boolean(view&&view.end<Date.parse(wf.axis?.end))};}
function workflowPanButton(side,ko,hidden){return '<button type="button" class="workflow-pan-button" data-workflow-pan="'+side+'"'+(hidden?' hidden':'')+' aria-label="'+esc(side==='left'?workflowText(ko,'화면 밖 이전 시간으로 이동','Pan to earlier time outside view'):workflowText(ko,'화면 밖 이후 시간으로 이동','Pan to later time outside view'))+'"><svg width="14" height="14" viewBox="0 0 20 20" aria-hidden="true" focusable="false"><path d="'+(side==='left'?'M12 4 6 10l6 6':'m8 4 6 6-6 6')+'" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></button>';}
function workflowPlotHTML(wf,prefs,ko,drawWindow=null){
  const view=workflowViewWindow(wf,prefs);if(!view)return '';
  const draw=drawWindow||view;
  const span=view.end-view.start,axis=wf.axis,rows=Array.isArray(wf.rows)?wf.rows:[];
  const selected=prefs.round==='all'?'all':String(prefs.round),visible=selected==='all'?rows:rows.filter(r=>String(r.round??'unknown')===selected);
  const originNS=workflowTimeNS(axis.start),origin=Date.parse(axis.start),relative=ns=>Number(ns-originNS)/1000000,pctNS=ns=>(relative(ns)-(view.start-origin))/span*100;
  const ticks=[0,.25,.5,.75,1].map(p=>'<span data-workflow-tick="'+p+'" style="left:'+p*100+'%;transform:translateX('+(p===0?0:p===1?-100:-50)+'%)">'+esc(workflowTimeLabel(view.start+span*p,view,ko))+'</span>').join('');
  const intervalHTML=(row,interval,index,layout,run)=>{
    const {from,to,point,fromNS,toNS}=workflowIntervalBounds(interval,axis.end);
    if(!Number.isFinite(from)||!Number.isFinite(to)||to<from||to<draw.start||from>draw.end)return '';
    const label=workflowIntervalType(interval,ko),outcome=workflowOutcome(interval.outcome,ko),key=workflowIntervalKey(row,interval);
    const detail=[workflowActorLabel(row,ko),workflowRoundLabel(row.round,ko),label,activityEventTime(interval.start,ko)+' — '+(interval.end?activityEventTime(interval.end,ko):workflowText(ko,'종료 미관측','End unobserved')),interval.measured?activityDisplayDuration(interval.duration_ms,ko):workflowText(ko,'활동 시간 미측정','Activity unmeasured'),outcome].filter(Boolean).join(' · ');
    const full=detail+'\n'+JSON.stringify({role:row.role,agent_id:row.agent_id,display_name:row.display_name,display_name_source:row.display_name_source,interval},null,2)+'\n'+workflowText(ko,'실행: ','Attempt: ')+(interval.attempt_id||'—')+'\n'+workflowText(ko,'배정: ','Assignment: ')+(interval.assignment_id||'—')+'\n'+workflowText(ko,'인계: ','Handoff: ')+(interval.handoff_id||'—')+(interval.evidence?'\n'+interval.evidence:'');
    const kind=['activity','activity_open','execution_report','handoff_review'].includes(interval.kind)?interval.kind:'execution_report';
    const lane=Math.max(0,Number(layout.lane)||0),pitch=layout.expanded?24:20/layout.lanes,height=point?24:layout.expanded?20:Math.max(2,Math.min(20,pitch-2));
    return '<button type="button" class="workflow-interval wf-role-'+workflowRoleClass(row.role)+' wf-kind-'+kind+(point?' wf-start-only':'')+(prefs.selection?.key===key?' is-selected':'')+'" data-workflow-interval="'+esc(key)+'" data-workflow-start="'+from+'" data-workflow-end="'+to+'" data-workflow-relative-start="'+relative(fromNS)+'" data-workflow-relative-end="'+relative(toNS)+'" data-workflow-detail="'+esc(full)+'" data-workflow-summary="'+esc(detail)+'" data-workflow-title="'+esc(label+' · '+(interval.measured?activityDisplayDuration(interval.duration_ms,ko):workflowText(ko,'미측정','Unmeasured')))+'" data-workflow-fingerprint="'+esc(JSON.stringify([row.display_name,row.display_name_source,row.name_evidence,interval]))+'" aria-label="'+esc(detail)+'" style="left:'+(Number(fromNS-run.fromNS)/Math.max(1,Number(run.toNS-run.fromNS))*100)+'%;width:'+(Number(toNS-fromNS)/Math.max(1,Number(run.toNS-run.fromNS))*100)+'%;top:'+(point&&!layout.expanded?2:lane*pitch+4)+'px;height:'+height+'px"><span class="activity-timeline-duration-label">'+esc(label)+(interval.measured?' · '+esc(activityDisplayDuration(interval.duration_ms,ko)):'')+'</span></button>';
  };
  const rowHTML=visible.slice(0,ACTIVITY_DETAIL_MAX_ROWS).map(row=>{
    const layout=workflowActivityLayout(row,axis.end),intervals=layout.entries.map(e=>e.interval);
    if(!intervals.length)return '';
    const lane=layout.lanes-1,expanded=Boolean(prefs.expandedRows?.[row.id]);
    const outcomes=[...new Set(intervals.map(i=>i.outcome).filter(o=>workflowOutcome(o,ko)))];
    const round=workflowRoundLabel(row.round,ko),value=Number.isFinite(row.observed_activity_ms)?activityDisplayDuration(row.observed_activity_ms,ko):workflowText(ko,'미측정','Unmeasured');
    const fullLabel=[workflowActorLabel(row,ko),round,value].join(' · ');
    const label='<button type="button" class="workflow-row-label" data-workflow-row-select="'+esc(row.id)+'" data-workflow-title="'+esc(fullLabel)+'" data-workflow-fingerprint="'+esc(JSON.stringify(row))+'" aria-label="'+esc(fullLabel)+'"><i class="workflow-role-swatch wf-role-'+workflowRoleClass(row.role)+'"></i><span>'+esc(workflowActorLabel(row,ko))+'<small>'+esc(round+' · '+workflowText(ko,'활동 누적 ','Activity total ')+value)+'</small></span></button>';
    const results=outcomes.length>1?'<span class="workflow-result-badge" data-workflow-row-outcome="mixed" data-workflow-result-count="'+outcomes.length+'" title="'+esc(outcomes.map(o=>workflowOutcome(o,ko)).join(' · '))+'">'+outcomes.length+'</span>':outcomes.map(o=>'<span class="workflow-result-badge" data-workflow-row-outcome="'+esc(o)+'" title="'+esc(workflowOutcome(o,ko))+'">'+esc(workflowOutcome(o,ko))+'</span>').join('');
    const runs=layout.runs.map(run=>{
      if(run.to<draw.start||run.from>draw.end)return '';
      const pitch=expanded?24:20/layout.lanes,height=expanded?20:Math.max(2,Math.min(20,pitch-2)),top=run.lane*pitch+4,total=Math.max(1,Number(run.toNS-run.fromNS)),members=run.entries.map(e=>workflowIntervalKey(row,e.interval));
      const dividers=run.entries.slice(1).map(e=>'<span class="workflow-divider" style="left:'+Number(e.fromNS-run.fromNS)/total*100+'%"></span>').join('');
      const open=run.entries.filter(e=>e.open).map(e=>'<span class="workflow-open-paint" style="left:'+Number(e.fromNS-run.fromNS)/total*100+'%;width:'+Number(e.toNS-e.fromNS)/total*100+'%"></span>').join('');
      return '<div class="workflow-run wf-role-'+workflowRoleClass(row.role)+'" data-workflow-run data-workflow-members="'+esc(JSON.stringify(members))+'" data-workflow-from-ns="'+run.fromNS+'" data-workflow-to-ns="'+run.toNS+'" style="left:'+pctNS(run.fromNS)+'%;width:'+(Number(run.toNS-run.fromNS)/1000000/span*100)+'%;--workflow-run-height:'+height+'px;--workflow-run-top:'+top+'px"><div class="workflow-run-surface'+(run.open?' is-open':'')+'" aria-hidden="true">'+open+dividers+'</div>'+run.entries.map(e=>intervalHTML(row,e.interval,e.index,{lane:e.lane,lanes:layout.lanes,expanded},run)).join('')+'</div>';
    }).join('');
    return '<div class="workflow-agent-row" data-workflow-row="'+esc(row.id)+'"><div class="workflow-agent-label">'+label+results+(lane?'<button type="button" class="workflow-row-expand" data-workflow-row-expand="'+esc(row.id)+'" aria-expanded="'+expanded+'" aria-label="'+esc(workflowText(ko,(expanded?'접기: ':'펼치기: ')+(lane+1)+'개 동시 구간',(expanded?'Collapse: ':'Expand: ')+(lane+1)+' concurrent intervals'))+'">'+(lane+1)+'</button>':'')+'</div><div class="workflow-agent-plot"><div class="workflow-agent-track" style="height:'+(expanded?(lane+1)*24+8:28)+'px">'+runs+'</div></div></div>';
  }).join('');
  return '<div class="workflow-ruler-row"><span>'+esc(workflowText(ko,'역할 · 회차','Role · round'))+'</span><div class="activity-time-ruler workflow-axis-track">'+ticks+'</div></div>'+rowHTML+(!rowHTML?'<p class="workflow-empty">'+esc(workflowText(ko,'실제 활동 구간 미측정 · 실행 보고는 근거에서 확인할 수 있습니다.','Actual activity unmeasured · execution reports are available in sources.'))+'</p>':'');
}
const workflowFloatingViews=new WeakMap();
let workflowActiveFloating=null;
// One nonmodal floating surface per chart. No polling or additional source reads.
function workflowFloatingView(root,viewport,wf,pref,ko){
  const abort=new AbortController(),options={signal:abort.signal},surface=document.createElement('div');
  surface.className='workflow-floating';surface.hidden=true;document.body.append(surface);
  let anchor=null,pinned=false,keyboard=false,positionFrame=0,disposed=false,returningFocus=false;
  const key=node=>node?.dataset.workflowInterval||JSON.stringify(['row',node?.dataset.workflowRowSelect]);
  const memberFingerprint=node=>node?.closest('.workflow-run')?.dataset.workflowMembers||null;
  const anchorVisible=node=>{if(!node.dataset.workflowInterval)return true;const view=workflowViewWindow(wf,pref),origin=Date.parse(wf.axis.start);return Number(node.dataset.workflowRelativeEnd)>=view.start-origin&&Number(node.dataset.workflowRelativeStart)<=view.end-origin;};
  const anchors=()=>[...root.querySelectorAll('[data-workflow-interval],[data-workflow-row-select]')];
  const visiblePeers=(node,row)=>{
    const view=workflowViewWindow(wf,pref),origin=Date.parse(wf.axis.start),left=view.start-origin,right=view.end-origin,members=memberFingerprint(node),memberKeys=members?new Set(JSON.parse(members)):null;
    const live=new Set(anchors().filter(n=>n.dataset.workflowInterval&&Number(n.dataset.workflowRelativeEnd)>=left&&Number(n.dataset.workflowRelativeStart)<=right).map(n=>n.dataset.workflowInterval));
    return row?.intervals.filter(i=>['activity','activity_open'].includes(i.kind)&&live.has(workflowIntervalKey(row,i))&&(!memberKeys||memberKeys.has(workflowIntervalKey(row,i))))||[];
  };
  const readingState=()=>{const active=document.activeElement,details=surface.querySelector('#workflowIntervalEvidence'),included=surface.querySelector('.workflow-included-events'),text=surface.querySelector('#workflowIntervalEvidence pre')?.firstChild,selection=getSelection();return {open:details?.open||false,includedOpen:included?.open||false,scrollTop:surface.scrollTop,focus:active===surface?'surface':active===surface.querySelector('#workflowIntervalEvidence>summary')?'summary':active===included?.querySelector('summary')?'included-summary':active?.dataset?.workflowMember?'member:'+active.dataset.workflowMember:active===surface.querySelector('button')?'close':null,selection:text&&selection?.anchorNode===text&&selection.focusNode===text?{anchor:selection.anchorOffset,focus:selection.focusOffset}:null};};

  const place=()=>{
    positionFrame=0;if(disposed||surface.hidden||!anchor)return;
    const r=anchor.getBoundingClientRect(),clip=viewport.getBoundingClientRect();
    if(!anchor.isConnected||r.right<clip.left||r.left>clip.right||r.bottom<0||r.top>innerHeight){close(false);return;}
    const margin=8,width=Math.min(pinned?360:300,innerWidth-margin*2);surface.style.width=width+'px';
    const topMargin=Math.max(margin,(document.querySelector('.topbar')?.getBoundingClientRect().bottom||0)+margin),belowSpace=innerHeight-margin-r.bottom-10,aboveSpace=r.top-topMargin-10;
    surface.style.maxHeight=Math.max(80,Math.max(belowSpace,aboveSpace))+'px';
    let h=surface.getBoundingClientRect().height;
    const useBelow=h<=belowSpace||belowSpace>=aboveSpace;
    surface.style.maxHeight=Math.max(80,useBelow?belowSpace:aboveSpace)+'px';h=surface.getBoundingClientRect().height;
    const left=Math.max(margin,Math.min(innerWidth-width-margin,(Math.max(r.left,clip.left)+Math.min(r.right,clip.right)-width)/2)),top=Math.max(topMargin,useBelow?r.bottom+10:r.top-h-10);
    surface.style.left=left+'px';surface.style.top=top+'px';
  };
  const queuePlace=()=>{if(!disposed&&!positionFrame)positionFrame=requestAnimationFrame(place);};
  const close=(restore=false,clear=true)=>{
    const previous=anchor;anchor?.classList.remove('is-selected');anchor?.removeAttribute('aria-describedby');anchor?.removeAttribute('aria-expanded');
    root.querySelectorAll('.workflow-interval.is-selected,.workflow-row-label.is-selected').forEach(node=>{node.classList.remove('is-selected');node.removeAttribute('aria-expanded');});
    anchor=null;pinned=false;surface.classList.remove('is-pinned');surface.hidden=true;surface.replaceChildren();if(clear)pref.selection=null;
    if(restore&&keyboard&&previous?.isConnected){returningFocus=true;previous.focus({preventScroll:true});returningFocus=false;}
  };
  const show=(node,pin=false,fromKeyboard=false,reading=null)=>{
    if(!node||disposed)return;
    if(pin&&pinned&&anchor===node){close(fromKeyboard);return;}
    close(false,!pin);anchor=node;pinned=pin;keyboard=fromKeyboard;
    const rowID=node.dataset.workflowRowSelect,entry=rowID?null:JSON.parse(node.dataset.workflowInterval),row=wf.rows.find(r=>r.id===(rowID||entry?.[0]));
    const interval=entry&&row?.intervals.find(i=>i.id===entry[1]&&i.kind===entry[2]);
    const peers=visiblePeers(node,row);
    if(!row){close(false);return;}
    const title=node.dataset.workflowTitle||node.dataset.workflowSummary||workflowActorLabel(row,ko);
    surface.id='workflowActivityFloating';surface.classList.toggle('is-pinned',pin);surface.setAttribute('role',pin?'dialog':'tooltip');
    surface.setAttribute('aria-label',title);surface.tabIndex=pin?-1:0;
    if(pin){
      const badges=[workflowActorLabel(row,ko),workflowRoundLabel(row.round,ko),workflowOutcome(interval?.outcome,ko)].filter(Boolean);
      const times=interval?'<dl><div><dt>'+esc(workflowText(ko,'시작','Start'))+'</dt><dd>'+esc(activityEventTime(interval.start,ko))+'</dd></div><div><dt>'+esc(workflowText(ko,'종료','End'))+'</dt><dd>'+esc(interval.end?activityEventTime(interval.end,ko):workflowText(ko,'종료 미관측','End unobserved'))+'</dd></div></dl>':'';
      surface.innerHTML='<div class="workflow-floating-head"><strong>'+esc(title)+'</strong><button type="button" class="workflow-floating-close" aria-label="'+esc(workflowText(ko,'구간 상세 닫기','Close interval details'))+'">×</button></div><div class="workflow-floating-badges">'+badges.map(b=>'<span>'+esc(b)+'</span>').join('')+'</div>'+times+'<details id="workflowIntervalEvidence"><summary>'+esc(workflowText(ko,'정확한 근거','Exact evidence'))+'</summary><pre>'+esc(JSON.stringify({role:row.role,agent_id:row.agent_id,display_name:row.display_name||null,display_name_source:row.display_name_source||'unverified',name_evidence:row.name_evidence,round:row.round,interval:interval||undefined,intervals:interval?undefined:row.intervals},null,2))+'</pre></details>';
      if(peers.length>1){
        const list=document.createElement('details');list.className='workflow-included-events';list.open=reading?.includedOpen??Boolean(interval&&node.getBoundingClientRect().width<7);
        list.innerHTML='<summary>'+esc(workflowText(ko,'포함 이벤트','Included events'))+' ('+peers.length+')</summary><div class="workflow-member-list">'+peers.map(i=>'<button type="button" class="workflow-member" data-workflow-member="'+esc(workflowIntervalKey(row,i))+'" aria-pressed="'+(i===interval)+'">'+esc(workflowIntervalType(i,ko)+' · '+(i.measured?activityDisplayDuration(i.duration_ms,ko):workflowText(ko,'미측정','Unmeasured')))+'<small>'+esc(activityEventTime(i.start,ko))+'</small></button>').join('')+'</div>';
        surface.insertBefore(list,surface.querySelector('#workflowIntervalEvidence'));
        list.querySelectorAll('[data-workflow-member]').forEach(button=>button.addEventListener('click',e=>{const target=anchors().find(a=>a.dataset.workflowInterval===button.dataset.workflowMember&&a.tabIndex!==-1);if(target)show(target,true,e.detail===0,{open:false,includedOpen:true,scrollTop:surface.scrollTop,focus:'member:'+button.dataset.workflowMember,selection:null});},options));
        list.addEventListener('toggle',queuePlace,options);
      }
      surface.querySelector('.workflow-floating-close').addEventListener('click',e=>{if(e.detail===0)keyboard=true;close(true);},options);
      surface.querySelector('#workflowIntervalEvidence').addEventListener('toggle',queuePlace,options);
      node.classList.add('is-selected');node.setAttribute('aria-expanded','true');
      pref.selection={key:key(node),fingerprint:node.dataset.workflowFingerprint,keyboard:fromKeyboard,membersFingerprint:memberFingerprint(node),visibleMembersFingerprint:JSON.stringify(peers.map(i=>workflowIntervalKey(row,i)))};
    }else{surface.textContent=title;node.setAttribute('aria-describedby',surface.id);}
    if(pin&&reading)surface.querySelector('#workflowIntervalEvidence').open=reading.open;
    surface.hidden=false;place();
    if(pin&&reading){
      const focus=reading.focus==='summary'?surface.querySelector('#workflowIntervalEvidence>summary'):reading.focus==='close'?surface.querySelector('button'):reading.focus==='surface'?surface:reading.focus==='included-summary'?surface.querySelector('.workflow-included-events>summary'):reading.focus?.startsWith('member:')?[...surface.querySelectorAll('[data-workflow-member]')].find(b=>b.dataset.workflowMember===reading.focus.slice(7)):null;
      if(focus)focus.focus({preventScroll:true});
      if(reading.selection){const text=surface.querySelector('pre')?.firstChild;if(text){const selection=getSelection(),limit=text.textContent.length;selection.setBaseAndExtent(text,Math.min(limit,reading.selection.anchor),text,Math.min(limit,reading.selection.focus));}}
      surface.scrollTop=reading.scrollTop;
    }else if(pin&&fromKeyboard)surface.focus({preventScroll:true});
  };
  const reconcile=()=>{
    if(pinned||pref.selection){const selection=pref.selection,node=selection&&anchors().find(n=>key(n)===selection.key&&n.dataset.workflowFingerprint===selection.fingerprint&&memberFingerprint(n)===selection.membersFingerprint&&anchorVisible(n));
      if(!node){close(false);return;}
      if(!pinned)show(node,true,Boolean(selection.keyboard&&selection.focused),selection.reading);
      else{const rowID=node.dataset.workflowRowSelect||JSON.parse(node.dataset.workflowInterval)[0],row=wf.rows.find(r=>r.id===rowID),visible=JSON.stringify(visiblePeers(node,row).map(i=>workflowIntervalKey(row,i)));
        if(visible!==selection.visibleMembersFingerprint){const reading=readingState();pinned=false;show(node,true,false,reading);}
        else{anchor?.classList.remove('is-selected');anchor=node;node.classList.add('is-selected');node.setAttribute('aria-expanded','true');queuePlace();}
      }
    }else if(anchor&&!anchor.isConnected)close(false);
  };
  viewport.addEventListener('pointerover',e=>{if(e.pointerType!=='touch'&&!pinned)show(e.target.closest('[data-workflow-interval],[data-workflow-row-select]'));},options);
  viewport.addEventListener('pointerout',e=>{if(!pinned&&anchor&&!anchor.contains(e.relatedTarget))close(false);},options);
  surface.addEventListener('keydown',e=>e.stopPropagation(),options);
  viewport.addEventListener('focusin',e=>{if(!returningFocus&&!pinned)show(e.target.closest('[data-workflow-interval],[data-workflow-row-select]'));},options);
  viewport.addEventListener('focusout',()=>{if(!pinned)close(false);},options);
  document.addEventListener('pointerdown',e=>{if(!surface.hidden&&!surface.contains(e.target)&&!anchor?.contains(e.target))close(false);}, {...options,capture:true});
  document.addEventListener('keydown',e=>{if(e.key==='Escape'&&!surface.hidden){e.preventDefault();e.stopImmediatePropagation();if(pinned)keyboard=true;close(true);}}, {...options,capture:true});
  window.addEventListener('resize',queuePlace,options);document.addEventListener('scroll',queuePlace,{...options,capture:true,passive:true});
  const observer=new MutationObserver(()=>{if(!root.isConnected)dispose();});observer.observe(document.body,{childList:true,subtree:true});
  const dispose=()=>{
    if(disposed)return;disposed=true;
    if(pinned&&pref.selection){
      pref.selection.focused=surface.contains(document.activeElement);
      pref.selection.reading=readingState();
    }
    abort.abort();observer.disconnect();cancelAnimationFrame(positionFrame);surface.remove();
  };
  return {show,reconcile,place:queuePlace,dispose};
}
function bindWorkflowControls(taskID,project,root){
  const pref=activityDetailPrefs(taskID,project),ko=state.language==='ko',payload=activityDetailCache.get(activityDetailKey(taskID,project)),wf=payload?.workflow;
  if(!wf)return;
  activityDetailResizeObservers.get(root)?.disconnect();
  workflowActiveFloating?.dispose();
  workflowFloatingViews.get(root)?.dispose();
  const viewport=root.querySelector('#workflowTimelineViewport'),canvas=root.querySelector('.workflow-timeline-canvas');
  const floating=viewport?workflowFloatingView(root,viewport,wf,pref,ko):null;if(floating){workflowFloatingViews.set(root,floating);workflowActiveFloating=floating;}
  const textMeasure=document.createElement('canvas').getContext('2d'),textWidths=new Map();
  let labelFont='',tickFont='',plotted=null,intervalNodes=null,tracks=null;
  const textWidth=(text,font)=>{const key=font+'|'+text;if(!textWidths.has(key)){textMeasure.font=font;textWidths.set(key,textMeasure.measureText(text).width);}return textWidths.get(key);};
  const fitLabels=()=>{
    if(!canvas)return;
    const plot=canvas.querySelector('.workflow-axis-track'),width=plot?.getBoundingClientRect().width||0;
    if(!intervalNodes)intervalNodes=[...canvas.querySelectorAll('[data-workflow-interval]')].map(button=>({button,label:button.querySelector('.activity-timeline-duration-label'),start:Number(button.dataset.workflowRelativeStart),end:Number(button.dataset.workflowRelativeEnd)}));
    const labels=intervalNodes.map(item=>item.label),ticks=[...root.querySelectorAll('.activity-time-ruler span')],view=workflowViewWindow(wf,pref),span=view.end-view.start;
    if(!labelFont&&labels[0])labelFont=getComputedStyle(labels[0]).font;
    if(!tickFont&&ticks[0])tickFont=getComputedStyle(ticks[0]).font;
    // Time geometry and cached text widths avoid a forced layout read for every
    // interval on every pan frame. Source observations are never rescanned.
    // Transparent hit regions stop at neighboring centers so tiny bars cannot
    // swallow another activity's painted center, even in a dense Fit view.
    const hitGroups=new Map();
    intervalNodes.forEach(item=>{const track=item.button.closest('.workflow-agent-track'),key=item.button.style.top;if(!hitGroups.has(track))hitGroups.set(track,new Map());const groups=hitGroups.get(track);if(!groups.has(key))groups.set(key,[]);groups.get(key).push(item);});
    hitGroups.forEach(groups=>groups.forEach(items=>{items.sort((a,b)=>(a.start+a.end)-(b.start+b.end));items.forEach((item,index)=>{const pixels=Math.max(0,item.end-item.start)/span*width,center=(item.start+item.end)/2,previous=items[index-1],next=items[index+1],left=Math.min(12,previous?(center-(previous.start+previous.end)/2)/span*width/2:12),right=Math.min(12,next?((next.start+next.end)/2-center)/span*width/2:12);item.button.style.setProperty('--workflow-hit-left',(pixels/2-Math.max(pixels/2,left))+'px');item.button.style.setProperty('--workflow-hit-width',(Math.max(pixels/2,left)+Math.max(pixels/2,right))+'px');});}));
    intervalNodes.forEach(({button,label,start,end})=>{const left=view.start-Date.parse(wf.axis.start),right=view.end-Date.parse(wf.axis.start),visible=end>=left&&start<=right,pixels=Math.max(0,Math.min(end,right)-Math.max(start,left))/span*width,visibility=parseFloat(button.style.height)>=14&&pixels>=textWidth(label.textContent,labelFont)+14?'visible':'hidden';if(label.style.visibility!==visibility)label.style.visibility=visibility;const tab=visible?0:-1;if(button.tabIndex!==tab)button.tabIndex=tab;if(visible)button.removeAttribute('aria-hidden');else button.setAttribute('aria-hidden','true');});
    let last=-Infinity;const ending=ticks.at(-1),endingLeft=width-(ending?textWidth(ending.textContent,tickFont):0);
    ticks.forEach((tick,i)=>{const p=Number(tick.dataset.workflowTick),w=textWidth(tick.textContent,tickFont),left=p*width-(p===0?0:p===1?w:w/2),right=left+w,visible=!i||i===ticks.length-1||(left>=last+8&&right<=endingLeft-8);tick.style.visibility=visible?'visible':'hidden';if(visible)last=right;});
  };
  const updateEdges=()=>{const edges=workflowEdgeState(wf,workflowViewWindow(wf,pref));for(const side of ['left','right']){const button=root.querySelector('[data-workflow-pan="'+side+'"]');if(button&&!edges[side]&&button===document.activeElement)viewport?.focus({preventScroll:true});if(button)button.hidden=!edges[side];const fade=root.querySelector('[data-workflow-fade="'+side+'"]');if(fade)fade.hidden=!edges[side];}};
  const displaySelection=()=>floating?.reconcile();
  const paint=()=>{
    if(!canvas||!canvas.isConnected)return;
    const focused=document.activeElement?.getAttribute?.('data-workflow-interval'),focusedRow=document.activeElement?.getAttribute?.('data-workflow-row-select'),focusedExpand=document.activeElement?.getAttribute?.('data-workflow-row-expand');
    const view=workflowViewWindow(wf,pref),span=view.end-view.start;
    if(!plotted||plotted.span!==span||plotted.round!==pref.round||view.start<plotted.draw.start||view.end>plotted.draw.end){
      const draw={start:Math.max(Date.parse(wf.axis.start),view.start-span),end:Math.min(Date.parse(wf.axis.end),view.end+span)};
      canvas.innerHTML=workflowPlotHTML(wf,pref,ko,draw);plotted={start:view.start,span,draw,round:pref.round};intervalNodes=null;tracks=[...canvas.querySelectorAll('.workflow-agent-track,.workflow-coverage-track')];
    }else{
      const transform='translateX('+((plotted.start-view.start)/span*100)+'%)';tracks.forEach(track=>track.style.transform=transform);
      canvas.querySelectorAll('[data-workflow-tick]').forEach(tick=>tick.textContent=workflowTimeLabel(view.start+span*Number(tick.dataset.workflowTick),view,ko));
    }
    canvas.dataset.viewStart=String(view.start);canvas.dataset.viewEnd=String(view.end);
    const output=root.querySelector('#workflowVisibleRange');if(output)output.textContent=workflowVisibleRange(view,ko,wf.axis);
    const full=Date.parse(wf.axis.end)-Date.parse(wf.axis.start),isFit=view.end-view.start>=full;
    root.querySelector('#workflowZoomOut').disabled=isFit;root.querySelector('#workflowZoomIn').disabled=view.end-view.start<=1;
    displaySelection();fitLabels();updateEdges();
    if(focusedRow||focusedExpand){const attr=focusedRow?'data-workflow-row-select':'data-workflow-row-expand',value=focusedRow||focusedExpand;[...canvas.querySelectorAll('['+attr+']')].find(button=>button.getAttribute(attr)===value)?.focus({preventScroll:true});}
    if(focused){const target=[...canvas.querySelectorAll('[data-workflow-interval]')].find(x=>x.dataset.workflowInterval===focused&&x.tabIndex===0);(target||viewport)?.focus({preventScroll:true});}
  };
  const select=(target,keyboard=false)=>floating?.show(target.closest?.('[data-workflow-interval],[data-workflow-row-select]'),true,keyboard);
  let frame=0;const queuePaint=()=>{if(!frame)frame=requestAnimationFrame(()=>{frame=0;paint();});};
  const zoom=(factor,anchor,ratio)=>{workflowZoomWindow(wf,pref,factor,anchor,ratio??.5);paint();};
  const fit=()=>{pref.window=null;workflowViewWindow(wf,pref);paint();};
  root.querySelector('#workflowZoomIn')?.addEventListener('click',()=>zoom(.5));root.querySelector('#workflowZoomOut')?.addEventListener('click',()=>zoom(2));root.querySelector('#workflowFit')?.addEventListener('click',fit);
  root.querySelectorAll('[data-workflow-pan]').forEach(button=>button.addEventListener('click',()=>{const v=workflowViewWindow(wf,pref),delta=(v.end-v.start)*.65*(button.dataset.workflowPan==='left'?-1:1);workflowSetWindow(wf,pref,v.start+delta,v.end+delta);paint();}));
  root.querySelector('#activityDetailRound')?.addEventListener('change',e=>{pref.round=e.target.value;paint();});
  root.querySelector('#activityDetailRefresh')?.addEventListener('click',()=>void loadTaskActivity(taskID,project));
  root.querySelector('#activityLedgerDetail')?.addEventListener('toggle',e=>{pref.ledger=e.target.open;});
  if(!viewport||!canvas){root.querySelectorAll('#workflowZoomIn,#workflowZoomOut,#workflowFit').forEach(button=>button.disabled=true);return;}
  let drag=null,suppressClick=0,touchGesture=null,touchSuppressWheelUntil=0;
  const touchContacts=new Map();
  const plotRect=()=>canvas.querySelector('.workflow-axis-track').getBoundingClientRect();
  const inPlot=(x,rect)=>x>=rect.left&&x<=rect.right;
  viewport.addEventListener('click',e=>{if(e.detail>0&&performance.now()<suppressClick){e.preventDefault();return;}suppressClick=0;const expand=e.target.closest?.('[data-workflow-row-expand]');if(expand){pref.expandedRows??={};pref.expandedRows[expand.dataset.workflowRowExpand]=!pref.expandedRows[expand.dataset.workflowRowExpand];plotted=null;paint();return;}select(e.target,e.detail===0);});
  root.addEventListener('keydown',e=>{
    if(e.target!==viewport&&!e.target.closest?.('[data-workflow-interval],[data-workflow-row-select],[data-workflow-row-expand],[data-workflow-pan]'))return;if(e.ctrlKey||e.metaKey||e.altKey)return;
    if(!['+','=','-','Home','0','ArrowLeft','ArrowRight'].includes(e.key))return;
    e.preventDefault();e.stopPropagation();
    const v=workflowViewWindow(wf,pref),span=v.end-v.start;
    if(e.key==='+'||e.key==='='){e.preventDefault();zoom(.5);}
    else if(e.key==='-'){e.preventDefault();zoom(2);}
    else if(e.key==='Home'||e.key==='0'){e.preventDefault();fit();}
    else if(e.key==='ArrowLeft'||e.key==='ArrowRight'){e.preventDefault();const delta=span*(e.key==='ArrowLeft'?-.2:.2);workflowSetWindow(wf,pref,v.start+delta,v.end+delta);paint();}
  });
  viewport.addEventListener('dblclick',e=>{if(e.ctrlKey||e.metaKey||e.altKey)return;const r=plotRect();if(!inPlot(e.clientX,r))return;if(performance.now()<touchSuppressWheelUntil){e.preventDefault();return;}const v=workflowViewWindow(wf,pref),ratio=(e.clientX-r.left)/r.width;e.preventDefault();zoom(e.shiftKey?2:.5,v.start+(v.end-v.start)*ratio,ratio);});
  viewport.addEventListener('wheel',e=>{
    if(e.metaKey||e.altKey)return;
    const r=plotRect();if(!inPlot(e.clientX,r))return;
    if(e.ctrlKey){
      e.preventDefault();e.stopPropagation();
      if(touchContacts.size>1||performance.now()<touchSuppressWheelUntil)return;
      const v=workflowViewWindow(wf,pref),ratio=(e.clientX-r.left)/r.width,scale=e.deltaMode===1?16:e.deltaMode===2?r.width:1;
      workflowZoomWindow(wf,pref,Math.exp(Math.max(-4,Math.min(4,(e.deltaY||e.deltaX)*scale*.002))),v.start+(v.end-v.start)*ratio,ratio);queuePaint();return;
    }
    const delta=Math.abs(e.deltaX)>Math.abs(e.deltaY)?e.deltaX:e.shiftKey?e.deltaY:0;if(!delta)return;
    const v=workflowViewWindow(wf,pref),scale=e.deltaMode===1?16:e.deltaMode===2?r.width:1,offset=delta*scale/r.width*(v.end-v.start);
    const next=workflowSetWindow(wf,pref,v.start+offset,v.end+offset);if(next.start!==v.start){e.preventDefault();queuePaint();}
  },{passive:false});
  const releaseTouchCapture=id=>{if(viewport.hasPointerCapture(id))viewport.releasePointerCapture(id);};
  const rebaseTouch=()=>{
    const contacts=[...touchContacts.values()],view=workflowViewWindow(wf,pref),rect=plotRect();
    if(contacts.length>1){
      const pair=touchGesture?.mode==='pinch'&&touchGesture.ids.every(id=>touchContacts.has(id))?touchGesture.ids.map(id=>touchContacts.get(id)):contacts.slice(0,2);
      const midpoint=(pair[0].x+pair[1].x)/2,ratio=Math.max(0,Math.min(1,(midpoint-rect.left)/rect.width));
      touchGesture={mode:'pinch',ids:pair.map(c=>c.id),distance:Math.max(4,Math.hypot(pair[1].x-pair[0].x,pair[1].y-pair[0].y)),anchor:view.start+(view.end-view.start)*ratio,span:view.end-view.start,rect};
      touchSuppressWheelUntil=performance.now()+350;suppressClick=performance.now()+350;drag=null;
    }else if(contacts.length){const c=contacts[0];touchGesture={mode:'single',id:c.id,x:c.x,y:c.y,start:view.start,end:view.end,rect,active:false,vertical:false};}
    else touchGesture=null;
  };
  const cancelTouches=()=>{const ids=[...touchContacts.keys()],multi=touchContacts.size>1||touchGesture?.mode==='pinch';touchContacts.clear();touchGesture=null;if(multi)touchSuppressWheelUntil=performance.now()+350;suppressClick=performance.now()+350;ids.forEach(releaseTouchCapture);};
  // Pointer capture belongs to this stable viewport, not the interval node that
  // a zoom paint replaces. TouchEvent is used only to suppress native two-touch
  // scrolling/zoom; all coordinates are processed once by captured pointers.
  viewport.addEventListener('touchstart',e=>{if(touchContacts.size>1&&e.cancelable)e.preventDefault();},{passive:false});
  viewport.addEventListener('pointerdown',e=>{
    if(e.pointerType==='touch'){
      const rect=plotRect();if(!inPlot(e.clientX,rect))return;
      touchContacts.set(e.pointerId,{id:e.pointerId,x:e.clientX,y:e.clientY});viewport.setPointerCapture(e.pointerId);
      if(touchContacts.size<=2)rebaseTouch();return;
    }
    if(e.button!==0||e.ctrlKey||e.metaKey||e.altKey)return;const rect=plotRect();if(!inPlot(e.clientX,rect))return;
    const view=workflowViewWindow(wf,pref);drag={id:e.pointerId,x:e.clientX,y:e.clientY,rect,start:view.start,end:view.end,range:e.shiftKey,active:false};
  });
  viewport.addEventListener('pointermove',e=>{
    if(e.pointerType==='touch'){
      if(!touchContacts.has(e.pointerId))return;if(!viewport.isConnected){cancelTouches();return;}
      touchContacts.set(e.pointerId,{id:e.pointerId,x:e.clientX,y:e.clientY});
      const g=touchGesture;if(!g)return;
      if(g.mode==='pinch'){
        if(!g.ids.includes(e.pointerId))return;
        const pair=g.ids.map(id=>touchContacts.get(id));if(pair.some(c=>!c)){rebaseTouch();return;}
        const distance=Math.max(4,Math.hypot(pair[1].x-pair[0].x,pair[1].y-pair[0].y)),midpoint=(pair[0].x+pair[1].x)/2,ratio=Math.max(0,Math.min(1,(midpoint-g.rect.left)/g.rect.width)),span=g.span*g.distance/distance;
        e.preventDefault();workflowSetWindow(wf,pref,g.anchor-span*ratio,g.anchor+span*(1-ratio));touchSuppressWheelUntil=performance.now()+350;suppressClick=performance.now()+350;queuePaint();return;
      }
      if(g.id!==e.pointerId||g.vertical)return;
      const dx=e.clientX-g.x,dy=e.clientY-g.y;if(!g.active){if(Math.max(Math.abs(dx),Math.abs(dy))<6)return;if(Math.abs(dy)>Math.abs(dx)){g.vertical=true;return;}g.active=true;}
      e.preventDefault();const offset=-dx/g.rect.width*(g.end-g.start);workflowSetWindow(wf,pref,g.start+offset,g.end+offset);suppressClick=performance.now()+350;queuePaint();return;
    }
    if(!drag||drag.id!==e.pointerId)return;const dx=e.clientX-drag.x,dy=e.clientY-drag.y;
    if(!drag.active){if(Math.abs(dx)<6)return;if(e.pointerType==='touch'&&Math.abs(dy)>Math.abs(dx)){drag=null;return;}drag.active=true;viewport.setPointerCapture(e.pointerId);}
    e.preventDefault();const span=drag.end-drag.start;
    if(drag.range){const left=Math.max(0,Math.min(drag.rect.width,Math.min(drag.x,e.clientX)-drag.rect.left)),right=Math.max(0,Math.min(drag.rect.width,Math.max(drag.x,e.clientX)-drag.rect.left));viewport.style.setProperty('--workflow-range-left',left+'px');viewport.style.setProperty('--workflow-range-width',(right-left)+'px');viewport.classList.add('is-selecting');}
    else{const offset=-dx/drag.rect.width*span;workflowSetWindow(wf,pref,drag.start+offset,drag.end+offset);queuePaint();}
  },{passive:false});
  const endDrag=e=>{
    if(e.pointerType==='touch'){
      if(!touchContacts.has(e.pointerId))return;if(!viewport.isConnected){cancelTouches();return;}
      if(e.type==='pointercancel'){cancelTouches();return;}
      const wasPinch=touchGesture?.mode==='pinch',active=touchGesture?.active;touchContacts.delete(e.pointerId);releaseTouchCapture(e.pointerId);
      if(wasPinch||active){suppressClick=performance.now()+350;if(wasPinch)touchSuppressWheelUntil=performance.now()+350;}
      if(wasPinch&&touchContacts.size>1&&touchGesture.ids.every(id=>touchContacts.has(id)))return;
      rebaseTouch();return;
    }
    if(!drag||drag.id!==e.pointerId)return;const d=drag;drag=null;viewport.classList.remove('is-selecting');
    if(viewport.hasPointerCapture(e.pointerId))viewport.releasePointerCapture(e.pointerId);
    if(d.active){suppressClick=performance.now()+100;if(d.range&&e.type!=='pointercancel'){const a=Math.max(0,Math.min(1,(d.x-d.rect.left)/d.rect.width)),b=Math.max(0,Math.min(1,(e.clientX-d.rect.left)/d.rect.width));workflowSetWindow(wf,pref,d.start+Math.min(a,b)*(d.end-d.start),d.start+Math.max(a,b)*(d.end-d.start));paint();}}
  };
  viewport.addEventListener('pointerup',endDrag);viewport.addEventListener('pointercancel',endDrag);
  viewport.addEventListener('lostpointercapture',e=>{if(touchContacts.has(e.pointerId))cancelTouches();else if(drag?.id===e.pointerId){drag=null;viewport.classList.remove('is-selecting');}});
  if(typeof ResizeObserver!=='undefined'){const observer=new ResizeObserver(()=>{fitLabels();floating?.place();if(touchContacts.size)rebaseTouch();});observer.observe(viewport);activityDetailResizeObservers.set(root,observer);}
  displaySelection();fitLabels();updateEdges();
  root.querySelector('#workflowZoomOut').disabled=workflowViewWindow(wf,pref).end-workflowViewWindow(wf,pref).start>=Date.parse(wf.axis.end)-Date.parse(wf.axis.start);
}

function workflowMetricsBody(payload,taskID){
  const ko=state.language==='ko',wf=payload.workflow,current=wf.current||{},measurement=wf.measurement||{},axis=wf.axis||{},prefs=activityDetailPrefs(taskID),rows=Array.isArray(wf.rows)?wf.rows:[],rounds=Array.isArray(wf.rounds)?wf.rounds:[];
  const view=workflowViewWindow(wf,prefs),edges=workflowEdgeState(wf,view),validAxis=Boolean(view),completed=current.phase==='completed';
  const roundValues=[...new Set(rows.filter(r=>r.round!=null).map(r=>Number(r.round)))].sort((a,b)=>a-b),selected=String(prefs.round);
  const actor=current.processing_agent?workflowActorLabel(rows.find(r=>r.agent_id===current.processing_agent&&r.role===current.processing_role)||{role:current.processing_role},ko):workflowText(ko,completed?'처리 없음 · 완료':'처리자 미확인',completed?'No active processor · completed':'Processing actor unverified');
  const completionHistory=Array.isArray(wf.lifecycle?.completion_history)?wf.lifecycle.completion_history:[],reopened=Boolean(axis.open&&completionHistory.length);
  const metricRows=[['전체 경과','Total elapsed',Number.isFinite(axis.elapsed_ms)?activityDisplayDuration(axis.elapsed_ms,ko):workflowText(ko,'경계 미확인','Bounds unverified')],['Worker 활동','Worker activity',workflowMetric(measurement.worker,ko)],['Controller 검토','Controller review',workflowMetric(measurement.review,ko)],['활동 미측정','Unmeasured activity',Number.isFinite(measurement.unmeasured_ms)?activityDisplayDuration(measurement.unmeasured_ms,ko):workflowText(ko,'범위 미확인','Range unverified')]];
  const status=measurement.status||'not_recorded',coverage=Number.isFinite(measurement.coverage_percent)?Math.max(0,Math.min(100,measurement.coverage_percent)):null;
  const snapshot=wf.control_snapshot||{},trust=snapshot.trust||'pending',sourceIssue=payload._client_error||snapshot.handoff_error||snapshot.error||'',stale=['stale','error','pending'].includes(trust),checked=snapshot.validated_at||snapshot.handoff_checked_at||wf.computed_at;
  const observation=workflowMeasurementLabel(status,ko)+(coverage==null?'':' '+coverage.toFixed(1)+'%');
  const elapsed=metricRows[0][2],unknown=metricRows[3][2];
  const summary='<div class="workflow-context"><div class="workflow-context-line"><strong>'+esc(workflowPhaseLabel(current.phase,ko))+'</strong>'+(!completed?'<span>'+esc(actor)+'</span>':'')+'<span>'+esc(workflowRoundLabel(current.round,ko))+'</span><span>'+esc(workflowText(ko,'전체 ','Total ')+elapsed)+'</span></div>'+(!completed?'<p class="workflow-next">'+esc(workflowText(ko,'다음: ','Next: ')+workflowActionLabel(current.next_action,ko))+'</p>':'')+'<p class="workflow-quality" data-measurement-status="'+esc(status)+'"><span>'+esc(observation)+'</span><span>'+esc(workflowText(ko,'활동 미측정 ','Unmeasured activity ')+unknown)+'</span></p></div>'+(reopened?'<p class="workflow-reopened">'+esc(workflowText(ko,'이전 완료 후 재개 · ','Resumed after completion · ')+activityEventTime(completionHistory.at(-1),ko))+'</p>':'')+(sourceIssue?'<p class="workflow-source-warning" role="status">'+esc(workflowText(ko,'갱신 실패 · 마지막 확인 결과를 표시합니다. 새로고침으로 재시도하세요.','Refresh failed · last verified result retained. Refresh to retry.'))+'</p>':stale?'<p class="workflow-source-note">'+esc(workflowText(ko,trust==='pending'?'원천 확인 전 · 확인 가능한 기록 기준':'원천이 오래됐습니다 · 마지막 확인 결과',trust==='pending'?'Source pending · available records':'Stale source · last verified result'))+'</p>':'');
  const controls='<div class="workflow-controls"><div class="workflow-zoom-buttons" role="group" aria-label="'+esc(workflowText(ko,'시간축 확대','Timeline zoom'))+'"><span class="workflow-zoom-label">'+esc(workflowText(ko,'시간축 확대','Timeline zoom'))+'</span><button id="workflowZoomOut" type="button" aria-label="'+esc(workflowText(ko,'시간축 축소','Zoom out'))+'" title="'+esc(workflowText(ko,'축소 (−)','Zoom out (−)'))+'">−</button><button id="workflowZoomIn" type="button" aria-label="'+esc(workflowText(ko,'시간축 확대','Zoom in'))+'" title="'+esc(workflowText(ko,'확대 (+)','Zoom in (+)'))+'">+</button><button id="workflowFit" type="button" title="'+esc(workflowText(ko,'전체 기간 보기 (Home)','Fit full period (Home)'))+'">'+esc(workflowText(ko,'전체 보기','Fit all'))+'</button></div><label class="workflow-round-filter"><span class="sr-only">'+esc(workflowText(ko,'업무 회차','Work round'))+'</span><select id="activityDetailRound"><option value="all"'+(selected==='all'?' selected':'')+'>'+esc(workflowText(ko,'모든 회차','All rounds'))+'</option>'+roundValues.map(n=>'<option value="'+n+'"'+(selected===String(n)?' selected':'')+'>'+esc(workflowRoundLabel(n,ko))+'</option>').join('')+(rows.some(r=>r.round==null)?'<option value="unknown"'+(selected==='unknown'?' selected':'')+'>'+esc(workflowRoundLabel(null,ko))+'</option>':'')+'</select></label><button type="button" id="activityDetailRefresh">'+esc(workflowText(ko,'새로고침','Refresh'))+'</button></div>';
  const timeline=validAxis?'<div id="workflowTimelineViewport" class="workflow-timeline-scroll" tabindex="0" aria-label="'+esc(workflowText(ko,'작업 시간축. 가로 드래그로 이동, Ctrl 휠이나 두 손가락으로 확대·축소, Shift 드래그로 구간 확대. 더블클릭 또는 +와 −로 확대 축소, 화살표로 이동, Home으로 전체 보기.','Workflow timeline. Drag to pan, Ctrl-wheel or pinch to zoom, Shift-drag to zoom a range. Double-click or +/− to zoom, arrows to pan, Home to fit.'))+'"><div class="workflow-timeline-canvas" data-axis-start="'+esc(axis.start)+'" data-axis-end="'+esc(axis.end)+'" data-view-start="'+view.start+'" data-view-end="'+view.end+'">'+workflowPlotHTML(wf,prefs,ko)+'</div><div class="workflow-edge-fade workflow-edge-left" data-workflow-fade="left" aria-hidden="true"'+(!edges.left?' hidden':'')+'></div><div class="workflow-edge-fade workflow-edge-right" data-workflow-fade="right" aria-hidden="true"'+(!edges.right?' hidden':'')+'></div></div><div class="workflow-chart-footer"><div class="workflow-pan-row">'+workflowPanButton('left',ko,!edges.left)+'<span class="workflow-navigation-help"><span class="workflow-pointer-help">'+esc(workflowText(ko,'드래그로 이동 · Ctrl+휠로 확대 · Shift+드래그로 구간 확대','Drag to pan · Shift-drag to zoom a range'))+'</span><span class="workflow-touch-help">'+esc(workflowText(ko,'가로로 밀어 이동 · 두 손가락으로 확대·축소','Swipe pan · pinch zoom'))+'</span></span>'+workflowPanButton('right',ko,!edges.right)+'</div><output id="workflowVisibleRange" class="workflow-visible-range" aria-label="'+esc(workflowText(ko,'보이는 시간 범위','Visible time range'))+'">'+esc(view?workflowVisibleRange(view,ko,axis):'—')+'</output></div>':'<p class="workflow-empty">'+esc(workflowText(ko,'시작·종료 경계를 확인할 수 없어 시간축을 그리지 않았습니다.','Time axis unavailable without verified bounds.'))+'</p>';
  const roundDetail='<p class="workflow-round-explanation">'+esc(workflowText(ko,'최초 작업과 보완차수는 같은 업무의 검토·수정 과정입니다. 재개나 새 업무를 뜻하지 않습니다.','Initial work and correction rounds describe review and corrections within this task, not resumed sessions or new scope.'))+'</p><details class="workflow-disclosure" id="workflowRoundDetails"><summary>'+esc(workflowText(ko,'회차와 검토·수정 관계','Rounds, reviews and corrections'))+' <span>'+rounds.length+'</span></summary><ol class="workflow-round-list">'+rounds.map(r=>'<li><strong>'+esc(workflowRoundLabel(r.number,ko))+'</strong><span>'+esc(activityEventTime(r.started_at,ko))+'</span>'+(r.reason?'<p>'+esc(r.reason)+'</p>':'<p>'+esc(workflowText(ko,'등록 또는 실제 활동 보고에 연결된 최초 작업입니다.','Initial work linked to registration or a real activity report.'))+'</p>')+(r.verification_results?.length?'<small>'+esc(workflowText(ko,'검증 결과: ','Verification: ')+r.verification_results.map(v=>workflowOutcome(v,ko)||v).join(' · '))+'</small>':'')+'<details><summary>'+esc(workflowText(ko,'연결 근거','Linked evidence'))+'</summary><pre>'+esc(JSON.stringify({basis:r.basis,previous_round:r.previous_round,handoff:r.handoff_id,assignment_ids:r.assignment_ids,attempt_ids:r.attempt_ids,handoff_ids:r.handoff_ids,commits:r.commits},null,2))+'</pre></details></li>').join('')+'</ol></details>';
  const totals=Object.entries(payload.summary?.by_type_ms||{}).filter(([,ms])=>Number.isFinite(ms)&&ms>=0).sort((a,b)=>b[1]-a[1]),max=Math.max(1,...totals.map(([,ms])=>ms));
  const events=Array.isArray(payload.events)?payload.events:[];
  const evidence='<details class="workflow-disclosure" id="workflowEvidenceDetails"><summary>'+esc(workflowText(ko,'측정 집계와 원천 근거','Measurement totals and sources'))+'</summary>'+'<details id="workflowExecutionEvidence"><summary>'+esc(workflowText(ko,'전체 시간축·실행 보고·미측정 경계','Full axis, execution reports and unmeasured bounds'))+'</summary><pre>'+esc(JSON.stringify({axis:axis,lifecycle:wf.lifecycle,current:wf.current,unmeasured_intervals:wf.unmeasured_intervals,rows:rows},null,2))+'</pre></details>'+'<dl class="workflow-source-facts"><div><dt>'+esc(workflowText(ko,'현재 처리자','Processing actor'))+'</dt><dd>'+esc(actor)+'</dd></div><div><dt>'+esc(workflowText(ko,'담당 Worker','Assigned Worker'))+'</dt><dd>'+esc(workflowAgentLabel(current.assigned_worker,ko))+'</dd></div></dl><div class="workflow-measurements">'+metricRows.map(([kr,en,value])=>'<div><span>'+esc(workflowText(ko,kr,en))+'</span><strong>'+esc(value)+'</strong></div>').join('')+'</div><p>'+esc(workflowText(ko,'실행 보고와 활동 측정은 다릅니다. 미측정은 작업이 없었다는 뜻이 아닙니다.','Execution reports differ from activity measurement. Unmeasured does not mean no work.'))+'</p>'+'<dl class="workflow-source-facts"><div><dt>'+esc(workflowText(ko,'마지막 원천 확인','Source last checked'))+'</dt><dd>'+esc(activityEventTime(checked,ko))+'</dd></div><div><dt>'+esc(workflowText(ko,'API 조회','API read'))+'</dt><dd>'+esc(activityEventTime(payload._client_fetched_at,ko))+'</dd></div><div><dt>'+esc(workflowText(ko,'원천 상태','Source status'))+'</dt><dd>'+esc(({verified:workflowText(ko,'확인됨','Verified'),stale:workflowText(ko,'오래된 결과','Stale'),pending:workflowText(ko,'확인 전','Pending'),error:workflowText(ko,'갱신 오류','Refresh error')})[trust]||trust)+'</dd></div></dl>'+(sourceIssue?'<p class="workflow-source-warning">'+esc(sourceIssue)+'</p>':'')+'<h4>'+esc(workflowText(ko,'활동 유형별 누적 · 전체 회차','Cumulative activity · all rounds'))+'</h4><div class="activity-type-bars">'+totals.map(([type,ms])=>'<div class="activity-type-row"><span>'+esc(activityDisplayType(type,ko))+'</span><div class="activity-type-track"><div style="width:'+ms/max*100+'%"></div></div><strong>'+esc(activityDisplayDuration(ms,ko))+'</strong></div>').join('')+'</div><p>'+esc(workflowText(ko,'누적 Agent 시간은 겹칠 수 있으며 전체 경과와 합산하지 않습니다.','Agent intervals can overlap and are not added to end-to-end elapsed time.'))+'</p><details id="activityLedgerDetail"'+(prefs.ledger?' open':'')+'><summary>'+esc(workflowText(ko,'Activity 원장 상세','Activity ledger detail'))+' ('+events.length+')</summary><div class="activity-ledger-scroll"><table class="activity-ledger-table"><thead><tr>'+['시각','역할 · 활동','회차','근거'].map((v,i)=>'<th scope="col">'+esc(ko?v:['Time','Role · activity','Round','Evidence'][i])+'</th>').join('')+'</tr></thead><tbody>'+events.slice(0,ACTIVITY_DETAIL_MAX_LEDGER).map(e=>'<tr><td>'+esc(activityEventTime(e.occurred_at,ko))+'</td><td>'+esc(activityDisplayRole(e.role,ko)+' · '+activityDisplayType(e.type,ko))+'</td><td>'+esc(workflowText(ko,'원장 값 ','Recorded ')+String(e.round??'—'))+'</td><td>'+esc(e.evidence||e.explanation||'—')+'<small>'+esc([e.event_id,e.agent_id,e.runtime_attempt_id,e.assignment_id,e.handoff_id].filter(Boolean).join(' · '))+'</small></td></tr>').join('')+'</tbody></table></div></details></details>';
  return '<div class="workflow-view">'+summary+controls+timeline+roundDetail+evidence+'</div>';
}

function activityMetricsBody(payload,taskID=state.detail){
  if(payload?.workflow?.version===1)return workflowMetricsBody(payload,taskID);
  if(payload?.error){const ko=state.language==='ko';return '<div class="workflow-load-error"><p role="status">'+esc(workflowText(ko,'작업 흐름 기록을 불러오지 못했습니다. 본문은 계속 볼 수 있습니다.','Workflow records could not be loaded. The task document is still available.'))+'</p><button type="button" id="activityDetailRefresh">'+esc(workflowText(ko,'기록 다시 불러오기','Retry record'))+'</button><details><summary>'+esc(workflowText(ko,'오류 상세','Error details'))+'</summary><p>'+esc(payload.error)+'</p></details></div>';}
  if(!payload)return '<p class="summary">'+esc(state.language==='ko'?'활동 기록을 불러오는 중…':'Loading activity history…')+'</p>';
  if(payload.error)return '<p class="summary">'+esc(payload.error)+'</p>';
  const ko=state.language==='ko',s=payload.summary||{},segments=Array.isArray(s.segments)?s.segments:[],
    lc=payload.lifecycle_coverage||{},waitRows=Array.isArray(lc.wait_intervals)?lc.wait_intervals:[],
    events=Array.isArray(payload.events)?payload.events:[];
  const prefs=activityDetailPrefs(taskID);
  const lastRead=payload._client_fetched_at?
    '<small class="activity-last-read" title="'+esc(ko?'마지막 API 요청 시각입니다. 원천 데이터의 최종 변경 시각을 의미하지 않습니다.':'Time of latest API request, not last upstream modification.')+'">'+
      esc((ko?'마지막 조회: ':'Last fetched: ')+activityEventTime(payload._client_fetched_at,ko))+'</small>':'';
  const rounds=[...new Set(segments.map(x=>Number(x.round)||0))].sort((a,b)=>a-b);
  const selectedRound=prefs.round==='all'?'all':Number(prefs.round);
  const activeRound=selectedRound==='all'||rounds.includes(selectedRound)?selectedRound:'all';
  const filtered=activeRound==='all'?segments:segments.filter(x=>(Number(x.round)||0)===activeRound);
  const settings='<div class="activity-detail-controls" role="group" aria-label="'+esc(ko?'작업시간 차트 설정':'Activity chart controls')+'">'+
    '<label>'+esc(ko?'작업 회차':'Work round')+
      '<select id="activityDetailRound"><option value="all"'+(activeRound==='all'?' selected':'')+'>'+esc(ko?'전체':'All rounds')+'</option>'+
      rounds.map(n=>'<option value="'+n+'"'+(activeRound===n?' selected':'')+'>'+esc(n===0?(ko?'최초 작업':'Initial work'):(ko?'재작업 ':'Rework ')+n)+'</option>').join('')+'</select></label>'+
    '<label>'+esc(ko?'차트':'Chart')+
      '<select id="activityDetailMode"><option value="timeline"'+(prefs.mode==='timeline'?' selected':'')+'>'+esc(ko?'시간축 Gantt':'Timeline Gantt')+'</option>'+
      '<option value="rounds"'+(prefs.mode==='rounds'?' selected':'')+'>'+esc(ko?'회차별 스택':'Stacked by round')+'</option></select></label>'+
    '<label>'+esc(ko?'확대':'Zoom')+' <output id="activityDetailZoomValue">'+prefs.zoom+'×</output>'+
      '<input type="range" id="activityDetailZoom" min="1" max="8" step="1" value="'+prefs.zoom+'"'+
      (prefs.mode==='rounds'?' disabled':'')+' aria-label="'+esc(ko?'시간축 확대':'Timeline zoom')+'"/></label>'+
    '<label>'+esc(ko?'빠른 이동':'Quick focus')+
      '<select id="activityDetailFocus"'+(prefs.mode==='rounds'?' disabled':'')+'>'+
      [['manual',ko?'직접 탐색':'Manual'],['last50',ko?'최근 절반':'Recent half'],['last25',ko?'최근 ¼':'Recent quarter']].
      map(([v,label])=>'<option value="'+v+'"'+((prefs.focus||'manual')===v?' selected':'')+'>'+esc(label)+'</option>').join('')+
      '</select></label>'+
    '<label>'+esc(ko?'Agent 그룹':'Group by agent')+
      '<input type="checkbox" id="activityDetailGroupAgents"'+(prefs.groupAgents?' checked':'')+
      (prefs.mode==='rounds'?' disabled':'')+'/></label>'+
    '<button type="button" id="activityDetailRefresh">'+esc(ko?'기록 새로고침':'Refresh evidence')+'</button>'+lastRead+
    '<button type="button" id="activityRuntimeDiagnose">'+esc(ko?'실행 기록 대조':'Compare runtime evidence')+'</button></div>';
  if(!segments.length&&!s.open_activities&&!lc.registered_at&&!waitRows.length&&!events.length)
    return settings+'<p class="summary">'+esc(ko?'아직 관측된 Activity·Lifecycle 시간 근거가 없습니다.':'No observed Activity or lifecycle timing evidence yet.')+'</p>';
  const metrics=[
    [ko?'누적 Agent 시간':'Cumulative agent time',s.agent_time_ms],
    [ko?'관측 경과시간':'Observed wall time',s.observed_wall_ms],
    [ko?'최초 Worker 작업':'Initial worker work',s.initial_worker_ms],
    [ko?'재작업 합계':'Rework total',s.rework_worker_ms],
    [ko?'최초 Controller 검토':'Initial controller review',s.controller_first_review_ms],
    [ko?'재검토 합계':'Rereview total',s.controller_rereview_ms],
  ];
  const workerObserved=segments.some(x=>String(x.role).toLowerCase()==='worker');
  const boxes='<div class="activity-metric-grid">'+metrics.map(([label,ms],idx)=>{
    const missingWorker=(idx===2||idx===3)&&!workerObserved&&!s.open_activities;
    const display=missingWorker?(ko?'기록 없음':'Not observed'):(ms==null?'—':activityDisplayDuration(ms,ko));
    return '<div class="activity-metric"><strong>'+esc(display)+'</strong><span>'+esc(label)+'</span></div>';
  }).join('')+'</div>';
  const agentIdentities=new Set(events.filter(e=>e.agent_id&&e.role).map(e=>String(e.role)+'|'+String(e.agent_id)));
  const agentInferred=events.filter(e=>e.confidence==='inferred').length;
  const workerUnknown=!workerObserved;
  const observationQuality='<details class="activity-quality"'+(workerUnknown?' open':'')+'><summary>'+
    esc(ko?'활동 기록의 관측 품질':'Activity observation quality')+'</summary>'+
    '<p class="summary">'+esc(ko?
      '원장 이벤트 '+events.length+'건 · 시간 구간 '+segments.length+'개 · 기록된 역할·Agent 조합 '+agentIdentities.size+'개 · Agent 추정 근거 '+agentInferred+'건':
      'Ledger events '+events.length+' · timed intervals '+segments.length+' · recorded role/agent pairs '+agentIdentities.size+' · inferred events '+agentInferred)+
    '</p>'+(!workerObserved?'<p class="timing-note warning">'+esc(ko?
      'Worker의 종료까지 확인된 작업시간 구간이 없습니다. 실제 미착수인지, 기록·신원 연결이 누락된 것인지는 현재 원장만으로 판단할 수 없습니다.':
      'No closed Worker work interval was observed. This ledger alone cannot distinguish no execution from missing evidence or identity linkage.')+'</p>':'')+
    '<p class="summary">'+esc(ko?
      '이 수치는 Activity 원장에 기록된 범위만 나타냅니다. 전체 Runtime 실행 목록이나 Hook 지원 상태의 완전성을 증명하지 않습니다.':
      'These counts cover the Activity ledger only; they do not attest full runtime inventory or Hook availability.')+'</p></details>';
  const runtimeEvidence=payload.runtime_diagnostic;
  const runtimeStatus=(code)=>{
    const labels={activity_recorded:[ko?'Activity 기록 연결':'Activity matched'],
      no_matching_activity:[ko?'Activity 기록 없음':'No Activity record'],
      identity_unverified:[ko?'실행 신원 미확정':'Identity unverified'],
      identity_mismatch:[ko?'실행 신원 불일치':'Identity mismatch']};
    return labels[code]?labels[code][0]:String(code||'—');
  };
  const runtimeView=runtimeEvidence?'<details class="activity-runtime-comparison" open><summary>'+
    esc(ko?'Runtime·Activity 실행 근거 대조':'Runtime and Activity evidence comparison')+'</summary>'+
    '<p class="summary">'+esc(ko?
      'Task 연결 실행 '+(runtimeEvidence.total_attempts??runtimeEvidence.attempts.length)+'건 · Activity 연결 '+runtimeEvidence.matched_attempts+'건 · 기록 없음 '+runtimeEvidence.without_activity+'건 · 신원 미확정 '+runtimeEvidence.identity_unknown+'건 · 불일치 '+runtimeEvidence.identity_mismatch+'건':
      'Task-bound attempts '+(runtimeEvidence.total_attempts??runtimeEvidence.attempts.length)+' · matched '+runtimeEvidence.matched_attempts+' · without Activity '+runtimeEvidence.without_activity+' · unverified '+runtimeEvidence.identity_unknown+' · mismatched '+runtimeEvidence.identity_mismatch)+'</p>'+
    '<p class="summary">'+esc(ko?
      '진단을 요청한 시점의 해당 태스크에 연결된 실행만 비교했습니다. 태스크 연결이 없는 실행은 임의로 포함하지 않습니다. Activity 기록이 없다는 사실은 작업이 없었다는 증거가 아닙니다.':
      'Only task-bound attempts at the time of the explicit check are compared. Missing Activity records do not prove no execution.')+'</p>'+
    '<p class="summary">'+esc(ko?
      'Hook 설정은 수집 가능성을 뜻하며 실제 수신 성공을 보증하지 않습니다. 아래 Hook 시각은 최근 전이 기록에 한정됩니다.':
      'Hook setup indicates collection potential, not proof of delivery. Hook times cover recent transitions only.')+'</p>'+
    (Array.isArray(runtimeEvidence.hook_statuses)?
      '<p class="summary">'+runtimeEvidence.hook_statuses.map(h=>esc(h.provider+' · '+(h.status==='configured'?
        (ko?'설정 확인':'Configured'):h.status==='not_configured'?(ko?'설정 미확인':'Not configured'):(ko?'상태 확인 불가':'Unknown status')))).join(' / ')+'</p>':'')+
    '<div class="activity-ledger-scroll"><table class="activity-ledger-table"><thead><tr>'+
    [ko?'실행 ID':'Attempt',ko?'Agent / 역할':'Agent / role',ko?'상태':'Status',ko?'Activity 기록':'Activity records',ko?'최근 Hook 근거':'Recent Hook evidence'].map(x=>'<th scope="col">'+esc(x)+'</th>').join('')+
    '</tr></thead><tbody>'+runtimeEvidence.attempts.map(a=>'<tr><td>'+esc(a.attempt_id||'—')+'</td><td>'+esc((a.agent_id||'—')+' · '+(a.agent_type||'—'))+'</td>'+
    '<td>'+esc(runtimeStatus(a.diagnostic))+'</td><td>'+esc(String(a.matched_activity_events||0))+'</td>'+
    '<td>'+esc(a.recent_hook_transitions?
      ((ko?'최근 '+a.recent_hook_transitions+'건 · ':'Recent '+a.recent_hook_transitions+' · ')+activityEventTime(a.latest_recent_hook_at,ko)):
      (ko?'최근 근거 없음':'None in recent transitions'))+'</td></tr>').join('')+
    '</tbody></table></div>'+((runtimeEvidence.truncated)?'<p class="summary">'+esc(ko?'안전한 조회량 제한으로 일부 실행만 표시했습니다.':'Attempt list truncated for bounded diagnostic.')+'</p>':'')+'</details>':
    (payload.runtime_diagnostic_error?'<p class="timing-note warning">'+esc(ko?'실행 대조를 수행하지 못했습니다: ':'Runtime comparison unavailable: ')+esc(payload.runtime_diagnostic_error)+'</p>':'');
  const totals=Object.entries(s.by_type_ms||{}).filter(([,ms])=>ms>0).sort((a,b)=>b[1]-a[1]);
  const all=totals.reduce((sum,[,ms])=>sum+ms,0);
  const byType=totals.length?'<h3>'+esc(ko?'활동 유형별 누적 (전체 회차)':'Cumulative by activity (all rounds)')+'</h3><div class="activity-type-bars">'+
    totals.map(([name,ms])=>'<div class="activity-type-row"><span title="'+esc(name)+'">'+esc(activityDisplayType(name,ko))+'</span><div class="activity-type-track"><div style="width:'+Math.max(0,Math.min(100,all?ms/all*100:0))+'%"></div></div><strong>'+esc(activityDisplayDuration(ms,ko))+'</strong></div>').join('')+'</div>':'';
  const timelineData=filtered.map(x=>({label:activityDisplayType(x.type,ko)+' · '+x.role+' · '+(ko?'회차 ':'Round ')+x.round,
    role:String(x.role||''),agent:String(x.agent_id||''),attempt:String(x.attempt_id||''),from:x.started_at,to:x.ended_at,ms:x.duration_ms,wait:false}))
    .concat(activeRound==='all'?waitRows.map(x=>({label:(ko?'대기 · ':'Wait · ')+activityDisplayWait(x.kind,ko),
      role:'',agent:'',attempt:'',from:x.started_at,to:x.ended_at,ms:x.duration_ms,wait:true})):[]);
  // Retain the full time axis while filtering so round comparisons do not
  // silently rebase the data. Clip rendering, not the evidence itself.
  let begin=Infinity,end=-Infinity;
  for(const item of segments){
    for(const t of [Date.parse(item.started_at),Date.parse(item.ended_at)]){
      if(Number.isFinite(t)){if(t<begin)begin=t;if(t>end)end=t;}
    }
  }
  for(const item of waitRows){
    for(const t of [Date.parse(item.started_at),Date.parse(item.ended_at)]){
      if(Number.isFinite(t)){if(t<begin)begin=t;if(t>end)end=t;}
    }
  }
  if(!Number.isFinite(begin)||!Number.isFinite(end)){begin=0;end=0;}
  const span=Math.max(1,end-begin);
  const validTimeline=timelineData.filter(x=>Number.isFinite(Date.parse(x.from))&&Number.isFinite(Date.parse(x.to))&&Date.parse(x.to)>=Date.parse(x.from));
  const shownTimeline=validTimeline.slice(0,ACTIVITY_DETAIL_MAX_ROWS);
  const ruler='<div class="activity-time-ruler" aria-hidden="true">'+[0,.25,.5,.75,1].map(p=>'<span>'+
    esc(new Date(begin+(end-begin)*p).toLocaleString(ko?'ko-KR':'en-US',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}))+
    '</span>').join('')+'</div>';
  const timelineRow=x=>{
    const left=Math.max(0,Math.min(100,(Date.parse(x.from)-begin)/span*100));
    const width=Math.max(0,Math.min(100-left,(Date.parse(x.to)-Date.parse(x.from))/span*100));
    const tooltip=x.label+' · '+activityEventTime(x.from,ko)+' → '+activityEventTime(x.to,ko)+' · '+activityDisplayDuration(x.ms,ko)+
      (x.agent?' · '+x.agent:'');
    return '<div class="activity-timeline-row"><span title="'+esc(x.label)+'">'+esc(x.label)+'</span>'+
      '<div class="activity-timeline-track'+(x.wait?' activity-timeline-wait':'')+'"><div role="img" aria-label="'+esc(tooltip)+
      '" title="'+esc(tooltip)+'" tabindex="0" style="left:'+left+'%;width:'+width+
      '%;overflow:hidden;white-space:nowrap;display:flex;align-items:center;justify-content:center">'+
      '<small class="activity-timeline-duration-label" style="display:none;color:white;font-size:10px;pointer-events:none">'+
      esc(activityDisplayDuration(x.ms,ko))+'</small></div></div><strong>'+esc(activityDisplayDuration(x.ms,ko))+'</strong></div>';
  };
  const renderTimelineRows=()=>{
    if(!prefs.groupAgents)return shownTimeline.map(timelineRow).join('');
    const groups=new Map();
    for(const item of shownTimeline){
      // Use exact actor identity for grouping; do not mix different attempts.
      const key=item.wait?'wait':JSON.stringify([item.role,item.agent,item.attempt]);
      if(!groups.has(key))groups.set(key,{wait:item.wait,role:item.role,agent:item.agent,attempt:item.attempt,items:[]});
      groups.get(key).items.push(item);
    }
    return [...groups.values()].map(group=>{
      const label=group.wait?(ko?'대기 구간':'Waiting intervals'):
        activityDisplayRole(group.role,ko)+' · '+(group.agent|| (ko?'Agent 미확인':'Agent unknown'));
      const description=group.wait?'':(ko?'실행':'Attempt')+' '+(group.attempt||'—');
      return '<details class="activity-agent-group" open><summary title="'+esc(description)+'">'+esc(label)+
        ' <small>('+group.items.length+(ko?'건':' intervals')+')</small></summary>'+
        group.items.map(timelineRow).join('')+'</details>';
    }).join('');
  };
  const timeline=shownTimeline.length?'<h3>'+esc(ko?'실제 시간축 Gantt':'Actual-time Gantt')+'</h3>'+
    '<p class="summary">'+esc(activityEventTime(new Date(begin).toISOString(),ko))+' — '+esc(activityEventTime(new Date(end).toISOString(),ko))+'</p>'+
    '<div class="activity-timeline-scroll" tabindex="0" aria-label="'+esc(ko?'가로 스크롤 가능한 작업시간 차트':'Horizontally scrollable activity timeline')+'">'+
      '<div class="activity-timeline-canvas" style="min-width:'+(prefs.zoom*100)+'%">'+ruler+
      '<div class="activity-time-axis">'+renderTimelineRows()+'</div></div></div><p class="summary" id="activityTimelineSelection" role="status" hidden></p>':'<p class="summary">'+esc(ko?'선택한 회차에 측정된 작업 구간이 없습니다.':'No measured intervals for this round.')+'</p>';
  const group=activityRoundBreakdown(filtered);
  const groupKeys=[...group.keys()].sort((a,b)=>a-b);
  const maxRound=Math.max(1,...groupKeys.map(k=>[...group.get(k).values()].reduce((n,ms)=>n+ms,0)));
  const kinds=[...new Set(filtered.map(x=>x.type))].sort();
  const stacked=groupKeys.length?'<h3>'+esc(ko?'회차별 Agent 시간 구성':'Agent time composition by round')+'</h3>'+
    '<div class="activity-stacked-legend">'+kinds.map((name,i)=>'<span><i class="activity-stack-swatch activity-stack-color-'+(i%8)+'"></i>'+esc(activityDisplayType(name,ko))+'</span>').join('')+'</div>'+
    '<div class="activity-stacked-rounds">'+groupKeys.map(k=>{
      const entries=kinds.map((name,i)=>({name,ms:group.get(k).get(name)||0,color:i%8})).filter(x=>x.ms>0);
      const sum=entries.reduce((n,x)=>n+x.ms,0);
      return '<div class="activity-stacked-row"><span>'+esc(k===0?(ko?'최초':'Initial'):(ko?'재작업 ':'Rework ')+k)+'</span>'+
        '<div class="activity-stacked-track" role="img" aria-label="'+esc(entries.map(x=>x.name+' '+activityDisplayDuration(x.ms,ko)).join(', '))+'">'+
          '<div class="activity-stacked-bar" style="width:'+Math.max(0,Math.min(100,sum/maxRound*100))+'%">'+
          entries.map(x=>'<span class="activity-stack-color-'+x.color+'" style="width:'+(x.ms/sum*100)+'%" title="'+esc(x.name+' · '+activityDisplayDuration(x.ms,ko))+'"></span>').join('')+
          '</div></div><strong>'+esc(activityDisplayDuration(sum,ko))+'</strong></div>';
    }).join('')+'</div>':'<p class="summary">'+esc(ko?'선택한 회차에 측정된 Agent 시간이 없습니다.':'No measured agent time in the selected rounds.')+'</p>';
  const chart=(prefs.mode==='rounds'?stacked:timeline);
  const chartNote=(prefs.mode==='rounds'?ko?'스택은 에이전트별 투입시간입니다. 병렬 실행 시 실제 경과시간과 다를 수 있습니다.':'Stacks show summed agent time; concurrent agents can exceed wall time.':
     ko?'실제 시작·종료 시각을 사용했습니다. 회차 필터에서는 대기를 제외하며, 확대해도 시간 길이는 변경되지 않습니다.':'Actual start/end times. Round filters exclude unattributed waiting. Zoom changes scale, not elapsed time.');
  const trimNote=validTimeline.length>ACTIVITY_DETAIL_MAX_ROWS&&prefs.mode==='timeline'?
    '<p class="summary">'+esc(ko?'화면 성능을 위해 '+ACTIVITY_DETAIL_MAX_ROWS+'개 구간만 표시했습니다. 전체 기록은 아래 원장에서 확인하세요.':'Showing first '+ACTIVITY_DETAIL_MAX_ROWS+' intervals for performance; full evidence is available in the API.')+'</p>':'';
  const lifetimeMetrics=lc.registered_at?[
    [ko?'등록~완료 전체 경과':'Registered to completed',lc.total_elapsed_ms],
    [ko?'등록 후 배정 대기':'Registration queue',lc.registration_queue_ms],
    [ko?'배정 후 착수 대기':'Dispatch wait',lc.dispatch_wait_ms],
    [ko?'명시적 대기':'Explicit waiting',lc.explicit_wait_ms],
    [ko?'미귀속 경과시간':'Unattributed elapsed',lc.unattributed_elapsed_ms],
  ]:[];
  const lifetimeView=lifetimeMetrics.length?'<h3>'+esc(ko?'Lifecycle 기반 경과·대기 분석':'Lifecycle elapsed and waiting analysis')+'</h3>'+
    '<div class="activity-metric-grid">'+lifetimeMetrics.map(([label,ms])=>'<div class="activity-metric"><strong>'+esc(ms==null?'—':activityDisplayDuration(ms,ko))+'</strong><span>'+esc(label)+'</span></div>').join('')+'</div>'+
    '<p class="summary">'+esc(ko?'Lifecycle 등록~완료 범위를 기준으로 한 경과·대기입니다. 아래 Gantt는 관측 가능한 활동·대기 구간만 표시하므로 시작·종료 시각과 전체 길이가 다를 수 있습니다. 각 시간은 중첩될 수 있어 합산하지 않습니다. CPU 사용시간이나 Task Mecca 원인 지연을 의미하지 않습니다.':'Lifecycle measures registered-to-completed elapsed and waits. The Gantt covers only observed activity and wait intervals, so its endpoints may differ. Intervals can overlap and must not be summed. Not CPU time or evidence of Task Mecca-induced delay.')+'</p>'+
    ((lc.wait_activity_overlap_ms||0)>0?'<p class="timing-note warning">'+esc(ko?'활동·대기 근거 겹침: ':'Activity/wait overlap: ')+esc(activityDisplayDuration(lc.wait_activity_overlap_ms,ko))+'</p>':''):'';
  const unknown=(s.open_activities||s.invalid_transitions||s.invalid_corrections||lc.open_wait_count||payload.lifecycle_findings)?
    '<p class="timing-note warning">'+esc(ko?'미확정: 열린 활동 '+(s.open_activities||0)+'건 · 잘못된 전환 '+(s.invalid_transitions||0)+'건 · 정정 오류 '+(s.invalid_corrections||0)+'건 · 열린 대기 '+(lc.open_wait_count||0)+'건. 불명확한 구간은 임의 계산하지 않았습니다.':
     'Incomplete evidence: open activities '+(s.open_activities||0)+', invalid transitions '+(s.invalid_transitions||0)+', invalid corrections '+(s.invalid_corrections||0)+', open waits '+(lc.open_wait_count||0)+'. Unknown intervals are not estimated.')+'</p>':'';
  const unclassified=events.filter(e=>e.type==='other'&&e.action!=='correct').slice(-30);
  const unclassifiedGroups=new Map();
  for(const e of unclassified){
    const key=JSON.stringify([e.proposed_type||'other',e.agent_id||'-']);
    if(!unclassifiedGroups.has(key))unclassifiedGroups.set(key,[]);
    unclassifiedGroups.get(key).push(e);
  }
  const unclassifiedView=unclassified.length?'<h3>'+esc(ko?'미분류 활동 및 Agent 제안':'Unclassified activity suggestions')+'</h3><div class="activity-unclassified">'+
    [...unclassifiedGroups.values()].map(group=>{
      const first=group[0],label=activityDisplayType(first.proposed_type||'other',ko);
      return '<details class="activity-unclassified-item"><summary><strong title="'+esc(first.proposed_type||'other')+'">'+esc(label)+'</strong> · '+
        esc(group.length+(ko?'건':' records'))+'</summary>'+
        group.map(e=>'<div><small>'+esc(e.agent_id||'-')+' · '+esc(e.confidence==='inferred'?(ko?'Agent 추정':'Agent inferred'):(e.confidence||'unknown'))+
          '</small><p>'+esc(e.explanation||'-')+'</p><small>'+esc(e.evidence||'-')+'</small></div>').join('')+'</details>';
    }).join('')+'</div>':'';
  const tr=payload.test_report||{};
  const testOverview=tr.runs&&tr.runs.length?'<h3>'+esc(ko?'검증·빌드 실행 분석':'Test and build executions')+'</h3>'+
    '<div class="activity-metric-grid">'+
    '<div class="activity-metric"><strong>'+esc(activityDisplayDuration(tr.full_regression_ms,ko))+'</strong><span>'+esc(ko?'전체 회귀 누적':'Full regression total')+'</span></div>'+
    '<div class="activity-metric"><strong>'+esc(String(tr.full_regression_count||0))+'</strong><span>'+esc(ko?'전체 회귀 횟수':'Full regression count')+'</span></div>'+
    '<div class="activity-metric"><strong>'+esc(String(tr.repetition_candidates||0))+'</strong><span>'+esc(ko?'동일 SHA·명령·범위 재실행 후보':'Repeat candidates')+'</span></div></div>'+
    '<p class="summary">'+esc(ko?'반복 후보는 불필요한 테스트로 확정된 것이 아닙니다.':'Repeated test candidates are not necessarily wasteful.')+'</p>':(tr.runs?'<p class="summary">'+esc(ko?'이 분석 범위에 연결된 검증·빌드 실행 기록이 없습니다. 실제 실행 여부와 CI 연계 상태는 별도로 확인해야 합니다.':'No test or build execution evidence is linked to this scope. Actual execution and CI linkage require separate confirmation.')+'</p>':'');
  const lastRows=events.slice(-ACTIVITY_DETAIL_MAX_LEDGER).reverse();
  const ledger='<details id="activityLedgerDetail"'+(prefs.ledger?' open':'')+'><summary>'+esc(ko?'Activity 원장 상세':'Activity event ledger')+
      ' ('+events.length+')</summary>'+
      '<div class="activity-ledger-scroll"><table class="activity-ledger-table"><thead><tr>'+
      [ko?'시각':'Time',ko?'이벤트':'Event',ko?'주체 / 회차':'Actor / round',ko?'활동':'Activity',ko?'근거 / 출처':'Evidence / source'].map(x=>'<th scope="col">'+esc(x)+'</th>').join('')+'</tr></thead><tbody>'+
      lastRows.map(e=>'<tr><td>'+esc(activityEventTime(e.occurred_at,ko))+'</td><td>'+esc(e.action||'—')+
       '<small>'+esc(e.event_id||'—')+'</small></td><td>'+esc(e.role||'—')+' · '+esc(e.agent_id||'—')+
       '<small>'+esc((ko?'회차 ':'Round ')+(e.round??0))+'</small></td><td>'+esc(e.type||'—')+
       (e.supersedes_id?'<small>'+esc((ko?'정정 대상: ':'Supersedes: ')+e.supersedes_id)+'</small>':'')+
       '</td><td>'+esc(e.evidence||'—')+'<small>'+esc((e.source||'—')+' · '+(e.confidence||'—'))+'</small></td></tr>').join('')+
      '</tbody></table></div>'+((events.length>ACTIVITY_DETAIL_MAX_LEDGER)?'<p class="summary">'+esc(ko?'최신 '+ACTIVITY_DETAIL_MAX_LEDGER+'건을 표시합니다. 전체 원장은 Activity API에서 조회할 수 있습니다.':'Showing the latest '+ACTIVITY_DETAIL_MAX_LEDGER+' records; the API retains the full ledger.')+'</p>':'')+'</details>';
  const gap=s.unattributed_gap_ms>0?'<p class="summary">'+esc(ko?'활동 관측 구간 내 기록 공백: ':'Unattributed within observed activity span: ')+esc(activityDisplayDuration(s.unattributed_gap_ms,ko))+
    ' <span title="'+esc(ko?'첫 관측 활동부터 마지막 관측 활동까지의 범위에서 원장 활동이 차지하지 않는 시간입니다. 등록~완료 전체 경과의 미귀속 시간과 기준 범위가 다르며 합산할 수 없습니다.':'Unattributed time inside the observed activity window; differs from lifecycle elapsed unknown, and must not be added to it.')+'">ⓘ</span></p>':'';
  return settings+boxes+observationQuality+runtimeView+lifetimeView+byType+chart+'<p class="summary">'+esc(chartNote)+'</p>'+trimNote+
    testOverview+unclassifiedView+gap+unknown+ledger;
}
const activityDetailResizeObservers=new WeakMap();
function bindActivityDetailControls(taskID,project=state.project){
  const root=typeof document.getElementById==='function'?document.getElementById('activityMetricsBody'):null;
  if(!root||typeof root.querySelector!=='function')return;
  const pref=activityDetailPrefs(taskID,project);
  if(root.querySelector('.workflow-view')){bindWorkflowControls(taskID,project,root);return;}
  const previousObserver=activityDetailResizeObservers.get(root);
  const scrollToRecent=()=>{
    const scroller=root.querySelector('.activity-timeline-scroll');
    if(scroller && (pref.focus==='last50'||pref.focus==='last25')){
      scroller.scrollLeft=Math.max(0,scroller.scrollWidth-scroller.clientWidth);
    }
  };
  if(previousObserver)previousObserver.disconnect();
  const fitTimelineLabels=()=>{
    const labels=[...root.querySelectorAll('.activity-timeline-duration-label')];
    // Batch writes before reads to avoid a per-bar forced layout.
    for(const label of labels)label.style.display='inline-block';
    const fits=labels.map(label=>label.parentElement.getBoundingClientRect().width>=label.scrollWidth+12);
    labels.forEach((label,i)=>{label.style.display=fits[i]?'inline-block':'none';});
    const ticks=[...root.querySelectorAll('.activity-time-ruler span')];
    ticks.forEach(t=>{t.style.visibility='visible';});
    if(ticks.length>2){
      const rects=ticks.map(t=>t.getBoundingClientRect());
      let lastRight=rects[0].right;
      for(let i=1;i<ticks.length-1;i++){
        if(rects[i].left<lastRight+8 || rects[i].right>rects[ticks.length-1].left-8)ticks[i].style.visibility='hidden';
        else lastRight=rects[i].right;
      }
    }
  };
  if(typeof ResizeObserver!=='undefined'){
    const observer=new ResizeObserver(()=>{fitTimelineLabels();scrollToRecent();});
    const scroll=root.querySelector('.activity-timeline-scroll');
    const canvas=root.querySelector('.activity-timeline-canvas');
    if(scroll)observer.observe(scroll);
    if(canvas)observer.observe(canvas);
    activityDetailResizeObservers.set(root,observer);
  }
  if(typeof queueMicrotask==='function')queueMicrotask(()=>{fitTimelineLabels();scrollToRecent();});
  const canvasForSelection=root.querySelector('.activity-timeline-canvas');
  const selectedDetail=root.querySelector('#activityTimelineSelection');
  if(canvasForSelection&&selectedDetail){
    const showSelected=target=>{
      const interval=target&&typeof target.closest==='function'?target.closest('[data-workflow-interval],[role="img"]'):null;
      if(!interval||!canvasForSelection.contains(interval))return;
      selectedDetail.textContent=interval.getAttribute('data-workflow-detail')||interval.getAttribute('aria-label')||'';
      selectedDetail.hidden=false;
    };
    canvasForSelection.addEventListener('click',e=>showSelected(e.target));
    canvasForSelection.addEventListener('keydown',e=>{
      if(e.key==='Enter'||e.key===' '){
        const interval=e.target&&typeof e.target.closest==='function'?e.target.closest('[data-workflow-interval],[role="img"]'):null;
        if(interval){e.preventDefault();showSelected(interval);}
      }
    });
  }
  const modify=(id,fn)=>{
    const control=root.querySelector('#'+id);
    if(control)control.addEventListener('change',()=>{
      if(state.project!==project||state.detail!==taskID)return;
      fn(control.value);renderActivityDetailControls(taskID,project);
    });
  };
  modify('activityDetailRound',value=>{pref.round=value;});
  modify('activityDetailMode',value=>{pref.mode=value==='rounds'?'rounds':'timeline';});
  modify('activityDetailZoom',value=>{pref.zoom=Math.max(1,Math.min(8,Number(value)||1));pref.focus='manual';});
  modify('activityDetailFocus',value=>{
    pref.focus=value==='last50'||value==='last25'?value:'manual';
    if(pref.focus==='last50')pref.zoom=2;
    else if(pref.focus==='last25')pref.zoom=4;
    else pref.zoom=1;
  });
  const groupToggle=root.querySelector('#activityDetailGroupAgents');
  if(groupToggle)groupToggle.addEventListener('change',()=>{
    if(state.project!==project||state.detail!==taskID)return;
    pref.groupAgents=Boolean(groupToggle.checked);
    renderActivityDetailControls(taskID,project);
  });
  const zoom=root.querySelector('#activityDetailZoom');
  if(zoom)zoom.addEventListener('input',()=>{
    pref.zoom=Math.max(1,Math.min(8,Number(zoom.value)||1));
    pref.focus='manual';
    const focus=root.querySelector('#activityDetailFocus');
    if(focus)focus.value='manual';
    const label=root.querySelector('#activityDetailZoomValue');
    if(label)label.textContent=zoom.value+'×';
    const canvas=root.querySelector('.activity-timeline-canvas');
    if(canvas)canvas.style.minWidth=(Number(zoom.value)*100)+'%';
    fitTimelineLabels();
  });
  const diagnose=root.querySelector('#activityRuntimeDiagnose');
  if(diagnose)diagnose.addEventListener('click',()=>{
    activityDetailCache.delete(activityDetailKey(taskID,project));
    diagnose.disabled=true;
    void loadTaskActivity(taskID,project,true);
  });
  const ledger=root.querySelector('#activityLedgerDetail');
  if(ledger)ledger.addEventListener('toggle',()=>{pref.ledger=ledger.open;});
  const refresh=root.querySelector('#activityDetailRefresh');
  if(refresh)refresh.addEventListener('click',()=>{
    void loadTaskActivity(taskID,project);
  });
}
function renderActivityDetailControls(taskID,project=state.project){
  if(state.project!==project||state.detail!==taskID)return;
  const root=typeof document.getElementById==='function'?document.getElementById('activityMetricsBody'):null;
  if(!root)return;
  const disclosures=[...(root.querySelectorAll?.('details[id]')||[])].map(d=>[d.id,d.open]);
  const scroll=root.querySelector?.('.activity-timeline-scroll')?.scrollLeft||0;
  const active=typeof document!=='undefined'?document.activeElement:null;
  const activeId=root.contains?.(active)?active?.id:null;
  const activeInterval=root.contains?.(active)?active?.getAttribute?.('data-workflow-interval'):null;
  const activeRow=root.contains?.(active)?active?.getAttribute?.('data-workflow-row-select'):null,activeExpand=root.contains?.(active)?active?.getAttribute?.('data-workflow-row-expand'):null;
  const selected=root.querySelector?.('#activityTimelineSelection');
  const selection=selected&&!selected.hidden?selected.textContent:null;
  const y=typeof window!=='undefined'?window.scrollY:0,x=typeof window!=='undefined'?window.scrollX:0;
  workflowFloatingViews.get(root)?.dispose();workflowFloatingViews.delete(root);
  root.innerHTML=activityMetricsBody(activityDetailCache.get(activityDetailKey(taskID,project)),taskID);
  for(const [id,open] of disclosures){const el=root.querySelector?.('#'+id);if(el)el.open=open;}
  bindActivityDetailControls(taskID,project);
  const scroller=root.querySelector?.('.activity-timeline-scroll');if(scroller&&activityDetailPrefs(taskID,project).focus==='manual')scroller.scrollLeft=scroll;
  const output=root.querySelector?.('#activityTimelineSelection');if(output&&selection&&!root.querySelector('.workflow-view')){output.textContent=selection;output.hidden=false;}
  if(activeId)root.querySelector?.('#'+activeId)?.focus?.({preventScroll:true});
  if(activeRow||activeExpand){const attr=activeRow?'data-workflow-row-select':'data-workflow-row-expand',value=activeRow||activeExpand;[...(root.querySelectorAll?.('['+attr+']')||[])].find(button=>button.getAttribute(attr)===value)?.focus?.({preventScroll:true});}
  if(activeInterval){const button=[...(root.querySelectorAll?.('[data-workflow-interval]')||[])].find(button=>button.getAttribute('data-workflow-interval')===activeInterval);(button||root.querySelector?.('#workflowTimelineViewport'))?.focus?.({preventScroll:true});}
  if(typeof window!=='undefined'&&window.scrollY!==y)window.scrollTo?.({left:x,top:y,behavior:'auto'});
}

async function loadTaskActivity(id,project=state.project,runtimeDiagnostic=false){
  const key=activityDetailKey(id,project);
  if(activityDetailPending.has(key))return activityDetailPending.get(key);
  const job=(async()=>{
    let payload;
    try{
      payload=await fetchActivity(id,project,runtimeDiagnostic);
      if(payload&&typeof payload==='object'&&!payload.error)payload._client_fetched_at=new Date().toISOString();
    }catch(e){const previous=activityDetailCache.get(key);payload=previous?.workflow?{...previous,_client_error:String(e?.message||e),_client_failed_at:new Date().toISOString()}:{error:String(e?.message||e)}}
    activityDetailCache.set(key,payload);
    if(state.project===project&&state.detail===id){
      const el=typeof document.getElementById==='function'?document.getElementById('activityMetricsBody'):null;
      if(el)renderActivityDetailControls(id,project);
    }
    return payload;
  })();
  activityDetailPending.set(key,job);
  try{return await job}finally{activityDetailPending.delete(key)}
}
function activityDetailSection(task){
  const ko=state.language==='ko';
  return '<section class="section detail-section lifecycle-section workflow-section" id="lifecycle" data-toc-label="'+esc(ko?'작업 흐름':'Workflow')+'"><h2>'+esc(ko?'작업 흐름':'Workflow')+'</h2><div id="activity-metrics"><div id="activityMetricsBody">'+activityMetricsBody(activityDetailCache.get(activityDetailKey(task.id)),task.id)+'</div></div><details class="workflow-disclosure" id="workflowLifecycleEvents"><summary>'+esc(ko?'Lifecycle 사건 기록':'Lifecycle event record')+'</summary><div id="workflowLifecycleEvidenceBody">'+detailLifecycleBody(task)+'</div></details></section>';
}

export function disposeWorkflow(){const root=document.getElementById("activityMetricsBody");if(root){activityDetailResizeObservers.get(root)?.disconnect();activityDetailResizeObservers.delete(root);workflowFloatingViews.get(root)?.dispose();workflowFloatingViews.delete(root);}workflowActiveFloating?.dispose();workflowActiveFloating=null;}
export {activityDetailSection,bindActivityDetailControls,loadTaskActivity,syncWorkflowTimelineEntry};
