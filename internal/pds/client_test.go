package pds_test

import (
	"context"
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
	if !docs.Exhausted || len(docs.Records) != 150 {
		t.Fatalf("got %d documents, want 150", len(docs.Records))
	}
	seen := map[string]bool{}
	for _, d := range docs.Records {
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

func TestListDocumentsStopsWithoutFinalCursor(t *testing.T) {
	c, fake := newClient(t)
	fake.OmitFinalCursor()
	for i := range 150 {
		fake.Put(pds.DocumentCollection, fmt.Sprintf("3mx%04d", i), map[string]any{"title": "x"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	docs, err := c.ListDocuments(ctx, did)
	if err != nil {
		t.Fatalf("ListDocuments: %v (a PDS that omits the last cursor must not loop)", err)
	}
	if !docs.Exhausted || len(docs.Records) != 150 || len(fake.Requests()) != 2 {
		t.Errorf("got %d documents in %d requests, want 150 in 2", len(docs.Records), len(fake.Requests()))
	}
}

func TestListDocumentsEmpty(t *testing.T) {
	c, _ := newClient(t)
	docs, err := c.ListDocuments(context.Background(), did)
	if err != nil || !docs.Exhausted || len(docs.Records) != 0 {
		t.Errorf("ListDocuments on an empty repo = %v, %v", docs, err)
	}
}

func TestListDocumentsDiscardsIncompleteListing(t *testing.T) {
	const uri = "at://" + did + "/" + pds.DocumentCollection + "/older"
	for _, tc := range []struct {
		name   string
		status int
		page   string
	}{
		{"missing records", 200, `{}`},
		{"null records", 200, `{"records":null}`},
		{"non-array records", 200, `{"records":{}}`},
		{"null value", 200, `{"records":[{"uri":"` + uri + `","cid":"cid","value":null}]}`},
		{"missing cid", 200, `{"records":[{"uri":"` + uri + `","value":{}}]}`},
		{"malformed uri", 200, `{"records":[{"uri":"not a record","cid":"cid","value":{}}]}`},
		{"malformed cursor", 200, `{"records":[],"cursor":null}`},
		{"non-progressing cursor", 200, `{"records":[],"cursor":"first"}`},
		{"failed later page", 503, `{"error":"Unavailable"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, fake := newClient(t)
			fake.Put(pds.DocumentCollection, "first", map[string]string{"title": "One"})
			fake.Respond("com.atproto.repo.listRecords", "first", tc.status, tc.page)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			result, err := c.ListDocuments(ctx, did)
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("listing failed to detect an invalid page before the deadline")
			}
			if err == nil || result.Exhausted || len(result.Records) != 0 {
				t.Errorf("incomplete listing returned %+v, %v", result, err)
			}
		})
	}
}

func TestListDocumentsDecodes(t *testing.T) {
	c, fake := newClient(t)
	raw := `{"$type":"site.standard.document","site":"at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26","path":"/articles/welcome","title":"Welcome","description":"Hi.","publishedAt":"2026-09-28T00:00:00.000Z","updatedAt":"2026-10-20T20:18:27.757Z","textContent":"Hello","tags":["writing"],"content":{"$type":"at.markpub.markdown","flavor":"gfm","text":{"$type":"at.markpub.text","markdown":"## Hello"}},"coverImage":{"$type":"blob","ref":{"$link":"bafkreicover"},"mimeType":"image/png","size":42},"bskyPostRef":{"uri":"at://did:plc:ogl52fc2rdooori2ayf74can/app.bsky.feed.post/3mydgsmc5ycfg","cid":"bafyreipost"},"com.example.extra":{"kept":true}}`
	cid := fake.Put(pds.DocumentCollection, "3mydgsmc5ycfg", []byte(raw))

	docs, err := c.ListDocuments(context.Background(), did)
	if err != nil || len(docs.Records) != 1 {
		t.Fatalf("ListDocuments = %d docs, %v", len(docs.Records), err)
	}
	d := docs.Records[0]
	if d.URI != "at://"+did+"/site.standard.document/3mydgsmc5ycfg" || d.CID != cid {
		t.Errorf("URI, CID = %q, %q", d.URI, d.CID)
	}
	v := d.Value
	if v.Path != "/articles/welcome" || v.Title != "Welcome" || v.Description != "Hi." || v.TextContent != "Hello" ||
		v.PublishedAt != "2026-09-28T00:00:00.000Z" || v.UpdatedAt != "2026-10-20T20:18:27.757Z" ||
		len(v.Tags) != 1 || v.Tags[0] != "writing" || !strings.HasSuffix(v.Site, "/3mxcsltpg2k26") {
		t.Errorf("decoded %+v", v)
	}
	if v.CoverImage == nil || v.CoverImage.Ref.Link != "bafkreicover" || v.CoverImage.MIMEType != "image/png" {
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

func TestMissingLookupClassification(t *testing.T) {
	for _, tc := range []struct {
		name    string
		blob    bool
		status  int
		body    string
		missing bool
	}{
		{"record 404", false, 404, "not found", true},
		{"blob 404", true, 404, "", true},
		{"named blob absence", true, 400, `{"error":"BlobNotFound"}`, true},
		{"invalid blob request", true, 400, `{"error":"InvalidRequest","message":"invalid CID"}`, false},
		{"server failure", false, 500, `{"error":"RecordNotFound"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, fake := newClient(t)
			var err error
			if tc.blob {
				fake.Respond("com.atproto.sync.getBlob", "", tc.status, tc.body)
				_, _, err = c.GetBlob(t.Context(), did, "blob")
			} else {
				fake.Respond("com.atproto.repo.getRecord", "", tc.status, tc.body)
				_, err = c.GetPost(t.Context(), did, "post")
			}
			if err == nil || errors.Is(err, pds.ErrNotFound) != tc.missing {
				t.Errorf("error = %v, want missing=%t", err, tc.missing)
			}
		})
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

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReadDeadline(t *testing.T) {
	// Observe the deadline at the HTTP boundary without sleeping for five seconds.
	for _, timeout := range []time.Duration{0, time.Second, time.Minute} {
		t.Run(timeout.String(), func(t *testing.T) {
			want := 5 * time.Second
			if timeout > 0 && timeout < want {
				want = timeout
			}
			start := time.Now()
			hc := &http.Client{Timeout: timeout, Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				if remaining := time.Until(deadline); !ok || remaining > want || deadline.Before(start.Add(want)) {
					t.Errorf("request has deadline=%t, remaining=%v, want %v", ok, remaining, want)
				}
				return nil, context.DeadlineExceeded
			})}
			c := pds.New("http://pds.test", hc)
			if _, err := c.GetPost(context.Background(), did, "key"); !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("error=%v", err)
			}
			if hc.Timeout != timeout {
				t.Error("caller-owned HTTP client was modified")
			}
		})
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
