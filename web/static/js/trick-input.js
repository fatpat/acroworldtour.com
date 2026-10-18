// Acronym autocomplete for trick inputs (simulator, judging screen). The
// <datalist id="trick-list"> names its catalog in data-src (default: the
// simulator's). A simulator field holds several tricks (space- or
// comma-separated), so the options are the catalog acronyms prefixed with
// what was already typed. Inputs are found by delegation, so inputs added
// by htmx work too. Without JS the plain text field works.
const list = document.getElementById("trick-list");
if (list) {
  let tricks = [];
  fetch(list.dataset.src || "/simulator/tricks.json", { headers: { Accept: "application/json" } })
    .then((r) => (r.ok ? r.json() : []))
    .then((data) => {
      tricks = data;
    })
    .catch(() => {});

  const refresh = (input) => {
    const m = /^(.*[\s,;])?([^\s,;]*)$/.exec(input.value);
    const head = m[1] || "";
    const word = m[2].toUpperCase();
    list.replaceChildren();
    if (word === "") return;
    let shown = 0;
    for (const t of tricks) {
      if (!t.acronym.toUpperCase().startsWith(word)) continue;
      const o = document.createElement("option");
      o.value = head + t.acronym;
      o.label = t.name;
      list.append(o);
      if (++shown >= 20) break;
    }
  };
  document.addEventListener("input", (e) => {
    if (e.target instanceof HTMLInputElement && e.target.matches("[data-trick-input]")) refresh(e.target);
  });
}
