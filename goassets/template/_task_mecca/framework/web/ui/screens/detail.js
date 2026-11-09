import { state, routeURL } from "../state.js";
import { esc, text, status, markdown, fmtSec, date } from "../format.js";
import { heading, link, icon } from "../components/shared.js";
import { lifecycleSnapshotNotice } from "../components/lifecycle.js";
import { activityDetailSection } from "../components/workflow.js";
function section(label, value, id) {
  if (value == null || value === "") return "";
  return `<section class="document-section detail-section" id="${id}" data-toc-label="${esc(label)}"><h2>${esc(label)}</h2><div class="markdown">${markdown(typeof value === "string" ? value : JSON.stringify(value, null, 2))}</div></section>`;
}
const jump = (id, label, tab = "overview") => `<a href="${esc(routeURL({detailTab:tab}))}#${id}" data-detail-target="${id}">${label}</a>`;
function requirements(task) {
  const d=task.document||{},r=d.requirements||{};
  if(d.contract_kind==='simple')return section(text('작업 정의','Task definition'),r.goal,'task-definition')+section(text('완료 기준','Acceptance'),r.acceptance||r.acceptance_items,'acceptance');
  if(Object.values(r).some(v=>typeof v==='string'&&v.trim())){
    const field=(label,value)=>value?`<h3>${label}</h3><div class="markdown">${markdown(value)}</div>`:'';
    const group=(id,label,body)=>`<section class="document-section detail-section" id="${id}" data-toc-label="${label}"><h2>${label}</h2>${body||`<p class="muted">${text('기록 없음','Not recorded')}</p>`}</section>`;
    return section(text('배경·문제','Background'),r.background||text('기록 없음','Not recorded'),'background')+
      group('requirements',text('요구사항·목표','Requirements & goal'),field(text('목표','Goal'),r.goal)+field(text('요구사항','Requirements'),r.requirements))+
      group('scope',text('작업 범위','Scope'),field(text('포함 범위','In scope'),r.scope_in)+field(text('제외 범위','Out of scope'),r.scope_out)+field(text('제약','Constraints'),r.constraints))+
      section(text('완료 기준','Acceptance'),r.acceptance||r.acceptance_items||text('기록 없음','Not recorded'),'acceptance');
  }
  return section(text('작업 본문','Task description'),d.legacy_description||task.fields?.설명||task.raw_markdown,'legacy-task');
}
function outcome(task) {
  const result=task.document?.result_summary||{};
  const rows=[['achievement_1','주요 결과 1','Achievement 1'],['achievement_2','주요 결과 2','Achievement 2'],['achievement_3','주요 결과 3','Achievement 3'],['verdict','결과 판단','Verdict'],['outstanding','남은 작업','Outstanding'],['user_action','사용자 확인','User action']].filter(([key])=>String(result[key]||'').trim());
  if(!rows.length)return '';
  return `<section class="document-section detail-section" id="outcome-summary" data-toc-label="${text('결과 요약','Outcome')}"><h2>${text('결과 요약','Outcome')}</h2><dl>${rows.map(([key,ko,en])=>`<div><dt>${text(ko,en)}</dt><dd>${esc(result[key])}</dd></div>`).join('')}</dl></section>`;
}
export function detail(task) {
  const d=task.document||{},r=d.requirements||{},s=d.summary||{};
  const base=link(icon('back')+text('백로그로','Back to backlog'),{detail:null,detailTab:'overview'},'back-link').replace('<a ','<a id="backBtn" ');
  const summary=`<section class="summary-sheet detail-section" id="human-summary" data-toc-label="${text('작업 개요','Task overview')}"><h2>${text('작업 개요','Task overview')}</h2><dl>${[
    ['목적','Purpose',s.purpose||r.goal],['핵심 변경','Key change',s.change],['상태·결과','Status / result',s.status_result],['확인·후속','Checks / follow-up',s.follow_up]
  ].map(([ko,en,v])=>`<div><dt>${text(ko,en)}</dt><dd>${esc(v||text('기록 없음','Not recorded'))}</dd></div>`).join('')}</dl><nav class="detail-tabs" aria-label="${text('본문 바로 이동','Jump to section')}">${jump('human-summary',text('개요','Overview'))}${jump(d.contract_kind==='simple'?'task-definition':Object.keys(r).length?'requirements':'legacy-task',text('요건','Requirements'),'requirements')}${jump('lifecycle',text('라이프사이클','Lifecycle'),'lifecycle')}${jump('verification',text('실행·검증','Execution & validation'),'evidence')}</nav></section>`;
  const related=relatedTasks(task).replace('<section class="related-tasks">','<section class="related-tasks detail-section" id="related-work" data-toc-label="'+text('연결된 작업','Connected work')+'">');
  const operations=executionMetadata(task).replace('<section class="document-section">','<section class="document-section detail-section" id="operations" data-toc-label="'+text('실행과 관측','Execution and observation')+'">');
  const source=`<details class="raw-source detail-section" id="source" data-toc-label="${text('작업 원문','Task source')}"><summary>${text('작업 원문','Task source')}</summary><pre class="source-code">${esc(task.raw_markdown||'')}</pre></details>`;
  const body=summary+outcome(task)+activityDetailSection(task)+related+operations+
    (task.attention_reason?section(text('확인이 필요한 근거','Attention evidence'),task.attention_reason,'attention-evidence'):'')+
    requirements(task)+section(text('작업 메모','Work notes'),d.notes||task.fields?.메모,'progress-result')+
    section(text('실행 근거','Execution evidence'),d.execution||d.result||d.results||task.execution||task.fields?.결과,'execution')+
    section(text('검증','Validation'),d.validation||task.validation||d.verification||task.fields?.검증||text('기록 없음','Not recorded'),'verification')+source;
  return `<div class="detail-layout"><article class="detail-reading">${base}<div class="detail-identity"><span>${esc(task.id)}</span><span class="status ${esc(task.state)}">${esc(status(task.state))}</span></div>${heading(task.title,task.agent||text('담당 미확인','Assignee unknown'))}${sourceIdentity(task)}<div id="detailLifecycleNotice">${task.content_only?`<p class="muted">${text('실행·라이프사이클 근거를 확인하고 있습니다.','Checking lifecycle and execution evidence.')}</p>`:lifecycleSnapshotNotice(task)}</div><div class="detail-body">${body}</div></article><button class="detail-toc-fab" type="button" aria-controls="detail-toc" aria-expanded="false" aria-label="${text('목차 열기','Open table of contents')}">${icon('index')}</button><div class="detail-toc-scrim" aria-hidden="true"></div><aside id="detail-toc" class="detail-toc" aria-label="${text('본문 목차','Table of contents')}"><div class="detail-toc-head"><strong>${text('목차','Contents')}</strong><button class="detail-toc-close" type="button" aria-label="${text('목차 닫기','Close table of contents')}">×</button></div><nav class="detail-toc-nav"></nav></aside></div>`;
}

