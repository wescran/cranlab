package syndicate_test

import (
	"context"
	"encoding/json/jsontext"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/syndicate"
)

// The publisher edits the document after the runner's read, just before its
// first authenticated write. Only real conditional requests hit the fake.
type concurrentEdit struct {
	*pds.Client
	edit func()
}

func (c concurrentEdit) CreateSession(ctx context.Context, did, password string) (string, error) {
	c.edit()
	return c.Client.CreateSession(ctx, did, password)
}

func TestConflictRechecksEditedArticle(t *testing.T) {
	for _, tc := range []struct {
		name, edit string
		wantLink   bool
	}{
		{"keep concurrent title edit", strings.Replace(docJSON, `"extra":`, `"title":"my edit","extra":`, 1), true},
		{"do not link after a rename", strings.Replace(docJSON, "/articles/old", "/articles/renamed", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := pdstest.New(t, did)
			c := pds.New(f.URL, nil)
			f.Put(pds.DocumentCollection, "key", jsontext.Value(docJSON))
			f.Put(pds.PostCollection, "key", jsontext.Value(postJSON))
			racing := concurrentEdit{c, func() { f.Put(pds.DocumentCollection, "key", jsontext.Value(tc.edit)) }}
			syndicate.New(articles{{URI: docURI, Slug: "old"}}, racing, did, publication, origin, pdstest.Password, slog.Default(), time.Now).Run(t.Context())
			doc, err := c.GetDocument(t.Context(), did, "key")
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantLink {
				if doc.Value.Title != "my edit" || doc.Value.BskyPostRef == nil || doc.Value.BskyPostRef.URI != postURI {
					t.Fatalf("lost edit or link: %s", doc.Raw)
				}
			} else if doc.Value.BskyPostRef != nil || doc.Value.Path != "/articles/renamed" {
				t.Fatalf("clobbered renamed Article: %s", doc.Raw)
			}
		})
	}
}
