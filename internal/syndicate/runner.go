// Package syndicate creates Announcements and repairs missing Article links. It never runs
// from a page request; the syncer calls Run after a successful refresh.
package syndicate

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
)

var errLogin = errors.New("login failed")

// ArticleSource supplies the successfully committed read snapshot.
type ArticleSource interface {
	Articles(context.Context) ([]content.Article, error)
}

// Client uses public reads to verify a creation or repair before acquiring a session.
type Client interface {
	LatestCommit(context.Context, string) (string, error)
	GetPost(context.Context, string, string) (pds.Record[pds.Post], error)
	GetDocument(context.Context, string, string) (pds.Record[pds.Document], error)
	CreateSession(context.Context, string, string) (string, error)
	UploadBlob(context.Context, string, string, []byte) (pds.Blob, error)
	CreatePost(context.Context, string, string, string, string, map[string]any) (pds.StrongRef, error)
	LinkDocument(context.Context, string, string, string, pds.Record[pds.Document], pds.StrongRef) error
}

type Runner struct {
	now                                func() time.Time
	mu                                 sync.Mutex
	source                             ArticleSource
	client                             Client
	did, publication, origin, password string
	logger                             *slog.Logger
}

func New(source ArticleSource, client Client, did, publication, origin, password string, logger *slog.Logger, now func() time.Time) *Runner {
	return &Runner{now: now, source: source, client: client, did: did, publication: publication, origin: origin, password: password, logger: logger}
}

type run struct {
	*Runner
	at        time.Time
	token     string
	remaining int
}

func (r *Runner) Run(ctx context.Context) {
	if r.password == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	articles, err := r.source.Articles(ctx)
	if err != nil {
		r.logger.Warn("syndication: article snapshot unavailable")
		return
	}
	claimed := make(map[string]bool)
	for _, a := range articles {
		if a.AnnouncementURI != "" {
			claimed[a.AnnouncementURI] = true
		}
	}
	batch := run{Runner: r, at: r.now(), remaining: 3}
	articles = slices.Clone(articles)
	slices.SortFunc(articles, func(a, b content.Article) int {
		return cmp.Or(a.Published.Compare(b.Published), strings.Compare(a.URI, b.URI))
	})
	for _, a := range articles {
		if ctx.Err() != nil {
			return
		}
		if a.AnnouncementURI != "" {
			continue
		}
		key := pds.RKey(a.URI)
		if key == "" || a.URI != "at://"+r.did+"/"+pds.DocumentCollection+"/"+key {
			r.logger.Warn("syndication: unexpected Article identity")
			continue
		}
		if claimed["at://"+r.did+"/"+pds.PostCollection+"/"+key] {
			r.logger.Warn("syndication: Announcement already linked to another Article", "key", key)
			continue
		}
		err := batch.announce(ctx, a, key, batch.remaining > 0)
		if errors.Is(err, errCreateConflict) || errors.Is(err, pds.ErrSwapConflict) {
			// Retry once from fresh state. A link conflict may only repair.
			err = batch.announce(ctx, a, key, errors.Is(err, errCreateConflict))
		}
		if err != nil {
			r.logger.Warn("syndication: review Article", "key", key, "reason", err)
			if errors.Is(err, errLogin) {
				return // do not repeat a failed login for every Article
			}
		}
	}
}

func (r *run) announce(ctx context.Context, a content.Article, key string, allowCreate bool) error {
	canCreate := allowCreate && a.HasPublished && eligible(a.Published, r.at)
	var commit string
	var headErr error
	if canCreate {
		// Take the precondition before either getRecord. A later head could
		// include an edit that our eligibility/absence checks never saw.
		commit, headErr = r.client.LatestCommit(ctx, r.did)
	}
	post, err := r.client.GetPost(ctx, r.did, key)
	absent := errors.Is(err, pds.ErrNotFound)
	if absent && !canCreate {
		return nil
	}
	if absent && (headErr != nil || commit == "") {
		return errors.New("repo commit lookup failed")
	}
	if err != nil && !absent {
		return errors.New("Announcement lookup failed")
	}
	canonical := r.origin + content.ArticlePrefix + url.PathEscape(a.Slug)
	if !absent && !verifiedPost(post, "at://"+r.did+"/"+pds.PostCollection+"/"+key, canonical) {
		return errors.New("conflicting post")
	}
	doc, err := r.client.GetDocument(ctx, r.did, key)
	if err != nil {
		return errors.New("document lookup failed")
	}
	if doc.URI != a.URI || doc.CID == "" || doc.Value.Type != pds.DocumentCollection || doc.Value.Site != r.publication || doc.Value.Path != content.ArticlePrefix+a.Slug {
		return errors.New("Article identity changed")
	}
	if ref := doc.Value.BskyPostRef; ref != nil {
		if absent || ref.URI != post.URI || ref.CID == "" {
			return errors.New("Article has a conflicting Announcement link")
		}
		return nil
	}
	var text string
	if absent {
		date, err := time.Parse(time.RFC3339, doc.Value.PublishedAt)
		if err != nil || !eligible(date, r.at) {
			return nil
		}
		text = doc.Value.Title + "\n\n" + canonical
		if !validPostText(text) {
			return errors.New("Article exceeds Announcement text limits")
		}
	}
	if r.token == "" {
		r.token, err = r.client.CreateSession(ctx, r.did, r.password)
		if err != nil {
			return errLogin
		}
	}
	ref := pds.StrongRef{URI: post.URI, CID: post.CID}
	if absent {
		ref, err = r.create(ctx, key, doc.Value, canonical, text, commit)
		if err != nil {
			return err
		}
	}
	if err := r.client.LinkDocument(ctx, r.token, r.did, key, doc, ref); err != nil {
		if errors.Is(err, pds.ErrSwapConflict) {
			return pds.ErrSwapConflict
		}
		return errors.New("conditional link failed")
	}
	return nil
}

// Unlike display classification, a matching key OR a self-link is not proof.
// Require a top-level post at the exact AT-URI, pointing to this exact page.
func verifiedPost(post pds.Record[pds.Post], uri, canonical string) bool {
	p := post.Value
	if post.URI != uri || post.CID == "" || p.Type != pds.PostCollection || (p.Reply != nil && p.Reply.Kind() != 'n') {
		return false
	}
	if p.Embed == nil || p.Embed.Type != pds.EmbedExternal || p.Embed.External == nil || p.Embed.External.URI != canonical {
		return false
	}
	for _, facet := range p.Facets {
		for _, f := range facet.Features {
			if f.Type == "app.bsky.richtext.facet#link" && f.URI != canonical {
				return false
			}
		}
	}
	return true
}
