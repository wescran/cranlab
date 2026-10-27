package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"willnorris.com/go/microformats"

	"github.com/wescran/cranlab/internal/config"
	site "github.com/wescran/cranlab/web"
)

func testConfig() config.Config {
	return config.Config{
		ListenAddr:       "127.0.0.1:8080",
		SiteURL:          "https://cranlab.ca",
		DID:              "did:plc:ogl52fc2rdooori2ayf74can",
		PDSURL:           "http://127.0.0.1:3000",
		PDSPublicURL:     "https://weslandia.ca",
		PublicationURI:   "at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26",
		WebmentionDomain: "cranlab.ca",
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := New(testConfig(), Deps{Now: func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// parseMF parses a response body as microformats, as an IndieWeb reader would.
func parseMF(t *testing.T, body string) *microformats.Data {
	t.Helper()
	base, _ := url.Parse("https://cranlab.ca/")
	return microformats.Parse(strings.NewReader(body), base)
}

func findItem(data *microformats.Data, typ string) *microformats.Microformat {
	for _, item := range data.Items {
		for _, t := range item.Type {
			if t == typ {
				return item
			}
		}
	}
	return nil
}

func property(item *microformats.Microformat, name string) string {
	values := item.Properties[name]
	if len(values) == 0 {
		return ""
	}
	switch v := values[0].(type) {
	case string:
		return v
	case map[string]string:
		return v["value"]
	case map[string]interface{}:
		s, _ := v["value"].(string)
		return s
	}
	return ""
}

func TestHome(t *testing.T) {
	rec := get(t, newTestHandler(t), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>CranLab</title>") {
		t.Error("home <title> should be just CranLab")
	}
	for _, href := range []string{`href="/about"`, `href="/articles"`, `href="/notes"`, `href="/tags"`, `href="/index.xml"`} {
		if !strings.Contains(body, href) {
			t.Errorf("navigation is missing %s", href)
		}
	}

	data := parseMF(t, body)
	card := findItem(data, "h-card")
	if card == nil {
		t.Fatal("no h-card on the home page")
	}
	if got := property(card, "name"); got != "Wesley Cranston" {
		t.Errorf("h-card name = %q", got)
	}
	if got := property(card, "url"); got != "https://cranlab.ca/" {
		t.Errorf("h-card url = %q, want https://cranlab.ca/", got)
	}
	if photo := property(card, "photo"); !strings.HasPrefix(photo, "https://cranlab.ca/static/images/profile.jpeg?v=") {
		t.Errorf("h-card photo = %q, want the versioned profile image", photo)
	}
	if note := property(card, "note"); !strings.Contains(note, "Welcome to CranLab") {
		t.Errorf("h-card note = %q, want the home intro", note)
	}
}

func TestHead(t *testing.T) {
	for _, path := range []string{"/", "/about"} {
		t.Run(path, func(t *testing.T) {
			data := parseMF(t, get(t, newTestHandler(t), path).Body.String())
			wantRel := map[string]string{
				"webmention": "https://webmention.io/cranlab.ca/webmention",
				"me":         "https://github.com/wescran",
			}
			for rel, want := range wantRel {
				if !contains(data.Rels[rel], want) {
					t.Errorf("rel=%s = %v, want %s", rel, data.Rels[rel], want)
				}
			}
			if !contains(data.Rels["me"], "https://bsky.app/profile/cranlab.ca") {
				t.Errorf("rel=me = %v, want the Bluesky profile", data.Rels["me"])
			}
			if !contains(data.Rels["atproto"], "https://bsky.app/profile/cranlab.ca") {
				t.Errorf(`want <link rel="me atproto"> for the Bluesky profile, got rel=atproto %v`, data.Rels["atproto"])
			}
			feed := "https://cranlab.ca/index.xml"
			if !contains(data.Rels["alternate"], feed) || data.RelURLs[feed] == nil || data.RelURLs[feed].Type != "application/rss+xml" {
				t.Errorf("want an RSS autodiscovery link to %s", feed)
			}
			if !hasPrefix(data.Rels["stylesheet"], "https://cranlab.ca/static/css/main.css?v=") {
				t.Errorf("stylesheet = %v, want the versioned main.css", data.Rels["stylesheet"])
			}
			if !hasPrefix(data.Rels["preload"], "https://cranlab.ca/static/fonts/Geist-Variable.woff2?v=") {
				t.Errorf("preload = %v, want the versioned Geist font", data.Rels["preload"])
			}
		})
	}
}

func TestAbout(t *testing.T) {
	rec := get(t, newTestHandler(t), "/about")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /about = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>About Me | CranLab</title>") {
		t.Error(`want <title>About Me | CranLab</title>`)
	}
	if !strings.Contains(body, "My name is Wesley Cranston") {
		t.Error("the About text is missing")
	}
}

func TestNotFound(t *testing.T) {
	for _, path := range []string{"/nope", "/about/more", "/static"} {
		rec := get(t, newTestHandler(t), path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("GET %s Content-Type = %q", path, ct)
		}
		if !strings.Contains(rec.Body.String(), "Page Not Found") {
			t.Errorf("GET %s: want the 404 page", path)
		}
	}
}

func TestServiceUnavailablePage(t *testing.T) {
	h := newTestHandler(t).(*server)
	rec := httptest.NewRecorder()
	h.renderError(rec, httptest.NewRequest(http.MethodGet, "/", nil), http.StatusServiceUnavailable)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "temporarily unavailable") {
		t.Error(`want the 503 page ("temporarily unavailable")`)
	}
}

func TestTrailingSlashRedirect(t *testing.T) {
	tests := map[string]string{
		"/about/":      "/about",
		"/about/?x=1":  "/about?x=1",
		"/articles/a/": "/articles/a",
	}
	for target, want := range tests {
		rec := get(t, newTestHandler(t), target)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s = %d, want 301", target, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != want {
			t.Errorf("GET %s Location = %q, want %q", target, loc, want)
		}
	}
	if rec := get(t, newTestHandler(t), "/"); rec.Code != http.StatusOK {
		t.Errorf("GET / = %d, want 200 (no redirect)", rec.Code)
	}
}

func TestRobots(t *testing.T) {
	rec := get(t, newTestHandler(t), "/robots.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /robots.txt = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "User-agent: *") || !strings.Contains(body, "Allow: /") {
		t.Errorf("robots.txt = %q, want everything allowed", body)
	}
}

func TestHealthz(t *testing.T) {
	rec := get(t, newTestHandler(t), "/healthz")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("GET /healthz = %d %q", rec.Code, rec.Body.String())
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / = %d, want 405", rec.Code)
	}
}

func TestStaticFromPage(t *testing.T) {
	h := newTestHandler(t)
	css := regexp.MustCompile(`/static/css/main\.css\?v=[0-9a-f]{12}`).FindString(get(t, h, "/").Body.String())
	if css == "" {
		t.Fatal("no versioned main.css link on the home page")
	}
	rec := get(t, h, css)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", css, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", cc)
	}
	want, _ := fs.ReadFile(site.FS, "static/css/main.css")
	got, _ := io.ReadAll(rec.Body)
	if string(got) != string(want) {
		t.Error("served CSS differs from web/static/css/main.css")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func hasPrefix(list []string, prefix string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
