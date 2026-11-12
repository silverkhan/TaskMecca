const { test } = require("node:test"),
  assert = require("node:assert/strict"),
  fs = require("node:fs"),
  path = require("node:path"),
  vm = require("node:vm");
const { session } = require("./helpers/native_ui_vm.cjs"),
  boot = fs.readFileSync(
    "goassets/template/_task_mecca/framework/web/ui/boot.js",
    "utf8",
  );
function upgrade(values = {}, locale = "en-US", dark = false) {
  const storage = new Map(Object.entries(values)),
    writes = [],
    localStorage = {
      getItem: (key) => storage.get(key) || null,
      setItem: (key, value) => {
        writes.push([key, value]);
        storage.set(key, value);
      },
    },
    document = { documentElement: { dataset: {} } },
    navigator = { language: locale };
  vm.runInNewContext(boot, {
    localStorage,
    document,
    navigator,
    matchMedia: () => ({ matches: dark }),
  });
  const native = session({ localStorage, navigator });
  return {
    storage,
    writes,
    firstPaint: document.documentElement,
    state: native.load("state.js").state,
  };
}
for (const mode of ["light", "dark", "system"])
  test(
    "native upgrade preserves legacy " +
      mode +
      " before first paint and module startup",
    () => {
      const result = upgrade({ "task-mecca-theme": mode }, "en-US", true);
      assert.equal(result.firstPaint.dataset.mode, mode);
      assert.equal(
        result.firstPaint.dataset.theme,
        mode === "system" ? "dark" : mode,
      );
      assert.equal(result.state.mode, mode);
      assert.equal(result.storage.get("task-mecca-smoke-mode"), mode);
      assert.equal(
        result.writes.length,
        1,
        "migration is one-time across bootstrap and state",
      );
    },
  );
for (const mode of ["light", "dark", "system"])
  test("saved Smoke " + mode + " wins over the old theme", () => {
    const result = upgrade({
      "task-mecca-smoke-mode": mode,
      "task-mecca-theme": mode === "light" ? "dark" : "light",
    });
    assert.equal(result.firstPaint.dataset.mode, mode);
    assert.equal(result.state.mode, mode);
    assert.equal(result.writes.length, 0);
  });
for (const [saved, locale, expected] of [
  ["ko", "en-US", "ko"],
  ["en", "ko-KR", "en"],
  [null, "ko-KR", "ko"],
  [null, "en-GB", "en"],
  [null, "fr-FR", "en"],
  ["invalid", "KO-kr", "ko"],
])
  test("saved language / locale " + saved + " / " + locale, () => {
    const result = upgrade(
      saved ? { "task-mecca-language": saved } : {},
      locale,
    );
    assert.equal(result.firstPaint.lang, expected);
    assert.equal(result.state.language, expected);
  });
test("invalid modes fall back to System and retain the locale policy", () => {
  const result = upgrade(
    { "task-mecca-smoke-mode": "neon", "task-mecca-theme": "unknown" },
    "ko-KR",
    false,
  );
  assert.equal(result.firstPaint.dataset.mode, "system");
  assert.equal(result.firstPaint.dataset.theme, "light");
  assert.equal(result.state.mode, "system");
  assert.equal(result.state.language, "ko");
});
test("blocked preference storage does not prevent first paint or native startup", () => {
  const localStorage = {
      getItem() {
        throw Error("blocked");
      },
      setItem() {
        throw Error("blocked");
      },
    },
    document = { documentElement: { dataset: {} } },
    navigator = { language: "en-US" };
  assert.doesNotThrow(() =>
    vm.runInNewContext(boot, {
      localStorage,
      document,
      navigator,
      matchMedia: () => ({ matches: false }),
    }),
  );
  const native = session({ localStorage, navigator });
  assert.equal(native.load("state.js").state.mode, "system");
  assert.equal(document.documentElement.lang, "en");
});
