package syndicate_test

import (
	"bytes"
	"encoding/json/jsontext"
	"log/slog"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/syndicate"
)

func TestAuthenticatedErrorsAreLoggedWithoutSecrets(t *testing.T) {
	for _, method := range []string{"com.atproto.server.createSession", "com.atproto.repo.putRecord"} {
		t.Run(method, func(t *testing.T) {
			f := pdstest.New(t, did)
			source := articles{}
			for _, key := range []string{"a", "b"} {
				source = append(source, content.Article{URI: "at://" + did + "/" + pds.DocumentCollection + "/" + key, Slug: key})
				f.Put(pds.DocumentCollection, key, jsontext.Value(strings.ReplaceAll(docJSON, "/articles/old", "/articles/"+key)))
				f.Put(pds.PostCollection, key, jsontext.Value(strings.ReplaceAll(postJSON, "/articles/old", "/articles/"+key)))
			}
			f.Respond(method, "", 400, `{"error":"InvalidRequest","message":"fixture-password fixture-access-token fixture-refresh-token"}`)
			var logs bytes.Buffer
			syndicate.New(source, pds.New(f.URL, nil), did, publication, origin, pdstest.Password, slog.New(slog.NewTextHandler(&logs, nil))).Run(t.Context())
			logins := 0
			for _, req := range f.Requests() {
				if req.Path == "/xrpc/com.atproto.server.createSession" {
					logins++
				}
			}
			if logins != 1 {
				t.Errorf("got %d logins in a single run", logins)
			}
			if logs.Len() == 0 {
				t.Fatal("missing review log")
			}
			for _, secret := range []string{pdstest.Password, pdstest.AccessJWT, "fixture-refresh-token"} {
				if strings.Contains(logs.String(), secret) {
					t.Fatal("credential in logs")
				}
			}
		})
	}
}
