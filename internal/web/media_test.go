package web

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

func TestMediaStreamsBeforeUpstreamFinishes(t *testing.T) {
	// Larger than net/http's write buffer: the client can read the prefix
	// before it lets the PDS finish. The PDS client bounds the wait at 5s.
	prefix := bytes.Repeat([]byte("a"), 64<<10)
	finish := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("did") != testDID || r.URL.Query().Get("cid") != catCID {
			t.Errorf("unexpected blob lookup: %v", r.URL)
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Content-Length", "65537")
		_, _ = w.Write(prefix)
		select {
		case <-finish: // Drop the final byte instead of completing the blob.
		case <-r.Context().Done():
			t.Error("upstream canceled before client received the prefix")
		}
	}))
	defer upstream.Close()

	h, err := New(testConfig(), Deps{Blobs: pds.New(upstream.URL, upstream.Client())})
	if err != nil {
		t.Fatal(err)
	}
	site := httptest.NewServer(h)
	defer site.Close()
	res, err := site.Client().Get(site.URL + "/media/" + catCID + "?did=did:plc:someoneelse")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := make([]byte, 1024)
	if _, err := io.ReadFull(res.Body, buf); err != nil || !bytes.Equal(buf, prefix[:len(buf)]) {
		t.Fatalf("reading initial blob chunk: %v", err)
	}
	close(finish)
	if _, err := io.Copy(io.Discard, res.Body); err == nil {
		t.Error("truncated blob was served as a complete, immutable response")
	}
}

func TestMediaContentTypes(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct{ upstream, served string }{
		{"image/png", "image/png"}, {"image/jpeg", "image/jpeg"}, {"image/gif", "image/gif"},
		{"image/webp", "image/webp"}, {"image/avif", "image/avif"},
		{"Image/PNG; name=cat.png", "image/png"},
		{"image/svg+xml", "application/octet-stream"},
	} {
		f.pds.PutBlob(catCID, tc.upstream, []byte("png bytes"))
		rec := get(t, f.handler, "/media/"+catCID)
		if got := rec.Result().Header.Get("Content-Type"); got != tc.served {
			t.Errorf("upstream %q served as %q, want %q", tc.upstream, got, tc.served)
		}
	}
}

func TestMediaWithoutBlobReader(t *testing.T) {
	h, err := New(testConfig(), Deps{})
	if err != nil {
		t.Fatal(err)
	}
	rec := get(t, h, "/media/"+catCID)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured blob source = %d, want 503", rec.Code)
	}
}

func TestMediaUnavailable(t *testing.T) {
	f := newFixture(t)
	f.pds.FailWith(http.StatusInternalServerError)
	rec := get(t, f.handler, "/media/"+catCID)
	if cc := rec.Result().Header.Get("Cache-Control"); rec.Code != http.StatusServiceUnavailable || cc != "no-store" {
		t.Errorf("unavailable blob = %d, Cache-Control %q; want 503, no-store", rec.Code, cc)
	}
}
