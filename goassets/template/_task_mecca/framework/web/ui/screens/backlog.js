import { state } from "../state.js";
import { esc, text, count, status } from "../format.js";
import {
  heading,
  taskRows,
  pageControls,
  empty,
  icon,
  link,
} from "../components/shared.js";
const sortOptions = [
  ["id_desc", "최신 ID", "Newest ID"],
  ["id_asc", "이전 ID", "Oldest ID"],
  ["updated_desc", "최근 수정", "Recently updated"],
  ["updated_asc", "오래된 수정", "Least recently updated"],
];
export function filterSummary() {
  const selected =
      state.status === "all"
        ? text("전체 상태", "All states")
        : state.status.split(",").map(status).join(", "),
    sort =
      sortOptions.find(([value]) => value === state.sort) || sortOptions[0];
  return [
    selected,
    text(sort[1], sort[2]),
    (state.pageSize === "auto" ? text("자동", "Auto") : state.pageSize + " " + text("개/쪽", "rows/page")),
    state.tags.length ? text("태그 ", "Tags ") + state.tags.join(", ") : "",
  ]
    .filter(Boolean)
    .join(" · ");
}
export function backlogResults(data) {
  const items = data.items || [];
  return `<div class="list-summary"><span>${text("작업", "Tasks")} <strong>${count(data.total)}</strong></span><span>${data.backlog_presence?.present === false ? text("백로그 없음", "No backlog") : text("파일과 관측된 상태", "File and observed states")}</span></div>${items.length ? `<ul class="task-list backlog-list">${taskRows(items)}</ul>` : empty(text("조건에 맞는 작업이 없습니다", "No tasks match these filters"))}${pageControls({ ...data, has_more: data.page < data.pages }, "backlog")}`;
}
function tagCatalog(tags) {
  const groups=new Map(); for(const tag of tags){const prefix=tag.tag.includes(':')?tag.tag.split(':')[0]:text('일반','General'); if(!groups.has(prefix))groups.set(prefix,[]);groups.get(prefix).push(tag);}
  return `<details class="tag-catalog"><summary>${text('태그 목록에서 선택','Choose from tag catalog')} (${tags.length})</summary>${[...groups].map(([name,items])=>`<fieldset><legend>${esc(name)}</legend><div class="notification-kinds">${items.map(tag=>`<label title="${esc(tag.description||'')}"><input type="checkbox" data-tag-filter="${esc(tag.tag)}" ${state.tags.includes(tag.tag)?'checked':''}>${esc(tag.tag)}${tag.count!=null?` <small>(${count(tag.count)})</small>`:''}</label>`).join('')}</div></fieldset>`).join('')}</details>`;
}
export function backlog(data) {
  const tags = data.tag_options || [];
  return `${link(icon("back") + text("모든 프로젝트", "All projects"), { view: "projects", detail: null, project: "", backlog: "" }, "back-link")}<div class="backlog-heading">${heading(state.project.split("/").filter(Boolean).at(-1) || text("백로그", "Backlog"), text("작업 목록", "Task backlog"))}</div><form id="filters" class="filters"><label class="search-label">${icon("search")}<span class="live-region">${text("작업 검색", "Search tasks")}</span><input name="q" type="search" value="${esc(state.query)}" placeholder="${text("제목, ID, 내용 검색", "Search title, ID, content")}" autocomplete="off"></label><button type="submit" class="primary-apply">${text("검색", "Search")}</button></form><details class="tag-filter filter-more" ${state.toolsOpen ? "open" : ""}><summary><span>${icon("arrow")}${text("필터·정렬", "Filters & sort")}</span><span id="filterSummary">${esc(filterSummary())}</span></summary><div class="filter-settings">${data.selection?.candidates?.length?`<label>${text('백로그 위치','Backlog location')}<select id="backlog-location"><option value="">Auto</option>${data.selection.candidates.map(c=>`<option value="${esc(c.path)}" ${state.backlog===c.path?'selected':''}>${esc(c.label||c.path)}</option>`).join('')}</select></label>`:''}<label>${text("상태", "State")}<input type="hidden" name="status" form="filters" value="${esc(state.status)}"><div class="filter-statuses" role="group" aria-label="${text('상태 필터','Status filters')}">${['all','ready','doing','hold','blocked','done'].map(value=>`<button type="button" data-status-filter="${value}" aria-pressed="${state.status.split(',').includes(value)}">${value==='all'?text('전체','All'):status(value)}</button>`).join('')}</div></label><label>${text("정렬", "Sort")}<select name="sort" form="filters">${sortOptions.map(([value, ko, en]) => `<option value="${value}" ${state.sort === value ? "selected" : ""}>${text(ko, en)}</option>`).join("")}</select></label><label>${text("페이지 크기", "Page size")}<select name="page_size" form="filters">${["auto", 10, 20, 50].map((value) => `<option value="${value}" ${state.pageSize === value ? "selected" : ""}>${value === "auto" ? text("자동", "Auto") : value}</option>`).join("")}</select></label></div><form id="tags"><label>${text("태그 (쉼표로 구분)", "Tags (comma separated)")}<input name="tags" value="${esc(state.tags.join(","))}" list="tag-options"></label><datalist id="tag-options">${tags.map((tag) => `<option value="${esc(tag.tag)}">${esc(tag.description || "")}</option>`).join("")}</datalist>${tagCatalog(tags)}<button>${text("태그 적용", "Apply tags")}</button></form><p class="project-location">${text("프로젝트 경로", "Project path")}: ${esc(state.project)}</p>${data.selection?.selected ? `<p class="project-location">${text("백로그 경로", "Backlog path")}: ${esc(data.selection.selected)}</p>` : ""}</details><div id="listRequestStatus" role="status" aria-live="polite"></div><div id="backlogResults">${backlogResults(data)}</div>`;
}
