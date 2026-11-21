package store_test

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/store"
)

// This fixture is the pre-domain snapshot format, not a new-format round trip.
func TestWarmLegacyNote(t *testing.T) {
	const did, publication, site = "did:plc:test", "publication", "https://local.test"
	path := filepath.Join(t.TempDir(), "cache.db")
	published := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	old := struct {
		RKey, URI, Text string
		Published       *time.Time
		Facets          []pds.Facet
		Embed           *pds.Embed
	}{
		RKey: "old", URI: "at://did:plc:test/app.bsky.feed.post/old", Text: "🍁 hi", Published: &published,
		Facets: []pds.Facet{{
			Index:    pds.ByteSlice{ByteStart: 5, ByteEnd: 7},
			Features: []pds.Feature{{Type: "app.bsky.richtext.facet#mention", DID: "did:plc:other"}},
		}},
		Embed: &pds.Embed{
			Type:  pds.EmbedRecordWithMedia,
			Quote: &pds.StrongRef{URI: "at://did:plc:other/app.bsky.feed.post/original"},
			Media: &pds.Embed{Type: pds.EmbedVideo, Video: &pds.Video{Alt: "caption"}},
		},
	}
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(old); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE article_snapshot(id INTEGER PRIMARY KEY, did TEXT, publication TEXT);
 CREATE TABLE post_snapshot(id INTEGER PRIMARY KEY, did TEXT, publication TEXT, site TEXT);
 CREATE TABLE posts(rkey TEXT PRIMARY KEY, uri TEXT, published TEXT, candidate INTEGER, value BLOB);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO article_snapshot VALUES(1,?,?)`, did, publication); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO post_snapshot VALUES(1,?,?,?)`, did, publication, site); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO posts VALUES(?,?,'',1,?)`, old.RKey, old.URI, b.Bytes()); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path, did, publication, site)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	note, ok, err := s.FindNote(t.Context(), "old")
	if err != nil || !ok {
		t.Fatalf("FindNote: %v, %v", ok, err)
	}
	alt := "caption"
	want := content.Note{
		RKey: "old", URI: "at://did:plc:test/app.bsky.feed.post/old", Text: "🍁 hi", Published: &published,
		Facets: []content.Facet{{Start: 5, End: 7, URL: "https://bsky.app/profile/did:plc:other"}},
		Embed:  &content.NoteEmbed{QuoteURI: "at://did:plc:other/app.bsky.feed.post/original", VideoAlt: &alt},
	}
	if !reflect.DeepEqual(note, want) {
		t.Fatalf("lost warm Note content: %+v", note)
	}
}
