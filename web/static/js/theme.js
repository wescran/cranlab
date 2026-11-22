// Without a saved override the CSS color scheme follows the OS.
(() => {
  const root = document.documentElement;
  const key = "cranlab-theme";
  try {
    const saved = localStorage.getItem(key);
    if (saved === "light" || saved === "dark") root.dataset.theme = saved;
  } catch { /* Storage may be disabled. */ }

  document.addEventListener("DOMContentLoaded", () => {
    const system = matchMedia("(prefers-color-scheme: dark)");
    const button = document.getElementById("theme-toggle");
    const dark = () => (root.dataset.theme || (system.matches ? "dark" : "light")) === "dark";
    const sync = () => {
      button.textContent = dark() ? "☀ Light" : "◐ Dark";
      button.setAttribute("aria-label", dark() ? "Switch to light theme" : "Switch to dark theme");
    };
    button.addEventListener("click", () => {
      root.dataset.theme = dark() ? "light" : "dark";
      try { localStorage.setItem(key, root.dataset.theme); } catch {}
      sync();
    });
    system.addEventListener("change", sync);
    sync();
    button.hidden = false;
  });
})();
