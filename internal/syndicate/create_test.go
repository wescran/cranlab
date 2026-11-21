package syndicate_test

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/syndicate"
	assets "github.com/wescran/cranlab/web"
)

var daySeven = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func document(key, date string) pds.Document {
	return pds.Document{Type: pds.DocumentCollection, Site: publication, Path: "/articles/" + key, Title: "Café 🐈", Description: "Card copy.", PublishedAt: date}
}

func seed(f *pdstest.Server, key string, d pds.Document) content.Article {
	cid := f.Put(pds.DocumentCollection, key, d)
	return content.Articles([]pds.Record[pds.Document]{{URI: "at://" + did + "/" + pds.DocumentCollection + "/" + key, CID: cid, Value: d}}, publication)[0]
}

func TestThreeOldestEligiblePerRun(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	var source articles
	for _, tc := range []struct{ key, date string }{
		{"today", "2026-11-08T12:00:00Z"}, {"future", "2026-11-08T12:00:01Z"}, {"third", "2026-11-06T12:00:00Z"},
		{"invalid", "not a date"}, {"second", "2026-11-04T11:00:00Z"}, {"first", "2026-11-01T12:00:00Z"}, {"expired", "2026-11-01T11:59:59Z"},
	} {
		source = append(source, seed(f, tc.key, document(tc.key, tc.date)))
	}
	r := syndicate.New(source, c, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return daySeven })
	r.Run(t.Context())
	for _, key := range []string{"first", "second", "third"} {
		if _, err := c.GetPost(t.Context(), did, key); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	if _, err := c.GetPost(t.Context(), did, "today"); !errors.Is(err, pds.ErrNotFound) {
		t.Fatalf("created a fourth post: %v", err)
	}
	r.Run(t.Context())
	if _, err := c.GetPost(t.Context(), did, "today"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"expired", "future", "invalid"} {
		if _, err := c.GetPost(t.Context(), did, key); !errors.Is(err, pds.ErrNotFound) {
			t.Errorf("ineligible %s: %v", key, err)
		}
	}
}

func TestAnnouncementWithCoverAndTextExcerpt(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	doc := document("old", "2026-11-08T01:00:00Z")
	doc.Description = ""
	doc.TextContent = strings.Repeat("界", 300) + "more content"
	doc.CoverImage = &pds.Blob{Type: "blob", MIMEType: "image/png", Size: 3}
	doc.CoverImage.Ref.Link = "bafkreicover"
	f.PutBlob("bafkreicover", "image/png", []byte{1, 2, 3})
	a := seed(f, "old", doc)
	syndicate.New(articles{a}, c, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return daySeven }).Run(t.Context())
	post, err := c.GetPost(t.Context(), did, "old")
	if err != nil {
		t.Fatal(err)
	}
	card := post.Value.Embed.External
	if card.Thumb == nil || *card.Thumb != *doc.CoverImage || card.Description != strings.Repeat("界", 300) {
		t.Fatalf("unexpected card: %+v", card)
	}
}

func TestDaySevenAnnouncement(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	a := seed(f, "old", document("old", "2026-11-01T12:00:00Z"))
	r := syndicate.New(articles{a}, c, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return daySeven })
	r.Run(t.Context())
	post, err := c.GetPost(t.Context(), did, "old")
	if err != nil {
		t.Fatal(err)
	}
	p := post.Value
	if p.Type != pds.PostCollection || p.Text != "Café 🐈\n\n"+articleURL || p.CreatedAt != "2026-11-08T12:00:00Z" || !slices.Equal(p.Langs, []string{"en"}) {
		t.Fatalf("unexpected post: %s", post.Raw)
	}
	if len(p.Facets) != 1 || p.Facets[0].Index != (pds.ByteSlice{ByteStart: 12, ByteEnd: 12 + len(articleURL)}) || !slices.Equal(p.Facets[0].Features, []pds.Feature{{Type: "app.bsky.richtext.facet#link", URI: articleURL}}) {
		t.Fatalf("facets: %+v", p.Facets)
	}
	if p.Embed == nil || p.Embed.Type != pds.EmbedExternal || p.Embed.External == nil {
		t.Fatalf("missing card: %s", post.Raw)
	}
	card := p.Embed.External
	if card.URI != articleURL || card.Title != "Café 🐈" || card.Description != "Card copy." || card.Thumb == nil {
		t.Fatalf("card: %+v", card)
	}
	body, mime, err := c.GetBlob(t.Context(), did, card.Thumb.Ref.Link)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	want, err := assets.FS.ReadFile("static/images/card-default.png")
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" || !bytes.Equal(data, want) {
		t.Fatal("wrong default thumbnail")
	}
	doc, err := c.GetDocument(t.Context(), did, "old")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Value.BskyPostRef == nil || *doc.Value.BskyPostRef != (pds.StrongRef{URI: post.URI, CID: post.CID}) {
		t.Fatalf("wrong link: %s", doc.Raw)
	}
}
