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
export function backlog(data) {
  const items = data.items || [],
    tags = data.tag_catalog || [];
  return `${link(icon("back") + text("모든 프로젝트", "All projects"), { view: "projects", detail: null, project: "", backlog: "" }, "back-link")}${heading(state.project.split("/").filter(Boolean).at(-1) || text("백로그", "Backlog"), state.project)}<form id="filters" class="filters"><label class="search-label">${icon("search")}<span class="live-region">${text("작업 검색", "Search tasks")}</span><input name="q" type="search" value="${esc(state.query)}" placeholder="${text("제목, ID, 내용 검색", "Search title, ID, content")}" autocomplete="off"></label><label>${text("상태", "State")}<select name="status">${["all", "todo", "doing", "hold", "done"].map((v) => `<option value="${v}" ${state.status === v ? "selected" : ""}>${v === "all" ? text("전체 상태", "All states") : status(v)}</option>`).join("")}</select></label><label>${text("정렬", "Sort")}<select name="sort">${[
    ["id_desc", "최신 ID", "Newest ID"],
    ["id_asc", "이전 ID", "Oldest ID"],
    ["updated_desc", "최근 수정", "Recently updated"],
    ["title_asc", "제목", "Title"],
  ]
    .map(
      ([v, k, e]) =>
        `<option value="${v}" ${state.sort === v ? "selected" : ""}>${text(k, e)}</option>`,
    )
    .join(
      "",
    )}</select></label><label>${text("페이지 크기", "Page size")}<select name="page_size">${[10, 20, 50].map((n) => `<option ${state.pageSize === n ? "selected" : ""}>${n}</option>`).join("")}</select></label><button type="submit">${text("적용", "Apply")}</button></form><details class="tag-filter" ${state.tags.length ? "open" : ""}><summary>${text("태그 필터", "Filter by tags")} ${state.tags.length ? `(${state.tags.length})` : ""}</summary><form id="tags"><label>${text("태그 (쉼표로 구분)", "Tags (comma separated)")}<input name="tags" value="${esc(state.tags.join(","))}" list="tag-options"></label><datalist id="tag-options">${tags.map((t) => `<option value="${esc(typeof t === "string" ? t : t.name || t.tag)}">`).join("")}</datalist><button>${text("태그 적용", "Apply tags")}</button></form></details><div class="list-summary"><span>${text("작업", "Tasks")} <strong>${count(data.total)}</strong></span><span>${data.backlog_presence?.present === false ? text("백로그 없음", "No backlog") : text("파일과 관측된 상태를 표시합니다", "File and observed states are shown")}</span></div>${items.length ? `<ul class="task-list backlog-list">${taskRows(items)}</ul>` : empty(text("조건에 맞는 작업이 없습니다", "No tasks match these filters"))}${pageControls({ ...data, has_more: data.page < data.pages }, "backlog")}${data.selection?.candidates?.length ? `<details class="source-errors"><summary>${text("백로그 경로", "Backlog location")}</summary><p>${esc(data.selection.selected || state.backlog)}</p></details>` : ""}`;
}
