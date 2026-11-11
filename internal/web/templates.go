package web

import (
	"html/template"
	"io/fs"
	"time"
)

// templateFuncs returns the helpers available in templates: asset (a static
// file's versioned URL), date ("January 2, 2006") and datetime (RFC 3339),
// both in UTC.
func templateFuncs(s *static) template.FuncMap {
	return template.FuncMap{
		"asset": s.url,
		"date": func(t time.Time) string {
			return t.UTC().Format("January 2, 2006")
		},
		"datetime": func(t time.Time) string {
			return t.UTC().Format(time.RFC3339)
		},
	}
}

// parsePages parses each page with the shared layout and icons, keyed by page
// name. Each page needs its own set because every page defines "main".
func parsePages(fsys fs.FS, funcs template.FuncMap) (map[string]*template.Template, error) {
	templateMap := make(map[string]*template.Template)
	for _, pg := range []string{"home", "about", "404", "503", "articles", "article", "notes", "note", "tags", "tag"} {
		t, err := template.New("base.html").Funcs(funcs).ParseFS(fsys, "templates/base.html", "templates/icons.html", "templates/partials.html", "templates/"+pg+".html")
		if err != nil {
			return nil, err
		}
		templateMap[pg] = t
	}
	return templateMap, nil
}
