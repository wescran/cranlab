package main

import (
	"encoding/json/jsontext"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/wescran/cranlab/internal/config"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
)

func TestStartupRepairsOnlyWithPassword(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "read only", true: "repair"}[enabled], func(t *testing.T) {
			const did = "did:plc:owner"
			fake := pdstest.New(t, did)
			cfg := config.Config{DID: did, PublicationURI: "at://" + did + "/site.standard.publication/pub", SiteURL: "https://example.invalid", PDSURL: fake.URL, CacheDBPath: filepath.Join(t.TempDir(), "cache.db")}
			if enabled {
				cfg.AppPassword = pdstest.Password
			}
			fake.Put(pds.DocumentCollection, "post", pds.Document{Type: pds.DocumentCollection, Site: cfg.PublicationURI, Title: "Archived", Path: "/articles/archive"})
			fake.Put(pds.PostCollection, "post", jsontext.Value(`{"$type":"app.bsky.feed.post","embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.invalid/articles/archive"}}}`))
			h, stop, err := siteHandler(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest("GET", "/articles/archive", nil))
			if rr.Code != 200 {
				t.Fatalf("read path returned %d", rr.Code)
			}
			logins, puts := 0, 0
			for _, req := range fake.Requests() {
				switch req.Path {
				case "/xrpc/com.atproto.server.createSession":
					logins++
				case "/xrpc/com.atproto.repo.putRecord":
					puts++
				}
			}
			want := 0
			if enabled {
				want = 1
			}
			if logins != want || puts != want {
				t.Fatalf("logins=%d puts=%d, want %d each", logins, puts, want)
			}
		})
	}
}
