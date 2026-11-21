// Package content holds the site's rules about records: which documents are
// articles, which posts are notes, and how tags are counted.
package content

import (
	"cmp"
	"maps"
	"net/url"
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

// Announcements indexes both the deterministic key and the exact referenced URI
// of every published Article. This display filter is not proof for a write.
func Announcements(articles []Article) map[string]bool {
	refs := make(map[string]bool)
	for _, a := range articles {
		owner, _, _ := strings.Cut(strings.TrimPrefix(a.URI, "at://"), "/")
		refs["at://"+owner+"/"+pds.PostCollection+"/"+pds.RKey(a.URI)] = true
		if a.AnnouncementURI != "" {
			refs[a.AnnouncementURI] = true
		}
	}
	return refs
}

func IsNote(p pds.Record[pds.Post], announcements map[string]bool, siteURL string) bool {
	if (len(p.Value.Reply) > 0 && p.Value.Reply.Kind() != 'n') || announcements[p.URI] {
		return false
	}
	e := p.Value.Embed
	if e != nil && e.Type == pds.EmbedRecordWithMedia {
		e = e.Media
	}
	if e == nil || e.Type != pds.EmbedExternal || e.External == nil {
		return true
	}
	return !articleLink(e.External.URI, siteURL)
}

func articleLink(raw, siteURL string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	site, err := url.Parse(siteURL)
	if err != nil || site.Hostname() == "" || u.Scheme != site.Scheme || !strings.EqualFold(u.Hostname(), site.Hostname()) || port(u) != port(site) {
		return false
	}
	// Match route segments before unescaping: an encoded slash is not a route separator.
	dir, segment, ok := strings.Cut(strings.TrimPrefix(strings.TrimSuffix(u.EscapedPath(), "/"), "/"), "/")
	dir, _ = url.PathUnescape(dir)
	slug, _ := url.PathUnescape(segment)
	return ok && dir == "articles" && slug != "" && slug != "." && slug != ".." && !strings.Contains(slug, "/")
}

func port(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
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
