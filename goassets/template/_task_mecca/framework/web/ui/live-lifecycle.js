import { state } from "./state.js";
import { esc, text } from "./format.js";
import { get, invalidateContent } from "./data.js";
import {
  detailLifecycleBody,
  lifecycleSnapshotNotice,
} from "./components/lifecycle.js";
import { loadTaskActivity } from "./components/workflow.js";
let contentRefresh = () => {};
export function configureLifecycle(callback) {
  contentRefresh = callback;
}
function patchLifecycleEvidence(section, task) {
  const target =
    section.querySelector?.("#workflowLifecycleEvidenceBody") || section;
  target.innerHTML = detailLifecycleBody(task);
  target
    .querySelector?.("#lifecycleRetryBtn")
    ?.addEventListener("click", retryTaskLifecycle);
}
function refreshVisibleLifecycleNotice() {
  const target = document.getElementById?.("detailLifecycleNotice");
  if (target && state.detailTask) {
    target.innerHTML = lifecycleSnapshotNotice(state.detailTask);
    target
      .querySelector?.("#lifecycleRetryBtn")
      ?.addEventListener("click", retryTaskLifecycle);
  }
}
function refreshVisibleLifecycleSection() {
  refreshVisibleLifecycleNotice();
  const section = document.getElementById?.("lifecycle");
  if (section && state.detailTask)
    patchLifecycleEvidence(section, state.detailTask);
}
export function retryTaskLifecycle() {
  return refreshViewedTaskLifecycle(
    state.project,
    state.backlog,
    state.detail,
    { force: true },
  );
}
export function stopLifecycleChecks() {
  lifecycleSnapshotRetrySequence++;
  viewedLifecycleRequestSequence++;
}
function taskTimingNow() {
  return typeof performance !== "undefined" && performance.now
    ? performance.now()
    : Date.now();
}
function recordTaskTiming(phase, url, started, response) {
  const end = taskTimingNow(),
    serverTiming = response?.headers?.get?.("Server-Timing") || "";
  const resourceURL =
    typeof URL !== "undefined" &&
    typeof location !== "undefined" &&
    location.href
      ? new URL(url, location.href).href
      : url;
  const entries =
    typeof performance !== "undefined"
      ? performance.getEntriesByName?.(resourceURL) || []
      : [];
  const resource = entries[entries.length - 1];
  const timing = {
    phase,
    request_url: url,
    route:
      typeof location !== "undefined" && location.hostname?.endsWith(".ts.net")
        ? "tailscale"
        : typeof location !== "undefined" &&
            ["localhost", "127.0.0.1", "::1"].includes(location.hostname)
          ? "localhost"
          : "other",
    at: new Date().toISOString(),
    browser_ms: end - started,
    ttfb_ms:
      resource && resource.responseStart
        ? resource.responseStart - resource.requestStart
        : null,
    transfer_ms:
      resource && resource.responseEnd
        ? resource.responseEnd - resource.responseStart
        : null,
    server_timing: serverTiming,
  };
  state.taskLoadTimings = [...(state.taskLoadTimings || []), timing].slice(-50);
  if (typeof performance !== "undefined")
    try {
      performance.measure("task-mecca:" + phase, { start: started, end });
    } catch (_) {}
  return timing;
}
function lifecycleVisualFingerprint(value) {
  const lifecycle = value || {};
  const events = (lifecycle.events || []).map((event) => {
    const copy = { ...event };
    delete copy.interval; // live duration ticks already update locally
    return copy;
  });
  return JSON.stringify({
    events,
    current_state: lifecycle.current_state || "",
    current_state_at: lifecycle.current_state_at || "",
    lifecycle_inferred: Boolean(lifecycle.lifecycle_inferred),
    timing_incomplete: Boolean(lifecycle.timing_incomplete),
  });
}

let viewedLifecycleLastFetchAt = 0,
  lifecycleSnapshotRetrySequence = 0,
  viewedLifecycleRequestSequence = 0;
