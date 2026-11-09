import { bindDocumentTools } from "./document-tools.js";
import { refreshSidebar, bindExpansionShell, paintProjects, sidebarProjects, syncProjectDiagnostics } from "./expansion.js";
import { deliverCanonicalEvents } from "./screens/notifications.js";
import { managedViews, management, bindManagement } from "./screens/management.js";
import { mountExecutionEnvironment, refreshExecutionEnvironment, stopExecutionEnvironment, resumeExecutionEnvironment } from "./environment.js";
import { connectLiveSource, closeLiveSource } from "./live-source.js";
import {
  configureLifecycle,
  refreshViewedTaskLifecycle,
  stopLifecycleChecks,
  checkContentRevision,
} from "./live-lifecycle.js";
import { state, readRoute, routeURL, persist, effectivePageSize } from "./state.js";
import { text, esc } from "./format.js";
import {
  fetchOverview as fetchOverviewRequest,
  fetchBacklog,
  fetchTask,
  get,
  requestContent,
  prefetchContent,
  cachedContent,
  invalidateContent,
} from "./data.js";
import { shell, heading, failure, empty, link } from "./components/shared.js";
import { home } from "./screens/home.js";
import { backlog, backlogResults, filterSummary } from "./screens/backlog.js";
import { detail } from "./screens/detail.js";
import {
  bindActivityDetailControls,
  loadTaskActivity,
  syncWorkflowTimelineEntry,
  disposeWorkflow,
} from "./components/workflow.js";
let controller,
  epoch = 0,
  pollTimer,
  overviewData,
  taskData,
  backlogData,
  searchTimer,
  polls = 0;
const cursorHistory = {
  attention: [],
  working: [],
  projects: [],
  activity: [],
};
const main = () => document.getElementById("main");
const announce = (value) => {
  document.getElementById("announcement").textContent = value;
};
let membershipChanged = false;
async function fetchOverview(signal) {
  try {
    return await fetchOverviewRequest(signal);
  } catch (error) {
    if (error.status !== 409 || signal?.aborted) throw error;
    state.overviewCursors = {};
    state.overviewPages = {};
    for (const key of Object.keys(cursorHistory)) cursorHistory[key] = [];
    membershipChanged = true;
    return fetchOverviewRequest(signal);
  }
}
function appearance() {
  document.documentElement.lang = state.language;
  document.documentElement.dataset.mode = state.mode;
  document.documentElement.dataset.theme =
    state.mode === "system"
      ? matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : state.mode;
}
function saveReading() {
  history.replaceState(
    {
      ...history.state,
      readingY: scrollY,
      toolsOpen: document.querySelector(".filter-more")?.open || false,
      focusID:
        document.activeElement?.closest(".task-row")?.querySelector(".task-id")
          ?.textContent || null,
    },
    "",
  );
}
function navigate(
  url,
  { replace = false, restore = false, preserve = false } = {},
) {
  const input = document.activeElement,
    focus = input?.name
      ? {
          name: input.name,
          start: input.selectionStart,
          end: input.selectionEnd,
        }
      : null,
    y = scrollY,
    fromList = !state.detail && state.view === "backlog" ? location.href : null;
  saveReading();
  if (replace) history.replaceState(history.state || {}, "", url);
  else history.pushState({ fromList }, "", url);
  readRoute();
  render({
    restore,
    preserve,
    readingY: y,
    inputFocus: preserve ? focus : null,
  });
}
let shellLanguage = null,
  renderedBacklogKey = null;
const backlogKey = () =>
  [state.project, state.backlog, state.language].join("|");
