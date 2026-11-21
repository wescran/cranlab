package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/render"
)

// pdsTimeout bounds the PDS calls for one request.
const pdsTimeout = 5 * time.Second

// notesPerPage is how many notes /notes shows; the home page shows homeNotes.
const (
	notesPerPage = 20
	homeNotes    = 5
)

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
		URL: a.PermalinkPath(), Title: a.Title,
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

func (s *server) noteView(n content.Note) NoteView {
	return NoteView{URL: "/notes/" + url.PathEscape(n.RKey), Text: render.Facets(n.Text, n.Facets), Published: n.Published, Embed: render.EmbedView(n.Embed, n.URI), BlueskyURL: render.PostURL(n.URI), AuthorURL: s.cfg.SiteURL + "/"}
}

func (s *server) noteViews(notes []content.Note) []NoteView {
	views := make([]NoteView, 0, len(notes))
	for _, n := range notes {
		views = append(views, s.noteView(n))
	}
	return views
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	articles, err := s.deps.Store.Articles(r.Context())
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	notes, _, err := s.deps.Store.Notes(r.Context(), "", homeNotes)
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	data := HomePage{Articles: summaries(articles[:min(5, len(articles))]), Notes: s.noteViews(notes)}
	s.render(w, r, "home", http.StatusOK, Page{Description: "CranLab by Wesley Cranston", Data: data})
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
	view.Interactions = s.interactionGroups(r.Context(), view.URL)
	page := Page{Title: a.Title, Description: view.Description, Data: view, CanonicalURL: s.cfg.SiteURL + view.URL}
	page.DocumentID = strings.TrimPrefix(a.URI, "at://")
	if cidPattern.MatchString(a.CoverCID) {
		page.CoverCID = a.CoverCID
	}
	s.render(w, r, "article", http.StatusOK, page)
}

func (s *server) notes(w http.ResponseWriter, r *http.Request) {
	notes, next, err := s.deps.Store.Notes(r.Context(), r.URL.Query().Get("cursor"), notesPerPage)
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	data := NotesPage{Notes: s.noteViews(notes)}
	if next != "" {
		data.OlderURL = "/notes?cursor=" + url.QueryEscape(next)
	}
	s.render(w, r, "notes", http.StatusOK, Page{Title: "Notes", Description: "Notes by Wesley Cranston", Data: data})
}

func (s *server) note(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("rkey")
	if !content.ValidRKey(key) {
		s.renderError(w, r, http.StatusNotFound)
		return
	}
	n, ok, err := s.deps.Store.FindNote(r.Context(), key)
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	if !ok {
		s.renderError(w, r, http.StatusNotFound)
		return
	}
	title := render.Excerpt(n.Text, 60)
	if title == "" {
		title = "Note"
	}
	view := s.noteView(n)
	view.Interactions = s.interactionGroups(r.Context(), view.URL)
	s.render(w, r, "note", http.StatusOK, Page{Title: title, Description: render.Excerpt(n.Text, 160), Data: view, CanonicalURL: s.cfg.SiteURL + view.URL})
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
