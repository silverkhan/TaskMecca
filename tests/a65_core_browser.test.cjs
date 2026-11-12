const { test } = require("node:test"),
  assert = require("node:assert/strict"),
  path = require("node:path"),
  fs = require("node:fs");
const runtime = process.env.WORKFLOW_PLAYWRIGHT_MODULE,
  fixture = require("./helpers/a65_fixture.cjs");
if (process.env.WORKFLOW_REQUIRE_BROWSER === "1" && !runtime)
  throw Error("Required native Smoke browser runtime missing");
test(
  "native Smoke core: overview, navigation, filters, document-first detail and finite reads",
  { skip: !runtime, timeout: 90000 },
  async () => {
    const server = await fixture.start(
        path.resolve("goassets/template/_task_mecca/framework/web"),
      ),
      errors = [],
      cases = [];
    let browser;
    try {
      const { chromium } = require(runtime);
      browser = await chromium.launch({
        headless: true,
        executablePath: process.env.CHROME_EXECUTABLE,
      });
      const context = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      });
      await context.route("**/*", (r) =>
        r
          .request()
          .url()
          .startsWith(server.origin + "/")
          ? r.continue()
          : r.abort(),
      );
      const page = await context.newPage();
      page.on("pageerror", (e) => errors.push(e.message));
      server.state.loadingResponses = 1;
      await page.goto(server.origin);
      await page.locator(".notice").first().waitFor();
      await page.waitForFunction(
        () => document.querySelectorAll("#attention .task-row").length === 4,
      );
      assert.equal(
        server.state.requests.filter((u) => u.startsWith("/api/overview?"))
          .length,
        2,
      );
      assert.equal(
        server.state.requests.filter((u) =>
          /^\/api\/(snapshot|workload)/.test(u),
        ).length,
        0,
      );
      const attentionIDs = new Set();
      for (let attentionPage = 1; attentionPage <= 3; attentionPage++) {
        const more = page.locator("#attentionMore");
        if (await more.count()) {
          assert.equal(await more.evaluate((el) => el.open), false);
          await more.locator("summary").click();
        }
        for (const id of await page
          .locator("#attention .task-row:visible .task-id")
          .allTextContents())
          attentionIDs.add(id);
        if (attentionPage < 3) {
          await page.locator('[data-page="attention:next"]').click();
          await page.waitForFunction(
            (expected) =>
              document
                .querySelector("#attention .pagination")
                ?.textContent.includes(" " + expected + " "),
            attentionPage + 1,
          );
        }
      }
      assert.equal(
        attentionIDs.size,
        9,
        "preview disclosure and opaque pagination retain access to every known attention item",
      );
      assert.match(
        await page.locator("#projects").textContent(),
        /미확인|Unknown/,
      );
      assert.ok(
        !(await page
          .locator("#projects li")
          .last()
          .textContent()
          .then((s) => /진행\s*0|Working\s*0/.test(s))),
      );
      await page.locator('[data-page="activity:next"]').click();
      await page.waitForFunction(() =>
        document
          .querySelector("#activity .pagination")
          ?.textContent.includes("2"),
      );
      assert.ok(
        server.state.requests.some((u) =>
          u.includes("activity_cursor=opaque-activity-2"),
        ),
      );
      cases.push(
        "cold loading becomes observed data; unknown differs from zero; opaque independent cursor; no per-project fanout",
      );
      server.state.rejectOverviewCursorOnce = true;
      server.state.revision = "fixture:2";
      server.state.emit();
      await page.waitForFunction(() =>
        [...document.querySelectorAll(".notice")].some((n) =>
          /프로젝트 구성이 변경|Project membership changed/.test(n.textContent),
        ),
      );
      assert.ok(
        server.state.requests
          .filter((u) => u.startsWith("/api/overview?"))
          .at(-1)
          .indexOf("_cursor=") === -1,
      );
      cases.push(
        "registry change recovers a stale cursor once with a first-page notice",
      );
      const beforeLateHint = server.state.requests.length;
      server.state.revision = "fixture:1";
      server.state.emit();
      await page.waitForTimeout(100);
      assert.equal(server.state.requests.length, beforeLateHint);
      server.state.revision = "fixture:2";
      cases.push(
        "late same-epoch global SSE hints do not trigger stale refetch",
      );
      const before = server.state.requests.length;
      await page.evaluate(() => {
        window.__smokeMutations = 0;
        window.__smokeObserver = new MutationObserver(
          (records) => (window.__smokeMutations += records.length),
        );
        window.__smokeObserver.observe(document.body, {
          childList: true,
          subtree: true,
        });
      });
      await page.waitForTimeout(1600);
      assert.equal(server.state.requests.length, before);
      assert.equal(await page.evaluate(() => window.__smokeMutations), 0);
      await page.evaluate(() => window.__smokeObserver.disconnect());
      cases.push("completed overview has no idle request or DOM growth");
      await page.locator("#projects .project-link").first().click();
      await page.locator("#filters").waitFor();
      await page.locator('.filter-more > summary').click();
      await page.locator('.tm-combo').filter({has:page.locator('select[name=page_size]')}).getByRole('combobox').click();
      await page.getByRole('option',{name:'20',exact:true}).click();
      await page.waitForFunction(() => document.querySelectorAll('.backlog-list .task-row').length === 20);
      const q = page.locator("input[name=q]");
      await page.evaluate(() => {
        window.__smokeNodes = {
          rail: document.querySelector(".rail"),
          topbar: document.querySelector(".topbar"),
          search: document.querySelector("[name=q]"),
          form: document.querySelector("#filters"),
          main: document.querySelector("main"),
        };
      });
      await q.fill("FLOW-12");
      await page.waitForTimeout(450);
      await page.waitForFunction(() => location.search.includes("q=FLOW-12"));
      await page.waitForFunction(() => document.activeElement?.name === "q");
      assert.equal(await q.evaluate((e) => e === document.activeElement), true);
      await page.waitForFunction(
        () =>
          document.querySelector("main").getAttribute("aria-busy") === "false",
      );
      assert.equal(
        await page.evaluate(
          () =>
            window.__smokeNodes.rail === document.querySelector(".rail") &&
            window.__smokeNodes.topbar === document.querySelector(".topbar") &&
            window.__smokeNodes.search === document.querySelector("[name=q]") &&
            window.__smokeNodes.form === document.querySelector("#filters") &&
            window.__smokeNodes.main === document.querySelector("main"),
        ),
        true,
        "search patches results without replacing shell or focused input",
      );
      await q.fill("");
      await page.locator("#filters").evaluate((f) => f.requestSubmit());
      await page.waitForFunction(
        () => !new URL(location.href).searchParams.get("q"),
      );
      await page.locator('[data-page="backlog:next"]').click();
      await page.waitForFunction(
        () => new URL(location.href).searchParams.get("page") === "2",
      );
      await page.locator(".task-row").first().waitFor();
      await page.evaluate(() => scrollTo(0, 500));
      await page.waitForFunction(
        () =>
          document.querySelector("main").getAttribute("aria-busy") === "false",
      );
      await page.locator(".task-row").nth(8).scrollIntoViewIfNeeded();
      const listURL = page.url(),
        readingY = await page.evaluate(() => scrollY),
        id = await page.locator(".task-id").nth(8).textContent();
      await page.locator(".task-row").nth(8).locator("a").click();
      await page.locator(".detail-tabs").waitFor();
      await page.locator("#backBtn").click();
      await page.waitForURL(listURL);
      await page.locator("#filters").waitFor();
      await page.waitForFunction((y) => Math.abs(scrollY - y) < 2, readingY);
      assert.equal(new URL(page.url()).searchParams.get("page"), "2");
      assert.ok(
        Math.abs((await page.evaluate(() => scrollY)) - readingY) < 2,
        "back restores reading rather than resetting to top",
      );
      assert.ok(
        await page.locator(".task-row").filter({ hasText: id }).count(),
      );
      cases.push(
        "debounced search retains focus; URL page state and list reading survive dedicated detail/back",
      );
      if (!(await page.locator(".filter-more").evaluate((el) => el.open)))
        await page.locator(".filter-more > summary").click();
      await page.locator("[data-status-filter=doing]").click();
      await page.waitForFunction(
        () => new URL(location.href).searchParams.get("status") === "doing",
      );
      await page.waitForFunction(
        () =>
          document.querySelector("main").getAttribute("aria-busy") === "false",
      );
      await page.locator(".task-row").first().waitFor();
      assert.equal(await page.locator(".task-row").count(), 6);
      if (!(await page.locator(".filter-more").evaluate((el) => el.open)))
        await page.locator(".filter-more > summary").click();
      await page.locator("input[name=tags]").fill("frontend,core");
      await page.locator("#tags").evaluate((f) => f.requestSubmit());
      await page.waitForFunction(
        () =>
          new URL(location.href).searchParams.get("tags") === "frontend,core",
      );
      await page.waitForFunction(
        () =>
          document.querySelector("main").getAttribute("aria-busy") === "false",
      );
      await page.locator(".task-row").first().waitFor();
      assert.equal(await page.locator(".task-row").count(), 3);
      await page.reload();
      await page.locator("#filters").waitFor();
      assert.equal(
        await page.locator("input[name=status]").inputValue(),
        "doing",
      );
      assert.equal(
        await page.locator("input[name=tags]").inputValue(),
        "frontend,core",
      );
      cases.push(
        "status, multitag, page and refresh context remain URL-addressable",
      );
      let release;
      await context.route("**/api/tasks/FLOW-1?*", async (route) => {
        if (
          new URL(route.request().url()).searchParams.get("projection") ===
          "lifecycle"
        )
          await new Promise((r) => (release = r));
        await route.continue();
      });
      await page.goto(
        server.origin + "/tasks/FLOW-1?project=/fixtures/task-mecca",
      );
      await page.locator(".summary-sheet").waitFor();
      assert.ok(release, "canonical request is independent of visible content");
      await page.locator(".raw-source summary").click();
      const body = await page.locator(".raw-source").textContent();
      release();
      await page.waitForTimeout(80);
      assert.equal(await page.locator(".raw-source").textContent(), body);
      await context.unroute("**/api/tasks/FLOW-1?*");
      await page.locator(".detail-tabs a").nth(2).click();
      await page.locator("#workflowTimelineViewport").waitFor();
      const domain = () =>
          page
            .locator(".workflow-timeline-canvas")
            .evaluate((el) => [
              Number(el.dataset.viewStart),
              Number(el.dataset.viewEnd),
            ]),
        full = await domain();
      await page.locator("#workflowZoomIn").click();
      assert.ok((await domain())[1] - (await domain())[0] < full[1] - full[0]);
      await page.locator("#workflowFit").click();
      assert.deepEqual(await domain(), full);
      await page.reload();
      await page.locator("#workflowTimelineViewport").waitFor();
      assert.deepEqual(await domain(), full);
      cases.push(
        "content paints before slow canonical projection; source is preserved; lifecycle zoom/fit and direct reload work",
      );
      await context.close();
      for (const width of [1440, 1280, 320, 390, 430])
        for (const theme of ["light", "dark"]) {
          const c = await browser.newContext({
            viewport: { width, height: width === 1280 ? 900 : 1000 },
            hasTouch: width < 760,
          });
          await c.addInitScript(
            ({ theme, width }) => {
              localStorage.setItem("task-mecca-smoke-mode", theme);
              localStorage.setItem(
                "task-mecca-language",
                width % 2 ? "en" : "ko",
              );
            },
            { theme, width },
          );
          const p = await c.newPage();
          p.on("pageerror", (e) => errors.push(e.message));
          for (const url of [
            "/",
            "/?view=backlog&project=/fixtures/task-mecca",
            "/tasks/FLOW-1?project=/fixtures/task-mecca&tab=lifecycle",
          ]) {
            await p.goto(server.origin + url);
            await p.locator("h1").waitFor();
            assert.equal(
              await p.evaluate(
                () => document.documentElement.scrollWidth > innerWidth,
              ),
              false,
            );
            assert.equal(
              await p.locator("html").getAttribute("data-theme"),
              theme,
            );
            if (url === "/") {
              await p.locator("#projects .project-link").first().waitFor();
              if (width >= 1100)
                assert.equal(
                  await p
                    .locator("#projects .project-link")
                    .evaluateAll((rows) =>
                      rows.every(
                        (row) =>
                          row.getBoundingClientRect().bottom <= innerHeight,
                      ),
                    ),
                  true,
                  "all three project identities/counts stay in the first desktop viewport",
                );
              if (width < 760)
                assert.ok(
                  (await p
                    .locator("#attention .task-main")
                    .first()
                    .evaluate((el) => el.getBoundingClientRect().width)) >= 200,
                  "mobile evidence uses the available row width",
                );
            }
            if (url.includes("view=backlog")) {
              await p.locator(".task-row").first().waitFor();
              assert.equal(
                await p.locator(".filter-more").evaluate((el) => el.open),
                false,
              );
              assert.ok(await p.locator(".task-updated").first().isVisible());
              if (width < 760)
                assert.ok(
                  (await p
                    .locator(".task-row")
                    .first()
                    .evaluate((el) => el.getBoundingClientRect().top)) < 450,
                  "compact tools leave the first mobile task in view",
                );
            }
          }
          await p.keyboard.press("Tab");
          assert.notEqual(
            await p.evaluate(() => document.activeElement.tagName),
            "BODY",
          );
          await c.close();
        }
      cases.push(
        "five viewport widths, Light/Dark, Korean/English and keyboard focus",
      );
      const z = await browser.newPage({
        viewport: { width: 1440, height: 1000 },
      });
      await z.goto(server.origin);
      await z.locator("h1").waitFor();
      await z.evaluate(() => (document.documentElement.style.zoom = "2"));
      assert.equal(
        await z.evaluate(
          () => document.documentElement.scrollWidth > innerWidth,
        ),
        false,
      );
      await z.close();
      cases.push(
        "200% CSS zoom layout-equivalent has no document overflow; physical browser/device zoom unclaimed",
      );
      const finite = await browser.newPage();
      server.state.loadingResponses = 100;
      const start = server.state.requests.filter((u) =>
        u.startsWith("/api/overview?"),
      ).length;
      await finite.goto(server.origin);
      await finite.waitForFunction(
        () =>
          [...document.querySelectorAll(".notice")].some((n) =>
            /자동 확인을 멈췄습니다|Automatic checks paused/.test(
              n.textContent,
            ),
          ),
        null,
        { timeout: 12000 },
      );
      const stopped = server.state.requests.filter((u) =>
        u.startsWith("/api/overview?"),
      ).length;
      assert.equal(stopped - start, 13);
      await finite.waitForTimeout(700);
      assert.equal(
        server.state.requests.filter((u) => u.startsWith("/api/overview?"))
          .length,
        stopped,
      );
      await finite.locator('a[href*="view=settings"]').click();
      await finite.locator(".management-sheet").first().waitFor();
      await finite.waitForTimeout(700);
      assert.equal(
        server.state.requests.filter((u) => u.startsWith("/api/overview?"))
          .length,
        stopped,
      );
      await finite.close();
      cases.push(
        "cold/pending polling stops after initial+12 retries and after route departure",
      );
      assert.deepEqual(errors, []);
      const out = process.env.WORKFLOW_EVIDENCE_DIR || "a65-core-evidence";
      fs.mkdirSync(out, { recursive: true });
      fs.writeFileSync(
        path.join(out, "native-core-contract.json"),
        JSON.stringify(
          {
            synthetic: true,
            browser: browser.version(),
            passed: true,
            cases,
            errors,
            limits: [
              "No physical device or assistive-technology validation",
              "CSS zoom is a layout equivalent, not native browser zoom",
            ],
          },
          null,
          2,
        ),
      );
    } finally {
      await browser?.close();
      await server.close();
    }
  },
);
