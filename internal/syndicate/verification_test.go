package syndicate_test

import (
	"bytes"
	"encoding/json/jsontext"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/syndicate"
)

func TestOnlyVerifiedUnclaimedAnnouncementPermitsLogin(t *testing.T) {
	envelope := func(uri string) string {
		return fmt.Sprintf(`{"uri":%q,"cid":"fixture-cid","value":%s}`, uri, postJSON)
	}
	for _, tc := range []struct {
		name, post, freshDoc string
		status               int
		response             string
		claimed, review      bool
	}{
		{name: "absent"},
		{name: "upstream failure", status: 503, response: `{"error":"InternalError","message":"fixture-password fixture-access-token fixture-refresh-token"}`, review: true},
		{name: "foreign owner", status: 200, response: envelope(strings.Replace(postURI, did, "did:plc:elsewhere", 1)), review: true},
		{name: "other collection", status: 200, response: envelope(docURI), review: true},
		{name: "other key", status: 200, response: envelope(postURI + "2"), review: true},
		{name: "key alone", post: `{"$type":"app.bsky.feed.post","text":"a Note"}`, review: true},
		{name: "different canonical Article", post: strings.ReplaceAll(postJSON, "/articles/old", "/articles/other"), review: true},
		{name: "different origin", post: strings.ReplaceAll(postJSON, origin, origin+".example"), review: true},
		{name: "reply", post: strings.Replace(postJSON, `"embed":`, `"reply":{},"embed":`, 1), review: true},
		{name: "other record type", post: strings.Replace(postJSON, pds.PostCollection, "some.other.type", 1), review: true},
		{name: "conflicting link facet", post: strings.Replace(postJSON, `"embed":`, `"facets":[{"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://different.invalid/"}]}],"embed":`, 1), review: true},
		{name: "fresh Article has a different link", post: postJSON, freshDoc: strings.Replace(docJSON, `"extra":`, `"bskyPostRef":{"uri":"at://other/app.bsky.feed.post/other","cid":"other"},"extra":`, 1), review: true},
		{name: "already referenced by another Article", post: postJSON, claimed: true, review: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := pdstest.New(t, did)
			c := pds.New(f.URL, nil)
			doc := docJSON
			if tc.freshDoc != "" {
				doc = tc.freshDoc
			}
			f.Put(pds.DocumentCollection, "key", jsontext.Value(doc))
			if tc.post != "" {
				f.Put(pds.PostCollection, "key", jsontext.Value(tc.post))
			}
			if tc.status != 0 {
				f.Respond("com.atproto.repo.getRecord", "", tc.status, tc.response)
			}
			source := articles{{URI: docURI, Slug: "old"}}
			if tc.claimed {
				source = append(source, articles{{URI: docURI + "2", Slug: "other", AnnouncementURI: postURI}}...)
			}
			var logs bytes.Buffer
			syndicate.New(source, c, did, publication, origin, pdstest.Password, slog.New(slog.NewTextHandler(&logs, nil)), time.Now).Run(t.Context())
			for _, u := range f.Requests() {
				if u.Path != "/xrpc/com.atproto.repo.getRecord" {
					t.Fatalf("unexpected request: %s", u.Path)
				}
			}
			if (logs.Len() > 0) != tc.review {
				t.Fatalf("expected review=%v, logs=%q", tc.review, logs.String())
			}
			for _, secret := range []string{pdstest.Password, pdstest.AccessJWT, "fixture-refresh-token"} {
				if strings.Contains(logs.String(), secret) {
					t.Fatal("secret in review log")
				}
			}
		})
	}
}
