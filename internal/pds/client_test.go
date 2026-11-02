package pds_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
)

const did = "did:plc:ogl52fc2rdooori2ayf74can"

func newClient(t *testing.T) (*pds.Client, *pdstest.Server) {
	t.Helper()
	fake := pdstest.New(t, did)
	return pds.New(fake.URL, fake.Client()), fake
}

func TestListDocumentsFollowsCursor(t *testing.T) {
	c, fake := newClient(t)
	for i := range 150 {
		fake.Put(pds.DocumentCollection, fmt.Sprintf("3mx%04d", i), map[string]any{"title": fmt.Sprint("Doc ", i)})
	}
	docs, err := c.ListDocuments(context.Background(), did)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 150 {
		t.Fatalf("got %d documents, want 150", len(docs))
	}
	seen := map[string]bool{}
	for _, d := range docs {
		if seen[d.URI] {
			t.Errorf("duplicate %s", d.URI)
		}
		seen[d.URI] = true
	}
	for _, u := range fake.Requests() {
		q := u.Query()
		if u.Path != "/xrpc/com.atproto.repo.listRecords" || q.Get("collection") != pds.DocumentCollection || q.Get("repo") != did {
			t.Errorf("unexpected request %s", u.String())
		}
		if q.Get("limit") != "100" {
			t.Errorf("limit = %q, want 100", q.Get("limit"))
		}
	}
}

func TestListDocumentsEmpty(t *testing.T) {
	c, _ := newClient(t)
	docs, err := c.ListDocuments(context.Background(), did)
	if err != nil || len(docs) != 0 {
		t.Errorf("ListDocuments on an empty repo = %v, %v", docs, err)
	}
}

func TestListDocumentsDecodes(t *testing.T) {
	c, fake := newClient(t)
	raw := `{"$type":"site.standard.document","site":"at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26","path":"/articles/welcome","title":"Welcome","description":"Hi.","publishedAt":"2026-09-28T00:00:00.000Z","updatedAt":"2026-10-20T20:18:27.757Z","textContent":"Hello","tags":["writing"],"content":{"$type":"at.markpub.markdown","flavor":"gfm","text":{"$type":"at.markpub.text","markdown":"## Hello"}},"coverImage":{"$type":"blob","ref":{"$link":"bafkreicover"},"mimeType":"image/png","size":42},"bskyPostRef":{"uri":"at://did:plc:ogl52fc2rdooori2ayf74can/app.bsky.feed.post/3mydgsmc5ycfg","cid":"bafyreipost"},"com.example.extra":{"kept":true}}`
	cid := fake.Put(pds.DocumentCollection, "3mydgsmc5ycfg", json.RawMessage(raw))

	docs, err := c.ListDocuments(context.Background(), did)
	if err != nil || len(docs) != 1 {
		t.Fatalf("ListDocuments = %d docs, %v", len(docs), err)
	}
	d := docs[0]
	if d.URI != "at://"+did+"/site.standard.document/3mydgsmc5ycfg" || d.CID != cid {
		t.Errorf("URI, CID = %q, %q", d.URI, d.CID)
	}
	v := d.Value
	if v.Path != "/articles/welcome" || v.Title != "Welcome" || v.Description != "Hi." || v.TextContent != "Hello" ||
		v.PublishedAt != "2026-09-28T00:00:00.000Z" || v.UpdatedAt != "2026-10-20T20:18:27.757Z" ||
		len(v.Tags) != 1 || v.Tags[0] != "writing" || !strings.HasSuffix(v.Site, "/3mxcsltpg2k26") {
		t.Errorf("decoded %+v", v)
	}
	if v.CoverImage == nil || v.CoverImage.Ref.Link != "bafkreicover" || v.CoverImage.MimeType != "image/png" {
		t.Errorf("CoverImage = %+v", v.CoverImage)
	}
	if v.BskyPostRef == nil || v.BskyPostRef.CID != "bafyreipost" {
		t.Errorf("BskyPostRef = %+v", v.BskyPostRef)
	}
	if !strings.Contains(string(v.Content), `"markdown":"## Hello"`) {
		t.Errorf("Content = %s", v.Content)
	}
	if string(d.Raw) != raw {
		t.Errorf("Raw is not the record's exact JSON:\n%s", d.Raw)
	}
}

