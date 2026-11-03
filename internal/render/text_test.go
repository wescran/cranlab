package render

import (
	"strings"
	"testing"
)

func TestReadingTime(t *testing.T) {
	tests := map[int]int{0: 1, 1: 1, 200: 1, 201: 2, 400: 2, 1001: 6}
	for words, want := range tests {
		if got := ReadingTime(strings.Repeat("word ", words)); got != want {
			t.Errorf("ReadingTime(%d words) = %d, want %d", words, got, want)
		}
	}
	if got := ReadingTime("one\ntwo\tthree   four"); got != 1 {
		t.Errorf("any whitespace separates words: got %d", got)
	}
}

func TestExcerpt(t *testing.T) {
	tests := []struct {
		text string
		n    int
		want string
	}{
		{"Short enough.", 20, "Short enough."},
		{"Exactly ten", 11, "Exactly ten"},
		{"  Lots   of\n\nspace  ", 50, "Lots of space"},
		{"The quick brown fox jumps over the lazy dog", 20, "The quick brown fox…"},
		{"Exactly ten more words", 11, "Exactly ten…"},
		{"One, two, three, four", 15, "One, two…"},
		{"Supercalifragilistic", 10, "Supercalif…"},
		// n counts characters, not bytes: these accents are two bytes each.
		{"Café crème brûlée is lovely", 17, "Café crème brûlée…"},
		{"Café crème brûlée is lovely", 16, "Café crème…"},
	}
	for _, tt := range tests {
		if got := Excerpt(tt.text, tt.n); got != tt.want {
			t.Errorf("Excerpt(%q, %d) = %q, want %q", tt.text, tt.n, got, tt.want)
		}
	}
}

func TestPostURL(t *testing.T) {
	tests := map[string]string{
		"at://did:plc:abc/app.bsky.feed.post/3mxc6hxwdnc2s": "https://bsky.app/profile/did:plc:abc/post/3mxc6hxwdnc2s",
		"at://did:plc:abc/app.bsky.graph.list/3mlist":       "",
		"at://did:plc:abc/app.bsky.feed.post":               "",
		"https://bsky.app/profile/x/post/y":                 "",
		"":                                                  "",
	}
	for uri, want := range tests {
		if got := PostURL(uri); got != want {
			t.Errorf("PostURL(%q) = %q, want %q", uri, got, want)
		}
	}
}
