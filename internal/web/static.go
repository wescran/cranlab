package web

import (
	"io/fs"
	"net/http"
)

// static serves files from a filesystem (web/static in production) under
// /static/, with URLs that change whenever a file's content changes, so they
// can be cached forever.
type static struct {
	// Fill in: the filesystem and each file's content hash.
}

// newStatic hashes every file in fsys once (SHA-256 of its bytes).
func newStatic(fsys fs.FS) (*static, error) {
	panic("not implemented")
}

// url returns "/static/<name>?v=<hash>", where hash is the first 12 hex
// characters of the file's SHA-256. An unknown name is an error, so a typo in
// a template fails loudly instead of linking to nothing.
func (s *static) url(name string) (string, error) {
	panic("not implemented")
}

// ServeHTTP serves the file named by the request's {path...} wildcard.
//
//   - Found: the file's bytes, Content-Type from its extension (text/css;
//     charset=utf-8 for .css, font/woff2, image/svg+xml, …), and
//     Cache-Control "public, max-age=31536000, immutable". The ?v= value is
//     not checked: any version serves the current file.
//   - Missing, or a directory: 404 (plain, not the HTML page).
func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}
