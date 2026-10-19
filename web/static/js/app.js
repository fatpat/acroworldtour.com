// Marks the page as script-enabled (CSS then shows the mobile menu toggle),
// wires the menu toggle and hardens htmx for the strict CSP.
document.documentElement.classList.add("js");

const toggle = document.querySelector(".nav__toggle");
const menu = document.querySelector(".nav__menu");
if (toggle && menu) {
  toggle.addEventListener("click", () => {
    const open = menu.classList.toggle("nav__menu--open");
    toggle.setAttribute("aria-expanded", String(open));
  });
}

if (window.htmx) {
  htmx.config.allowEval = false;
  htmx.config.includeIndicatorStyles = false;
}

// Admin files page: copy the URL next to the button.
document.addEventListener("click", (e) => {
  const btn = e.target.closest("[data-copy]");
  if (!btn) return;
  const input = btn.parentElement.querySelector("input");
  if (!input) return;
  input.select();
  if (navigator.clipboard) navigator.clipboard.writeText(input.value).catch(() => {});
});

// Account page: copy the recovery codes, one per line. The button only
// shows when this script runs.
for (const btn of document.querySelectorAll("[data-copy-list]")) {
  btn.hidden = false;
  btn.addEventListener("click", () => {
    const list = document.getElementById(btn.dataset.copyList);
    if (!list || !navigator.clipboard) return;
    const text = Array.from(list.querySelectorAll("li"), (li) => li.textContent).join("\n");
    navigator.clipboard.writeText(text).then(() => {
      btn.textContent = "Copied";
    }).catch(() => {});
  });
}
