import { state } from "../state.js";
import { esc, text, date, count } from "../format.js";
import {
  heading,
  taskRows,
  pageControls,
  empty,
  link,
  icon,
} from "../components/shared.js";
function coverage(s) {
  return `<span class="coverage">${s.status === "stale" ? text("오래된 정보", "Stale") : s.status === "partial" ? text("부분 조회", "Partial") : text("조회 완료", "Observed")}${s.coverage && !s.coverage.complete ? " · " + count(s.coverage.scanned_projects) + " / " + count(s.coverage.total_projects) + " " + text("프로젝트", "projects") : ""}</span>`;
}
function projectState(value) {
  return (
    {
      ok: text("관측됨", "Observed"),
      stale: text("오래된 정보", "Stale"),
      partial: text("부분 조회", "Partial"),
      paused: text("관측 중지", "Monitoring paused"),
      missing: text("경로 없음", "Path unavailable"),
      error: text("조회 오류", "Read error"),
      loading: text("읽는 중", "Loading"),
    }[value] ||
    value ||
    text("미확인", "Unknown")
  );
}
export function projectRows(items = []) {
  return `<ul class="project-list">${items
    .map((p) => {
      const fullPath = p.path || p.project || "",
        shortPath = fullPath.split("/").filter(Boolean).slice(-2).join("/");
      return `<li>${link(`<span class="project-mark">${esc((p.name || "P").slice(0, 1))}</span><div class="project-identity"><strong>${esc(p.name || fullPath)}</strong><p class="project-meta"><span class="project-path" title="${esc(fullPath)}">${esc(shortPath ? "…/" + shortPath : fullPath)}</span><span class="project-state">${esc(projectState(p.status))}</span></p></div><div class="project-counts"><span>${text("진행", "Working")} <b>${count(p.counts?.working)}</b></span><span>${text("확인", "Attention")} <b>${count(p.counts?.attention)}</b></span></div>${icon("arrow")}`, { view: "backlog", detail: null, project: fullPath, backlog: "", page: 1, query: "", tags: [], status: "all" }, "project-link")}</li>`;
    })
    .join("")}</ul>`;
}
export function activityRows(items = []) {
  return `<ol class="event-list">${items.map((a) => `<li><time>${esc(date(a.at || a.created_at || a.updated_at))}</time><div><strong>${esc(a.title || a.message || a.kind || a.type || text("활동 기록", "Activity record"))}</strong><p>${esc(a.project_name || a.project || "")}${a.task_id ? " · " + link(esc(a.task_id), { detail: a.task_id, view: "backlog", project: a.project || "", backlog: a.backlog || "" }) : ""}</p><small>${text("알림 저널에서 관측", "Observed in notification journal")}</small></div></li>`).join("")}</ol>`;
}
export function home(data, only = null) {
  const sections = data.sections || {},
    names = {
      attention: [
        text("판단이 필요한 작업", "Needs your attention"),
        text(
          "확인이 필요한 근거부터 살펴보세요.",
          "Start with the evidence that needs a decision.",
        ),
      ],
      working: [
        text("진행 중인 작업", "Work in progress"),
        text(
          "실제 기록과 잠정 파일 상태를 구분합니다.",
          "Observed execution and provisional file state stay distinct.",
        ),
      ],
      projects: [
        text("프로젝트", "Projects"),
        text(
          "관리 중인 프로젝트를 한눈에 살펴보세요.",
          "Keep every managed project in view.",
        ),
      ],
      activity: [
        text("최근 활동", "Recent activity"),
        text(
          "기존 알림 저널의 관측 기록입니다.",
          "Observed events from the notification journal.",
        ),
      ],
    };
  return (
    heading(
      only ? names[only][0] : text("관제 홈", "Control home"),
      only
        ? names[only][1]
        : text(
            "여러 프로젝트의 현재 상황과 다음 확인 지점.",
            "Across your projects, see what needs attention next.",
          ),
      `<span class="freshness">${text("관측", "Observed")} ${esc(date(data.freshness?.observed_at))}${data.freshness?.stale ? " · " + text("오래됨", "Stale") : ""}</span>`,
    ) +
    `${data.status !== "ok" || data.loading || data.pending ? `<div class="notice" role="status"><strong>${data.loading ? text("프로젝트를 읽고 있습니다", "Reading project data") : data.pending ? text("관측 근거를 확인하고 있습니다", "Checking observed evidence") : text("일부 정보만 확인되었습니다", "Some information is incomplete")}</strong><span>${text("미확인 값은 0으로 계산하지 않습니다.", "Unknown values are not counted as zero.")}</span>${data.loading || data.pending ? `<span>${text("잠시 후 자동으로 다시 확인합니다.", "Checking again shortly.")}</span>` : ""}</div>` : ""}<div class="overview-flow ${only ? "overview-only" : ""}">${(only
      ? [only]
      : ["attention", "working", "projects", "activity"]
    )
      .map((key) => {
        const s = sections[key] || { items: [], status: "partial" },
          items = s.items || [];
        return `<section class="overview-section ${key}" id="${key}"><div class="section-heading"><div><h2>${names[key][0]} ${s.total != null ? `<span class="section-total">${count(s.total)}</span>` : ""}</h2><p>${names[key][1]}</p></div>${coverage(s)}</div>${items.length ? (key === "projects" ? projectRows(items) : key === "activity" ? activityRows(items) : `${homeTaskRows(items, key, only)}`) : empty(s.status === "partial" ? text("아직 확인하지 못한 범위입니다", "This coverage has not been observed yet") : text("해당 기록이 없습니다", "No matching records"))}${pageControls(s, key)}</section>`;
      })
      .join(
        "",
      )}</div>${data.errors?.length ? `<details class="source-errors"><summary>${text("조회 오류", "Source errors")} (${data.errors.length})</summary>${data.errors.map((e) => `<p>${esc(e.project)}: ${esc(e.message)}</p>`).join("")}</details>` : ""}`
  );
}

function homeTaskRows(items, key, only) {
  if (key !== "attention" || only || items.length <= 2)
    return `<ul class="task-list">${taskRows(items)}</ul>`;
  return `<ul class="task-list">${taskRows(items.slice(0, 2))}</ul><details id="attentionMore" class="attention-more" ${state.attentionExpanded ? "open" : ""}><summary>${icon("arrow")}<span>${text("현재 페이지의 나머지 ", "More on this page (")}${items.length - 2}${text("개 보기", ")")}</span></summary><ul class="task-list">${taskRows(items.slice(2))}</ul></details>`;
}
