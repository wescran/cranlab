package web

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"willnorris.com/go/microformats"

	"github.com/wescran/cranlab/internal/config"
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
	return newFixture(t).handler
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
	if got := property(card, "uid"); got != "https://cranlab.ca/" {
		t.Errorf("representative h-card uid = %q, want canonical home URL", got)
	}
	if photo := property(card, "photo"); !strings.HasPrefix(photo, "https://cranlab.ca/static/images/profile-200.jpeg?v=") {
		t.Errorf("h-card photo = %q, want the versioned profile image", photo)
	}
	if note := property(card, "note"); !strings.Contains(note, "Welcome to CranLab") {
		t.Errorf("h-card note = %q, want the home intro", note)
	}
	if !strings.Contains(body, "&copy; 2026 Wesley Cranston") {
		t.Error("footer should show © with the current year (from Deps.Now)")
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
				t.Errorf("preload = %v, want the Geist font at the URL main.css uses", data.Rels["preload"])
			}
		})
	}
}

func TestAbout(t *testing.T) {
	rec := get(t, newTestHandler(t), "/about")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /about = %d, want 200", rec.Code)
	}
	if got := rec.Result().Header.Get("Cache-Control"); got != "public, max-age=60, stale-while-revalidate=60, stale-if-error=604800" {
		t.Errorf("GET /about Cache-Control = %q", got)
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
	h := newTestHandler(t)
	for _, path := range []string{"/nope", "/about/more", "/static", "/blog/old-post", "/blog/old-post/", "/absent%2F"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("GET %s Content-Type = %q", path, ct)
		}
		if got := rec.Result().Header.Get("Cache-Control"); got != "public, max-age=60" {
			t.Errorf("GET %s Cache-Control = %q", path, got)
		}
		if !strings.Contains(rec.Body.String(), "Page Not Found") {
			t.Errorf("GET %s: want the 404 page", path)
		}
	}
}

func TestServiceUnavailablePage(t *testing.T) {
	h := newTestHandler(t).(*server)
	h.mux.HandleFunc("GET /test-outage", func(w http.ResponseWriter, r *http.Request) {
		h.unavailable(w, r, errors.New("source unavailable"))
	})
	rec := get(t, h, "/test-outage")
	if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("503 Cache-Control = %q", got)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "temporarily unavailable") {
		t.Error(`want the 503 page ("temporarily unavailable")`)
	}
}

func TestTrailingSlashRedirect(t *testing.T) {
	tests := map[string]string{
		"/about/":                  "/about",
		"/about/?x=1":              "/about?x=1",
		"/articles/a/":             "/articles/a",
		"/tags/a%23b/?q=a%2Bb&x=1": "/tags/a%23b?q=a%2Bb&x=1",
		"/tags/a%2F/":              "/tags/a%2F",
		"//elsewhere.example/foo/": "/elsewhere.example/foo",
		"///?":                     "/?",
	}
	h := newTestHandler(t)
	for target, want := range tests {
		rec := get(t, h, target)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s = %d, want 301", target, rec.Code)
		}
		if got := rec.Result().Header.Get("Cache-Control"); got != "public, max-age=86400" {
			t.Errorf("GET %s Cache-Control = %q", target, got)
		}
		if loc := rec.Header().Get("Location"); loc != want {
			t.Errorf("GET %s Location = %q, want %q", target, loc, want)
		}
	}
}

// Use a streaming handler behind the real HTTP policy so these checks don't
// depend on the feed, identity and blob readers or their backing data.
func TestRouteCachePolicy(t *testing.T) {
	s := &server{mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("down") {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "fixture")
	})
	const page = "public, max-age=60, stale-while-revalidate=60, stale-if-error=604800"
	for _, tc := range []struct {
		path   string
		status int
		cache  string
	}{
		{"/", 200, page},
		{"/index.xml", 200, page},
		{"/.well-known/site.standard.publication", 200, page},
		{"/media/" + catCID, 200, "public, max-age=31536000, immutable"},
		{"/media/" + catCID + "?down", 503, "no-store"},
	} {
		rec := get(t, s, tc.path)
		if got := rec.Result().Header.Get("Cache-Control"); rec.Code != tc.status || got != tc.cache {
			t.Errorf("GET %s = %d, Cache-Control %q; want %d, %q", tc.path, rec.Code, got, tc.status, tc.cache)
		}
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
	page := get(t, h, "/about").Body.String()
	css := regexp.MustCompile(`/static/css/main\.css\?v=[0-9a-f]{12}`).FindString(page)
	if css == "" {
		t.Fatal("no versioned main.css link on the about page")
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
	for _, path := range []string{"/static/css/main.css", "/static/css/main.css?v=stale"} {
		asset := get(t, h, path)
		if got := asset.Result().Header.Get("Cache-Control"); asset.Code != http.StatusOK || strings.Contains(got, "immutable") {
			t.Errorf("GET %s = %d, Cache-Control = %q; want a mutable asset", path, asset.Code, got)
		}
	}
	hash := sha256.Sum256(rec.Body.Bytes())
	if !strings.HasSuffix(css, fmt.Sprintf("?v=%x", hash[:6])) {
		t.Error("stylesheet URL does not version the served bytes")
	}
	preload := regexp.MustCompile(`<link rel="preload" href="(/static/fonts/Geist-Variable\.woff2\?v=[0-9a-f]{12})"`).FindStringSubmatch(page)
	if preload == nil || !strings.Contains(rec.Body.String(), preload[1]) {
		t.Fatal("body font preload and stylesheet must reference the same versioned URL")
	}
	for _, name := range []string{"Geist-Variable", "Geist-Italic-Variable", "GeistMono-Variable", "GeistMono-Italic-Variable"} {
		font := regexp.MustCompile(`/static/fonts/` + name + `\.woff2\?v=[0-9a-f]{12}`).FindString(rec.Body.String())
		if font == "" {
			t.Fatalf("no versioned font URL for %s", name)
		}
		r := get(t, h, font)
		if r.Code != 200 || !strings.Contains(r.Header().Get("Cache-Control"), "immutable") {
			t.Errorf("font %s: %d, Cache-Control %q", font, r.Code, r.Header().Get("Cache-Control"))
		}
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
