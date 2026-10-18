package web

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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
// public pages and API responses they change, until ctx is done. The
// subscription is in place when it returns.
//
// Cloudflare's free plan purges by exact URL only, so query-string variants
// (/competitions/{code}?list=…) cannot be enumerated: the base URL is purged
// and the variants rely on the short max-age of live pages (30 s).
func StartPurger(ctx context.Context, bus *service.Bus, purger cache.Purger, db *store.DB, baseURL string) {
	if bus == nil || purger == nil {
		return
	}
	q := gen.New(db.Read)
	startPurger(ctx, bus, purger, purgeDelay, func(ctx context.Context, e service.Event) ([]string, error) {
		return PurgeURLs(ctx, q, baseURL, e)
	})
}

func startPurger(ctx context.Context, bus *service.Bus, purger cache.Purger, delay time.Duration, resolve URLResolver) {
	events, unsubscribe := bus.SubscribeAll()
	go func() {
		defer unsubscribe()
		pending := map[string]struct{}{}
		var timer <-chan time.Time
		flush := func() {
			timer = nil
			urls := make([]string, 0, len(pending))
			for u := range pending {
				urls = append(urls, u)
			}
			clear(pending)
			sort.Strings(urls)
			pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			if err := purger.Purge(pctx, urls); err != nil {
				slog.Error("cdn purge", "urls", len(urls), "err", err)
			}
		}
		for {
			select {
			case <-ctx.Done():
				if len(pending) > 0 {
					flush()
				}
				return
			case e, ok := <-events:
				if !ok {
					return
				}
				urls, err := resolve(ctx, e)
				if err != nil {
					slog.Error("cdn purge: resolve", "topic", e.Topic, "err", err)
					continue
				}
				for _, u := range urls {
					pending[u] = struct{}{}
				}
				if timer == nil && len(pending) > 0 {
					timer = time.After(delay)
				}
			case <-timer:
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
