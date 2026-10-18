package civl

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFetchPilot(t *testing.T) {
	var headCalls int
	var ua string
	mux := http.NewServeMux()
	mux.HandleFunc("/pilot/90001", func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Write(fixture(t, "pilot.html"))
	})
	mux.HandleFunc("/uploads/images/profile/alex.jpg", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headCalls++
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// highres lives on the CDN host; point the fixture's photo to the test server
	page := strings.ReplaceAll(string(fixture(t, "pilot.html")), "https://cdn.example.test", srv.URL)
	mux.HandleFunc("/pilot/90002", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, page) })

	c := New(srv.Client(), srv.URL)
	got, err := c.FetchPilot(context.Background(), 90002)
	if err != nil {
		t.Fatal(err)
	}
	want := PilotData{
		CivlID:            90002,
		Name:              "Alex Example",
		Country:           "fra",
		CivlLink:          srv.URL + "/pilot/90002",
		About:             "Fictional pilot   from nowhere. Loves infinite tumbling.",
		Photo:             srv.URL + "/uploads/resize/profile/header/alex.jpg",
		PhotoHighres:      srv.URL + "/uploads/images/profile/alex.jpg",
		BackgroundPicture: srv.URL + "/uploads/bg/alex.jpg",
		SocialLinks: []Link{
			{"facebook", "https://facebook.example.test/alex"},
			{"instagram", "https://instagram.example.test/alex"},
			{"Personal site", "https://alex.example.test"},
		},
		Sponsors: []Sponsor{
			{"Wing Co", "https://wing.example.test", srv.URL + "/s/wing.png"},
			{"Harness Inc", "", srv.URL + "/s/harness.png"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if headCalls != 1 {
		t.Errorf("HEAD calls = %d, want 1", headCalls)
	}
	if ua != "" && ua != "awt (+https://www.acroworldtour.com)" {
		t.Errorf("user agent %q", ua)
	}
}

func TestFetchPilotHighresMissingFallsBack(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("/pilot/1", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.ReplaceAll(string(fixture(t, "pilot.html")), "https://cdn.example.test", srvURL))
	})
	srv := httptest.NewServer(mux) // highres path is a 404
	defer srv.Close()
	srvURL = srv.URL
	got, err := New(srv.Client(), srv.URL).FetchPilot(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.PhotoHighres != got.Photo || got.Photo == "" {
		t.Errorf("highres %q photo %q", got.PhotoHighres, got.Photo)
	}
}

func TestFetchPilotUserAgent(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Write(fixture(t, "pilot.html"))
	}))
	defer srv.Close()
	if _, err := New(srv.Client(), srv.URL).FetchPilot(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if ua != "awt (+https://www.acroworldtour.com)" {
		t.Errorf("user agent %q", ua)
	}
}

func TestFetchPilotErrors(t *testing.T) {
	for _, code := range []int{404, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		_, err := New(srv.Client(), srv.URL).FetchPilot(context.Background(), 42)
		srv.Close()
		var se *StatusError
		if !errors.As(err, &se) || se.Status != code {
			t.Fatalf("code %d: err = %v", code, err)
		}
		if !strings.Contains(err.Error(), srv.URL+"/pilot/42") || !strings.Contains(err.Error(), strconv.Itoa(code)) {
			t.Errorf("error %q lacks url/status", err)
		}
	}
}

func TestFetchPilotMissingName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html><body></body></html>") }))
	defer srv.Close()
	if _, err := New(srv.Client(), srv.URL).FetchPilot(context.Background(), 1); err == nil {
		t.Fatal("want error")
	}
}

func TestFetchPilotContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(fixture(t, "pilot.html")) }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(srv.Client(), srv.URL).FetchPilot(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

type rankingServer struct {
	*httptest.Server
	postForm   map[string][]string
	postCSRF   string
	postCookie string
}

func newRankingServer(t *testing.T, exportStatus int) *rankingServer {
	rs := &rankingServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ranking/paragliding-aerobatics/pilots", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sess", Value: "abc"})
		io.WriteString(w, strings.ReplaceAll(string(fixture(t, "ranking.html")), "__BASE__", rs.URL))
	})
	mux.HandleFunc("/ranking/export-new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		r.ParseForm()
		rs.postForm = r.PostForm
		rs.postCSRF = r.Header.Get("X-CSRF-Token")
		if ck, err := r.Cookie("sess"); err == nil {
			rs.postCookie = ck.Value
		}
		if r.URL.Query().Get("type") != "pilots" || strings.Contains(r.URL.RawQuery, "async") {
			t.Errorf("export query %q", r.URL.RawQuery)
		}
		if exportStatus != 200 {
			w.WriteHeader(exportStatus)
			return
		}
		w.Write(fixture(t, "ranking.xlsx"))
	})
	rs.Server = httptest.NewServer(mux)
	t.Cleanup(rs.Close)
	return rs
}

func TestRankingSession(t *testing.T) {
	rs := newRankingServer(t, 200)
	s, err := New(rs.Client(), rs.URL).rankingSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.exportURL != rs.URL+"/ranking/export-new?type=pilots&discipline=5" || s.csrf != "tok-123" {
		t.Errorf("%+v", s)
	}
	if len(s.cookies) != 1 || s.cookies[0].Name != "sess" {
		t.Errorf("cookies %v", s.cookies)
	}
}

func TestRankingSessionMissingParts(t *testing.T) {
	for name, page := range map[string]string{
		"no url":  `<meta name="csrf-token" content="x">`,
		"no csrf": `<script>var u = 'https://h.test/ranking/export-new?a=1&async=1';</script>`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, page) }))
		_, err := New(srv.Client(), srv.URL).rankingSession(context.Background())
		srv.Close()
		if err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestFetchRankings(t *testing.T) {
	rs := newRankingServer(t, 200)
	now := time.Date(2026, 10, 18, 13, 47, 0, 0, time.UTC)
	got, err := New(rs.Client(), rs.URL).fetchRankings(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]int{90001: 1, 90002: 2, 90003: 3, 90004: 9999}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if rs.postCSRF != "tok-123" || rs.postCookie != "abc" {
		t.Errorf("csrf %q cookie %q", rs.postCSRF, rs.postCookie)
	}
	f := rs.postForm
	if f["search[rankingDate]"][0] != "2026-10-01" || f["date_[month]"][0] != "10" || f["date_[year]"][0] != "2026" || f["date_[day]"][0] != "01" {
		t.Errorf("form %v", f)
	}
}

func TestFetchRankingsExportStatus(t *testing.T) {
	rs := newRankingServer(t, 403)
	_, err := New(rs.Client(), rs.URL).FetchRankings(context.Background())
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 403 || !strings.Contains(err.Error(), "/ranking/export-new") || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchRankingsPageStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	_, err := New(srv.Client(), srv.URL).FetchRankings(context.Background())
	if err == nil || !strings.Contains(err.Error(), "/ranking/paragliding-aerobatics/pilots") || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRankingXLSXGarbage(t *testing.T) {
	if _, err := parseRankingXLSX([]byte("not xlsx")); err == nil {
		t.Fatal("want error")
	}
}
