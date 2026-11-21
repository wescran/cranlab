package web

import (
	"context"
	"log/slog"

	"github.com/wescran/cranlab/internal/content"
)

// InteractionReader looks up responses to one canonical entry, independently
// of the Article/Note snapshot. A future Backfeed merger can serve this same seam.
type InteractionReader interface {
	Interactions(context.Context, string) ([]content.Interaction, error)
}

type interactionGroup struct {
	Label, Property string
	Facepile        bool
	Items           []content.Interaction
}

func (s *server) interactionGroups(ctx context.Context, path string) []interactionGroup {
	if s.deps.Interactions == nil {
		return nil
	}
	items, err := s.deps.Interactions.Interactions(ctx, s.cfg.SiteURL+path)
	if err != nil {
		slog.WarnContext(ctx, "interactions unavailable", "path", path, "err", err)
		return nil
	}
	byKind := map[content.InteractionKind][]content.Interaction{}
	for _, item := range items {
		byKind[item.Kind] = append(byKind[item.Kind], item)
	}
	var groups []interactionGroup
	for _, spec := range []struct {
		kind            content.InteractionKind
		label, property string
		facepile        bool
	}{
		{content.Like, "Likes", "p-like", true}, {content.Repost, "Reposts", "p-repost", true},
		{content.Reply, "Replies", "p-comment", false}, {content.Mention, "Mentions", "p-mention", false},
		{content.Bookmark, "Bookmarks", "p-mention", false}, {content.RSVP, "RSVPs", "p-mention", false},
	} {
		if items := byKind[spec.kind]; len(items) > 0 {
			groups = append(groups, interactionGroup{Label: spec.label, Property: spec.property, Facepile: spec.facepile, Items: items})
		}
	}
	return groups
}
