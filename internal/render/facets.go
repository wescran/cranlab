package render

import (
	"cmp"
	"html/template"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/weburl"
)

// escapeText escapes text for HTML and turns newlines into <br>.
func escapeText(text string) string {
	return strings.ReplaceAll(template.HTMLEscapeString(text), "\n", "<br>")
}

// boundary reports whether byte i of s starts a character (or is the end of s).
func boundary(s string, i int) bool {
	return i == len(s) || utf8.RuneStart(s[i])
}

// Facets renders a post's text as escaped HTML, turning its facets (UTF-8
// byte ranges) into links: link URIs (http and https only), mentions to their
// Bluesky profile and hashtags to Bluesky's hashtag page. Newlines become
// <br>. Facets that are empty, overlap an earlier one, fall outside the text
// or split a character are ignored.
func Facets(text string, facets []content.Facet) template.HTML {
	facetCopy := slices.Clone(facets)
	slices.SortStableFunc(facetCopy, func(a, b content.Facet) int {
		return cmp.Compare(a.Start, b.Start)
	})
	pos := 0
	var output strings.Builder
	for _, f := range facetCopy {
		start, end := f.Start, f.End
		if start >= end || start < pos || end > len(text) || !boundary(text, start) || !boundary(text, end) {
			continue
		}
		link := f.URL
		if !weburl.Absolute(link) {
			continue
		}
		output.WriteString(escapeText(text[pos:start]))
		output.WriteString(`<a href="` + template.HTMLEscapeString(link) + `">`)
		output.WriteString(escapeText(text[start:end]))
		output.WriteString("</a>")
		pos = end
	}
	output.WriteString(escapeText(text[pos:]))
	return template.HTML(output.String())
}
