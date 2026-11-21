// Package web serves the site: routes, page rendering and static files.
package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wescran/cranlab/internal/config"
	"github.com/wescran/cranlab/internal/content"
	site "github.com/wescran/cranlab/web"
)

// SiteTitle is the site's name, used in <title> and the RSS feed.
const SiteTitle = "CranLab"

// Deps are the server's collaborators.
type Deps struct {
	Store content.ContentStore
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
	// Blobs streams owner media.
	Blobs BlobReader
}

// Page is the data every page template receives.
type Page struct {
	// Title is the page's title; empty on the home page.
	Title string
	// Description fills <meta name="description">.
	Description  string
	CanonicalURL string
	CoverCID     string
	// DocumentID is an Article record identifier without the literal at:// scheme.
	DocumentID string
	// Path is the request path, used to mark the current navigation item.
	Path string
	// Year is the current year, for the footer.
	Year int
	// Config contains only publicly visible template settings.
	Config PublicConfig
	// Data is the page's own content: one of the view types in views.go.
	Data any
}

type PublicConfig struct{ SiteURL, WebmentionDomain string }

type server struct {
	cfg    config.Config
	deps   Deps
	static *static
	pages  map[string]*template.Template // keyed by page name
	mux    *http.ServeMux
}

// New returns the site's handler. Templates are parsed up front, so a broken
// template fails at startup rather than on a request.
func New(cfg config.Config, deps Deps) (http.Handler, error) {
	mux := http.NewServeMux()
	staticFS, err := fs.Sub(site.FS, "static")
	if err != nil {
		return nil, err
	}
	staticHandler, err := newStatic(staticFS)
	if err != nil {
		return nil, err
	}
	pages, err := parsePages(site.FS, templateFuncs(staticHandler))
	if err != nil {
		return nil, err
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	s := &server{
		cfg:    cfg,
		deps:   deps,
		static: staticHandler,
		pages:  pages,
		mux:    mux,
	}
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		s.renderError(w, r, http.StatusNotFound)
	})
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, "about", http.StatusOK, Page{
			Description: "About Wesley Cranston",
			Title:       "About Me",
		})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /.well-known/site.standard.publication", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(cfg.PublicationURI))
	})
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	})
	// Without this exact route, the router redirects /static to /static/,
	// which the trailing-slash redirect sends straight back.
	mux.HandleFunc("GET /static", func(w http.ResponseWriter, r *http.Request) {
		s.renderError(w, r, http.StatusNotFound)
	})
	mux.Handle("GET /static/{path...}", staticHandler)
	snapshotRoute := func(pattern string, handler http.HandlerFunc) {
		if deps.Store == nil {
			handler = func(w http.ResponseWriter, r *http.Request) { s.unavailable(w, r, content.ErrNoSnapshot) }
		}
		mux.HandleFunc(pattern, handler)
	}
	snapshotRoute("GET /{$}", s.home)
	snapshotRoute("GET /index.xml", s.feed)
	snapshotRoute("GET /articles", s.articles)
	snapshotRoute("GET /articles/{slug}", s.article)
	snapshotRoute("GET /notes", s.notes)
	snapshotRoute("GET /notes/{rkey}", s.note)
	snapshotRoute("GET /tags", s.tags)
	snapshotRoute("GET /tags/{tag}", s.tag)
	mux.HandleFunc("GET /media/{cid}", s.media)

	return s, nil
}

// ServeHTTP permanently redirects paths with a trailing slash to the same path
// without it, then routes the request.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rPath := r.URL.EscapedPath()
	if strings.HasSuffix(rPath, "/") && rPath != "/" && !strings.HasPrefix(rPath, "/blog/") {
		// Exactly one leading slash: Location must be a path, never an authority.
		target := "/" + strings.Trim(rPath, "/")
		if r.URL.RawQuery != "" || r.URL.ForceQuery {
			target += "?" + r.URL.RawQuery
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Location", target)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}
	// Successful public pages (including the feed and publication identity)
	// share a short freshness window and can be served stale on an outage.
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/media/"):
			w.Header().Set("Cache-Control", immutableCache)
		case strings.HasPrefix(path, "/static/"), path == "/healthz", path == "/robots.txt":
			// Static verifies the content version itself. Leave utility endpoints alone.
		default:
			w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=60, stale-if-error=604800")
		}
	}
	s.mux.ServeHTTP(cacheWriter{w}, r)
}

const immutableCache = "public, max-age=31536000, immutable"

// cacheWriter replaces the success policy before an error is sent. This also
// covers plain-text errors (e.g. from media/static) without buffering a body.
type cacheWriter struct{ http.ResponseWriter }

func (w cacheWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w cacheWriter) WriteHeader(status int) {
	switch {
	case status == http.StatusNotFound:
		w.Header().Set("Cache-Control", "public, max-age=60")
	case status >= 500:
		w.Header().Set("Cache-Control", "no-store")
	case status >= 300 && status != http.StatusNotModified:
		w.Header().Del("Cache-Control")
	}
	w.ResponseWriter.WriteHeader(status)
}

// renderError writes the 404 or 503 page with that status.
func (s *server) renderError(w http.ResponseWriter, r *http.Request, status int) {
	switch status {
	case http.StatusNotFound:
		s.render(w, r, "404", http.StatusNotFound, Page{
			Title: "Page Not Found",
		})
	case http.StatusServiceUnavailable:
		s.render(w, r, "503", http.StatusServiceUnavailable, Page{
			Title: "Temporarily unavailable",
		})
	}
}

// render executes a page template into a buffer and writes it only if that
// succeeded, so a failure never sends a half-written page.
func (s *server) render(w http.ResponseWriter, r *http.Request, name string, status int, p Page) {
	p.Path = r.URL.Path
	if p.CanonicalURL == "" {
		p.CanonicalURL = s.cfg.SiteURL + r.URL.EscapedPath()
	}
	p.Config = PublicConfig{SiteURL: s.cfg.SiteURL, WebmentionDomain: s.cfg.WebmentionDomain}
	p.Year = s.deps.Now().Year()

	var buf bytes.Buffer
	if err := s.pages[name].Execute(&buf, p); err != nil {
		slog.Error("render", "page", name, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
