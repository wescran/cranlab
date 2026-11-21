// Package store maintains the disposable SQLite projection of the owner's repo.
package store

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
	_ "modernc.org/sqlite"
)

type Store struct {
	db               *sql.DB
	did, publication string
}

const schema = `
CREATE TABLE IF NOT EXISTS article_snapshot (id INTEGER PRIMARY KEY CHECK(id=1), did TEXT NOT NULL, publication TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS articles (slug TEXT PRIMARY KEY, position INTEGER NOT NULL UNIQUE, article BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS article_tags (tag TEXT NOT NULL, slug TEXT NOT NULL, PRIMARY KEY(tag,slug));`

// Open binds the cache to this DID and Publication. A file initialized for a
// different identity never supplies a last-good fallback.
func Open(path, did, publication string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Serialize short DB transactions. No connection is held during a PDS read.
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, did: did, publication: publication}, nil
}

func (s *Store) Close() error { return s.db.Close() }

type DocumentSource interface {
	ListDocuments(context.Context, string) (pds.DocumentListing, error)
}

// RefreshArticles replaces everything, including the initialized marker, in
// one commit. A partial listing never reaches the delete step.
func (s *Store) RefreshArticles(ctx context.Context, source DocumentSource) error {
	listing, err := source.ListDocuments(ctx, s.did)
	if err != nil {
		return err
	}
	if !listing.Exhausted {
		return errors.New("document listing not exhausted")
	}
	articles := content.Articles(listing.Records, s.publication)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM article_tags; DELETE FROM articles;`); err != nil {
		return err
	}
	for pos, a := range articles {
		data, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO articles(slug,position,article) VALUES(?,?,?)`, a.Slug, pos, data); err != nil {
			return fmt.Errorf("index article %q: %w", a.Slug, err)
		}
		for _, tag := range a.Tags {
			if _, err := tx.ExecContext(ctx, `INSERT INTO article_tags(tag,slug) VALUES(?,?)`, tag, a.Slug); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO article_snapshot(id,did,publication) VALUES(1,?,?)`, s.did, s.publication); err != nil {
		return err
	}
	return tx.Commit()
}

// Check the snapshot identity and read its rows under the same transaction.
func query[T any](ctx context.Context, s *Store, scan func(*sql.Rows) (T, error), stmt string, args ...any) ([]T, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id int
	err = tx.QueryRowContext(ctx, `SELECT id FROM article_snapshot WHERE did=? AND publication=?`, s.did, s.publication).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, content.ErrNoSnapshot
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanArticle(rows *sql.Rows) (content.Article, error) {
	var b []byte
	var a content.Article
	if err := rows.Scan(&b); err != nil {
		return a, err
	}
	err := json.Unmarshal(b, &a)
	return a, err
}

func (s *Store) Articles(ctx context.Context) ([]content.Article, error) {
	return query(ctx, s, scanArticle, `SELECT article FROM articles ORDER BY position`)
}

func (s *Store) FindArticle(ctx context.Context, slug string) (content.Article, bool, error) {
	as, err := query(ctx, s, scanArticle, `SELECT article FROM articles WHERE slug=?`, slug)
	if err != nil || len(as) == 0 {
		return content.Article{}, false, err
	}
	return as[0], true, nil
}

func (s *Store) Tags(ctx context.Context) ([]content.TagCount, error) {
	return query(ctx, s, func(rows *sql.Rows) (content.TagCount, error) {
		var t content.TagCount
		err := rows.Scan(&t.Tag, &t.Count)
		return t, err
	}, `SELECT tag,count(*) FROM article_tags GROUP BY tag ORDER BY tag`)
}

func (s *Store) WithTag(ctx context.Context, tag string) ([]content.Article, error) {
	return query(ctx, s, scanArticle, `SELECT article FROM article_tags JOIN articles USING(slug) WHERE tag=? ORDER BY position`, tag)
}