function scheduleLifecycleSnapshotCheck(project, backlog, id, budget = 5) {
  if (budget <= 0 || typeof setTimeout !== "function") return;
  const sequence = ++lifecycleSnapshotRetrySequence;
  setTimeout(() => {
    if (
      document.visibilityState === "hidden" ||
      sequence !== lifecycleSnapshotRetrySequence ||
      state.view !== "backlog" ||
      state.project !== project ||
      state.backlog !== backlog ||
      state.detail !== id ||
      !state.detailTask
    )
      return;
    refreshViewedTaskLifecycle(project, backlog, id, {
      force: true,
      retryBudget: budget - 1,
    }).catch(() => {});
  }, 1200);
}
async function refreshViewedTaskLifecycle(project, backlog, id, options = {}) {
  if (
    state.view !== "backlog" ||
    state.detail !== id ||
    state.project !== project ||
    state.backlog !== backlog ||
    !state.detailTask
  )
    return false;
  if (typeof document !== "undefined" && document.visibilityState === "hidden")
    return false;
  const now = Date.now();
  if (!options.force && now - viewedLifecycleLastFetchAt < 10000) return false;
  viewedLifecycleLastFetchAt = now;
  const old = state.detailTask;
  const params = new URLSearchParams({ project, projection: "lifecycle" });
  if (backlog) params.set("backlog", backlog);
  const timingStarted = taskTimingNow(),
    timingURL =
      "/api/tasks/" + encodeURIComponent(id) + "?" + params.toString();
  const requestSequence = ++viewedLifecycleRequestSequence;
  let response, payload;
  try {
    response = await fetchWithDeadline(timingURL, { cache: "no-store" });
    if (!response.ok) throw new Error("Could not load lifecycle projection");
    payload = await response.json();
  } catch (error) {
    if (
      requestSequence === viewedLifecycleRequestSequence &&
      state.detail === id &&
      state.project === project &&
      state.backlog === backlog
    ) {
      state.detailLifecycleError = String(error?.message || error);
      refreshVisibleLifecycleSection();
      if (options.errorRetryBudget !== 0) {
        const sequence = ++lifecycleSnapshotRetrySequence;
        setTimeout(() => {
          if (
            sequence === lifecycleSnapshotRetrySequence &&
            state.project === project &&
            state.backlog === backlog &&
            state.detail === id &&
            document.visibilityState !== "hidden"
          )
            void refreshViewedTaskLifecycle(project, backlog, id, {
              force: true,
              retryBudget: 0,
              errorRetryBudget: 0,
            }).catch(() => {});
        }, 1200);
      }
    }
    throw error;
  }
  if (
    requestSequence !== viewedLifecycleRequestSequence ||
    state.view !== "backlog" ||
    state.detail !== id ||
    state.project !== project ||
    state.backlog !== backlog ||
    !state.detailTask ||
    state.detailTask.raw_markdown !== old.raw_markdown
  )
    return false;
  const before = lifecycleVisualFingerprint(state.detailTask.lifecycle);
  const after = lifecycleVisualFingerprint(payload.lifecycle);
  const oldNotice = JSON.stringify({
    trust: state.detailTask.snapshot?.trust,
    error: state.detailLifecycleError || "",
  });
  const newer = {
    ...state.detailTask,
    ...payload,
    raw_markdown: state.detailTask.raw_markdown,
    document: state.detailTask.document,
    content_only: false,
    lifecycle: payload.lifecycle || {},
    activity: payload.activity || {},
  };
  if (payload.snapshot?.trust === "pending" || payload.not_found) {
    state.detailTask = {
      ...state.detailTask,
      snapshot: { ...payload.snapshot, trust: "stale" },
    };
    refreshVisibleLifecycleSection();
    recordTaskTiming("lifecycle:refresh", timingURL, timingStarted, response);
    scheduleLifecycleSnapshotCheck(
      project,
      backlog,
      id,
      options.retryBudget ?? 5,
    );
    return false;
  }
  state.detailLifecycleError = "";
  state.detailTask = newer;
  recordTaskTiming("lifecycle:refresh", timingURL, timingStarted, response);
  if (payload.snapshot?.trust === "stale")
    scheduleLifecycleSnapshotCheck(
      project,
      backlog,
      id,
      options.retryBudget ?? 5,
    );
  else lifecycleSnapshotRetrySequence++;
  refreshVisibleLifecycleNotice();
  if (
    before === after &&
    oldNotice === JSON.stringify({ trust: newer.snapshot?.trust, error: "" })
  )
    return true;
  const section =
    typeof document !== "undefined"
      ? document.getElementById?.("lifecycle")
      : null;
  if (section && !state.raw) {
    const rect = section.getBoundingClientRect?.();
    const previouslyAbove = rect && rect.bottom <= 0;
    const oldHeight = rect?.height || 0;
    const previousScroll = typeof window !== "undefined" ? window.scrollY : 0;
    // Patch only lifecycle children. Keep section identity for TOC observer,
    // preserve Markdown, open disclosures, focus and reading position.
    patchLifecycleEvidence(section, newer);
    if (section.querySelector?.("#activityMetricsBody"))
      void loadTaskActivity(id, project);
    if (previouslyAbove && typeof window !== "undefined") {
      const delta = (section.getBoundingClientRect?.().height || 0) - oldHeight;
      if (delta)
        window.scrollTo?.({
          left: window.scrollX,
          top: previousScroll + delta,
          behavior: "auto",
        });
    }
  }
  return true;
}

