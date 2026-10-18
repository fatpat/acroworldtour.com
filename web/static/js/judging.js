// Judging screen (spec 05 §6.2, §8 "Live scoring"): htmx glue and the
// keyboard flow. The page works without it (plain form posts); this only
// makes entry faster.
{
  const root = document.querySelector(".judging");
  if (root) {
    // Request ordering is declared in judging.templ (hx-sync on the form
    // and its buttons: the live simulate queues, save/edit/delete abort a
    // pending simulate). Here: 422 (invalid values) and 409 (refused save)
    // answer with a fragment to show, and an answer for a form that was
    // replaced meanwhile (save, edit, other participant) is stale and
    // dropped.
    document.body.addEventListener("htmx:beforeSwap", (e) => {
      const s = e.detail.xhr.status;
      if (s === 422 || s === 409) {
        e.detail.shouldSwap = true;
        e.detail.isError = false;
      }
      if (!document.body.contains(e.detail.elt)) e.detail.shouldSwap = false;
    });

    // aria-invalid follows the error slots that the live score refreshes, and
    // a re-rendered form focuses its new trick input.
    const settle = () => {
      for (const slot of root.querySelectorAll("span.field__error[id^='err-']")) {
        const input = document.getElementById(slot.id.slice(4));
        if (!input) continue;
        if (slot.textContent.trim() !== "") input.setAttribute("aria-invalid", "true");
        else input.removeAttribute("aria-invalid");
      }
      const auto = root.querySelector("#flight [autofocus]");
      if (auto) {
        auto.removeAttribute("autofocus"); // once: later swaps must not steal focus
        auto.focus();
      }
    };
    document.body.addEventListener("htmx:afterSettle", settle);
    document.body.addEventListener("htmx:oobAfterSwap", settle);

    const form = () => document.getElementById("flight-form");
    const clickOp = (op) => {
      const b = form()?.querySelector(`button[name="op"][value="${op}"]`);
      if (b && !b.disabled) b.click();
    };
    const focusNext = (from) => {
      const items = [...form().querySelectorAll(
        'input:not([type="hidden"]):not([disabled]), button:not([disabled]):not([tabindex="-1"])',
      )];
      const next = items[items.indexOf(from) + 1];
      if (next) next.focus();
    };

    // Enter never saves: in the last trick it adds a trick, elsewhere it moves
    // to the next field. Alt+Up / Alt+Down move a trick, Alt+Delete removes it.
    root.addEventListener("keydown", (e) => {
      const t = e.target;
      if (!(t instanceof HTMLInputElement) || t.form?.id !== "flight-form") return;
      if (t.type !== "text" && t.type !== "number") return;
      const k = t.name === "trick" ? Number(t.id.replace("trick-", "")) : -1;
      if (e.key === "Enter") {
        e.preventDefault();
        const tricks = t.form.querySelectorAll('input[name="trick"]');
        if (k >= 0 && k === tricks.length - 1) clickOp("trick-add");
        else focusNext(t);
        return;
      }
      if (k < 0 || !e.altKey) return;
      const op = { ArrowUp: "trick-up", ArrowDown: "trick-down", Delete: "trick-remove" }[e.key];
      if (op) {
        e.preventDefault();
        clickOp(`${op}:${k}`);
      }
    });
  }
}
