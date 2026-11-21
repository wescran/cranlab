package web

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
)

func TestOlderNotesCursorSurvivesNewInsertAndDeletedBoundary(t *testing.T) {
	fake := pdstest.New(t, testDID)
	for i := range 22 {
		fake.Put(pds.PostCollection, fmt.Sprintf("3n%04d", i), map[string]string{"text": "note", "createdAt": "0001-01-01T00:00:00Z"})
	}
	f := startFixture(t, fake, true)
	_, body := f.get(t, "/notes")
	older := regexp.MustCompile(`href="(/notes\?cursor=[^"]+)" rel="next"`).FindStringSubmatch(body)
	if older == nil || older[1] == "/notes?cursor=3n0002" {
		t.Fatalf("expected opaque cursor: %v", older)
	}
	fake.Delete(pds.PostCollection, "3n0002")
	fake.Put(pds.PostCollection, "3n9999", map[string]string{"text": "new", "createdAt": "2026-10-01T00:00:00Z"})
	if err := f.store.RefreshPosts(t.Context(), pds.New(fake.URL, fake.Client()), true); err != nil {
		t.Fatal(err)
	}
	_, body = f.get(t, older[1])
	if got := strings.Join(noteKeys(body), ","); got != "3n0001,3n0000" {
		t.Errorf("older tied notes after update: %q", got)
	}
}

func TestQuoteAndSelfReplyServedFromSnapshot(t *testing.T) {
	fake := pdstest.New(t, testDID)
	ref := map[string]string{"uri": "at://" + testDID + "/app.bsky.feed.post/original", "cid": "cid"}
	fake.Put(pds.PostCollection, "quote", map[string]any{"text": "🍁 go.dev", "facets": []any{map[string]any{"index": map[string]int{"byteStart": 5, "byteEnd": 11}, "features": []any{map[string]string{"$type": "app.bsky.richtext.facet#link", "uri": "https://go.dev"}}}}, "embed": map[string]any{"$type": pds.EmbedRecord, "record": ref}})
	fake.Put(pds.PostCollection, "reply", map[string]any{"text": "Self reply", "reply": map[string]any{"parent": ref, "root": ref}})
	f := startFixture(t, fake, true)
	fake.FailWith(503)
	before := len(fake.Requests())
	code, body := f.get(t, "/notes/quote")
	if code != 200 {
		t.Fatal(code)
	}
	mustContain(t, body, `🍁 <a href="https://go.dev">go.dev</a>`, `href="https://bsky.app/profile/`+testDID+`/post/original"`, `class="p-author h-card"`, `class="u-syndication"`)
	mustNotContain(t, body, `<h1`, `<h2`)
	for _, path := range []string{"/notes", "/"} {
		code, body = f.get(t, path)
		if code != 200 || !strings.Contains(body, `href="/notes/quote"`) || strings.Contains(body, "Self reply") {
			t.Errorf("bad snapshot on %s (%d)", path, code)
		}
	}
	if code, _ := f.get(t, "/notes/reply"); code != 404 {
		t.Fatalf("self-reply: %d", code)
	}
	if extra := fake.Requests()[before:]; len(extra) > 0 {
		t.Errorf("snapshot pages called PDS: %v", extra)
	}
}
