package web

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"awt/internal/cache"
	"awt/internal/service"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// purgeDelay coalesces the events of a burst (a judge saving several
// flights) into one purge call.
const purgeDelay = 500 * time.Millisecond

// URLResolver lists the public URLs an event invalidates.
type URLResolver func(ctx context.Context, e service.Event) ([]string, error)

// StartPurger subscribes to every Bus event and purges the CDN copies of the
// public pages and API responses they change, until ctx is done (pending
// topics are flushed first). The subscription is in place when it returns.
//
// Cloudflare's free plan purges by exact URL only, so query-string variants
// (/competitions/{code}?list=...) cannot be enumerated: the base URL is purged
// and the variants rely on the short max-age of live pages (30 s). The same
// bound covers the rare event the Bus drops for the purger (its buffer is
// large and drops are logged): a stale page lives at most one max-age.
func StartPurger(ctx context.Context, bus *service.Bus, purger cache.Purger, db *store.DB, baseURL string) {
	if bus == nil || purger == nil {
		return
	}
	q := gen.New(db.Read)
	startPurger(ctx, bus, purger, purgeDelay, func(ctx context.Context, e service.Event) ([]string, error) {
		return PurgeURLs(ctx, q, baseURL, e)
	})
}

// startPurger runs two goroutines: the receiver only records the topics of
// incoming events (no database or HTTP work, so the Bus buffer keeps
// draining), and the worker resolves and purges them every delay.
func startPurger(ctx context.Context, bus *service.Bus, purger cache.Purger, delay time.Duration, resolve URLResolver) {
	events, unsubscribe := bus.SubscribeAll()
	var (
		mu      sync.Mutex
		pending = map[string]service.Event{} // by topic
		wake    = make(chan struct{}, 1)
		done    = make(chan struct{}) // closed when the receiver has stopped
	)
	record := func(e service.Event) {
		mu.Lock()
		pending[e.Topic] = e
		mu.Unlock()
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	go func() {
		defer close(done)
		defer unsubscribe()
		var lastDropped int64
		for {
			select {
			case <-ctx.Done():
				for { // events already buffered
					select {
					case e := <-events:
						record(e)
					default:
						return
					}
				}
			case e := <-events:
				record(e)
				if d := bus.DroppedAll(); d != lastDropped {
					slog.Warn("cdn purge: events dropped", "total", d)
					lastDropped = d
				}
			}
		}
	}()
	flush := func() {
		mu.Lock()
		batch := pending
		pending = map[string]service.Event{}
		mu.Unlock()
		if len(batch) == 0 {
			return
		}
		// a purge started at shutdown must still complete
		pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		set := map[string]struct{}{}
		for _, e := range batch {
			urls, err := resolve(pctx, e)
			if err != nil {
				slog.Error("cdn purge: resolve", "topic", e.Topic, "err", err)
				continue
			}
			for _, u := range urls {
				set[u] = struct{}{}
			}
		}
		if len(set) == 0 {
			return
		}
		urls := make([]string, 0, len(set))
		for u := range set {
			urls = append(urls, u)
		}
		sort.Strings(urls)
		if err := purger.Purge(pctx, urls); err != nil {
			slog.Error("cdn purge", "urls", len(urls), "err", err)
		}
	}
	go func() {
		for {
			select {
			case <-done:
				flush()
				return
			case <-wake:
				select { // coalesce the burst
				case <-time.After(delay):
				case <-done:
				}
				flush()
			}
		}
	}()
}

// PurgeURLs is the set of absolute public URLs affected by an event.
func PurgeURLs(ctx context.Context, q *gen.Queries, baseURL string, e service.Event) ([]string, error) {
	base := strings.TrimRight(baseURL, "/")
	kind, code, _ := strings.Cut(e.Topic, ":")
	var paths []string
	switch kind {
	case "season":
		paths = []string{"/results", "/seasons/" + code, "/api/v1/seasons/" + code}
	case "competition":
		c, err := q.GetCompetitionByCode(ctx, code)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		runs, err := q.ListRuns(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		seasons, err := q.PublicCompetitionSeasons(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		paths = []string{"/results", "/competitions",
			"/competitions/" + code, "/competitions/" + code + "/details", "/api/v1/competitions/" + code}
		for i := range runs {
			n := strconv.Itoa(i + 1)
			paths = append(paths, "/competitions/"+code+"/runs/"+n, "/api/v1/competitions/"+code+"/runs/"+n)
		}
		for _, s := range seasons {
			paths = append(paths, "/seasons/"+s.SeasonCode, "/api/v1/seasons/"+s.SeasonCode)
		}
	default:
		return nil, nil
	}
	urls := make([]string, len(paths))
	for i, p := range paths {
		urls[i] = base + p
	}
	return urls, nil
}
