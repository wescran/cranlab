package content

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

const (
	did         = "did:plc:ogl52fc2rdooori2ayf74can"
	publication = "at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26"
)

func doc(rkey, path, published string, tags ...string) pds.Record[pds.Document] {
	return pds.Record[pds.Document]{
		URI:   "at://" + did + "/site.standard.document/" + rkey,
		Value: pds.Document{Site: publication, Path: path, Title: rkey, PublishedAt: published, Tags: tags},
	}
}

func slugs(articles []Article) []string {
	out := []string{}
	for _, a := range articles {
		out = append(out, a.Slug)
	}
	return out
}

func TestArticlesFiltersAndSorts(t *testing.T) {
	other := doc("3mother", "/articles/other-site", "2026-10-05T00:00:00Z")
	other.Value.Site = "at://did:plc:someone/site.standard.publication/3mx"
	docs := []pds.Record[pds.Document]{
		doc("3mold", "/articles/old", "2026-09-01T00:00:00.000Z"),
		doc("3mnew", "/articles/new", "2026-10-20T20:18:27.757Z"),
		other,
		doc("3mnote", "/notes/not-an-article", "2026-10-10T00:00:00Z"),
		doc("3mdeep", "/articles/a/b", "2026-10-10T00:00:00Z"),
		doc("3mempty", "/articles/", "2026-10-10T00:00:00Z"),
		doc("3mmid", "/articles/mid", "2026-10-01T00:00:00Z"),
		doc("3mbad", "/articles/bad-date", "yesterday"),
	}
	got := Articles(docs, publication)
	if want := []string{"new", "mid", "old", "bad-date"}; !reflect.DeepEqual(slugs(got), want) {
		t.Errorf("slugs = %v, want %v (publication only, /articles/<slug> only, newest first, unparsable dates last)", slugs(got), want)
	}
	if got[0].Published.Format("2006-01-02T15:04:05.000") != "2026-10-20T20:18:27.757" {
		t.Errorf("Published = %v", got[0].Published)
	}
	if !got[3].Published.IsZero() {
		t.Errorf("an unparsable date should be zero, got %v", got[3].Published)
	}
}

func TestArticlesTieBreak(t *testing.T) {
	docs := []pds.Record[pds.Document]{
		doc("3mb", "/articles/b", "2026-10-01T00:00:00Z"),
		doc("3ma", "/articles/a", "2026-10-01T00:00:00Z"),
	}
	if got := slugs(Articles(docs, publication)); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("same date: slugs = %v, want [a b]", got)
	}
}

func TestFindArticle(t *testing.T) {
	articles := Articles([]pds.Record[pds.Document]{doc("3mw", "/articles/welcome", "2026-09-28T00:00:00Z")}, publication)
	if a, ok := FindArticle(articles, "welcome"); !ok || a.Slug != "welcome" {
		t.Errorf("FindArticle(welcome) = %+v, %v", a, ok)
	}
	for _, slug := range []string{"", "Welcome", "welcome/", "missing"} {
		if _, ok := FindArticle(articles, slug); ok {
			t.Errorf("FindArticle(%q) found something", slug)
		}
	}
}

func post(rkey, text string, reply bool) pds.Record[pds.Post] {
	p := pds.Record[pds.Post]{URI: "at://" + did + "/app.bsky.feed.post/" + rkey, Value: pds.Post{Text: text}}
	if reply {
		p.Value.Reply = []byte(`{"root":{},"parent":{}}`)
	}
	return p
}

func TestIsNote(t *testing.T) {
	ann := Announcements([]pds.Record[pds.Document]{doc("3mart", "/articles/a", "")})
	if !ann["3mart"] || len(ann) != 1 {
		t.Errorf("Announcements = %v", ann)
	}
	if !IsNote(post("3mpost", "hi", false), ann) {
		t.Error("a top-level post is a note")
	}
	if IsNote(post("3mreply", "hi", true), ann) {
		t.Error("a reply isn't a note")
	}
	if IsNote(post("3mart", "New article", false), ann) {
		t.Error("an announcement isn't a note")
	}
}

// lister serves posts newest first in pages, like listRecords: the cursor is
// the record key of the last post returned, returned with every non-empty
// page.
type lister struct {
	posts []pds.Record[pds.Post] // newest first
	calls []string               // cursors requested
	err   error
}

func (l *lister) ListPosts(_ context.Context, _ string, cursor string, limit int) ([]pds.Record[pds.Post], string, error) {
	l.calls = append(l.calls, cursor)
	if l.err != nil {
		return nil, "", l.err
	}
	if limit < 1 || limit > 100 {
		return nil, "", fmt.Errorf("limit %d out of range", limit)
	}
	start := 0
	if cursor != "" {
		for start < len(l.posts) && pds.RKey(l.posts[start].URI) >= cursor {
			start++
		}
	}
	end := min(start+limit, len(l.posts))
	page := l.posts[start:end]
	if len(page) == 0 {
		return page, "", nil
	}
	return page, pds.RKey(page[len(page)-1].URI), nil
}

