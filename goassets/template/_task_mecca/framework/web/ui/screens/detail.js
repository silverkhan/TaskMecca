import { state } from "../state.js";
import { esc, text, status, markdown, fmtSec, date } from "../format.js";
import { heading, link, icon } from "../components/shared.js";
import { lifecycleSnapshotNotice } from "../components/lifecycle.js";
import { activityDetailSection } from "../components/workflow.js";
function section(label, value) {
  if (value == null || value === "") return "";
  return `<section class="document-section"><h2>${esc(label)}</h2><div class="markdown">${markdown(typeof value === "string" ? value : JSON.stringify(value, null, 2))}</div></section>`;
}
export function detail(task) {
  const d = task.document || {},
    r = d.requirements || {},
    s = d.summary || {},
    tab = state.detailTab,
    base = link(
      icon("back") + text("백로그로", "Back to backlog"),
      { detail: null, detailTab: "overview" },
      "back-link",
    ).replace("<a ", '<a id="backBtn" ');
  const tabs = [
    ["overview", "개요", "Overview"],
    ["requirements", "요건", "Requirements"],
    ["lifecycle", "라이프사이클", "Lifecycle"],
    ["evidence", "실행·검증", "Execution & validation"],
  ];
  return (
    base +
    `<div class="detail-identity"><span>${esc(task.id)}</span><span class="status ${esc(task.state)}">${esc(status(task.state))}</span></div>` +
    heading(task.title, task.agent || text("담당 미확인", "Assignee unknown")) +
    sourceIdentity(task) +
    `<div id="detailLifecycleNotice">${task.content_only ? `<p class="muted">${text("실행·라이프사이클 근거를 확인하고 있습니다.", "Checking lifecycle and execution evidence.")}</p>` : lifecycleSnapshotNotice(task)}</div><nav class="detail-tabs" aria-label="${text("작업 영역", "Task sections")}">${tabs.map(([v, k, e]) => link(text(k, e), { detailTab: v }, tab === v ? "active" : "")).join("")}</nav><div class="detail-body">${
      tab === "lifecycle"
        ? activityDetailSection(task)
        : tab === "requirements"
          ? `${
              Object.values(r).some(
                (value) => typeof value === "string" && value.trim(),
              )
                ? Object.entries(r)
                    .filter(([key]) => key !== "acceptance_items")
                    .map(([k, v]) =>
                      section(
                        {
                          background: text("배경·문제", "Background"),
                          goal: text("목표", "Goal"),
                          requirements: text("요구사항", "Requirements"),
                          scope_in: text("포함 범위", "In scope"),
                          scope_out: text("제외 범위", "Out of scope"),
                          constraints: text("제약", "Constraints"),
                          acceptance: text("완료 기준", "Acceptance"),
                        }[k] || k,
                        v,
                      ),
                    )
                    .join("")
                : section(
                    text("작업 원문", "Task source"),
                    task.raw_markdown || d.legacy_description,
                  )
            }`
          : tab === "evidence"
            ? `${executionMetadata(task)}${section(text("작업 메모", "Work notes"), d.notes || task.fields?.메모)}${section(text("실행 근거", "Execution evidence"), d.execution || d.result || d.results || task.execution || task.fields?.결과)}${section(text("검증", "Validation"), d.validation || task.validation || d.verification || task.fields?.검증)}<details class="raw-source" open><summary>${text("원문과 상세 근거", "Source and detailed evidence")}</summary><pre class="source-code">${esc(task.raw_markdown)}</pre></details>`
            : `<section class="summary-sheet"><h2>${text("작업 개요", "Task overview")}</h2><dl>${[
                ["목적", "Purpose", s.purpose || r.goal],
                ["핵심 변경", "Key change", s.change],
                ["상태·결과", "Status / result", s.status_result],
                ["확인·후속", "Checks / follow-up", s.follow_up],
              ]
                .map(
                  ([k, e, v]) =>
                    `<div><dt>${text(k, e)}</dt><dd>${esc(v || text("기록 없음", "Not recorded"))}</dd></div>`,
                )
                .join(
                  "",
                )}</dl></section>${relatedTasks(task)}${task.attention_reason ? section(text("확인이 필요한 근거", "Attention evidence"), typeof task.attention_reason === "string" ? task.attention_reason : JSON.stringify(task.attention_reason)) : ""}<div class="detail-quicklinks">${link(text("요건 읽기", "Read requirements") + icon("arrow"), { detailTab: "requirements" })}${link(text("작업 흐름 확인", "Inspect workflow") + icon("arrow"), { detailTab: "lifecycle" })}</div><details class="raw-source"><summary>${text("작업 원문", "Task source")}</summary><pre class="source-code">${esc(task.raw_markdown)}</pre></details>`
    }</div>`
  );
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