function updateShellNavigation() {
  const active = state.view;
  paintProjects();
  document.querySelectorAll(".rail-link").forEach((a) => {
    const view = new URL(a.href).searchParams.get("view") || "home";
    if(!a.closest('.material-project-row')) a.href=routeURL({view,detail:null,...['home','projects','storage','preview','release-notes'].includes(view)?{project:'',backlog:''}:{}});
    const current = view === active && (view !== "backlog" || new URL(a.href).searchParams.get("project") === state.project);
    a.classList.toggle("active", current);
    if (current) a.setAttribute("aria-current", "page");
    else a.removeAttribute("aria-current");
  });
}
function renderShell() {
  disposeWorkflow();
  if (!main() || shellLanguage !== state.language) {
    document.getElementById("app").innerHTML = shell();
    bindExpansionShell();
    void refreshSidebar().then(()=>{ for(const p of sidebarProjects()) void deliverCanonicalEvents(p.path).catch(()=>{}); });
    shellLanguage = state.language;
    document.getElementById("refresh").onclick = () => {
      void refreshExecutionEnvironment();
      void syncProjectDiagnostics(true);
      if (state.detail)
        invalidateContent({
          id: state.detail,
          project: state.project,
          backlog: state.backlog,
        });
      taskData = null;
      render({ preserve: true });
    };
    document.getElementById("language").onclick = () => {
      state.language = state.language === "ko" ? "en" : "ko";
      persist("task-mecca-language", state.language);
      render({ preserve: true });
    };
    document.getElementById("appearance").onchange = (e) => {
      state.mode = e.target.value;
      persist("task-mecca-smoke-mode", state.mode);
      appearance();
    };
  }
  document.getElementById("appearance").value = state.mode;
  window.TaskMeccaSelectUI?.sync();
  updateShellNavigation();
  appearance();
  mountExecutionEnvironment();
}
function syncBacklogControls() {
  const f = document.getElementById("filters");
  if (!f) return;
  for (const [name, value] of Object.entries({
    q: state.query,
    status: state.status,
    sort: state.sort,
    page_size: String(state.pageSize),
  })) {
    const control = f.elements.namedItem(name);
    if (control && control.value !== value) control.value = value;
  document.querySelectorAll('[data-status-filter]').forEach(b => b.setAttribute('aria-pressed',String(state.status.split(',').includes(b.dataset.statusFilter))));
  }
  document.querySelectorAll("[data-tag-filter]").forEach(c=>c.checked=state.tags.includes(c.dataset.tagFilter));
  const tags = document.querySelector("#tags [name=tags]");
  if (tags && document.activeElement !== tags)
    tags.value = state.tags.join(",");
  const more = document.querySelector(".filter-more");
  if (more) more.open = state.toolsOpen;
  const summary = document.getElementById("filterSummary");
  if (summary) summary.textContent = filterSummary();
}
function patchBacklogResults(data) {
  const before = JSON.stringify(backlogData?.tag_options);
  backlogData = data;
  document.getElementById("backlogResults").innerHTML = backlogResults(data);
  if (before !== JSON.stringify(data.tag_options)) {
    const catalog = document.getElementById("tag-options");
    if (catalog)
      catalog.innerHTML = (data.tag_options || [])
        .map(
          (tag) =>
            '<option value="' +
            esc(tag.tag) +
            '">' +
            esc(tag.description || "") +
            "</option>",
        )
        .join("");
  }
  syncBacklogControls();
}
function ready({
  preserve = false,
  restore = false,
  readingY,
  inputFocus,
} = {}) {
  bindScreen();
  connectLiveSource(liveChange, streamFailure, overviewData?.revision || "");
  const wanted = restore
    ? history.state?.readingY || 0
    : preserve
      ? (readingY ?? scrollY)
      : 0;
  requestAnimationFrame(() => {
    scrollTo(0, wanted);
    if (inputFocus) {
      const input = main().querySelector('[name="' + inputFocus.name + '"]');
      input?.focus({ preventScroll: true });
      if (inputFocus.start != null)
        input?.setSelectionRange?.(inputFocus.start, inputFocus.end);
    } else if (restore && history.state?.focusID) {
      const row = [...document.querySelectorAll(".task-row")].find(
        (r) =>
          r.querySelector(".task-id")?.textContent === history.state.focusID,
      );
      row?.querySelector("a")?.focus({ preventScroll: true });
    } else if (
      !preserve &&
      !document.activeElement?.closest(".workflow-floating") &&
      !document.activeElement?.matches("input,select,button,a,summary")
    )
      main()?.focus({ preventScroll: true });
  });
  announce(text("화면을 불러왔습니다", "Screen loaded"));
}
async function render(options = {}) {
  if (
    !Object.hasOwn(options, "preserve") &&
    state.detail &&
    state.detailContext ===
      [state.project, state.backlog, state.detail].join("|")
  )
    options.preserve = true;
  if (options.preserve && options.readingY == null) options.readingY = scrollY;
  const quickBacklog =
    state.view === "backlog" &&
    !state.detail &&
    document.getElementById("backlogResults") &&
    renderedBacklogKey === backlogKey();
  clearTimeout(pollTimer);
  clearTimeout(searchTimer);
  controller?.abort();
  controller = new AbortController();
  const signal = controller.signal,
    current = ++epoch;
  polls = 0;
  if (quickBacklog) {
    syncBacklogControls();
    document.getElementById("listRequestStatus").textContent = text(
      "목록을 확인하고 있습니다…",
      "Updating results…",
    );
    main().setAttribute("aria-busy", "true");
  } else {
    renderShell();
    main().setAttribute("aria-busy", "false");
    main().innerHTML = `<div class="loading" role="status">${text("정보를 불러오는 중…", "Loading information…")}</div>`;
  }
  syncWorkflowTimelineEntry();
  stopLifecycleChecks();
  try {
    if (state.detail) {
      const context = [state.project, state.backlog, state.detail].join("|");
      if (taskData?.context === context) {
        state.detailTask =
          state.detailTask?.id === state.detail &&
          state.detailContext === context
            ? state.detailTask
            : taskData.task;
        state.detailContext = context;
        main().innerHTML = detail(state.detailTask);
        ready(options);
      } else {
        const task = await requestContent({
          id: state.detail,
          project: state.project,
          backlog: state.backlog,
        });
        if (current !== epoch) return;
        taskData = { context, task };
        state.detailTask = task;
        state.detailContext = context;
        state.detailLifecycleError = "";
        state.contentRevision = "";
        state.signalRevision = "";
        main().innerHTML = detail(task);
        ready(options);
      }
      if (state.detailTab === "lifecycle") {
        bindActivityDetailControls(state.detail);
        void loadTaskActivity(state.detail, state.project);
      }
      try {
        await refreshViewedTaskLifecycle(
          state.project,
          state.backlog,
          state.detail,
          { force: true },
        );
      } catch {}
      void checkContentRevision(true).catch(() => {});
      return;
    }
    if (["home", "activity"].includes(state.view)) {
      overviewData = await fetchOverview(signal);
      if (current !== epoch) return;
      showOverview(options);
      schedulePoll(current);
      return;
    }
    if (state.view === "backlog") {
      if (!state.project) {
        main().innerHTML =
          heading(text("프로젝트를 선택하세요", "Choose a project"), "") +
          link(
            text("프로젝트 보기", "View projects"),
            { view: "projects" },
            "button",
          );
        ready(options);
        return;
      }
      const next = await fetchBacklog(signal);
      if (current !== epoch) return;
      const actualPage = Number(next.page);
      if (
        Number.isInteger(actualPage) &&
        actualPage > 0 &&
        actualPage !== state.page
      ) {
        state.page = actualPage;
        history.replaceState(history.state, "", routeURL());
      }
      if (next.selection?.selected && !state.backlog) {
        state.backlog = next.selection.selected;
        history.replaceState(history.state, "", routeURL());
      }
      if (quickBacklog) patchBacklogResults(next);
      else {
        backlogData = next;
        main().innerHTML = backlog(next);
      }
      renderedBacklogKey = backlogKey();
      main().setAttribute("aria-busy", "false");
      document.getElementById("listRequestStatus").textContent = "";
      ready(options);
      return;
    }
    if (managedViews.includes(state.view)) {
      const markup = await management(signal);
      if (current !== epoch) return;
      main().innerHTML = markup;
    } else main().innerHTML = failure(text("알 수 없는 화면입니다", "Unknown screen"));
    ready(options);
  } catch (e) {
    if (e.name === "AbortError" || current !== epoch) return;
    main().setAttribute("aria-busy", "false");
    if (quickBacklog && document.getElementById("listRequestStatus"))
      document.getElementById("listRequestStatus").innerHTML =
        '<div class="notice" role="alert"><span>' +
        esc(
          text(
            "갱신하지 못해 이전 결과를 유지합니다.",
            "Refresh failed. Previous results are retained.",
          ),
        ) +
        " " +
        esc(e.message) +
        '</span><button id="retry">' +
        esc(text("다시 시도", "Retry")) +
        "</button></div>";
    else main().innerHTML = failure(e);
    bindScreen();
    announce(
      text(
        "조회 실패. 다시 시도할 수 있습니다.",
        "Load failed. You can retry.",
      ),
    );
  }
}
function showOverview(options = { preserve: true }) {
  main().innerHTML = home(
    overviewData,
    state.view === "home" ? null : state.view,
  );
  if (membershipChanged) {
    membershipChanged = false;
    const note = document.createElement("p");
    note.className = "notice";
    note.textContent = text(
      "프로젝트 구성이 변경되어 첫 페이지에서 다시 확인했습니다.",
      "Project membership changed. Results restarted from the first page.",
    );
    main().prepend(note);
  }
  ready(options);
}
function schedulePoll(current) {
  if (
    !(overviewData.loading || overviewData.pending) ||
    document.hidden ||
    current !== epoch
  )
    return;
  if (polls >= 12) {
    const note = document.createElement("p");
    note.className = "notice";
    note.textContent = text(
      "자동 확인을 멈췄습니다. 새로고침으로 계속 확인할 수 있습니다.",
      "Automatic checks paused. Refresh to check again.",
    );
    main().prepend(note);
    return;
  }
  pollTimer = setTimeout(
    async () => {
      if (document.hidden || current !== epoch) return;
      polls++;
      try {
        const data = await fetchOverview(controller.signal);
        if (current !== epoch) return;
        overviewData = data;
        showOverview();
        schedulePoll(current);
      } catch (e) {
        if (e.name !== "AbortError") {
          const note = document.createElement("p");
          note.className = "notice";
          note.textContent = text(
            "갱신에 실패했습니다. 새로고침으로 다시 확인하세요.",
            "Refresh failed. Use Refresh to try again.",
          );
          main().prepend(note);
        }
      }
    },
    Math.max(500, Math.min(10000, Number(overviewData.retry_after_ms) || 500)),
  );
}
let measuredAutoSize=null,autoLayoutObserver;
function scheduleAutoSize(){clearTimeout(autoResizeTimer);autoResizeTimer=setTimeout(()=>{if(state.view==='backlog'&&!state.detail&&state.pageSize==='auto'&&effectivePageSize()!==state.autoRequestedSize)void render({preserve:true});},150);}
function bindScreen() {
  bindDocumentTools(main());
  autoLayoutObserver?.disconnect();
  if(state.view==='backlog'&&!state.detail&&state.pageSize==='auto') {
    measuredAutoSize=effectivePageSize();
    autoLayoutObserver=new ResizeObserver(scheduleAutoSize);
    for(const element of [document.querySelector('.workspace'),document.querySelector('.backlog-list')])if(element)autoLayoutObserver.observe(element);
    if(measuredAutoSize!==state.autoRequestedSize)scheduleAutoSize();
  }
  document.querySelectorAll('[data-tag-filter]').forEach(input=>input.onchange=()=>{const values=new Set(state.tags);if(input.checked)values.add(input.dataset.tagFilter);else values.delete(input.dataset.tagFilter);navigate(routeURL({tags:[...values],page:1}),{replace:true,preserve:true});});
  document.getElementById('backlog-location')?.addEventListener('change',e => { persist('task-mecca-backlog-folder',e.target.value); navigate(routeURL({backlog:e.target.value,page:1}),{replace:true,preserve:true}); });
  document.querySelectorAll('[data-status-filter]').forEach(b => b.onclick = () => {
    const value=b.dataset.statusFilter, current=new Set(state.status==='all'?[]:state.status.split(','));
    if(value==='all') current.clear(); else if(current.has(value)) current.delete(value); else current.add(value);
    navigate(routeURL({status:[...current].join(',')||'all',page:1}),{replace:true,preserve:true});
  });
  if (managedViews.includes(state.view) && !state.detail) bindManagement(render);
  const attentionMore = document.getElementById("attentionMore");
  if (attentionMore)
    attentionMore.ontoggle = () => {
      if (attentionMore.isConnected)
        state.attentionExpanded = attentionMore.open;
    };
  document.querySelectorAll(".task-row a[data-route]").forEach((a) => {
    const prefetch = () => {
      const u = new URL(a.href),
        id = decodeURIComponent(u.pathname.split("/").pop());
      prefetchContent({
        id,
        project: u.searchParams.get("project") || "",
        backlog: u.searchParams.get("backlog") || "",
      });
    };
    a.addEventListener("pointerenter", prefetch, { once: true });
    a.addEventListener("focus", prefetch, { once: true });
  });
  document.getElementById("retry")?.addEventListener("click", () => render());
  document.querySelectorAll("[data-page]").forEach(
    (button) =>
      (button.onclick = () => {
        const [key, direction] = button.dataset.page.split(":"),
          delta = direction === "next" ? 1 : -1;
        if (key === "backlog") {
          navigate(
            routeURL({ page: Math.max(1, state.page + delta), detail: null }),
          );
          return;
        }
        if (key === "attention") state.attentionExpanded = false;
        const section = overviewData.sections[key];
        if (delta > 0) {
          cursorHistory[key].push(state.overviewCursors[key] || "");
          state.overviewCursors[key] = section.next_cursor || "";
        } else state.overviewCursors[key] = cursorHistory[key].pop() || "";
        state.overviewPages[key] = Math.max(
          1,
          (state.overviewPages[key] || 1) + delta,
        );
        render({ preserve: true });
      }),
  );
  const f = document.getElementById("filters");
  if (f) {
    const apply = () => {
      const d = new FormData(f);
      navigate(
        routeURL({
          query: d.get("q").trim(),
          status: d.get("status"),
          sort: d.get("sort"),
          pageSize: d.get("page_size") === "auto" ? "auto" : Number(d.get("page_size")),
          page: 1,
          detail: null,
        }),
        { replace: true, preserve: true },
      );
    };
    f.onsubmit = (e) => {
      e.preventDefault();
      clearTimeout(searchTimer);
      apply();
    };
    f.querySelector("[name=q]").oninput = () => {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(apply, 400);
    };
    document
      .querySelectorAll('select[form="filters"]')
      .forEach((select) => (select.onchange = () => {
        if(select.name==='page_size'){persist('task-mecca-list-page-mode-v1',select.value==='auto'?'auto':'manual');if(select.value!=='auto')persist('task-mecca-list-page-size',select.value);}
        if(select.name==='sort')persist('task-mecca-list-sort',select.value);
        apply();
      }));
  }
  const more = document.querySelector(".filter-more");
  if (more)
    more.ontoggle = () => {
      if (more.isConnected) state.toolsOpen = more.open;
    };
  const tags = document.getElementById("tags");
  if (tags)
    tags.onsubmit = (e) => {
      e.preventDefault();
      navigate(
        routeURL({
          tags: new FormData(tags)
            .get("tags")
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean),
          page: 1,
          detail: null,
        }),
        { replace: true, preserve: true },
      );
    };
}
document.addEventListener("focusin", (e) => {
  const row = e.target.closest(".task-row");
  if (row) {
    state.selected = row.dataset.id;
    document
      .querySelectorAll(".task-row")
      .forEach((r) => r.classList.toggle("keyboard-selected", r === row));
  }
});
document.addEventListener("keydown", (e) => {
  if (
    e.defaultPrevented ||
    e.metaKey ||
    e.ctrlKey ||
    e.altKey ||
    e.target.closest(
      "input,select,textarea,button,summary,#workflowTimelineViewport,.workflow-floating",
    )
  )
    return;
  if (e.key === "/" && !state.detail && state.view === "backlog") {
    e.preventDefault();
    document.querySelector("[name=q]")?.focus();
    return;
  }
  if ((e.key === "Escape" || e.key === "ArrowLeft") && state.detail) {
    e.preventDefault();
    document.getElementById("backBtn")?.click();
    return;
  }
  if (state.view !== "backlog" || state.detail) return;
  const rows = [...document.querySelectorAll(".task-row")];
  if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    const current = rows.findIndex((r) => r.dataset.id === state.selected),
      index = Math.max(
        0,
        Math.min(rows.length - 1, current + (e.key === "ArrowDown" ? 1 : -1)),
      );
    rows[index]?.querySelector("a")?.focus();
    rows[index]?.scrollIntoView({ block: "nearest" });
  }
  if (e.key === "PageDown" || e.key === "PageUp") {
    const button = document.querySelector(
      '[data-page="backlog:' + (e.key === "PageDown" ? "next" : "prev") + '"]',
    );
    if (button && !button.disabled) {
      e.preventDefault();
      button.click();
    }
  }
});
document.addEventListener("click", (e) => {
  const a = e.target.closest("a[data-route]");
  if (!a || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey)
    return;
  e.preventDefault();
  if (a.id === "backBtn" && history.state?.fromList) {
    history.back();
    return;
  }
  navigate(a.href);
});
window.addEventListener("popstate", () => {
  readRoute();
  render({ restore: true });
});
document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    clearTimeout(pollTimer);
    stopLifecycleChecks();
    closeLiveSource();
  } else {
    void refreshExecutionEnvironment();
      void syncProjectDiagnostics(true);
    connectLiveSource(liveChange, streamFailure, overviewData?.revision || "");
    if (state.detail) void checkContentRevision().catch(() => {});
  }
});
window.addEventListener("focus", () => {
  if (state.detail) void checkContentRevision().catch(() => {});
});
window.addEventListener("task-mecca:revision", () => {
  if (state.detail) void checkContentRevision().catch(() => {});
});
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  if (state.mode === "system") appearance();
});
window.addEventListener("beforeunload", saveReading);
window.addEventListener("pagehide", () => {
  stopExecutionEnvironment();
  clearTimeout(pollTimer);
  controller?.abort();
  disposeWorkflow();
  closeLiveSource();
});
let sourceRefresh = null;
function streamFailure() {
  if (document.getElementById("streamStatus")) return;
  const note = document.createElement("p");
  note.id = "streamStatus";
  note.className = "notice";
  note.textContent = text(
    "실시간 연결이 중단되었습니다. 새로고침으로 다시 연결하세요.",
    "Live connection paused. Refresh to reconnect.",
  );
  main()?.prepend(note);
}
async function liveChange() {
  if (sourceRefresh || document.hidden) return;
  const current = epoch;
  for(const p of sidebarProjects()) void deliverCanonicalEvents(p.path,controller?.signal).catch(()=>{});
  void syncProjectDiagnostics(true);
  sourceRefresh = (async () => {
    if (state.detail) {
      await checkContentRevision();
      return;
    }
    if (state.view === "backlog") {
      const next = await fetchBacklog(controller.signal);
      if (current !== epoch) return;
      if (JSON.stringify(next) === JSON.stringify(backlogData)) return;
      const safe =
        state.page === 1 &&
        !state.query &&
        !state.tags.length &&
        state.status === "all" &&
        !getSelection()?.toString() &&
        !document.activeElement?.matches("input,select") &&
        !document.activeElement?.closest(".task-row");
      if (safe) {
        const y = scrollY;
        patchBacklogResults(next);
        ready({ preserve: true, readingY: y });
      } else if (!document.getElementById("listUpdateNotice")) {
        const notice = document.createElement("div");
        notice.id = "listUpdateNotice";
        notice.className = "notice";
        notice.innerHTML =
          "<span>" +
          esc(
            text(
              "작업 목록이 변경되었습니다. 현재 읽기 위치를 유지합니다.",
              "The task list changed. Your current reading is preserved.",
            ),
          ) +
          "</span><button>" +
          esc(text("목록 갱신", "Update list")) +
          "</button>";
        notice.querySelector("button").onclick = () =>
          render({ preserve: true });
        main().prepend(notice);
      }
      return;
    }
    if (managedViews.includes(state.view)) {
      if(!document.getElementById('managementUpdateNotice')) {
        const notice=document.createElement('div');notice.id='managementUpdateNotice';notice.className='notice';
        const label=document.createElement('span');label.textContent=text('기록이 변경되었습니다. 현재 입력과 읽기 위치를 유지합니다.','Records changed. Your current input and reading are preserved.');
        const update=document.createElement('button');update.textContent=text('새 기록 확인','Load current records');update.onclick=()=>render({preserve:true});notice.append(label,update);main().prepend(notice);
      }
      return;
    }
    if (["home", "activity"].includes(state.view)) {
      const next = await fetchOverview(controller.signal);
      if (current !== epoch) return;
      const fingerprint = (d) =>
        JSON.stringify({
          sections: d.sections,
          counts: d.counts,
          status: d.status,
          errors: d.errors,
          loading: d.loading,
          pending: d.pending,
        });
      if (fingerprint(next) !== fingerprint(overviewData)) {
        overviewData = next;
        showOverview({ preserve: true, readingY: scrollY });
        schedulePoll(current);
      }
    }
  })()
    .catch((e) => {
      if (e.name !== "AbortError") streamFailure();
    })
    .finally(() => {
      sourceRefresh = null;
    });
  return sourceRefresh;
}
configureLifecycle(() => {
  taskData = null;
  render({ preserve: true });
});
readRoute();
appearance();
render({ restore: true });
export { render };

window.addEventListener("pageshow", (event) => { if (event.persisted) void resumeExecutionEnvironment(); });

let autoResizeTimer;
addEventListener('resize',scheduleAutoSize,{passive:true});
