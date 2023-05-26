(function getCurrentYear(el) {
    if (!el) return;
    var currentYear = (new Date()).getFullYear();
    el.innerHTML = currentYear;
})(document.querySelector(".js-current-year"));