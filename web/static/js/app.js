// htmx hardening for the strict CSP: no eval, no injected <style>.
// Loaded after htmx; the page works without it (without JavaScript).
if (window.htmx) {
  htmx.config.allowEval = false;
  htmx.config.includeIndicatorStyles = false;
}
