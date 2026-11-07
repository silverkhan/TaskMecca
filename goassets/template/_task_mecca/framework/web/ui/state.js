const saved = (key, fallback) => {
  try {
    return localStorage.getItem(key) || fallback;
  } catch {
    return fallback;
  }
};
export const state = {
  view: "home",
  detail: null,
  project: "",
  backlog: "",
  language: saved("task-mecca-language", "ko"),
  mode: saved("task-mecca-smoke-mode", "system"),
  page: 1,
  pageSize: 20,
  query: "",
  status: "all",
  tags: [],
  sort: "id_desc",
  overviewPages: {},
  overviewCursors: {},
  detailTab: "overview",
  selected: null,
};
export function readRoute() {
  const u = new URL(location.href),
    p = u.searchParams;
  state.view = p.get("view") || "home";
  const task = u.pathname.match(/^\/tasks\/([^/]+)/);
  state.detail = task ? decodeURIComponent(task[1]) : null;
  if (task) state.view = "backlog";
  state.project = p.get("project") || "";
  state.backlog = p.get("backlog") || "";
  state.page = Math.max(1, Number(p.get("page")) || 1);
  state.pageSize = [10, 20, 50].includes(Number(p.get("page_size")))
    ? Number(p.get("page_size"))
    : 20;
  state.query = p.get("q") || "";
  state.status = p.get("status") || "all";
  state.tags = (p.get("tags") || "").split(",").filter(Boolean);
  state.sort = p.get("sort") || "id_desc";
  state.detailTab =
    p.get("tab") || (location.hash === "#lifecycle" ? "lifecycle" : "overview");
}
export function routeURL(changes = {}) {
  const v = { ...state, ...changes },
    u = new URL("/", location.origin);
  if (v.detail) u.pathname = "/tasks/" + encodeURIComponent(v.detail);
  if (v.view !== "home" && !v.detail) u.searchParams.set("view", v.view);
  for (const k of ["project", "backlog"]) if (v[k]) u.searchParams.set(k, v[k]);
  if (v.view === "backlog") {
    for (const [k, x] of Object.entries({
      page: v.page,
      page_size: v.pageSize,
      q: v.query,
      status: v.status,
      tags: v.tags.join(","),
      sort: v.sort,
    }))
      if (x && x !== "all") u.searchParams.set(k, x);
  }
  if (v.detail && v.detailTab !== "overview")
    u.searchParams.set("tab", v.detailTab);
  return u.pathname + u.search;
}
export function persist(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch {}
}
