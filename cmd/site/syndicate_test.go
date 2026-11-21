package main

import (
	"encoding/json/jsontext"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
			repairing, release, repaired := make(chan struct{}, 1), make(chan struct{}), make(chan struct{}, 1)
			if enabled {
				cfg.AppPassword = pdstest.Password
				slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/xrpc/com.atproto.repo.putRecord" {
						repairing <- struct{}{}
						select {
						case <-release:
							defer func() { repaired <- struct{}{} }()
						case <-r.Context().Done():
							return
						}
					}
					fake.Config.Handler.ServeHTTP(w, r)
				}))
				t.Cleanup(slow.Close)
				cfg.PDSURL = slow.URL
			}
			fake.Put(pds.DocumentCollection, "post", pds.Document{Type: pds.DocumentCollection, Site: cfg.PublicationURI, Title: "Archived", Path: "/articles/archive"})
			fake.Put(pds.PostCollection, "post", jsontext.Value(`{"$type":"app.bsky.feed.post","embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.invalid/articles/archive"}}}`))
			h, stop, err := siteHandler(t.Context(), cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			if enabled {
				select {
				case <-repairing:
				case <-time.After(5 * time.Second):
					t.Fatal("repair did not start")
				}
			}
			await(t, "Article available", func() bool {
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, httptest.NewRequest("GET", "/articles/archive", nil))
				return rr.Code == 200
			})
			close(release)
			if enabled {
				select {
				case <-repaired:
				case <-time.After(5 * time.Second):
					t.Fatal("repair did not complete")
				}
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
