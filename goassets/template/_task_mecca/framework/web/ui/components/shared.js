import { environmentSlot } from "../environment.js";
import { state, routeURL } from "../state.js";
import { esc, text, status, date, count } from "../format.js";
const paths = {
  home: "M3 10 12 3l9 7v10H3z M9 20v-7h6v7",
  projects: "M3 7h7l2-3h9v16H3z",
  activity: "M3 12h4l3-7 4 14 3-7h4",
  settings: "M4 7h16M4 17h16M8 4v6M16 14v6",
  back: "m14 5-7 7 7 7",
  arrow: "m9 5 7 7-7 7",
  refresh: "M20 8a8 8 0 1 0 0 8M20 3v5h-5",
  search: "M20 20l-5-5M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0",
  bell: "M6 9a6 6 0 0 1 12 0v6l2 3H4l2-3z M10 21h4",
};
export const icon = (name) =>
  `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="${paths[name] || paths.arrow}"/></svg>`;
export const link = (label, changes, cls = "") =>
  `<a class="${cls}" data-route href="${esc(routeURL(changes))}">${label}</a>`;
export function shell() {
  return `<aside class="rail"><a class="brand" data-route href="/"><img src="/logo.svg" alt=""><strong>Task Mecca</strong></a><nav aria-label="${text("주 탐색", "Main navigation")}">${[
    ["home", "관제 홈", "Control home"],
    ["projects", "프로젝트", "Projects"],
    ["activity", "최근 활동", "Recent activity"],
    ["notifications", "알림", "Notifications"],
  ]
    .map(([v, k, e]) =>
      link(
        icon(v === "notifications" ? "bell" : v) + `<span>${text(k, e)}</span>`,
        { view: v, detail: null, project: "", backlog: "" },
        "rail-link " + (state.view === v ? "active" : ""),
      ),
    )
    .join(
      "",
    )}</nav><div class="rail-bottom"><p>${text("작업의 근거까지, 한곳에서.", "Your work. Its evidence. One place.")}</p>${link(icon("settings") + `<span>${text("환경 설정", "Preferences")}</span>`, { view: "settings", detail: null }, "rail-link " + (state.view === "settings" ? "active" : ""))}<span class="stage-label">${text("핵심 프로토타입 · 검토용", "Core prototype · For review")}</span></div></aside><div class="workspace"><header class="topbar"><span>${text("프로젝트 관제", "Project control")}</span><div class="topbar-end"><button id="refresh" class="icon-button" title="${text("새로고침", "Refresh")}" aria-label="${text("현재 화면 새로고침", "Refresh current screen")}">${icon("refresh")}</button><button id="language" class="quiet">${state.language === "ko" ? "EN" : "한국어"}</button><select id="appearance" aria-label="${text("화면 모드", "Appearance")}">${[
    ["system", "시스템", "System"],
    ["light", "라이트", "Light"],
    ["dark", "다크", "Dark"],
  ]
    .map(
      ([v, k, e]) =>
        `<option value="${v}" ${state.mode === v ? "selected" : ""}>${text(k, e)}</option>`,
    )
    .join(
      "",
    )}</select></div></header>${environmentSlot()}<main id="main" tabindex="-1"></main></div><div class="live-region" role="status" aria-live="polite" id="announcement"></div>`;
}
export function heading(title, description, extra = "") {
  return `<div class="page-heading"><div><h1>${esc(title)}</h1>${description ? `<p>${esc(description)}</p>` : ""}</div>${extra}</div>`;
}
function updatedFact(task) {
  const value = task.updated_at || task.mtime;
  return typeof value === "string" && Number.isFinite(Date.parse(value))
    ? `<time class="task-updated" datetime="${esc(new Date(value).toISOString())}" title="${esc(value)}">${text("수정", "Updated")} ${esc(date(value))}</time>`
    : `<span class="task-updated">${text("수정 시각 미확인", "Updated: Unknown")}</span>`;
}
export function taskRows(items = []) {
  return items
    .map((task) => {
      const owner = task.agent || text("담당 미확인", "Assignee unknown"),
        project = task.project_name || task.project;
      const metadata = `<div class="task-meta">${project ? `<span class="task-project">${esc(project)}</span>` : ""}<span class="task-owner">${esc(owner)}</span>${updatedFact(task)}</div>`;
      return `<li class="task-row" data-id="${esc(task.id)}">${link(`<span class="task-id">${esc(task.id)}</span><div class="task-main"><strong>${esc(task.title)}</strong>${metadata}${task.reason ? `<p class="reason">${esc(typeof task.reason === "string" ? task.reason : task.reason.message || task.reason.label || task.reason.kind || text("판단 필요", "Needs attention"))}</p>` : ""}</div><span class="status ${esc(task.state)}">${esc(status(task.state))}${task.state_provisional ? " · " + text("잠정", "Provisional") : ""}</span>${icon("arrow")}`, { view: "backlog", detail: task.id, project: task.project || state.project, backlog: task.backlog || state.backlog, detailTab: "overview" }, "task-link")}</li>`;
    })
    .join("");
}
export function pageControls(section, key) {
  return `<div class="pagination"><span>${text("페이지", "Page")} ${count(section.page || 1)}${section.pages != null ? " / " + count(section.pages) : " · " + text("전체 범위 확인 중", "Coverage incomplete")}</span><div><button data-page="${key}:prev" ${!(section.page > 1) ? "disabled" : ""}>${text("이전", "Previous")}</button><button data-page="${key}:next" ${!section.has_more ? "disabled" : ""}>${text("다음", "Next")}</button></div></div>`;
}
export function empty(message) {
  return `<div class="empty"><strong>${esc(message)}</strong><p>${text("현재 조회 범위와 필터를 기준으로 표시합니다.", "Based on the current coverage and filters.")}</p></div>`;
}
export function failure(error) {
  return `<div class="error" role="alert"><h2>${text("정보를 불러오지 못했습니다", "Could not load information")}</h2><p>${esc(error.message || error)}</p><button id="retry">${text("다시 시도", "Retry")}</button></div>`;
}
