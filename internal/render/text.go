package render

// ReadingTime is the minutes needed to read text at 200 words a minute,
// rounded up, and at least 1.
func ReadingTime(text string) int {
	panic("not implemented")
}

// Excerpt shortens text to at most n characters (runes), with whitespace
// collapsed to single spaces. Longer text is cut after the last whole word
// that fits (or at n characters, if a single word is longer), trailing
// punctuation is trimmed, and "…" is added.
func Excerpt(text string, n int) string {
	panic("not implemented")
}

// PostURL turns a post's AT-URI into its bsky.app URL
// (at://<did>/app.bsky.feed.post/<rkey> →
// https://bsky.app/profile/<did>/post/<rkey>), or "" if uri isn't a post.
func PostURL(uri string) string {
	panic("not implemented")
}
