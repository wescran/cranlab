package render

import (
	"cmp"
	"html/template"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/wescran/cranlab/internal/pds"
)

// Facet feature types.
const (
	featureLink    = "app.bsky.richtext.facet#link"
	featureMention = "app.bsky.richtext.facet#mention"
	featureTag     = "app.bsky.richtext.facet#tag"
)

// facetHref returns the link for a facet's first usable feature, or "".
func facetHref(features []pds.Feature) string {
	for _, feature := range features {
		switch feature.Type {
		case featureLink:
			parsed, err := url.Parse(feature.URI)
			if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
				return feature.URI
			}
		case featureMention:
			if strings.HasPrefix(feature.DID, "did:") {
				return "https://bsky.app/profile/" + feature.DID
			}
		case featureTag:
			if feature.Tag != "" {
				return "https://bsky.app/hashtag/" + url.PathEscape(feature.Tag)
			}
		}
	}
	return ""
}

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
func Facets(text string, facets []pds.Facet) template.HTML {
	facetCopy := append([]pds.Facet(nil), facets...)
	slices.SortStableFunc(facetCopy, func(a, b pds.Facet) int {
		return cmp.Compare(a.Index.ByteStart, b.Index.ByteStart)
	})
	pos := 0
	var output strings.Builder
	for _, f := range facetCopy {
		start, end := f.Index.ByteStart, f.Index.ByteEnd
		if start >= end || start < pos || end > len(text) || !boundary(text, start) || !boundary(text, end) {
			continue
		}
		link := facetHref(f.Features)
		if link == "" {
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
