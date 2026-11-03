package render

import (
	"html/template"

	"github.com/wescran/cranlab/internal/pds"
)

// Facet feature types.
const (
	featureLink    = "app.bsky.richtext.facet#link"
	featureMention = "app.bsky.richtext.facet#mention"
	featureTag     = "app.bsky.richtext.facet#tag"
)

// Facets renders a post's text as HTML, turning its facets into links.
//
//   - Facet ranges are UTF-8 byte offsets into text. Process facets in order of
//     ByteStart (sort a copy), writing the text between them escaped.
//   - Skip a facet whose range is empty or backwards, starts before the end of
//     the previous linked facet (overlap) or before 0, ends past the text, or
//     starts or ends in the middle of a multi-byte character
//     (utf8.RuneStart).
//   - A facet's first usable feature decides the link: a link with an http or
//     https URI links to it; a mention links to https://bsky.app/profile/<did>;
//     a tag links to https://bsky.app/hashtag/<tag, path-escaped>. No usable
//     feature (e.g. a javascript: URI): the text stays plain.
//   - The result bypasses html/template's escaping, so escape everything
//     yourself (template.HTMLEscapeString), including href values. Newlines
//     become <br>.
//
// Output for a link: <a href="https://go.dev">go.dev</a>
func Facets(text string, facets []pds.Facet) template.HTML {
	panic("not implemented")
}
