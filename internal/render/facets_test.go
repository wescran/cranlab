package render

import (
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

func facet(start, end int, feature pds.Feature) pds.Facet {
	return pds.Facet{Index: pds.ByteSlice{ByteStart: start, ByteEnd: end}, Features: []pds.Feature{feature}}
}

func link(uri string) pds.Feature { return pds.Feature{Type: featureLink, URI: uri} }

func TestFacets(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		facets []pds.Facet
		want   string
	}{
		{"plain text", "Hello, world!", nil, "Hello, world!"},
		{"escapes text", `<b>"hi" & bye</b>`, nil, "&lt;b&gt;&#34;hi&#34; &amp; bye&lt;/b&gt;"},
		{"newlines", "one\ntwo", nil, "one<br>two"},
		{
			"link mid-sentence", "see go.dev now",
			[]pds.Facet{facet(4, 10, link("https://go.dev"))},
			`see <a href="https://go.dev">go.dev</a> now`,
		},
		{
			"mention", "hi @alice.test",
			[]pds.Facet{facet(3, 14, pds.Feature{Type: featureMention, DID: "did:plc:alice"})},
			`hi <a href="https://bsky.app/profile/did:plc:alice">@alice.test</a>`,
		},
		{
			"hashtag", "love #golang",
			[]pds.Facet{facet(5, 12, pds.Feature{Type: featureTag, Tag: "golang"})},
			`love <a href="https://bsky.app/hashtag/golang">#golang</a>`,
		},
		{
			// "🍁 " is 5 bytes: offsets count UTF-8 bytes, not characters.
			"byte offsets after emoji", "🍁 go.dev",
			[]pds.Facet{facet(5, 11, link("https://go.dev"))},
			`🍁 <a href="https://go.dev">go.dev</a>`,
		},
		{
			"accented text before a link", "café go.dev",
			[]pds.Facet{facet(6, 12, link("https://go.dev"))},
			`café <a href="https://go.dev">go.dev</a>`,
		},
		{
			"facets out of order", "a.dev b.dev",
			[]pds.Facet{facet(6, 11, link("https://b.dev")), facet(0, 5, link("https://a.dev"))},
			`<a href="https://a.dev">a.dev</a> <a href="https://b.dev">b.dev</a>`,
		},
		{
			"escapes the linked text and URL", `<x> link`,
			[]pds.Facet{facet(4, 8, link(`https://e.com/?a=1&b="2"`))},
			`&lt;x&gt; <a href="https://e.com/?a=1&amp;b=&#34;2&#34;">link</a>`,
		},
		{
			"javascript: links stay text", "click me",
			[]pds.Facet{facet(0, 8, link("javascript:alert(1)"))},
			"click me",
		},
		{
			"unknown feature stays text", "fancy",
			[]pds.Facet{facet(0, 5, pds.Feature{Type: "app.example.facet#sparkle"})},
			"fancy",
		},
		{"past the end", "short", []pds.Facet{facet(0, 99, link("https://e.com"))}, "short"},
		{"empty range", "short", []pds.Facet{facet(2, 2, link("https://e.com"))}, "short"},
		{"backwards range", "short", []pds.Facet{facet(3, 1, link("https://e.com"))}, "short"},
		{"negative start", "short", []pds.Facet{facet(-1, 3, link("https://e.com"))}, "short"},
		{
			"splits a multi-byte character", "🍁x",
			[]pds.Facet{facet(2, 5, link("https://e.com"))},
			"🍁x",
		},
		{
			"overlapping facet skipped", "abcdef",
			[]pds.Facet{facet(0, 4, link("https://one.dev")), facet(2, 6, link("https://two.dev"))},
			`<a href="https://one.dev">abcd</a>ef`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Facets(tt.text, tt.facets)); got != tt.want {
				t.Errorf("Facets =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}
