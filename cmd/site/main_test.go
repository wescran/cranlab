package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/config"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
)

func TestStartupBackfill(t *testing.T) {
	const did = "did:plc:owner"
	fake := pdstest.New(t, did)
	cfg := config.Config{DID: did, PublicationURI: "at://" + did + "/site.standard.publication/abc", PDSURL: fake.URL, CacheDBPath: filepath.Join(t.TempDir(), "cache.db")}
	start := func() (http.Handler, func()) {
		t.Helper()
		h, close, err := siteHandler(t.Context(), cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		return h, close
	}
	check := func(h http.Handler, path string, want int, contains string) {
		t.Helper()
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != want || !strings.Contains(r.Body.String(), contains) {
			t.Fatalf("GET %s = %d, want %d containing %q", path, r.Code, want, contains)
		}
	}
	fake.FailWith(503)
	h, close := start()
	check(h, "/articles/welcome", 503, "Temporarily unavailable")
	check(h, "/index.xml", 503, "Temporarily unavailable")
	check(h, "/notes/missing", 503, "Temporarily unavailable")
	close()
	fake.FailWith(0)
	fake.Put(pds.PostCollection, "top", map[string]string{"text": "Last-good Note"})
	fake.Put(pds.DocumentCollection, "doc", pds.Document{Site: cfg.PublicationURI, Path: "/articles/welcome", Title: "Welcome back", Tags: []string{" Intro ", "INTRO"}})
	h, close = start()
	check(h, "/articles/welcome", 200, "Welcome back")
	close()
	fake.FailWith(503)
	h, close = start()
	defer close()
	before := len(fake.Requests())
	check(h, "/articles", 200, "Welcome back")
	check(h, "/notes", 200, "Last-good Note")
	check(h, "/notes/top", 200, "Last-good Note")
	check(h, "/notes/missing", 404, "Page Not Found")
	check(h, "/index.xml", 200, "<title>Welcome back</title>")
	check(h, "/articles/welcome", 200, "Welcome back")
	check(h, "/tags", 200, `intro <span>1</span>`)
	check(h, "/tags/intro", 200, "Welcome back")
	check(h, "/articles/gone", 404, "Page Not Found")
	for _, u := range fake.Requests()[before:] {
		// The background listener may be connecting while pages are served.
		if u.Path != "/xrpc/com.atproto.sync.subscribeRepos" {
			t.Fatalf("page views made an upstream request: %s", u.Path)
		}
	}
}
