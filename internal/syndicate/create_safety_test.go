package syndicate_test

import (
	"encoding/json/jsontext"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/syndicate"
)

func TestFailedLinkRepairsNextDayAndNeverRewritesPost(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	source := articles{seed(f, "old", document("old", "2026-11-01T12:00:00Z"))}
	now := daySeven
	r := syndicate.New(source, c, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return now })
	f.Respond("com.atproto.repo.putRecord", "", 503, `{}`)
	r.Run(t.Context())
	// The repo precondition must bracket the public reads and creation.
	pinned, checked, created := false, false, false
	for _, u := range f.Requests() {
		q := u.Query()
		if u.Path == "/xrpc/com.atproto.sync.getLatestCommit" && q.Get("did") == did {
			pinned = true
		}
		if u.Path == "/xrpc/com.atproto.repo.getRecord" && !pinned {
			t.Fatal("read before capturing repo commit")
		}
		if u.Path == "/xrpc/com.atproto.repo.getRecord" && q.Get("collection") == pds.PostCollection && q.Get("rkey") == "old" && q.Get("repo") == did {
			checked = true
		}
		if u.Path == "/xrpc/com.atproto.repo.createRecord" {
			if !checked {
				t.Fatal("created without first checking for absence")
			}
			created = true
		}
	}
	if !created {
		t.Fatal("no create on day seven")
	}
	post, err := c.GetPost(t.Context(), did, "old")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := c.GetDocument(t.Context(), did, "old")
	if err != nil || doc.Value.BskyPostRef != nil {
		t.Fatalf("expected unlinked document: %s (%v)", doc.Raw, err)
	}
	now = now.Add(24 * time.Hour)
	f.Respond("com.atproto.repo.putRecord", "", 0, "")
	before := len(f.Requests())
	r.Run(t.Context())
	doc, err = c.GetDocument(t.Context(), did, "old")
	if err != nil || doc.Value.BskyPostRef == nil || *doc.Value.BskyPostRef != (pds.StrongRef{URI: post.URI, CID: post.CID}) {
		t.Fatalf("expected repaired document: %s (%v)", doc.Raw, err)
	}
	doc.Value.Title = "An edited title"
	source[0] = seed(f, "old", doc.Value)
	r.Run(t.Context())
	for _, u := range f.Requests()[before:] {
		if u.Path == "/xrpc/com.atproto.repo.createRecord" || u.Path == "/xrpc/com.atproto.repo.uploadBlob" {
			t.Fatal("attempt to recreate post:", u.Path)
		}
	}
	after, err := c.GetPost(t.Context(), did, "old")
	if err != nil || after.CID != post.CID {
		t.Fatalf("post changed: %s (%v)", after.Raw, err)
	}
}

func TestUncertainLookupNeverCreates(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	a := seed(f, "old", document("old", "2026-11-08T00:00:00Z"))
	f.Respond("com.atproto.repo.getRecord", "", 503, `{"error":"Unavailable"}`)
	syndicate.New(articles{a}, c, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return daySeven }).Run(t.Context())
	for _, u := range f.Requests() {
		if u.Path != "/xrpc/com.atproto.repo.getRecord" && u.Path != "/xrpc/com.atproto.sync.getLatestCommit" {
			t.Fatal("unexpected request:", u)
		}
	}
}

func TestConcurrentPostAtArticleKeyIsNotOverwritten(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	a := seed(f, "old", document("old", "2026-11-08T01:00:00Z"))
	var cid string
	racing := concurrentEdit{c, func() {
		cid = f.Put(pds.PostCollection, "old", jsontext.Value(`{"$type":"app.bsky.feed.post","text":"independent Note"}`))
	}}
	syndicate.New(articles{a}, racing, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return daySeven }).Run(t.Context())
	post, err := c.GetPost(t.Context(), did, "old")
	if err != nil || cid == "" || post.CID != cid {
		t.Fatalf("lost competing post: %s (%v)", post.Raw, err)
	}
	doc, err := c.GetDocument(t.Context(), did, "old")
	if err != nil || doc.Value.BskyPostRef != nil {
		t.Fatalf("incorrect link: %s (%v)", doc.Raw, err)
	}
}

func TestCreateRevalidatesConcurrentArticleChange(t *testing.T) {
	for _, change := range []string{"unpublish", "edit", "link"} {
		t.Run(change, func(t *testing.T) {
			f := pdstest.New(t, did)
			c := pds.New(f.URL, nil)
			d := document("old", "2026-11-08T01:00:00Z")
			a := seed(f, "old", d)
			racing := concurrentEdit{c, func() {
				switch change {
				case "unpublish":
					f.Delete(pds.DocumentCollection, "old")
				case "edit":
					d.Title = "Corrected title"
					f.Put(pds.DocumentCollection, "old", d)
				case "link":
					d.BskyPostRef = &pds.StrongRef{URI: postURI, CID: "other"}
					f.Put(pds.DocumentCollection, "old", d)
				}
			}}
			syndicate.New(articles{a}, racing, did, publication, origin, pdstest.Password, slog.Default(), func() time.Time { return daySeven }).Run(t.Context())
			post, err := c.GetPost(t.Context(), did, "old")
			if change == "edit" {
				if err != nil || post.Value.Text != "Corrected title\n\n"+articleURL {
					t.Fatalf("expected fresh Announcement: %q (%v)", post.Value.Text, err)
				}
			} else if !errors.Is(err, pds.ErrNotFound) {
				t.Fatalf("stale Announcement after %s: %q (%v)", change, post.Value.Text, err)
			}
		})
	}
}

func TestNoCredentialsNoAnnouncement(t *testing.T) {
	f := pdstest.New(t, did)
	c := pds.New(f.URL, nil)
	a := seed(f, "old", document("old", "2026-11-08T00:00:00Z"))
	syndicate.New(articles{a}, c, did, publication, origin, "", slog.Default(), func() time.Time { return daySeven }).Run(t.Context())
	if len(f.Requests()) != 0 {
		t.Fatal("disabled runner contacted PDS")
	}
	if _, err := c.GetPost(t.Context(), did, "old"); !errors.Is(err, pds.ErrNotFound) {
		t.Fatalf("expected no post: %v", err)
	}
}
