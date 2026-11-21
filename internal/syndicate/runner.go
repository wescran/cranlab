// Package syndicate links Articles to verified Announcements. It never runs
// from a page request; the syncer calls Run after a successful refresh.
package syndicate

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"sync"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
)

var errLogin = errors.New("login failed")

// ArticleSource supplies the successfully committed read snapshot.
type ArticleSource interface {
	Articles(context.Context) ([]content.Article, error)
}

// Client uses public reads to verify a repair before acquiring a session.
type Client interface {
	GetPost(context.Context, string, string) (pds.Record[pds.Post], error)
	GetDocument(context.Context, string, string) (pds.Record[pds.Document], error)
	CreateSession(context.Context, string, string) (string, error)
	LinkDocument(context.Context, string, string, string, pds.Record[pds.Document], pds.StrongRef) error
}

type Runner struct {
	mu                                 sync.Mutex
	source                             ArticleSource
	client                             Client
	did, publication, origin, password string
	logger                             *slog.Logger
}

func New(source ArticleSource, client Client, did, publication, origin, password string, logger *slog.Logger) *Runner {
	return &Runner{source: source, client: client, did: did, publication: publication, origin: origin, password: password, logger: logger}
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
	token := ""
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
		err := r.repair(ctx, a, key, &token)
		if errors.Is(err, pds.ErrSwapConflict) {
			// Bound the retry, and re-verify both records against fresh state.
			err = r.repair(ctx, a, key, &token)
		}
		if err != nil {
			r.logger.Warn("syndication: review Article", "key", key, "reason", err)
			if errors.Is(err, errLogin) {
				return // do not repeat a failed login for every Article
			}
		}
	}
}

func (r *Runner) repair(ctx context.Context, a content.Article, key string, token *string) error {
	post, err := r.client.GetPost(ctx, r.did, key)
	if errors.Is(err, pds.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("Announcement lookup failed")
	}
	if !verifiedPost(post, "at://"+r.did+"/"+pds.PostCollection+"/"+key, r.origin+content.ArticlePrefix+url.PathEscape(a.Slug)) {
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
		if ref.URI != post.URI || ref.CID == "" {
			return errors.New("Article has a conflicting Announcement link")
		}
		return nil
	}
	if *token == "" {
		*token, err = r.client.CreateSession(ctx, r.did, r.password)
		if err != nil {
			return errLogin
		}
	}
	if err := r.client.LinkDocument(ctx, *token, r.did, key, doc, pds.StrongRef{URI: post.URI, CID: post.CID}); err != nil {
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
