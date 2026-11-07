(() => {
  let mode = "system";
  try {
    const saved = localStorage.getItem("task-mecca-smoke-mode");
    mode = ["light", "dark", "system"].includes(saved) ? saved : "system";
  } catch {}
  document.documentElement.dataset.mode = mode;
  document.documentElement.dataset.theme =
    mode === "system"
      ? matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : mode;
})();
