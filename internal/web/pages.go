package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/render"
)

// pdsTimeout bounds the PDS calls for one request.
const pdsTimeout = 5 * time.Second

// notesPerPage is how many notes /notes shows; the home page shows homeNotes.
const (
	notesPerPage = 20
	homeNotes    = 5
)

// rkeyPattern is the record key syntax (atproto's "record-key" format).
var rkeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:~-]{1,512}$`)

// pdsContext returns the request's context, bounded by pdsTimeout.
func (s *server) pdsContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), pdsTimeout)
}

// unavailable logs a source failure and serves the 503 page.
func (s *server) unavailable(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("source unavailable", "path", r.URL.Path, "err", err)
	s.renderError(w, r, http.StatusServiceUnavailable)
}

// summarize describes an article for lists: URL "/articles/<slug,
// path-escaped>", title, description (or, without one, an excerpt of its
// textContent, 160 characters), published time, reading time and tag links.
func summarize(a content.Article) ArticleSummary {
	description := a.Description
	if description == "" {
		description = render.Excerpt(a.TextContent, 160)
	}
	return ArticleSummary{
		URL: content.ArticlePrefix + url.PathEscape(a.Slug), Title: a.Title,
		Description: description, Published: a.Published, HasPublished: a.HasPublished,
		ReadingTime: render.ReadingTime(a.TextContent), Tags: tagLinks(a.Tags),
	}
}

// tagLinks links each tag to "/tags/<tag, path-escaped>".
func tagLinks(tags []string) []TagLink {
	var links []TagLink
	for _, t := range tags {
		links = append(links, tagLink(t))
	}
	return links
}

func tagLink(t string) TagLink {
	return TagLink{Name: t, URL: "/tags/" + url.PathEscape(t)}
}

// summaries summarizes each article, keeping their order.
func summaries(articles []content.Article) []ArticleSummary {
	out := make([]ArticleSummary, 0, len(articles))
	for _, a := range articles {
		out = append(out, summarize(a))
	}
	return out
}

// noteView describes a post: URL "/notes/<rkey>", text with facets
// (render.Facets), CreatedAt parsed (RFC 3339; zero if invalid), embed
// (render.EmbedView) and its bsky.app URL.
func noteView(p pds.Record[pds.Post]) NoteView {
	panic("not implemented")
}

// noteViews describes each post, keeping their order.
func noteViews(posts []pds.Record[pds.Post]) []NoteView {
	panic("not implemented")
}

// home shows the latest article (if any) and the newest homeNotes notes.
// Description: "CranLab by Wesley Cranston". A PDS error is a 503.
func (s *server) home(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

// articles lists every article. Title "Articles", description "Articles by
// Wesley Cranston". A PDS error is a 503.
func (s *server) articles(w http.ResponseWriter, r *http.Request) {
	articles, err := s.deps.Store.Articles(r.Context())
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	s.render(w, r, "articles", http.StatusOK, Page{Title: "Articles", Description: "Articles by Wesley Cranston", Data: ArticlesPage{Articles: summaries(articles)}})
}

// article shows the article whose slug is {slug}, or the 404 page. Its body
// comes from render.ArticleBody; SyndicationURL is the bsky.app URL of its
// bskyPostRef, if any. Title and description are the article's (the
// description falls back to the excerpt, as in summarize). A PDS or render
// error is a 503.
func (s *server) article(w http.ResponseWriter, r *http.Request) {
	a, ok, err := s.deps.Store.FindArticle(r.Context(), r.PathValue("slug"))
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	if !ok {
		s.renderError(w, r, http.StatusNotFound)
		return
	}
	body, err := render.ArticleBody(a.Markdown, a.TextContent, render.Options{DID: s.cfg.DID, PDSPublicURL: s.cfg.PDSPublicURL})
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	view := ArticlePage{ArticleSummary: summarize(a), Body: body, SyndicationURL: render.PostURL(a.AnnouncementURI)}
	s.render(w, r, "article", http.StatusOK, Page{Title: a.Title, Description: view.Description, Data: view})
}

// notes lists notesPerPage notes, starting after ?cursor= (ignored unless it
// matches rkeyPattern, so junk never reaches the PDS). OlderURL is
// "/notes?cursor=<next, query-escaped>" when there may be more. Title
// "Notes", description "Notes by Wesley Cranston". A PDS error is a 503.
func (s *server) notes(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

// note shows the post {rkey} if it's a note. An rkey not matching
// rkeyPattern, a missing post (pds.ErrNotFound), a reply or an announcement
// is the 404 page; another PDS error is a 503. Title: the first 60
// characters of the text (render.Excerpt), or "Note" if empty; description:
// the first 160.
func (s *server) note(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

// tags lists every tag with its count. Title "Tags", description "Article
// tags". A PDS error is a 503.
func (s *server) tags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.deps.Store.Tags(r.Context())
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	var links []TagLink
	for _, t := range tags {
		l := tagLink(t.Tag)
		l.Count = t.Count
		links = append(links, l)
	}
	s.render(w, r, "tags", http.StatusOK, Page{Title: "Tags", Description: "Article tags", Data: TagsPage{Tags: links}})
}

// tag lists the articles tagged {tag} (exact match), or the 404 page if
// there are none. Title: the tag; description: "Articles tagged <tag>". A PDS
// error is a 503.
func (s *server) tag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	articles, err := s.deps.Store.WithTag(r.Context(), tag)
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	if len(articles) == 0 {
		s.renderError(w, r, http.StatusNotFound)
		return
	}
	s.render(w, r, "tag", http.StatusOK, Page{Title: tag, Description: "Articles tagged " + tag, Data: TagPage{Tag: tag, Articles: summaries(articles)}})
}
