package web

import (
	"html/template"
	"time"

	"github.com/wescran/cranlab/internal/render"
)

// The view types are what page templates receive as Page.Data. Templates
// escape every string field; template.HTML fields are already safe HTML.

// HomePage is the home page's content.
type HomePage struct {
	// Latest is the newest article, or nil if there are none.
	Latest *ArticleSummary
	// Notes are the most recent notes (up to 5).
	Notes []NoteView
}

// ArticlesPage lists every article.
type ArticlesPage struct {
	Articles []ArticleSummary
}

// ArticleSummary is an article in a list.
type ArticleSummary struct {
	// URL is the article's path, e.g. "/articles/welcome".
	URL          string
	Title        string
	Description  string
	Published    time.Time
	HasPublished bool
	ReadingTime  int // minutes
	Tags         []TagLink
}

// ArticlePage is one article.
type ArticlePage struct {
	ArticleSummary
	Body template.HTML
	// SyndicationURL is the announcement post on bsky.app, or "".
	SyndicationURL string
}

// NotesPage lists notes, newest first.
type NotesPage struct {
	Notes []NoteView
	// OlderURL links to the next page of older notes, or is "".
	OlderURL string
}

// NoteView is one note.
type NoteView struct {
	// URL is the note's path, e.g. "/notes/3mxc6hxwdnc2s".
	URL       string
	Text      template.HTML
	Published time.Time
	Embed     *render.Embed
	// BlueskyURL is the post on bsky.app.
	BlueskyURL string
}

// TagsPage lists every tag.
type TagsPage struct {
	Tags []TagLink
}

// TagPage lists the articles with one tag.
type TagPage struct {
	Tag      string
	Articles []ArticleSummary
}

// TagLink is a tag with its page's URL and, on the tags index, its count.
type TagLink struct {
	Name  string
	URL   string // "/tags/<name, path-escaped>"
	Count int
}
