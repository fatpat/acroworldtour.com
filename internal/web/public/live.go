package public

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"awt/internal/export"
	"awt/internal/store/gen"
	"awt/internal/web"

	"github.com/a-h/templ"
)

// fragmentMaxAge keeps the ranking fragment fresh: the live page refetches
// it on every event, so the CDN may only hold it for a few seconds.
const fragmentMaxAge = 5 * time.Second

// registerLive wires the live updates: the SSE stream, the ranking fragment
// the stream triggers, and the broadcast overlays.
func registerLive(r *web.Router, p *results) {
	r.Public("GET /live/competitions/{code}", &web.SSE{Bus: r.Deps().Bus, Exists: p.published, Drain: r.Deps().Draining})
	r.PublicCached("GET /competitions/{code}/fragment", http.HandlerFunc(p.fragment), fragmentMaxAge, 0)
	r.Public("GET /live/overlay/competitions/{code}/runs/{file}", http.HandlerFunc(p.overlayRun))
	r.Public("GET /live/overlay/competitions/{code}/current.svg", http.HandlerFunc(p.overlayCurrent))
}

// published reports whether code is a published, undeleted competition.
func (p *results) published(ctx context.Context, code string) (bool, error) {
	_, err := p.q.PublicGetCompetition(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// live reports whether the page should follow the SSE stream: only open
// competitions do (closed ones never change).
func (h compHeader) live() bool { return h.C.State == "open" }

// sseAttrs connects the page body to the competition's event stream.
func (h compHeader) sseAttrs() templ.Attributes {
	if !h.live() {
		return nil
	}
	return templ.Attributes{"hx-ext": "sse", "sse-connect": "/live/competitions/" + url.PathEscape(h.C.Code)}
}

func (h compHeader) fragmentAttrs(list, run string) templ.Attributes {
	if !h.live() {
		return nil
	}
	q := url.Values{}
	if list != "" {
		q.Set("list", list)
	}
	if run != "" {
		q.Set("run", run)
	}
	return templ.Attributes{
		"hx-get":     h.Href() + "/fragment?" + q.Encode(),
		"hx-trigger": "sse:update",
		"hx-swap":    "innerHTML",
	}
}

func (v overallView) liveAttrs(run string) templ.Attributes { return v.fragmentAttrs(v.Sel, run) }

func (v runView) liveAttrs(run string) templ.Attributes { return v.fragmentAttrs(v.Sel, run) }

// fragment serves only the ranking table of the overall (no ?run=) or of a
// run (?run=N, 1-based), for the live page to swap in.
func (p *results) fragment(w http.ResponseWriter, r *http.Request) {
	active := "overall"
	n := 0
	if run := r.URL.Query().Get("run"); run != "" {
		var err error
		if n, err = strconv.Atoi(run); err != nil {
			notFound(w, r)
			return
		}
		active = run
	}
	h, err := p.loadCompetition(r, active)
	if err != nil {
		fail(w, r, err)
		return
	}
	if n == 0 && active == "overall" {
		v, err := p.overall(r, h)
		if err != nil {
			serverError(w, r, err)
			return
		}
		renderPage(w, r, overallTable(v))
		return
	}
	if n < 1 || n > len(h.Runs) {
		notFound(w, r)
		return
	}
	v, err := p.run(r, h, n)
	if err != nil {
		serverError(w, r, err)
		return
	}
	renderPage(w, r, runTable(v))
}

func (p *results) overlayRun(w http.ResponseWriter, r *http.Request) {
	n, ok := strings.CutSuffix(r.PathValue("file"), ".svg")
	num, err := strconv.Atoi(n)
	if !ok || err != nil || num < 1 {
		notFound(w, r)
		return
	}
	h, err := p.loadCompetition(r, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	if num > len(h.Runs) {
		notFound(w, r)
		return
	}
	p.writeOverlay(w, r, h, num)
}

// overlayCurrent is the overlay of the open run (the last one if several).
func (p *results) overlayCurrent(w http.ResponseWriter, r *http.Request) {
	h, err := p.loadCompetition(r, "")
	if err != nil {
		fail(w, r, err)
		return
	}
	cur := 0
	for i, run := range h.Runs {
		if run.State == "open" {
			cur = i + 1
		}
	}
	if cur == 0 {
		writeSVG(w, export.EmptyOverlay())
		return
	}
	p.writeOverlay(w, r, h, cur)
}

// writeOverlay renders the latest saved (published) flight of run n, or an
// empty canvas while the run has none.
func (p *results) writeOverlay(w http.ResponseWriter, r *http.Request, h compHeader, n int) {
	ctx := r.Context()
	run := h.Runs[n-1]
	flightID, err := p.q.LiveLatestFlight(ctx, gen.LiveLatestFlightParams{CompetitionID: h.C.ID, RunIdx: run.Idx})
	if errors.Is(err, sql.ErrNoRows) {
		writeSVG(w, export.EmptyOverlay())
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	// Rank like the run tab does (published flights only, shared ranks) in
	// the overall list, which holds every participant.
	entries, err := p.q.PublicRunEntries(ctx, gen.PublicRunEntriesParams{CompetitionID: h.C.ID, RunIdx: run.Idx})
	if err != nil {
		serverError(w, r, err)
		return
	}
	var prev runCell
	count := 0
	for _, e := range entries {
		if e.ListKey != "overall" {
			continue
		}
		cell := runCell{Rank: sharedRank(count, prev, e.Score), Score: e.Score}
		count, prev = count+1, cell
		if e.FlightID != flightID {
			continue
		}
		pt := newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, nil)
		o := export.Overlay{Run: n, Name: pt.Name, Country: pt.Country, Rank: cell.Rank}
		switch {
		case e.DidNotStart:
			o.Score = "DNS"
		case e.FlightScore.Valid:
			o.Score = score(e.Score)
		}
		writeSVG(w, export.RenderOverlay(o))
		return
	}
	writeSVG(w, export.EmptyOverlay())
}

func writeSVG(w http.ResponseWriter, svg []byte) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(svg)
}
