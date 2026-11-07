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
    state.pageSize + " " + text("개/쪽", "rows/page"),
    state.tags.length ? text("태그 ", "Tags ") + state.tags.join(", ") : "",
  ]
    .filter(Boolean)
    .join(" · ");
}
export function backlogResults(data) {
  const items = data.items || [];
  return `<div class="list-summary"><span>${text("작업", "Tasks")} <strong>${count(data.total)}</strong></span><span>${data.backlog_presence?.present === false ? text("백로그 없음", "No backlog") : text("파일과 관측된 상태", "File and observed states")}</span></div>${items.length ? `<ul class="task-list backlog-list">${taskRows(items)}</ul>` : empty(text("조건에 맞는 작업이 없습니다", "No tasks match these filters"))}${pageControls({ ...data, has_more: data.page < data.pages }, "backlog")}`;
}
export function backlog(data) {
  const tags = data.tag_options || [];
  return `${link(icon("back") + text("모든 프로젝트", "All projects"), { view: "projects", detail: null, project: "", backlog: "" }, "back-link")}<div class="backlog-heading">${heading(state.project.split("/").filter(Boolean).at(-1) || text("백로그", "Backlog"), text("작업 목록", "Task backlog"))}</div><form id="filters" class="filters"><label class="search-label">${icon("search")}<span class="live-region">${text("작업 검색", "Search tasks")}</span><input name="q" type="search" value="${esc(state.query)}" placeholder="${text("제목, ID, 내용 검색", "Search title, ID, content")}" autocomplete="off"></label><button type="submit" class="primary-apply">${text("검색", "Search")}</button></form><details class="tag-filter filter-more" ${state.toolsOpen ? "open" : ""}><summary><span>${icon("arrow")}${text("필터·정렬", "Filters & sort")}</span><span id="filterSummary">${esc(filterSummary())}</span></summary><div class="filter-settings"><label>${text("상태", "State")}<select name="status" form="filters">${!["all", "todo", "doing", "hold", "done"].includes(state.status) ? `<option value="${esc(state.status)}" selected>${esc(state.status.split(",").map(status).join(", "))}</option>` : ""}${["all", "todo", "doing", "hold", "done"].map((value) => `<option value="${value}" ${state.status === value ? "selected" : ""}>${value === "all" ? text("전체 상태", "All states") : status(value)}</option>`).join("")}</select></label><label>${text("정렬", "Sort")}<select name="sort" form="filters">${sortOptions.map(([value, ko, en]) => `<option value="${value}" ${state.sort === value ? "selected" : ""}>${text(ko, en)}</option>`).join("")}</select></label><label>${text("페이지 크기", "Page size")}<select name="page_size" form="filters">${[10, 20, 50].map((value) => `<option ${state.pageSize === value ? "selected" : ""}>${value}</option>`).join("")}</select></label></div><form id="tags"><label>${text("태그 (쉼표로 구분)", "Tags (comma separated)")}<input name="tags" value="${esc(state.tags.join(","))}" list="tag-options"></label><datalist id="tag-options">${tags.map((tag) => `<option value="${esc(tag.tag)}">${esc(tag.description || "")}</option>`).join("")}</datalist><button>${text("태그 적용", "Apply tags")}</button></form><p class="project-location">${text("프로젝트 경로", "Project path")}: ${esc(state.project)}</p>${data.selection?.selected ? `<p class="project-location">${text("백로그 경로", "Backlog path")}: ${esc(data.selection.selected)}</p>` : ""}</details><div id="listRequestStatus" role="status" aria-live="polite"></div><div id="backlogResults">${backlogResults(data)}</div>`;
}
