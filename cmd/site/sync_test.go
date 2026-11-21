package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/wescran/cranlab/internal/config"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
)

func await(t *testing.T, label string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", label)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSubscribeReposUpdatesRunningSite(t *testing.T) {
	const did = "did:plc:local"
	fake := pdstest.New(t, did)
	cfg := config.Config{DID: did, PublicationURI: "at://" + did + "/site.standard.publication/site", PDSURL: fake.URL, CacheDBPath: filepath.Join(t.TempDir(), "cache.db")}
	// The Article's rkey sorts onto the second page on every scan.
	for i := 0; i < 100; i++ {
		fake.Put(pds.DocumentCollection, fmt.Sprintf("z%03d", i), pds.Document{})
	}
	h, stop, err := siteHandler(t.Context(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	nextConnection := func() *websocket.Conn {
		t.Helper()
		select {
		case c := <-fake.Subscriptions():
			return c
		case <-time.After(5 * time.Second):
			t.Fatal("no subscription")
			return nil
		}
	}
	c := nextConnection()
	ring := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := c.Write(ctx, websocket.MessageBinary, []byte("not CBOR")); err != nil {
			t.Fatal(err)
		}
	}
	page := func(path string, status int, match, absent string) bool {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		body := r.Body.String()
		return r.Code == status && strings.Contains(body, match) && (absent == "" || !strings.Contains(body, absent))
	}
	put := func(title, tag string) {
		fake.Put(pds.DocumentCollection, "a", pds.Document{Site: cfg.PublicationURI, Path: "/articles/example", Title: title, Tags: []string{tag}})
	}
	fake.Put(pds.PostCollection, "fresh", map[string]string{"text": "Fresh Note"})
	put("First edition", "draft")
	ring()
	await(t, "publish", func() bool { return page("/articles/example", 200, "First edition", "") })
	if !page("/articles", 200, "First edition", "") || !page("/tags", 200, "draft <span>1</span>", "") {
		t.Fatal("publish missed list or tag count")
	}

	await(t, "new note after repo signal", func() bool { return page("/notes/fresh", 200, "Fresh Note", "") })

	c.CloseNow()
	c = nextConnection()
	put("Second edition", "final")
	ring()
	await(t, "edit after reconnect", func() bool { return page("/articles/example", 200, "Second edition", "First edition") })
	if !page("/articles", 200, "Second edition", "First edition") || !page("/tags", 200, "final <span>1</span>", "/tags/draft") {
		t.Fatal("edit missed list or tag count")
	}

	fake.Delete(pds.DocumentCollection, "a")
	fake.Put(pds.PostCollection, "independent", map[string]string{"text": "Indexed despite Article outage"})
	const method = "com.atproto.repo.listRecords"
	fake.Respond(method, "z000", 503, `{"error":"Unavailable"}`)
	before := len(fake.Requests())
	ring()
	await(t, "failed second page", func() bool {
		for _, u := range fake.Requests()[before:] {
			if u.Query().Get("cursor") == "z000" {
				return true
			}
		}
		return false
	})
	if !page("/articles/example", 200, "Second edition", "") || !page("/articles", 200, "Second edition", "") || !page("/tags", 200, "final <span>1</span>", "") {
		t.Fatal("incomplete listing removed content")
	}
	await(t, "post refresh despite failed Article listing", func() bool { return page("/notes/independent", 200, "Indexed despite Article outage", "") })
	fake.Respond(method, "z000", 0, "")
	ring()
	await(t, "unpublish", func() bool { return page("/articles/example", 404, "", "") })
	if !page("/articles", 200, "", "Second edition") || !page("/tags", 200, "", "/tags/final") || !page("/tags/final", 404, "", "") {
		t.Fatal("unpublish left list or tag entries")
	}
}
