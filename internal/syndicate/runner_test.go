package syndicate_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"log/slog"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/syndicate"
)

const (
	did         = "did:plc:fixture"
	publication = "at://did:plc:fixture/site.standard.publication/pub"
	docURI      = "at://did:plc:fixture/site.standard.document/key"
	postURI     = "at://did:plc:fixture/app.bsky.feed.post/key"
	origin      = "https://fixture.invalid"
	articleURL  = origin + "/articles/old"
	docJSON     = `{"$type":"site.standard.document","site":"` + publication + `","path":"/articles/old","publishedAt":"2001-01-01T00:00:00Z","extra":{"counter":9007199254740993}}`
	postJSON    = `{"$type":"app.bsky.feed.post","text":"Old\n\n` + articleURL + `","embed":{"$type":"app.bsky.embed.external","external":{"uri":"` + articleURL + `"}}}`
)

type articles []content.Article

func (a articles) Articles(context.Context) ([]content.Article, error) { return a, nil }

func TestRepairAtAnyAgePreservesDocument(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	f.Put(pds.DocumentCollection, "key", jsontext.Value(docJSON))
	cid := f.Put(pds.PostCollection, "key", jsontext.Value(postJSON))
	var logs bytes.Buffer
	r := syndicate.New(articles{{URI: docURI, Slug: "old"}}, c, did, publication, origin, pdstest.Password, slog.New(slog.NewTextHandler(&logs, nil)), time.Now)
	r.Run(t.Context())
	doc, err := c.GetDocument(t.Context(), did, "key")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Value.BskyPostRef; got == nil || *got != (pds.StrongRef{URI: postURI, CID: cid}) {
		t.Fatalf("ref=%+v, logs=%s", got, &logs)
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(doc.Raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["extra"]) != `{"counter":9007199254740993}` {
		t.Fatalf("unknown field changed: %s", fields["extra"])
	}
}
