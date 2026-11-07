import { connectLiveSource, closeLiveSource } from "./live-source.js";
import {
  configureLifecycle,
  refreshViewedTaskLifecycle,
  stopLifecycleChecks,
  checkContentRevision,
} from "./live-lifecycle.js";
import { state, readRoute, routeURL, persist } from "./state.js";
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
import { backlog } from "./screens/backlog.js";
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
function renderShell() {
  disposeWorkflow();
  document.getElementById("app").innerHTML = shell();
  document.getElementById("refresh").onclick = () => {
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
  appearance();
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
  if (options.preserve && options.readingY == null) options.readingY = scrollY;
  clearTimeout(pollTimer);
  clearTimeout(searchTimer);
  controller?.abort();
  controller = new AbortController();
  const signal = controller.signal,
    current = ++epoch;
  polls = 0;
  renderShell();
  syncWorkflowTimelineEntry();
  stopLifecycleChecks();
  main().innerHTML = `<div class="loading" role="status">${text("정보를 불러오는 중…", "Loading information…")}</div>`;
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
    if (["home", "projects", "activity"].includes(state.view)) {
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
      backlogData = await fetchBacklog(signal);
      if (current !== epoch) return;
      const actualPage = Number(backlogData.page);
      if (
        Number.isInteger(actualPage) &&
        actualPage > 0 &&
        actualPage !== state.page
      ) {
        state.page = actualPage;
        history.replaceState(history.state, "", routeURL());
      }
      if (backlogData.selection?.selected && !state.backlog) {
        state.backlog = backlogData.selection.selected;
        history.replaceState(history.state, "", routeURL());
      }
      main().innerHTML = backlog(backlogData);
      ready(options);
      return;
    }
    if (state.view === "settings") {
      main().innerHTML =
        heading(
          text("환경 설정", "Preferences"),
          text("Smoke · 화면과 언어 설정", "Smoke · Appearance and language"),
        ) +
        `<section class="settings-sheet"><h2>${text("작업 환경", "Your workspace")}</h2><p>${text("상단의 화면 모드와 언어 선택은 모든 핵심 화면에 적용됩니다.", "Appearance and language controls in the top bar apply to every core screen.")}</p><dl><div><dt>${text("테마", "Theme")}</dt><dd>Smoke</dd></div><div><dt>${text("모드", "Mode")}</dt><dd>Light / Dark / System</dd></div></dl></section><section class="stage-note"><h2>${text("후속 검토 범위", "After design review")}</h2><p>${text("전체 운영, 채널·서비스·알림 구성과 추가 설정 화면은 핵심 디자인 검토 후 이어집니다.", "Full operations, channel, service, notification configuration and further settings follow the core design review.")}</p></section>`;
      ready(options);
      return;
    }
    main().innerHTML =
      heading(
        text("알림", "Notifications"),
        text(
          "현재 프로토타입은 관제 홈의 판단 필요와 최근 활동에서 기존 알림 근거를 보여 줍니다.",
          "This prototype shows existing notification evidence through attention and recent activity.",
        ),
      ) +
      link(
        text("판단이 필요한 작업 보기", "View attention"),
        { view: "home" },
        "button",
      ) +
      `<p class="stage-note">${text("알림 전달·설정 화면은 디자인 검토 후 확장합니다.", "Notification delivery and configuration follow design review.")}</p>`;
    ready(options);
  } catch (e) {
    if (e.name === "AbortError" || current !== epoch) return;
    main().innerHTML = failure(e);
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
function bindScreen() {
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
          pageSize: Number(d.get("page_size")),
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
    f.querySelectorAll("select").forEach((select) => (select.onchange = apply));
  }
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
        !document.activeElement?.matches("input,select");
      if (safe) {
        const y = scrollY;
        backlogData = next;
        main().innerHTML = backlog(next);
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
    if (["home", "projects", "activity"].includes(state.view)) {
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
