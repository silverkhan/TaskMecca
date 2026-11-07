// Public, isolated synthetic API. Never reads operating projects or ledgers.
const fs = require("node:fs"),
  path = require("node:path"),
  http = require("node:http");
const workflow = require("./workflow_fixture.cjs");
async function start(root) {
  const data = workflow.payload(),
    task = workflow.task(data),
    projects = [
      {
        name: "Task Mecca",
        path: "/fixtures/task-mecca",
        status: "ok",
        counts: { all: 128, working: 6, attention: 2, hold: 1, done: 99 },
      },
      {
        name: "Research Board",
        path: "/fixtures/research",
        status: "ok",
        counts: { all: 24, working: 3, attention: 1, hold: 0, done: 12 },
      },
      {
        name: "Archive",
        path: "/fixtures/archive",
        status: "missing",
        counts: null,
      },
    ];
  const rows = Array.from({ length: 128 }, (_, i) => ({
    ...task,
    id: "FLOW-" + (i + 1),
    title:
      i === 0
        ? "관제 홈과 작업 흐름을 하나로 연결"
        : i === 1
          ? "검증 결과를 확인하고 다음 작업 판단"
          : `프로젝트 작업 기록 ${i + 1}`,
    state: i < 6 ? "doing" : i < 8 ? "hold" : "done",
    tags: i % 2 ? ["design"] : ["frontend", "core"],
    project: i % 3 ? projects[0].path : projects[1].path,
    project_name: i % 3 ? projects[0].name : projects[1].name,
    backlog: workflow.backlog,
    updated_at: "2026-11-08T09:30:00+09:00",
  }));
  const state = {
    data,
    rows,
    projects,
    requests: [],
    loadingResponses: 0,
    failOverview: false,
    rejectOverviewCursorOnce: false,
    activityStatus: 200,
    streams: new Set(),
    revision: "fixture-v1",
    signal_revision: "signal-v1",
  };
  state.emit = () => {
    for (const stream of state.streams)
      stream.res.write(
        "event: " +
          stream.event +
          "\ndata: " +
          JSON.stringify({
            revision: state.revision,
            signal_revision: state.signal_revision,
            attention: [],
            notification_events: [],
          }) +
          "\n\n",
      );
  };
  const server = http.createServer((req, res) => {
    const u = new URL(req.url, "http://127.0.0.1");
    state.requests.push(u.pathname + u.search);
    res.setHeader("Cache-Control", "no-store");
    if (u.pathname === "/api/events" || u.pathname === "/api/overview/events") {
      res.setHeader("Content-Type", "text/event-stream");
      res.write(": ready\n\n");
      const stream = {
        res,
        event: u.pathname.includes("overview") ? "overview" : "attention",
      };
      state.streams.add(stream);
      req.on("close", () => state.streams.delete(stream));
      return;
    }
    if (u.pathname.startsWith("/api/")) {
      res.setHeader("Content-Type", "application/json");
      let value = {};
      if (u.pathname === "/api/revision")
        value = {
          revision: state.revision,
          signal_revision: state.signal_revision,
          states: rows,
        };
      else if (u.pathname === "/api/overview") {
        if (
          state.rejectOverviewCursorOnce &&
          [...u.searchParams.keys()].some((k) => k.endsWith("_cursor"))
        ) {
          state.rejectOverviewCursorOnce = false;
          res.writeHead(409);
          res.end(
            JSON.stringify({
              error: "overview registry changed; restart pagination",
            }),
          );
          return;
        }
        if (state.failOverview) {
          res.writeHead(503);
          res.end(JSON.stringify({ error: "Synthetic source unavailable" }));
          return;
        }
        const loading = state.loadingResponses-- > 0,
          sections = {};
        for (const name of ["attention", "working", "projects", "activity"]) {
          const page = Number(u.searchParams.get(name + "_page")) || 1,
            size = Number(u.searchParams.get("page_size")) || 8;
          let items =
            name === "projects"
              ? projects
              : name === "attention"
                ? rows.slice(0, 2).map((r, i) => ({
                    ...r,
                    reason: {
                      message: i
                        ? "검증 근거 확인 필요"
                        : "사용자 검토를 기다리고 있습니다",
                    },
                  }))
                : name === "working"
                  ? rows.slice(2, 10).map((r, i) => ({
                      ...r,
                      state: "doing",
                      state_provisional: i === 7,
                      evidence: {
                        source: i === 7 ? "backlog_file" : "lifecycle",
                      },
                    }))
                  : rows.slice(0, 11).map((r, i) => ({
                      id: "event-" + i,
                      task_id: r.id,
                      project: r.project,
                      title: `${r.id} · 관측된 작업 기록`,
                      at: r.updated_at,
                      source: "notification_events",
                    }));
          const total = items.length;
          sections[name] = {
            items: loading ? [] : items.slice((page - 1) * size, page * size),
            page,
            page_size: size,
            total: loading ? null : total,
            pages: loading ? null : Math.ceil(total / size),
            has_more: page * size < total,
            status: loading ? "partial" : "ok",
            coverage: {
              complete: !loading,
              scanned_projects: loading ? 0 : 3,
              total_projects: 3,
            },
            observed_at: loading ? null : "2026-11-08T09:30:00+09:00",
            ...(page * size < total
              ? { next_cursor: "opaque-" + name + "-" + (page + 1) }
              : {}),
          };
        }
        value = {
          schema_version: 1,
          revision: state.revision,
          generated_at: new Date().toISOString(),
          status: "partial",
          freshness: {
            observed_at: loading ? null : "2026-11-08T09:30:00+09:00",
            stale: false,
          },
          counts: {
            projects: 3,
            attention: loading ? null : 2,
            working: loading ? null : 8,
          },
          sections,
          errors: [
            { project: projects[2].path, message: "Project path unavailable" },
          ],
          loading,
          retry_after_ms: 500,
        };
      } else if (u.pathname === "/api/backlog/tasks") {
        const q = u.searchParams.get("q") || "",
          status = u.searchParams.get("status"),
          tags = (u.searchParams.get("tags") || "").split(",").filter(Boolean),
          page = Number(u.searchParams.get("page")) || 1,
          size = Number(u.searchParams.get("page_size")) || 20;
        let items = rows.filter(
          (r) =>
            (!q || [r.id, r.title].join(" ").includes(q)) &&
            (!status || status.split(",").includes(r.state)) &&
            tags.every((t) => r.tags.includes(t)),
        );
        if (u.searchParams.get("sort") === "id_desc")
          items = [...items].reverse();
        value = {
          items: items.slice((page - 1) * size, page * size),
          page,
          page_size: size,
          total: items.length,
          pages: Math.max(1, Math.ceil(items.length / size)),
          counts: { all: 128, working: 6 },
          tag_catalog: {
            stats: [
              { tag: "frontend", total: 64 },
              { tag: "core", total: 64 },
              { tag: "design", total: 64 },
            ],
            registry: [{ canonical: "frontend", description: "Frontend work" }],
          },
          backlog_selection: { selected: workflow.backlog, candidates: [] },
          backlog_presence: { present: true },
        };
      } else if (u.pathname.startsWith("/api/tasks/")) {
        const id = decodeURIComponent(u.pathname.split("/").pop());
        value = { ...(rows.find((r) => r.id === id) || task), id };
      } else if (u.pathname === "/api/activity") {
        res.statusCode = state.activityStatus;
        value =
          state.activityStatus === 200
            ? state.data
            : { error: "Synthetic activity failure" };
      }
      res.end(JSON.stringify(value));
      return;
    }
    let name = decodeURIComponent(u.pathname);
    if (name === "/" || name.startsWith("/tasks/")) name = "/index.html";
    const file = path.resolve(root, "." + name);
    if (!file.startsWith(path.resolve(root) + path.sep)) {
      res.writeHead(403);
      res.end();
      return;
    }
    fs.readFile(file, (e, bytes) => {
      if (e) {
        res.writeHead(404);
        res.end();
        return;
      }
      res.setHeader(
        "Content-Type",
        {
          ".js": "text/javascript",
          ".css": "text/css",
          ".html": "text/html",
          ".svg": "image/svg+xml",
          ".ttf": "font/ttf",
        }[path.extname(file)] || "application/octet-stream",
      );
      res.end(bytes);
    });
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  return {
    origin: "http://127.0.0.1:" + server.address().port,
    state,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}
module.exports = { start };
