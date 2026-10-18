package admin

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"awt/internal/export"
	"awt/internal/web"
)

const (
	xlsxType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	// printCSP lets the self-contained print pages carry their inline
	// stylesheet; nothing else is allowed (no script, no frames).
	printCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

// registerExports wires the printable and CIVL exports (spec 05 §8). They
// are open to admins and to the scorers of the competition.
func (h *handlers) registerExports(r *web.Router) {
	scorer(r, "/admin/competitions/{code}/export/results.xlsx", h.compExportXLSX)
	scorer(r, "/admin/competitions/{code}/export/results.html", h.compExportHTML)
	scorer(r, "/admin/competitions/{code}/runs/{n}/export/results.xlsx", h.runExportXLSX)
	scorer(r, "/admin/competitions/{code}/runs/{n}/export/results.html", h.runExportHTML)
	scorer(r, "/admin/competitions/{code}/runs/{n}/starting-order.html", h.startingOrderHTML)
}

// fileSafe keeps a download filename to [a-z0-9._-].
func fileSafe(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "export"
	}
	return b.String()
}

func writeXLSX(w http.ResponseWriter, r *http.Request, filename string, fn func(io.Writer) error) {
	var buf bytes.Buffer
	if err := fn(&buf); err != nil {
		if errors.Is(err, export.ErrRunNotFinal) {
			http.Error(w, "The run results are not final yet: close the run (every flight published) first.", http.StatusBadRequest)
			return
		}
		slog.Error("export xlsx", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", xlsxType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write(buf.Bytes())
}

func writeHTML(w http.ResponseWriter, r *http.Request, fn func(io.Writer) error) {
	var buf bytes.Buffer
	if err := fn(&buf); err != nil {
		slog.Error("export html", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", printCSP)
	_, _ = w.Write(buf.Bytes())
}

// limitRun reads ?limit_run=N (1-based: results after run N) and returns
// the 0-based index, -1 when absent. ok is false (and 400 written) when it
// is not a run of the competition.
func (h *handlers) limitRun(w http.ResponseWriter, r *http.Request, runs int) (int, bool) {
	raw := r.URL.Query().Get("limit_run")
	if raw == "" {
		return -1, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > runs {
		http.Error(w, "limit_run must be a run number of the competition", http.StatusBadRequest)
		return 0, false
	}
	return n - 1, true
}

func (h *handlers) compExportData(w http.ResponseWriter, r *http.Request) (export.CompetitionExport, string, bool) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return export.CompetitionExport{}, "", false
	}
	runs, err := h.svc.ListRuns(r.Context(), c.ID)
	if err != nil {
		h.fail(w, r, err, nil)
		return export.CompetitionExport{}, "", false
	}
	limit, ok := h.limitRun(w, r, len(runs))
	if !ok {
		return export.CompetitionExport{}, "", false
	}
	data, err := h.svc.CompetitionExportData(r.Context(), c.ID, limit)
	if err != nil {
		h.fail(w, r, err, nil)
		return data, "", false
	}
	name := fileSafe(c.Code)
	if limit >= 0 {
		name += "-after-run-" + strconv.Itoa(limit+1)
	}
	return data, name, true
}

func (h *handlers) compExportXLSX(w http.ResponseWriter, r *http.Request) {
	data, name, ok := h.compExportData(w, r)
	if !ok {
		return
	}
	writeXLSX(w, r, name+"-results.xlsx", func(wr io.Writer) error { return export.CompetitionXLSX(wr, data) })
}

func (h *handlers) compExportHTML(w http.ResponseWriter, r *http.Request) {
	data, _, ok := h.compExportData(w, r)
	if !ok {
		return
	}
	writeHTML(w, r, func(wr io.Writer) error { return export.ResultsHTML(wr, data) })
}

func (h *handlers) runExportData(w http.ResponseWriter, r *http.Request) (export.RunExport, string, bool) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return export.RunExport{}, "", false
	}
	data, err := h.svc.RunExportData(r.Context(), c.ID, run.Index)
	if err != nil {
		h.fail(w, r, err, nil)
		return data, "", false
	}
	return data, fileSafe(c.Code) + "-run-" + strconv.Itoa(run.Index+1), true
}

func (h *handlers) runExportXLSX(w http.ResponseWriter, r *http.Request) {
	data, name, ok := h.runExportData(w, r)
	if !ok {
		return
	}
	writeXLSX(w, r, name+"-results.xlsx", func(wr io.Writer) error { return export.RunXLSX(wr, data) })
}

func (h *handlers) runExportHTML(w http.ResponseWriter, r *http.Request) {
	data, _, ok := h.runExportData(w, r)
	if !ok {
		return
	}
	writeHTML(w, r, func(wr io.Writer) error { return export.RunResultsHTML(wr, data) })
}

func (h *handlers) startingOrderHTML(w http.ResponseWriter, r *http.Request) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	data, err := h.svc.StartingOrderExportData(r.Context(), c.ID, run.Index)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	writeHTML(w, r, func(wr io.Writer) error { return export.StartingOrderHTML(wr, data) })
}
