import { state } from "../state.js";
import { esc, date, status, fmtSec, text } from "../format.js";
export function detailLifecycleBody(task) {
  const lc = task.lifecycle,
    events = lc?.events || [];
  return `${lifecycleSnapshotNotice(task)}${task.file_state === "doing" && task.agent ? `<p class="muted">${text("현재 담당: ", "Current assignee: ")}<strong>${esc(task.agent)}</strong> · ${text("아래는 과거 담당자와 기록 주체를 포함한 전체 이력입니다.", "History below includes former assignees and event authors.")}</p>` : ""}${!lc ? `<p class="muted">${text("라이프사이클 미확인. 본문 상태와 실행 기록은 다를 수 있습니다.", "Lifecycle unknown. File state may differ from observed execution.")}</p>` : ""}${events.length ? `<ol class="event-list">${events.map((e) => `<li><time>${esc(date(e.at))}</time><div><strong>${esc(lifecycleEventLabel(e.label) || status(e.state))}</strong><p>${esc(lifecycleEvidenceLabel(e) || text("주체 미확인", "Actor unknown"))}${e.provisional ? " · " + text("잠정", "Provisional") : ""}</p>${e.interval ? `<span>${esc(e.interval)}</span>` : ""}</div></li>`).join("")}</ol>` : `<p class="muted">${text("관측된 사건 기록이 없습니다.", "No observed lifecycle events.")}</p>`}<p class="muted">${text("리드 타임", "Lead time")}: ${esc(fmtSec(lc?.lead_seconds))}</p>`;
}

export function lifecycleEventLabel(label) {
  return (
    {
      Registered: text("등록", "Registered"),
      Assigned: text("배정", "Assigned"),
      Started: text("착수", "Started"),
      "Controller review started": text(
        "Controller 검토 시작",
        "Controller review started",
      ),
      Waiting: text("대기", "Waiting"),
      Resumed: text("재개", "Resumed"),
      Hold: text("보류", "Hold"),
      Completed: text("완료", "Completed"),
    }[label] || label
  );
}
export function lifecycleEvidenceLabel(event) {
  const evidence = [];
  if (event.source === "git") evidence.push(text("Git 이력", "Git history"));
  else if (
    event.source &&
    !["durable_lifecycle", "execution_ledger"].includes(event.source)
  )
    evidence.push(event.source);
  if (event.assigned_worker)
    evidence.push(
      text("당시 배정: ", "Assigned then: ") + event.assigned_worker,
    );
  if (event.actor)
    evidence.push(text("기록 주체: ", "Recorded actor: ") + event.actor);
  if (event.kind === "started" && event.actor === "/root/controller")
    evidence.push(
      text("착수 Worker 식별 미확인", "Started Worker identity unverified"),
    );
  if (event.event_id) evidence.push("ID " + event.event_id);
  return evidence.join(" · ");
}
export function lifecycleSnapshotNotice(task) {
  const trust = task.snapshot?.trust,
    retry =
      '<button id="lifecycleRetryBtn">' +
      esc(text("다시 확인", "Retry")) +
      "</button>";
  if (state.detailLifecycleError || trust === "error")
    return (
      '<div class="notice" role="alert"><span>' +
      esc(
        text(
          "최신 라이프사이클을 확인하지 못했습니다. 이전 확인 결과를 유지합니다.",
          "Could not verify the latest lifecycle. Showing the previous snapshot.",
        ),
      ) +
      "</span>" +
      retry +
      "</div>"
    );
  if (trust === "pending")
    return (
      '<p class="notice" role="status">' +
      esc(
        text(
          "라이프사이클 근거를 확인하고 있습니다.",
          "Checking lifecycle evidence.",
        ),
      ) +
      "</p>"
    );
  if (trust === "stale")
    return (
      '<div class="notice" role="status"><span>' +
      esc(
        text(
          "이전 확인 결과입니다. 최신 상태 확인이 필요합니다.",
          "This is an earlier snapshot. Check again for the latest state.",
        ),
      ) +
      "</span>" +
      retry +
      "</div>"
    );
  return "";
}
