package render

import (
	"regexp"
	"strings"
)

var bskyPostRe = regexp.MustCompile(`^at://([^/]+)/app\.bsky\.feed\.post/([^/]+)$`)

// ReadingTime is the minutes needed to read text at 200 words a minute,
// rounded up, and at least 1.
func ReadingTime(text string) int {
	return max(1, (len(strings.Fields(text))+199)/200)
}

// Excerpt shortens text to at most n characters (runes), with whitespace
// collapsed to single spaces. Longer text is cut after the last whole word
// that fits (or at n characters, if a single word is longer), trailing
// punctuation is trimmed, and "…" is added.
func Excerpt(text string, n int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	short := string(runes[:n])
	if runes[n] != ' ' {
		lastSpace := strings.LastIndex(short, " ")
		if lastSpace > 0 {
			short = short[:lastSpace]
		}
	}
	return strings.TrimRight(short, " ,;:") + "…"
}

// PostURL turns a post's AT-URI into its bsky.app URL
// (at://<did>/app.bsky.feed.post/<rkey> →
// https://bsky.app/profile/<did>/post/<rkey>), or "" if uri isn't a post.
func PostURL(uri string) string {
	matches := bskyPostRe.FindStringSubmatch(uri)
	if matches == nil {
		return ""
	}
	return "https://bsky.app/profile/" + matches[1] + "/post/" + matches[2]
}
