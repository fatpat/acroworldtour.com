// Acronym autocomplete for the simulator. The tricks of a run are typed in one
// text field (space- or comma-separated), so the <datalist> options are the
// catalog acronyms prefixed with what was already typed. Without JS the plain
// text field works.
const list = document.getElementById("trick-list");
if (list) {
  let tricks = [];
  fetch("/simulator/tricks.json", { headers: { Accept: "application/json" } })
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
  for (const input of document.querySelectorAll("[data-trick-input]")) {
    input.addEventListener("input", () => refresh(input));
  }
}
