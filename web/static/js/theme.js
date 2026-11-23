(() => {
  const button = document.querySelector(".theme-control");
  const media = matchMedia("(prefers-color-scheme: dark)");
  const system = () => media.matches ? "dark" : "light";
  const key = "cranlab-theme";
  let override = "";
  try {
    const saved = localStorage.getItem(key);
    if (saved === "dark" || saved === "light") override = saved;
  } catch { /* Storage is optional. */ }

  const target = () => (override || system()) === "dark" ? "light" : "dark";
  function render() {
    document.documentElement.style.colorScheme = override;
    const next = target();
    button.textContent = next === "light" ? "Light" : "Dark";
    button.setAttribute("aria-label", `Switch to ${next} theme`);
  }

  button.addEventListener("click", () => {
    const next = target();
    override = next === system() ? "" : next;
    render();
    try {
      if (override) localStorage.setItem(key, override);
      else localStorage.removeItem(key);
    } catch { /* The change still applies to this page. */ }
  });
  media.addEventListener("change", render);
  render();
  button.hidden = false;
})();