func TestListPosts(t *testing.T) {
	c, fake := newClient(t)
	for _, rkey := range []string{"3maaa", "3mbbb", "3mccc"} {
		fake.Put(pds.PostCollection, rkey, map[string]any{"text": rkey})
	}

	page, cursor, err := c.ListPosts(context.Background(), did, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Value.Text != "3mccc" || page[1].Value.Text != "3mbbb" {
		t.Fatalf("first page = %+v", page)
	}
	if cursor == "" {
		t.Fatal("want a cursor after the first page")
	}

	page, _, err = c.ListPosts(context.Background(), did, cursor, 2)
	if err != nil || len(page) != 1 || page[0].Value.Text != "3maaa" {
		t.Fatalf("second page = %+v, %v", page, err)
	}

	reqs := fake.Requests()
	first, second := reqs[0].Query(), reqs[1].Query()
	if first.Get("limit") != "2" || first.Has("cursor") && first.Get("cursor") != "" {
		t.Errorf("first request query = %v", first)
	}
	if second.Get("cursor") != cursor || second.Get("collection") != pds.PostCollection {
		t.Errorf("second request query = %v", second)
	}
}

func TestListPostsEscapesCursor(t *testing.T) {
	c, fake := newClient(t)
	weird := "3m&limit=1 x"
	if _, _, err := c.ListPosts(context.Background(), did, weird, 20); err != nil {
		t.Fatal(err)
	}
	q := fake.Requests()[0].Query()
	if q.Get("cursor") != weird || q.Get("limit") != "20" {
		t.Errorf("cursor and limit not passed through intact: %v", q)
	}
}

func TestGetPost(t *testing.T) {
	c, fake := newClient(t)
	cid := fake.Put(pds.PostCollection, "3mxc6hxwdnc2s", map[string]any{"text": "hello", "createdAt": "2026-10-01T12:00:00.000Z"})
	p, err := c.GetPost(context.Background(), did, "3mxc6hxwdnc2s")
	if err != nil {
		t.Fatal(err)
	}
	if p.Value.Text != "hello" || p.CID != cid || p.URI != "at://"+did+"/app.bsky.feed.post/3mxc6hxwdnc2s" {
		t.Errorf("GetPost = %+v", p)
	}
}

func TestGetPostNotFound(t *testing.T) {
	c, _ := newClient(t)
	_, err := c.GetPost(context.Background(), did, "missing")
	if !errors.Is(err, pds.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestServerErrorIsNotNotFound(t *testing.T) {
	c, fake := newClient(t)
	fake.FailWith(http.StatusInternalServerError)
	_, err := c.GetPost(context.Background(), did, "x")
	if err == nil || errors.Is(err, pds.ErrNotFound) {
		t.Errorf("GetPost err = %v, want a non-NotFound error", err)
	}
	if err != nil && !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q should mention the status", err)
	}
	if _, err := c.ListDocuments(context.Background(), did); err == nil {
		t.Error("ListDocuments: want an error")
	}
	if _, _, err := c.ListPosts(context.Background(), did, "", 20); err == nil {
		t.Error("ListPosts: want an error")
	}
}

func TestContextDeadline(t *testing.T) {
	c, fake := newClient(t)
	fake.Delay(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.GetPost(ctx, did, "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("took %v: the context's deadline wasn't used", time.Since(start))
	}
}

func TestGetBlob(t *testing.T) {
	c, fake := newClient(t)
	fake.PutBlob("bafkreiimage", "image/jpeg", []byte("jpeg bytes"))
	body, contentType, err := c.GetBlob(context.Background(), did, "bafkreiimage")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "jpeg bytes" || contentType != "image/jpeg" {
		t.Errorf("GetBlob = %q, %q", data, contentType)
	}
	u := fake.Requests()[0]
	if u.Path != "/xrpc/com.atproto.sync.getBlob" || u.Query().Get("did") != did || u.Query().Get("cid") != "bafkreiimage" {
		t.Errorf("request = %s", u.String())
	}
}

func TestGetBlobNotFound(t *testing.T) {
	c, _ := newClient(t)
	_, _, err := c.GetBlob(context.Background(), did, "bafkreimissing")
	if !errors.Is(err, pds.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRKey(t *testing.T) {
	tests := map[string]string{
		"at://did:plc:x/app.bsky.feed.post/3mxc6hxwdnc2s":        "3mxc6hxwdnc2s",
		"at://did:plc:x/site.standard.document/3mydgsmc5ycfg":    "3mydgsmc5ycfg",
		"at://did:plc:x/site.standard.publication/3mxcsltpg2k26": "3mxcsltpg2k26",
	}
	for uri, want := range tests {
		if got := pds.RKey(uri); got != want {
			t.Errorf("RKey(%q) = %q, want %q", uri, got, want)
		}
	}
}
