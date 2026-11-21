package store_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/store"
)

func TestRecentAndFullPostRefresh(t *testing.T) {
	const did, pub, site = "did:plc:local", "at://did:plc:local/site.standard.publication/site", "https://cranlab.test"
	f := pdstest.New(t, did)
	c := pds.New(f.URL, f.Client())
	ctx := t.Context()
	db, err := store.Open(filepath.Join(t.TempDir(), "cache.db"), did, pub, site)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	refreshArticles := func() {
		t.Helper()
		if err := db.RefreshArticles(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	refreshPosts := func(full bool) {
		t.Helper()
		if err := db.RefreshPosts(ctx, c, full); err != nil {
			t.Fatal(err)
		}
	}
	check := func(key, text string) {
		t.Helper()
		note, found, err := db.FindNote(ctx, key)
		if err != nil || found != (text != "") || note.Text != text {
			t.Fatalf("note %s = %q, %t, %v; want %q", key, note.Text, found, err, text)
		}
	}
	put := func(key, text string) { f.Put(pds.PostCollection, key, map[string]string{"text": text}) }
	for i := 0; i < 100; i++ {
		put(fmt.Sprintf("z%03d", i), "newer")
	}
	put("a", "old")
	put("b", "before edit")
	put("c", "linked later")
	f.Put(pds.PostCollection, "d", map[string]any{"text": "manual link", "embed": map[string]any{"$type": pds.EmbedExternal, "external": map[string]string{"uri": site + "/articles/story"}}})
	refreshArticles()
	refreshPosts(true)
	check("a", "old")
	check("c", "linked later")
	check("d", "")
	f.Delete(pds.PostCollection, "a")
	put("b", "after edit")
	put("zz", "just published")
	f.Put(pds.DocumentCollection, "d", pds.Document{Site: pub, Path: "/articles/story", BskyPostRef: &pds.StrongRef{URI: "at://" + did + "/" + pds.PostCollection + "/c"}})
	refreshArticles()
	refreshPosts(false)
	check("a", "old")
	check("b", "before edit")
	check("c", "")
	check("zz", "just published")
	const method = "com.atproto.repo.listRecords"
	f.Respond(method, "z001", 503, `{}`)
	if err := db.RefreshPosts(ctx, c, true); err == nil {
		t.Fatal("accepted incomplete scan")
	}
	check("a", "old")
	check("b", "before edit")
	// A later page that replays records from the first is not proof of exhaustion.
	f.Respond(method, "z001", 200, `{"records":[{"uri":"at://did:plc:local/app.bsky.feed.post/zz","cid":"cid","value":{"text":"duplicate"}}]}`)
	if err := db.RefreshPosts(ctx, c, true); err == nil {
		t.Fatal("accepted overlapping pages")
	}
	check("a", "old")
	check("zz", "just published")
	f.Respond(method, "z001", 0, "")
	f.Delete(pds.DocumentCollection, "d")
	refreshArticles()
	refreshPosts(true)
	check("a", "")
	check("b", "after edit")
	check("c", "linked later")
	check("d", "")

	// One coalesced signal covers more than a page of new posts. Even when
	// the previous head has disappeared, this scan must not remove its Note.
	f.Delete(pds.PostCollection, "zz")
	for i := range 202 {
		put(fmt.Sprintf("zzz%03d", i), "burst")
	}
	refreshPosts(false)
	check("zzz000", "burst")
	check("zzz201", "burst")
	check("zz", "just published")
}
