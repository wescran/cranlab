package web

import (
	"html/template"
	"io/fs"
)

// templateFuncs are the helpers available in every template:
//
//   - asset "css/main.css" → the file's versioned URL (static.url); an unknown
//     file makes rendering fail.
//   - date t → "January 2, 2006" (in UTC).
//   - datetime t → RFC 3339, e.g. "2026-09-28T00:00:00Z" (in UTC), for
//     <time datetime=…> and dt-published.
func templateFuncs(s *static) template.FuncMap {
	panic("not implemented")
}

// parsePages parses each page template ("home", "about", "404", "503")
// together with base.html, so every page shares the layout and defines its
// own "main" block. The result is keyed by page name.
func parsePages(fsys fs.FS, funcs template.FuncMap) (map[string]*template.Template, error) {
	panic("not implemented")
}