// Bound foreground and background network calls. A request left pending while
// a laptop sleeps must not permanently hold the single-flight refresh gates.
async function fetchWithDeadline(url, options = {}, timeoutMs = 20000) {
  const controller = new AbortController();
  const upstream = options.signal;
  const onAbort = () => controller.abort();
  if (upstream?.aborted) controller.abort();
  else upstream?.addEventListener?.("abort", onAbort, { once: true });
  let timer;
  try {
    return await Promise.race([
      fetch(url, { ...options, signal: controller.signal }),
      new Promise((_, reject) => {
        timer = setTimeout(() => {
          controller.abort();
          reject(new Error("Request timed out; retrying on the next refresh"));
        }, timeoutMs);
      }),
    ]);
  } finally {
    clearTimeout(timer);
    upstream?.removeEventListener?.("abort", onAbort);
  }
}

let revisionCheckInFlight = null;
export async function checkContentRevision(establishOnly = false) {
  if (
    !state.project ||
    !state.detail ||
    !state.detailTask ||
    document.visibilityState === "hidden"
  )
    return;
  if (revisionCheckInFlight) return revisionCheckInFlight;
  const { project, backlog, detail: id } = state,
    old = state.detailTask;
  const stillCurrent = () =>
    state.view === "backlog" &&
    state.project === project &&
    state.backlog === backlog &&
    state.detail === id &&
    state.detailTask?.raw_markdown === old.raw_markdown;
  revisionCheckInFlight = (async () => {
    const p = new URLSearchParams({ project });
    if (backlog) p.set("backlog", backlog);
    const r = await fetchWithDeadline("/api/revision?" + p, {
      cache: "no-store",
    });
    if (!r.ok) return;
    const body = await r.json();
    if (!stillCurrent() || !body.revision) return;
    if (!state.contentRevision || establishOnly) {
      state.contentRevision = body.revision;
      state.signalRevision = body.signal_revision;
      return;
    }
    const fingerprint = body.revision + "|" + body.signal_revision;
    if (
      body.revision !== state.contentRevision &&
      state.pendingObservedRevision !== fingerprint
    ) {
      const content = await get(
        "/api/tasks/" +
          encodeURIComponent(id) +
          "?" +
          new URLSearchParams({ project, backlog, projection: "content" }),
      );
      if (!stillCurrent()) return;
      if (
        String(content.raw_markdown || "") !== String(old.raw_markdown || "")
      ) {
        state.pendingContentUpdate = true;
        state.pendingContentReason = "content";
        state.pendingObservedRevision = fingerprint;
        showContentUpdate();
      } else {
        invalidateContent({ id, project, backlog });
        state.contentRevision = body.revision;
      }
    }
    if (
      body.signal_revision !== state.signalRevision ||
      (!state.pendingContentUpdate &&
        Date.now() - viewedLifecycleLastFetchAt >= 10000)
    ) {
      try {
        const updated = await refreshViewedTaskLifecycle(project, backlog, id, {
          force: body.signal_revision !== state.signalRevision,
        });
        if (updated && stillCurrent())
          state.signalRevision = body.signal_revision;
      } catch {}
    }
  })().finally(() => {
    revisionCheckInFlight = null;
  });
  return revisionCheckInFlight;
}
function showContentUpdate() {
  if (document.getElementById?.("contentUpdateNotice")) return;
  const main = document.getElementById?.("main");
  if (!main?.prepend) return;
  const notice = document.createElement("div");
  notice.id = "contentUpdateNotice";
  notice.className = "notice";
  notice.setAttribute("role", "status");
  notice.innerHTML =
    "<span>" +
    esc(
      text(
        "작업 원문이 변경되었습니다. 읽던 내용은 유지합니다.",
        "Task source changed. Your current reading is preserved.",
      ),
    ) +
    '</span><button id="contentRefreshBtn">' +
    esc(text("최신 본문 읽기", "Read latest source")) +
    "</button>";
  main.prepend(notice);
  notice.querySelector("button").onclick = () => {
    invalidateContent({
      id: state.detail,
      project: state.project,
      backlog: state.backlog,
    });
    state.pendingContentUpdate = false;
    state.pendingObservedRevision = "";
    contentRefresh();
  };
}
export {
  lifecycleVisualFingerprint,
  refreshViewedTaskLifecycle,
  scheduleLifecycleSnapshotCheck,
  recordTaskTiming,
  fetchWithDeadline,
};