function safeSourceURL(value) {
  try {
    const url = new URL(value, location.origin);
    return ["http:", "https:"].includes(url.protocol) ? url.href : null;
  } catch {
    return null;
  }
}
function sourceIdentity(task) {
  const source = task.source || {},
    url = safeSourceURL(source.url),
    tags = task.tags || [];
  return `<div class="detail-source">${source.label ? `<span>${text("출처", "Source")}: ${url ? `<a href="${esc(url)}" target="_blank" rel="noopener noreferrer">${esc(source.label)}</a>` : esc(source.label)}</span>` : ""}${tags.length ? `<span class="detail-tags">${tags.map((tag) => `<span>${esc(tag)}</span>`).join("")}</span>` : ""}</div>`;
}
function relatedTasks(task) {
  const groups = [
    [text("선행 작업", "Dependencies"), task.depends_on],
    [text("관련 작업", "Related tasks"), task.related],
  ];
  return groups.some(([, ids]) => ids?.length)
    ? `<section class="related-tasks"><h2>${text("연결된 작업", "Connected work")}</h2>${groups.map(([label, ids]) => (ids?.length ? `<div><span>${label}</span>${ids.map((id) => link(esc(id), { detail: id, detailTab: "overview" })).join(" ")}</div>` : "")).join("")}</section>`
    : "";
}
function executionMetadata(task) {
  const rows = [
    [text("담당", "Assignee"), task.agent],
    [text("등록자", "Registrant"), task.registrant],
    [
      text("실행 상태", "Execution state"),
      task.fields?.Dispatch상태 || task.fields?.실행상태,
    ],
    [text("실행 근거", "Execution evidence"), task.fields?.실행근거],
    [text("대체 실행 근거", "Fallback evidence"), task.fields?.Fallback근거],
    [text("최근 수정", "Updated"), date(task.updated_at || task.mtime)],
    [text("관측 상태", "Observed health"), task.activity?.health],
    [
      text("최근 신호", "Last signal"),
      task.activity?.last_activity_at
        ? date(task.activity.last_activity_at)
        : null,
    ],
  ];
  return `<section class="document-section"><h2>${text("실행과 관측", "Execution and observation")}</h2><dl class="execution-metadata">${rows.map(([label, value]) => `<div><dt>${label}</dt><dd>${esc(value || text("미확인", "Unknown"))}</dd></div>`).join("")}</dl><div class="timing-facts">${[
    ["active", "활동", "Active"],
    ["wait", "대기", "Wait"],
    ["queue", "큐 대기", "Queue"],
    ["lead", "전체 경과", "Lead"],
  ]
    .map(
      ([key, ko, en]) =>
        `<span>${text(ko, en)} <strong>${esc(fmtSec(task[key + "_seconds"] ?? (key === "lead" ? task.lifecycle?.lead_seconds : undefined)))}</strong></span>`,
    )
    .join(
      "",
    )}</div><p class="muted">${text("미측정 시간은 0으로 표시하지 않습니다. 누적 활동 시간은 전체 경과와 합산하지 않습니다.", "Unmeasured time is not zero. Cumulative activity is not added to elapsed time.")}</p></section>`;
}
