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
