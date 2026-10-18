package web_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"awt/internal/cache"
	"awt/internal/service"
	"awt/internal/store/gen"
	"awt/internal/web"
)

func sseApp(t *testing.T, mutate func(*web.SSE)) (*httptest.Server, *web.SSE, *service.Bus) {
	t.Helper()
	deps := newDeps(t)
	bus := service.NewBus()
	s := &web.SSE{Bus: bus, Ping: 50 * time.Millisecond,
		Exists: func(_ context.Context, code string) (bool, error) { return code == "live-cup", nil }}
	if mutate != nil {
		mutate(s)
	}
	srv := httptest.NewServer(web.Routes(deps, web.ModuleFunc(func(r *web.Router) {
		r.Public("GET /live/competitions/{code}", s)
	})))
	t.Cleanup(srv.Close)
	return srv, s, bus
}

// lines reads the stream line by line on a channel.
func lines(body *bufio.Scanner) <-chan string {
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		for body.Scan() {
			ch <- body.Text()
		}
	}()
	return ch
}

func waitFor(t *testing.T, ch <-chan string, want string, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case l, ok := <-ch:
			if !ok {
				t.Fatalf("stream closed before %q", want)
			}
			if l == want {
				return
			}
		case <-deadline:
			t.Fatalf("no %q within %s", want, within)
		}
	}
}

func connect(t *testing.T, srv *httptest.Server, code string) (*http.Response, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/live/competitions/"+code, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = resp.Body.Close() })
	return resp, cancel
}

func TestSSEUpdateAndPing(t *testing.T) {
	srv, _, bus := sseApp(t, nil)
	resp, _ := connect(t, srv, "live-cup")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for k, v := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	ch := lines(bufio.NewScanner(resp.Body))
	waitFor(t, ch, ": ping", time.Second) // also proves the stream is subscribed
	bus.Publish(service.Event{Topic: "competition:live-cup", Kind: service.KindFlight, At: time.Now()})
	waitFor(t, ch, "event: update", time.Second)
	waitFor(t, ch, "data: flight", time.Second)
}

func TestSSEIgnoresOtherTopics(t *testing.T) {
	srv, _, bus := sseApp(t, func(s *web.SSE) { s.Ping = time.Hour })
	resp, _ := connect(t, srv, "live-cup")
	ch := lines(bufio.NewScanner(resp.Body))
	waitFor(t, ch, "", time.Second) // the retry preamble's blank line
	bus.Publish(service.Event{Topic: "competition:other", Kind: service.KindRun})
	bus.Publish(service.Event{Topic: "competition:live-cup", Kind: service.KindRun})
	waitFor(t, ch, "data: run", time.Second)
}

func TestSSEDisconnectUnsubscribes(t *testing.T) {
	srv, s, bus := sseApp(t, nil)
	var wg sync.WaitGroup
	cancels := make([]context.CancelFunc, 5)
	for i := range cancels {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, cancels[i] = connect(t, srv, "live-cup")
		}()
	}
	wg.Wait()
	if n := s.Active(); n != 5 {
		t.Fatalf("active = %d, want 5", n)
	}
	for _, c := range cancels {
		c()
	}
	deadline := time.Now().Add(2 * time.Second)
	for s.Active() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := s.Active(); n != 0 {
		t.Fatalf("active after disconnect = %d, want 0", n)
	}
	// no subscriber left: publishing drops nothing and never blocks
	for range 100 {
		bus.Publish(service.Event{Topic: "competition:live-cup", Kind: service.KindFlight})
	}
	if bus.Dropped() != 0 {
		t.Errorf("events dropped for a stale subscriber: %d", bus.Dropped())
	}
}

func TestSSEUnknownCompetitionIs404(t *testing.T) {
	srv, s, _ := sseApp(t, nil)
	resp, err := http.Get(srv.URL + "/live/competitions/hidden-cup")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status %d, want 404", resp.StatusCode)
	}
	if s.Active() != 0 {
		t.Errorf("active = %d", s.Active())
	}
}

func TestSSEConnectionLimit(t *testing.T) {
	srv, s, _ := sseApp(t, func(s *web.SSE) { s.MaxConns = 1 })
	connect(t, srv, "live-cup")
	resp, err := http.Get(srv.URL + "/live/competitions/live-cup")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("status %d, want 503", resp.StatusCode)
	}
	if s.Active() != 1 {
		t.Errorf("active = %d, want 1", s.Active())
	}
}

type fakePurger struct {
	mu    sync.Mutex
	calls [][]string
}

func (f *fakePurger) Purge(_ context.Context, urls []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, urls)
	return nil
}

func (f *fakePurger) all() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c...)
	}
	slices.Sort(out)
	return out
}

var _ cache.Purger = (*fakePurger)(nil)

func TestPurgeSubscriber(t *testing.T) {
	deps := newDeps(t)
	ctx := context.Background()
	q := gen.New(deps.DB.Write)
	if err := q.UpsertCompetition(ctx, gen.UpsertCompetitionParams{ID: "c1", Code: "live-cup", Name: "Live", Type: "solo",
		State: "open", StartDate: "2026-06-01", EndDate: "2026-06-02", Published: true, Config: "{}"}); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if err := q.UpsertRun(ctx, gen.UpsertRunParams{CompetitionID: "c1", Idx: int64(i), State: "open", Config: "{}"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.AddCompetitionSeason(ctx, gen.AddCompetitionSeasonParams{CompetitionID: "c1", Position: 0, SeasonCode: "aw-2026"}); err != nil {
		t.Fatal(err)
	}

	bus := service.NewBus()
	fp := &fakePurger{}
	pctx, stop := context.WithCancel(ctx)
	defer stop()
	web.StartPurger(pctx, bus, fp, deps.DB, "https://results.test/")
	// two events of a burst and one unknown competition: one purge call
	bus.Publish(service.Event{Topic: "competition:live-cup", Kind: service.KindFlight})
	bus.Publish(service.Event{Topic: "competition:live-cup", Kind: service.KindRun})
	bus.Publish(service.Event{Topic: "competition:ghost", Kind: service.KindRun})
	bus.Publish(service.Event{Topic: "season:aw-2026", Kind: service.KindCompetition})

	want := []string{
		"https://results.test/api/v1/competitions/live-cup",
		"https://results.test/api/v1/competitions/live-cup/runs/1",
		"https://results.test/api/v1/competitions/live-cup/runs/2",
		"https://results.test/api/v1/seasons/aw-2026",
		"https://results.test/competitions",
		"https://results.test/competitions/live-cup",
		"https://results.test/competitions/live-cup/details",
		"https://results.test/competitions/live-cup/runs/1",
		"https://results.test/competitions/live-cup/runs/2",
		"https://results.test/results",
		"https://results.test/seasons/aw-2026",
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(fp.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := fp.all(); !slices.Equal(got, want) {
		t.Errorf("purged URLs:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(fp.calls) != 1 {
		t.Errorf("purge calls = %d, want 1 (coalesced)", len(fp.calls))
	}
}
