export function initialPreferences() {
  const read = (key) => {
      try {
        return localStorage.getItem(key);
      } catch {
        return null;
      }
    },
    valid = (value) => ["light", "dark", "system"].includes(value),
    smoke = read("task-mecca-smoke-mode"),
    legacy = read("task-mecca-theme"),
    mode = valid(smoke) ? smoke : valid(legacy) ? legacy : "system";
  if (!valid(smoke) && valid(legacy))
    try {
      localStorage.setItem("task-mecca-smoke-mode", mode);
    } catch {}
  const savedLanguage = read("task-mecca-language"),
    locale = typeof navigator !== "undefined" ? navigator.language : "";
  return {
    mode,
    language: ["ko", "en"].includes(savedLanguage)
      ? savedLanguage
      : locale?.toLowerCase().startsWith("ko")
        ? "ko"
        : "en",
  };
}
const initial = initialPreferences();
export const state = {
  view: "home",
  detail: null,
  project: "",
  backlog: "",
  language: initial.language,
  mode: initial.mode,
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
  toolsOpen: false,
  attentionExpanded: false,
};
export function readRoute() {
  const u = new URL(location.href),
    p = u.searchParams;
  state.view = p.get("view") || (p.get("project") ? "backlog" : "home");
  state.view = ({hub:"home",release:"release-notes",attention:"notifications"})[state.view] || state.view;
  const task = u.pathname.match(/^\/tasks\/([^/]+)/);
  state.detail = task ? decodeURIComponent(task[1]) : null;
  if (task) state.view = "backlog";
  state.project = p.get("project") || "";
  state.backlog = p.get("backlog") || "";
  state.page = Math.max(1, Number(p.get("page")) || 1);
  state.pageSize = [10, 20, 50].includes(Number(p.get("page_size")))
    ? Number(p.get("page_size"))
    : "auto";
  state.query = p.get("q") || "";
  state.toolsOpen =
    typeof history !== "undefined" ? Boolean(history.state?.toolsOpen) : false;
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

export function effectivePageSize() {
  if (state.pageSize !== "auto") return state.pageSize;
  const row = document.querySelector(".backlog-list .task-row"), list = document.querySelector(".backlog-list");
  const rowHeight = row?.getBoundingClientRect().height || (innerWidth <= 760 ? 120 : 92);
  const start = list ? list.getBoundingClientRect().top + scrollY : (innerWidth <= 760 ? 340 : 300);
  return Math.max(1, Math.min(100, Math.floor((innerHeight - Math.min(start, innerHeight / 2) - 100) / rowHeight)));
}
