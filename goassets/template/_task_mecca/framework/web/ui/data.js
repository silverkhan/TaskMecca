import { state } from "./state.js";
export async function get(path, signal) {
  const controller = new AbortController(),
    abort = () => controller.abort();
  if (signal?.aborted) controller.abort();
  else signal?.addEventListener("abort", abort, { once: true });
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => {
      controller.abort();
      reject(new Error("Request timed out. Please retry."));
    }, 20000);
  });
  try {
    const r = await Promise.race([
      fetch(path, { cache: "no-store", signal: controller.signal }),
      timeout,
    ]);
    if (!r.ok) {
      const error = new Error(`HTTP ${r.status}`);
      error.status = r.status;
      throw error;
    }
    const body = await r.json();
    if (body.error) throw Error(body.error);
    return body;
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
  }
}
export const fetchActivity = (id, project, runtimeDiagnostic = false) =>
  get(
    "/api/activity?" +
      new URLSearchParams({
        project,
        task: id,
        ...(runtimeDiagnostic ? { runtime_diagnostic: "1" } : {}),
      }),
  );
export function fetchOverview(signal) {
  const p = new URLSearchParams({ page_size: "8" });
  for (const k of ["attention", "working", "projects", "activity"]) {
    p.set(k + "_page", state.overviewPages[k] || 1);
    if (state.overviewCursors[k])
      p.set(k + "_cursor", state.overviewCursors[k]);
  }
  return get("/api/overview?" + p, signal);
}
export function fetchBacklog(signal) {
  const p = new URLSearchParams({
    project: state.project,
    page: state.page,
    page_size: state.pageSize,
    sort: state.sort,
    projection: "summary",
  });
  if (state.backlog) p.set("backlog", state.backlog);
  if (state.query) p.set("q", state.query);
  if (state.status !== "all") p.set("status", state.status);
  if (state.tags.length) p.set("tags", state.tags.join(","));
  return get("/api/backlog/tasks?" + p, signal);
}
export function fetchTask(signal, projection = "content") {
  return get(
    "/api/tasks/" +
      encodeURIComponent(state.detail) +
      "?" +
      new URLSearchParams({
        project: state.project,
        ...(state.backlog ? { backlog: state.backlog } : {}),
        projection,
      }),
    signal,
  );
}
// Content and canonical lifecycle have separate caches. Foreground navigation joins
// one bounded speculative request; a route abort never cancels another consumer.
const contentCache = new Map(),
  contentPending = new Map(),
  contentVersions = new Map();
let prefetchInFlight = null;
const contentKey = ({ id, project, backlog = "" }) =>
  JSON.stringify([project, backlog, String(id).toUpperCase()]);
export function cachedContent(context) {
  return contentCache.get(contentKey(context))?.task || null;
}
export function invalidateContent(context) {
  if (!context) {
    for (const key of contentPending.keys())
      contentVersions.set(key, (contentVersions.get(key) || 0) + 1);
    contentPending.clear();
    contentCache.clear();
    return;
  }
  const key = contentKey(context);
  contentCache.delete(key);
  contentPending.delete(key);
  contentVersions.set(key, (contentVersions.get(key) || 0) + 1);
}
export async function requestContent(context, { fresh = false } = {}) {
  const key = contentKey(context);
  if (contentPending.has(key)) return contentPending.get(key);
  const cached = contentCache.get(key);
  if (!fresh && cached && Date.now() - cached.at < 30000) return cached.task;
  const p = new URLSearchParams({
    project: context.project,
    projection: "content",
  });
  if (context.backlog) p.set("backlog", context.backlog);
  const version = contentVersions.get(key) || 0;
  const job = get("/api/tasks/" + encodeURIComponent(context.id) + "?" + p)
    .then((task) => {
      if ((contentVersions.get(key) || 0) !== version) return task;
      contentCache.delete(key);
      contentCache.set(key, { task, at: Date.now() });
      while (contentCache.size > 24)
        contentCache.delete(contentCache.keys().next().value);
      return task;
    })
    .finally(() => {
      if (contentPending.get(key) === job) contentPending.delete(key);
    });
  contentPending.set(key, job);
  return job;
}
export function prefetchContent(context) {
  if (prefetchInFlight || cachedContent(context)) return;
  prefetchInFlight = requestContent(context)
    .catch(() => null)
    .finally(() => {
      prefetchInFlight = null;
    });
  return prefetchInFlight;
}
