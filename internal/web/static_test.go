package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"testing/fstest"
)

func TestStaticURL(t *testing.T) {
	s, err := newStatic(fstest.MapFS{"css/main.css": {Data: []byte("body{}")}})
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.url("css/main.css")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^/static/css/main\.css\?v=[0-9a-f]{12}$`).MatchString(u) {
		t.Errorf("url = %q, want /static/css/main.css?v=<12 hex>", u)
	}
	if _, err := s.url("css/missing.css"); err == nil {
		t.Error("url of a missing file: want an error")
	}
}

func TestStaticURLChangesWithContent(t *testing.T) {
	a, _ := newStatic(fstest.MapFS{"x.css": {Data: []byte("a")}})
	b, _ := newStatic(fstest.MapFS{"x.css": {Data: []byte("b")}})
	ua, _ := a.url("x.css")
	ub, _ := b.url("x.css")
	if ua == ub {
		t.Errorf("same URL %q for different content", ua)
	}
	again, _ := newStatic(fstest.MapFS{"x.css": {Data: []byte("a")}})
	if u, _ := again.url("x.css"); u != ua {
		t.Errorf("URL not stable: %q then %q", ua, u)
	}
}

func TestStaticServe(t *testing.T) {
	s, err := newStatic(fstest.MapFS{
		"css/main.css":            {Data: []byte("body{}")},
		"fonts/Geist.woff2":       {Data: []byte("woff2")},
		"icons/maple-leaf.svg":    {Data: []byte("<svg/>")},
		"images/profile.jpeg":     {Data: []byte("jpeg")},
		"images/card-default.png": {Data: []byte("png")},
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /static/{path...}", s)

	types := map[string]string{
		"/static/css/main.css":            "text/css; charset=utf-8",
		"/static/fonts/Geist.woff2":       "font/woff2",
		"/static/icons/maple-leaf.svg":    "image/svg+xml",
		"/static/images/profile.jpeg":     "image/jpeg",
		"/static/images/card-default.png": "image/png",
	}
	for path, want := range types {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?v=anything", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); ct != want {
			t.Errorf("GET %s Content-Type = %q, want %q", path, ct, want)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
			t.Errorf("GET %s Cache-Control = %q", path, cc)
		}
	}

	for _, path := range []string{"/static/css/missing.css", "/static/css", "/static/../go.mod"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		// 404, or a redirect from the router cleaning the path; never the file.
		if rec.Code == http.StatusOK {
			t.Errorf("GET %s = 200, want it refused", path)
		}
	}
}