// posts makes count posts with descending record keys "3m0999", "3m0998", …;
// every replyEvery-th is a reply (0: none).
func makePosts(count, replyEvery int) []pds.Record[pds.Post] {
	var out []pds.Record[pds.Post]
	for i := range count {
		rkey := fmt.Sprintf("3m%04d", 9999-i)
		out = append(out, post(rkey, rkey, replyEvery > 0 && i%replyEvery == replyEvery-1))
	}
	return out
}

func rkeys(posts []pds.Record[pds.Post]) []string {
	out := []string{}
	for _, p := range posts {
		out = append(out, pds.RKey(p.URI))
	}
	return out
}

func TestNotesFirstPage(t *testing.T) {
	l := &lister{posts: makePosts(50, 0)}
	notes, cursor, err := Notes(context.Background(), l, did, nil, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 20 || pds.RKey(notes[0].URI) != "3m9999" || pds.RKey(notes[19].URI) != "3m9980" {
		t.Fatalf("notes = %v", rkeys(notes))
	}
	if cursor != "3m9980" {
		t.Errorf("cursor = %q, want the last note's record key", cursor)
	}
}

func TestNotesContinuesFromCursor(t *testing.T) {
	l := &lister{posts: makePosts(50, 0)}
	first, cursor, _ := Notes(context.Background(), l, did, nil, "", 20)
	second, _, err := Notes(context.Background(), l, did, nil, cursor, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 20 || pds.RKey(second[0].URI) != "3m9979" {
		t.Errorf("second page = %v", rkeys(second))
	}
	seen := map[string]bool{}
	for _, p := range append(first, second...) {
		if seen[p.URI] {
			t.Errorf("%s returned twice", p.URI)
		}
		seen[p.URI] = true
	}
}

func TestNotesSkipsRepliesAndAnnouncements(t *testing.T) {
	posts := makePosts(30, 3) // every third is a reply
	ann := map[string]bool{"3m9999": true}
	l := &lister{posts: posts}
	notes, _, err := Notes(context.Background(), l, did, ann, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"3m9998", "3m9996", "3m9995", "3m9993", "3m9992"}
	if !reflect.DeepEqual(rkeys(notes), want) {
		t.Errorf("notes = %v, want %v", rkeys(notes), want)
	}
}

func TestNotesReadsMorePagesToFillOne(t *testing.T) {
	// 150 posts, only every 10th a note: 20 notes need two pages of 100.
	var posts []pds.Record[pds.Post]
	for i, p := range makePosts(150, 0) {
		if i%10 != 0 {
			p.Value.Reply = []byte(`{}`)
		}
		posts = append(posts, p)
	}
	l := &lister{posts: posts}
	notes, cursor, err := Notes(context.Background(), l, did, nil, "", 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 15 || len(l.calls) != 2 || cursor != pds.RKey(notes[14].URI) {
		t.Errorf("got %d notes in %d calls, cursor %q", len(notes), len(l.calls), cursor)
	}
}

func TestNotesRunsOut(t *testing.T) {
	l := &lister{posts: makePosts(7, 0)}
	notes, cursor, err := Notes(context.Background(), l, did, nil, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 7 || cursor != "" {
		t.Errorf("got %d notes, cursor %q; want 7 and no cursor", len(notes), cursor)
	}
}

func TestNotesGivesUpAfterTooManyPages(t *testing.T) {
	var posts []pds.Record[pds.Post]
	for _, p := range makePosts(2000, 0) {
		p.Value.Reply = []byte(`{}`) // nothing but replies
		posts = append(posts, p)
	}
	l := &lister{posts: posts}
	notes, cursor, err := Notes(context.Background(), l, did, nil, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 || len(l.calls) != 10 {
		t.Errorf("got %d notes in %d calls, want 0 in 10", len(notes), len(l.calls))
	}
	if cursor == "" {
		t.Error("want a cursor to continue from, since posts remain")
	}
}

func TestNotesError(t *testing.T) {
	boom := errors.New("boom")
	_, _, err := Notes(context.Background(), &lister{err: boom}, did, nil, "", 20)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the lister's error", err)
	}
}

func TestTags(t *testing.T) {
	articles := Articles([]pds.Record[pds.Document]{
		doc("3m1", "/articles/one", "2026-10-03T00:00:00Z", "homelab", "Go"),
		doc("3m2", "/articles/two", "2026-10-02T00:00:00Z", "writing", "homelab"),
		doc("3m3", "/articles/three", "2026-10-01T00:00:00Z", "atproto"),
	}, publication)
	want := []TagCount{{"atproto", 1}, {"Go", 1}, {"homelab", 2}, {"writing", 1}}
	if got := Tags(articles); !reflect.DeepEqual(got, want) {
		t.Errorf("Tags = %v, want %v (sorted ignoring case)", got, want)
	}
	if got := slugs(WithTag(articles, "homelab")); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Errorf("WithTag(homelab) = %v", got)
	}
	if got := WithTag(articles, "go"); len(got) != 0 {
		t.Errorf("tags match exactly: WithTag(go) = %v", slugs(got))
	}
	if got := Tags(nil); len(got) != 0 {
		t.Errorf("Tags(nil) = %v", got)
	}
}
