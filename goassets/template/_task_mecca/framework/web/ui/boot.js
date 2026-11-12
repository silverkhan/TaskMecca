(() => {
  const read = (key) => {
      try {
        return localStorage.getItem(key);
      } catch {
        return null;
      }
    },
    valid = (value) => ["light", "dark", "system"].includes(value);
  const smoke = read("task-mecca-smoke-mode"),
    legacy = read("task-mecca-theme"),
    mode = valid(smoke) ? smoke : valid(legacy) ? legacy : "system";
  if (!valid(smoke) && valid(legacy))
    try {
      localStorage.setItem("task-mecca-smoke-mode", mode);
    } catch {}
  const language = read("task-mecca-language");
  document.documentElement.lang = ["ko", "en"].includes(language)
    ? language
    : navigator.language?.toLowerCase().startsWith("ko")
      ? "ko"
      : "en";
  document.documentElement.dataset.mode = mode;
  document.documentElement.dataset.theme =
    mode === "system"
      ? matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : mode;
})();
