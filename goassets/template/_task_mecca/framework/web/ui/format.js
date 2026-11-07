import { state } from "./state.js";
export const esc = (v) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
export const text = (ko, en) => (state.language === "ko" ? ko : en);
export const fmtSec = (v) =>
  !Number.isFinite(v) || v < 0
    ? text("미측정", "Unmeasured")
    : v < 60
      ? Math.round(v) + "s"
      : v < 3600
        ? Math.round(v / 60) + "m"
        : (v / 3600).toFixed(1) + "h";
export const count = (v) =>
  v == null ? text("미확인", "Unknown") : Number(v).toLocaleString();
export const date = (v) =>
  !v || !Number.isFinite(Date.parse(v))
    ? text("시각 미확인", "Time unknown")
    : new Date(v).toLocaleString(state.language === "ko" ? "ko-KR" : "en-US", {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
export const status = (v) =>
  ({
    doing: text("진행 중", "Working"),
    working: text("진행 중", "Working"),
    done: text("완료", "Done"),
    hold: text("보류", "On hold"),
    todo: text("대기", "To do"),
    ready: text("준비", "Ready"),
    unknown: text("미확인", "Unknown"),
    stalled: text("실행 확인 필요", "Execution needs checking"),
    blocked: text("막힘", "Blocked"),
    needs_user: text("사용자 확인 필요", "Needs user input"),
    awaiting_finalize: text("완료 처리 대기", "Awaiting finalization"),
    controller_review_pending: text(
      "Controller 검토 대기",
      "Controller review pending",
    ),
    controller_review: text("Controller 검토 중", "Controller review"),
    controller_finalizing: text(
      "Controller 완료 처리 중",
      "Controller finalizing",
    ),
    controller_recovery: text(
      "Controller 복구 필요",
      "Controller recovery needed",
    ),
  })[v] ||
  v ||
  text("미확인", "Unknown");

function inline(value) {
  const source = String(value),
    pattern = /\[([^\]]+)\]\(([^\s)]+)\)/g;
  let result = "",
    last = 0;
  const decorate = (v) =>
    esc(v)
      .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
      .replace(/`([^`]+)`/g, "<code>$1</code>");
  for (const match of source.matchAll(pattern)) {
    result += decorate(source.slice(last, match.index));
    const href = match[2],
      safe = /^(https?:\/\/|\/(?!\/)|#)/i.test(href);
    result += safe
      ? '<a href="' +
        esc(href) +
        '"' +
        (/^https?:/i.test(href)
          ? ' target="_blank" rel="noopener noreferrer"'
          : "") +
        ">" +
        decorate(match[1]) +
        "</a>"
      : decorate(match[0]);
    last = match.index + match[0].length;
  }
  return result + decorate(source.slice(last));
}
export function markdown(value) {
  const lines = String(value || "").split("\n"),
    out = [];
  let code = null,
    list = false;
  const closeList = () => {
    if (list) {
      out.push("</ul>");
      list = false;
    }
  };
  for (const line of lines) {
    if (line.startsWith("```")) {
      closeList();
      if (code !== null) {
        out.push("<pre><code>" + esc(code.join("\n")) + "</code></pre>");
        code = null;
      } else code = [];
      continue;
    }
    if (code !== null) {
      code.push(line);
      continue;
    }
    const heading = line.match(/^(#{1,6})\s+(.+)/),
      bullet = line.match(/^\s*[-*]\s+(.+)/);
    if (bullet) {
      if (!list) {
        out.push("<ul>");
        list = true;
      }
      out.push("<li>" + inline(bullet[1]) + "</li>");
      continue;
    }
    closeList();
    if (heading) {
      const level = Math.min(4, heading[1].length + 1);
      out.push("<h" + level + ">" + inline(heading[2]) + "</h" + level + ">");
    } else if (line.startsWith("> "))
      out.push("<blockquote>" + inline(line.slice(2)) + "</blockquote>");
    else if (line.trim()) out.push("<p>" + inline(line) + "</p>");
  }
  closeList();
  if (code !== null)
    out.push("<pre><code>" + esc(code.join("\n")) + "</code></pre>");
  return out.join("");
}
