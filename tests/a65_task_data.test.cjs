const { test } = require("node:test"),
  assert = require("node:assert/strict"),
  path = require("node:path"),
  { pathToFileURL } = require("node:url");
const moduleURL = pathToFileURL(
  path.resolve("goassets/template/_task_mecca/framework/web/ui/data.js"),
).href;
const context = (id = "A-1", project = "/synthetic", backlog = "team") => ({
  id,
  project,
  backlog,
});
const reply = (body) => ({ ok: true, json: async () => body });
let data;
test.before(async () => {
  data = await import(moduleURL);
});
test.afterEach(() => {
  data.invalidateContent();
});
test("native data layer shares a pending hover request with foreground navigation", async () => {
  let finish,
    calls = 0;
  global.fetch = () => {
    calls++;
    return new Promise((resolve) => (finish = resolve));
  };
  const pre = data.prefetchContent(context()),
    open = data.requestContent(context());
  assert.equal(calls, 1);
  finish(reply({ id: "A-1", raw_markdown: "# Original" }));
  await pre;
  assert.equal((await open).raw_markdown, "# Original");
  assert.equal(data.cachedContent(context()).id, "A-1");
});
test("speculative work is bounded to one request, with no hidden queued scan", async () => {
  let finish,
    calls = 0;
  global.fetch = () => {
    calls++;
    return new Promise((resolve) => (finish = resolve));
  };
  const first = data.prefetchContent(context());
  for (let i = 2; i < 100; i++) data.prefetchContent(context("A-" + i));
  assert.equal(calls, 1);
  finish(reply({ id: "A-1" }));
  await first;
  assert.equal(calls, 1);
});
test("content cache keys include project and backlog", async () => {
  global.fetch = async (url) => reply({ id: "A-1", url });
  await data.requestContent(context());
  assert.equal(data.cachedContent(context("A-1", "/other")), null);
  assert.equal(data.cachedContent(context("A-1", "/synthetic", "other")), null);
});
test("cached reentry avoids a duplicate content request", async () => {
  let calls = 0;
  global.fetch = async () => {
    calls++;
    return reply({ id: "A-1" });
  };
  await data.requestContent(context());
  await data.requestContent(context());
  assert.equal(calls, 1);
});
test("explicit invalidation forces a new document and prevents stale pending cache resurrection", async () => {
  const replies = [];
  global.fetch = () => new Promise((resolve) => replies.push(resolve));
  const old = data.requestContent(context());
  data.invalidateContent(context());
  const latest = data.requestContent(context());
  assert.equal(replies.length, 2);
  replies[1](reply({ id: "A-1", raw_markdown: "# Latest" }));
  await latest;
  replies[0](reply({ id: "A-1", raw_markdown: "# Old" }));
  await old;
  assert.equal(data.cachedContent(context()).raw_markdown, "# Latest");
});
test("a failed speculative request never blocks foreground retry", async () => {
  let calls = 0;
  global.fetch = async () => {
    calls++;
    if (calls === 1) throw Error("offline");
    return reply({ id: "A-1" });
  };
  await data.prefetchContent(context());
  assert.equal(data.cachedContent(context()), null);
  assert.equal((await data.requestContent(context())).id, "A-1");
  assert.equal(calls, 2);
});
test("content requests are content-only; lifecycle is an independent projection", async () => {
  const urls = [];
  global.fetch = async (url) => {
    urls.push(url);
    return reply({ id: "A-1" });
  };
  await data.requestContent(context());
  assert.match(urls[0], /projection=content/);
  assert.doesNotMatch(urls[0], /projection=(summary|lifecycle)/);
});
test("failed HTTP responses surface a recoverable error and do not cache zero-like content", async () => {
  global.fetch = async () => ({ ok: false, status: 503 });
  await assert.rejects(data.requestContent(context()), /HTTP 503/);
  assert.equal(data.cachedContent(context()), null);
});
test("native document renderer preserves source links and escapes executable markup", async () => {
  const { markdown } = await import(
    pathToFileURL(
      path.resolve("goassets/template/_task_mecca/framework/web/ui/format.js"),
    ).href
  );
  const html = markdown(
    "[Evidence](https://example.com/a)\n[Unsafe](javascript:alert)\n<script>bad()</script>\n- **Observed**",
  );
  assert.match(html, /href="https:\/\/example.com\/a"/);
  assert.doesNotMatch(html, /href="javascript:/);
  assert.match(html, /&lt;script&gt;/);
  assert.match(html, /<ul><li><strong>Observed<\/strong>/);
});

test("native backlog consumes the real compact-list and tag catalog contract", async () => {
  let url;
  global.fetch = async (value) => {
    url = value;
    return reply({
      items: [{ id: "A-1", tags: ["area:frontend"] }],
      backlog_selection: { selected: "/synthetic/backlog" },
      tag_catalog: {
        stats: [{ tag: "area:frontend", total: 1 }],
        registry: [
          { canonical: "area:frontend", description: "Frontend" },
          { canonical: "area:backend" },
        ],
      },
    });
  };
  const result = await data.fetchBacklog();
  assert.equal(
    new URL(url, "http://synthetic").searchParams.has("projection"),
    false,
    "summary means counts-only on the actual Go API",
  );
  assert.equal(result.items.length, 1);
  assert.equal(result.selection.selected, "/synthetic/backlog");
  assert.deepEqual(
    result.tag_options.map((t) => t.tag),
    ["area:frontend", "area:backend"],
  );
});
test("captured actual Go compact-list response stays nonempty and exposes canonical tag names", async () => {
  const packet = require("./fixtures/a65/backlog-actual-compact.json");
  assert.equal(packet.synthetic, true);
  global.fetch = async () => reply(packet.response);
  const result = await data.fetchBacklog();
  assert.ok(result.items.length > 0);
  assert.equal(result.items.length, packet.response.items.length);
  assert.equal(
    result.selection.selected,
    packet.response.backlog_selection.selected,
  );
  assert.ok(result.tag_options.some((t) => t.tag === "area:frontend"));
  assert.ok(result.tag_options.some((t) => t.tag === "type:user-wait"));
});
test("actual legacy task with empty defined requirement fields still shows its source", () => {
  const packet = require("./fixtures/a65/task-actual-content.json"),
    native = require("./helpers/native_ui_vm.cjs").session({
      location: { origin: "http://synthetic.local" },
    });
  const state = native.load("state.js").state;
  state.detailTab = "requirements";
  const rendered = native.load("screens/detail.js").detail(packet.response);
  assert.ok(packet.response.raw_markdown.length > 0);
  assert.match(rendered, /격리된 공개 합성 데이터/);
  assert.match(rendered, /document-section/);
});
