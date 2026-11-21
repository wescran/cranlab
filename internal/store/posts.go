package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
)

type PostSource interface {
	ListPosts(context.Context, string, string, int) ([]pds.Record[pds.Post], string, error)
}

const postMarker = `SELECT id FROM post_snapshot WHERE did=? AND publication=? AND site=?`

// RefreshPosts upserts just the newest page on a signal. Only an exhausted full
// scan may replace the collection or initialize the snapshot after an outage.
func (s *Store) RefreshPosts(ctx context.Context, source PostSource, full bool) error {
	if !full {
		var id int
		err := s.db.QueryRowContext(ctx, postMarker, s.did, s.publication, s.siteURL).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			full = true
		} else if err != nil {
			return err
		}
	}
	var posts []pds.Record[pds.Post]
	seen := map[string]bool{}
	for cursor := ""; ; {
		page, next, err := source.ListPosts(ctx, s.did, cursor, 100)
		if err != nil {
			return err
		}
		if page == nil {
			return errors.New("missing posts page")
		}
		posts = append(posts, page...)
		if !full || next == "" {
			break
		}
		if seen[next] {
			return errors.New("repeated posts cursor")
		}
		seen[next] = true
		cursor = next
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if full {
		if _, err := tx.ExecContext(ctx, "DELETE FROM posts"); err != nil {
			return err
		}
	}
	insert := `INSERT INTO posts(rkey,uri,published,candidate,value) VALUES(?,?,?,?,?)`
	if !full {
		insert += ` ON CONFLICT(rkey) DO UPDATE SET uri=excluded.uri,published=excluded.published,candidate=excluded.candidate,value=excluded.value`
	}
	stmt, err := tx.PrepareContext(ctx, insert)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range posts {
		key := pds.RKey(p.URI)
		if !content.ValidRKey(key) {
			return fmt.Errorf("invalid post key %q", key)
		}
		n := content.Note{RKey: key, URI: p.URI, Text: p.Value.Text, Facets: p.Value.Facets, Embed: p.Value.Embed}
		if date, err := time.Parse(time.RFC3339, p.Value.CreatedAt); err == nil {
			utc := date.UTC()
			if utc.Year() >= 0 && utc.Year() <= 9999 {
				n.Published = &utc
			}
		}
		// Store the domain projection; reads never replay XRPC decoding.
		var data bytes.Buffer
		if err := gob.NewEncoder(&data).Encode(n); err != nil {
			return err
		}
		// Article references are applied at read time from the same transaction as
		// the Article snapshot. Only reply/embed eligibility is intrinsic to a post.
		if _, err := stmt.ExecContext(ctx, key, p.URI, dateKey(n), content.IsNote(p, nil, s.siteURL), data.Bytes()); err != nil {
			return err
		}
	}
	if full {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO post_snapshot VALUES(1,?,?,?)`, s.did, s.publication, s.siteURL); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const sortTime = "2006-01-02T15:04:05.000000000Z"
const selectNotes = `SELECT value FROM posts WHERE candidate=1 AND NOT EXISTS(SELECT 1 FROM announcement_refs WHERE uri=posts.uri)`

func (s *Store) readNotes(ctx context.Context, suffix string, args ...any) ([]content.Note, error) {
	return querySnapshot(ctx, s, articleSnapshot+" INTERSECT "+postMarker, []any{s.did, s.publication, s.did, s.publication, s.siteURL}, scanNote, selectNotes+suffix, args...)
}

func scanNote(rows *sql.Rows) (content.Note, error) {
	var n content.Note
	var raw []byte
	if err := rows.Scan(&raw); err != nil {
		return n, err
	}
	err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&n)
	return n, err
}

func (s *Store) FindNote(ctx context.Context, key string) (content.Note, bool, error) {
	ns, err := s.readNotes(ctx, " AND rkey=?", key)
	if len(ns) == 0 || err != nil {
		return content.Note{}, false, err
	}
	return ns[0], true, nil
}

func dateKey(n content.Note) string {
	if n.Published == nil {
		return ""
	}
	return n.Published.UTC().Format(sortTime)
}

func decodePosition(cursor string) (date, key string, ok bool) {
	// A timestamp, separator and a 512-byte record key fit in 724 base64 bytes.
	if len(cursor) == 0 || len(cursor) > 724 {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(cursor)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != cursor {
		return "", "", false
	}
	date, key, ok = strings.Cut(string(raw), "|")
	if !ok || !content.ValidRKey(key) {
		return "", "", false
	}
	if date != "" {
		t, err := time.Parse(sortTime, date)
		if err != nil || t.Format(sortTime) != date {
			return "", "", false
		}
	}
	return date, key, true
}

// A keyset cursor names a position, not a row or offset. Newer inserts and
// deletion of the boundary row do not move the next page's start.
func (s *Store) Notes(ctx context.Context, cursor string, limit int) ([]content.Note, string, error) {
	if limit < 1 || limit > 100 {
		return nil, "", errors.New("invalid notes limit")
	}
	suffix := ""
	var args []any
	if date, key, ok := decodePosition(cursor); ok {
		suffix = " AND (published,rkey)<(?,?)"
		args = []any{date, key}
	}
	notes, err := s.readNotes(ctx, suffix+" ORDER BY published DESC,rkey DESC LIMIT ?", append(args, limit+1)...)
	if err != nil {
		return nil, "", err
	}
	if len(notes) <= limit {
		return notes, "", nil
	}
	last := notes[limit-1]
	next := base64.RawURLEncoding.EncodeToString([]byte(dateKey(last) + "|" + last.RKey))
	return notes[:limit], next, nil
}
