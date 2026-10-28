// Package web serves the site: routes, page rendering and static files.
package web

import (
	"html/template"
	"net/http"
	"time"

	"github.com/wescran/cranlab/internal/config"
)

// SiteTitle is the site's name, used in <title> and the RSS feed.
const SiteTitle = "CranLab"

// Deps are the server's collaborators. Later tasks add the PDS client and the
// webmention client.
type Deps struct {
	// Now returns the current time; tests pass a fixed clock. Nil means time.Now.
	Now func() time.Time
}

// Page is the data every page template receives.
type Page struct {
	// Title is the page's own title. Empty on the home page, whose <title> is
	// just SiteTitle; other pages get "<Title> | CranLab".
	Title string
	// Description fills <meta name="description">.
	Description string
	// Path is the request path, e.g. "/about". The navigation marks the
	// current section with it.
	Path string
	// Year is the current year (from Deps.Now), for the footer's copyright.
	Year int
	// Config gives templates the site settings (SiteURL, WebmentionDomain, …).
	Config config.Config
}

type server struct {
	cfg    config.Config
	deps   Deps
	static *static
	pages  map[string]*template.Template // by page name: "home", "about", "404", "503"
	mux    *http.ServeMux
}

// New builds the site's handler, parsing every template up front so a broken
// template fails at startup rather than on a request.
//
// Routes (Go 1.22 patterns):
//   - GET /{$}             home page
//   - GET /about           About page
//   - GET /robots.txt      "User-agent: *" and "Allow: /", text/plain
//   - GET /healthz         "ok" (moved here from main)
//   - GET /static/{path...} embedded static files (see static)
//   - GET /static          the 404 page. Without this exact pattern, the router
//     redirects /static to /static/, which the trailing-slash rule redirects
//     back: an endless loop.
//   - GET / (catch-all)    the 404 page with status 404. Register it for GET
//     only, so other methods get the router's automatic 405.
//
// Every path with a trailing slash except "/" gets a 301 to the same path
// without it, keeping the query string ("/about/?x=1" → "/about?x=1").
//
// Pages are rendered into a buffer first and written only if rendering
// succeeded, with Content-Type "text/html; charset=utf-8".
func New(cfg config.Config, deps Deps) (http.Handler, error) {
	panic("not implemented")
}

// ServeHTTP redirects paths with a trailing slash (see New), then hands the
// request to the router. New returns the *server itself as the handler.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

// renderError writes the 404 or 503 page with that status. Later tasks use it
// for missing records and an unreachable PDS.
func (s *server) renderError(w http.ResponseWriter, r *http.Request, status int) {
	panic("not implemented")
}
