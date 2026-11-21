// Package content holds the site's rules about records: which documents are
// articles, which posts are notes, and how tags are counted.
package content

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/wescran/cranlab/internal/pds"
)

// ArticlePrefix is the path every article lives under.
const ArticlePrefix = "/articles/"

// Article is a document published on the site.
type Article struct {
	URI, Title, Description, TextContent string
	AnnouncementURI, CoverCID            string
	Tags                                 []string
	Markdown                             *string
	HasPublished                         bool
	// Slug is the last part of the article's path ("/articles/<slug>").
	Slug string
	// Published is PublishedAt parsed; zero if it isn't a valid timestamp.
	Published time.Time
}

// Articles projects the owner's documents from a validated PDS listing. It selects those
// whose Site is publication and whose Path is "/articles/<slug>" with a
// non-empty slug and no further "/". They're sorted newest first by
// Published, then by Slug.
func Articles(docs []pds.Record[pds.Document], publication string) []Article {
	var articles []Article
	for _, r := range docs {
		d := r.Value
		slug, ok := strings.CutPrefix(d.Path, ArticlePrefix)
		if d.Site != publication || !ok || slug == "" || strings.Contains(slug, "/") {
			continue
		}
		date, err := time.Parse(time.RFC3339, d.PublishedAt)
		a := Article{
			Slug: slug, Published: date, HasPublished: err == nil,
			URI: r.URI, Title: d.Title, Description: d.Description,
			TextContent: d.TextContent, Markdown: d.Markdown(),
		}
		if d.BskyPostRef != nil {
			a.AnnouncementURI = d.BskyPostRef.URI
		}
		if d.CoverImage != nil {
			a.CoverCID = d.CoverImage.Ref.Link
		}
		seen := map[string]bool{}
		for _, raw := range d.Tags {
			tag := strings.ToLower(strings.TrimSpace(raw))
			if tag != "" && !seen[tag] {
				seen[tag] = true
				a.Tags = append(a.Tags, tag)
			}
		}
		slices.Sort(a.Tags)
		articles = append(articles, a)
	}
	slices.SortFunc(articles, func(a, b Article) int {
		if a.HasPublished != b.HasPublished {
			if a.HasPublished {
				return -1
			}
			return 1
		}
		return cmp.Or(b.Published.Compare(a.Published), strings.Compare(a.Slug, b.Slug))
	})
	return articles
}

// FindArticle returns the article with this slug.
func FindArticle(articles []Article, slug string) (Article, bool) {
	for _, a := range articles {
		if a.Slug == slug {
			return a, true
		}
	}
	return Article{}, false
}

// Announcements returns the record keys of docs. A post with the same record
// key as a document is that document's announcement on Bluesky.
func Announcements(docs []pds.Record[pds.Document]) map[string]bool {
	panic("not implemented")
}

// IsNote reports whether a post is shown as a note: it isn't a reply and
// isn't an announcement.
func IsNote(p pds.Record[pds.Post], announcements map[string]bool) bool {
	panic("not implemented")
}

// PostLister lists posts one page at a time (satisfied by *pds.Client).
type PostLister interface {
	ListPosts(ctx context.Context, did, cursor string, limit int) ([]pds.Record[pds.Post], string, error)
}

// maxNotePages bounds how many pages Notes reads to fill one page of notes,
// in case most posts are replies.
const maxNotePages = 10

// Notes returns up to n notes, newest first, starting after cursor ("" for
// the newest), and the cursor to continue from.
//
// It reads pages of posts (limit 100) and keeps the notes, until it has n,
// a page is empty, the PDS returns no cursor, or it has read maxNotePages
// pages. The returned cursor is "" when the posts ran out; otherwise it's
// where to continue: the record key of the last note returned (when n were
// found), or the last page's cursor (when it gave up).
// A listRecords cursor is a record key, so the next call continues right
// after that note even if it stopped partway through a page.
func Notes(ctx context.Context, l PostLister, did string, announcements map[string]bool, cursor string, n int) ([]pds.Record[pds.Post], string, error) {
	panic("not implemented")
}

// TagCount is a tag and how many articles have it.
type TagCount struct {
	Tag   string
	Count int
}

// Tags counts the articles' tags, sorted by tag (case-insensitively).
func Tags(articles []Article) []TagCount {
	counts := map[string]int{}
	for _, a := range articles {
		for _, tag := range a.Tags {
			counts[tag]++
		}
	}
	var out []TagCount
	for _, tag := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, TagCount{tag, counts[tag]})
	}
	return out
}

// WithTag returns the articles that have tag, keeping their order.
func WithTag(articles []Article, tag string) []Article {
	var out []Article
	for _, a := range articles {
		if slices.Contains(a.Tags, tag) {
			out = append(out, a)
		}
	}
	return out
}
