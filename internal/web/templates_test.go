package web

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestTemplateFuncs(t *testing.T) {
	s, err := newStatic(fstest.MapFS{"css/main.css": {Data: []byte("body{}")}})
	if err != nil {
		t.Fatal(err)
	}
	funcs := templateFuncs(s)

	toronto, _ := time.LoadLocation("America/Toronto")
	when := time.Date(2026, 9, 27, 20, 0, 0, 0, toronto) // 2026-09-28 00:00 UTC

	date := funcs["date"].(func(time.Time) string)
	if got := date(when); got != "September 28, 2026" {
		t.Errorf("date = %q, want the UTC date", got)
	}
	datetime := funcs["datetime"].(func(time.Time) string)
	if got := datetime(when); got != "2026-09-28T00:00:00Z" {
		t.Errorf("datetime = %q, want RFC 3339 in UTC", got)
	}
	asset := funcs["asset"].(func(string) (string, error))
	if u, err := asset("css/main.css"); err != nil || u == "" {
		t.Errorf("asset = %q, %v", u, err)
	}
	if _, err := asset("nope.css"); err == nil {
		t.Error("asset of a missing file: want an error")
	}
}

func TestParsePagesSharesTheLayout(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/base.html":  {Data: []byte(`<title>{{.Title}}</title>{{block "main" .}}{{end}}`)},
		"templates/home.html":  {Data: []byte(`{{define "main"}}home{{end}}`)},
		"templates/about.html": {Data: []byte(`{{define "main"}}about{{end}}`)},
		"templates/404.html":   {Data: []byte(`{{define "main"}}missing{{end}}`)},
		"templates/503.html":   {Data: []byte(`{{define "main"}}down{{end}}`)},
	}
	pages, err := parsePages(fsys, templateFuncs(&static{}))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"home": "home", "about": "about", "404": "missing", "503": "down"} {
		tmpl := pages[name]
		if tmpl == nil {
			t.Errorf("no %q page", name)
			continue
		}
		var b strings.Builder
		if err := tmpl.Execute(&b, Page{Title: "T"}); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := b.String(); got != "<title>T</title>"+want {
			t.Errorf("%s rendered %q", name, got)
		}
	}
}
