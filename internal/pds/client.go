// Package pds reads records and blobs from an atproto PDS over its public
// XRPC endpoints.
package pds

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// ErrNotFound is returned (wrapped) when a record or blob doesn't exist.
var ErrNotFound = errors.New("pds: not found")

// Client reads from one PDS. It never logs in; every read is public.
//
// Requests are GETs to <baseURL>/xrpc/<method>?<query>; build the query with
// url.Values so values are escaped. Any non-2xx response is an error naming
// the method, the HTTP status and the XRPC error name from the JSON body
// ({"error": "RecordNotFound", "message": "…"}); RecordNotFound wraps
// ErrNotFound. A body that isn't JSON still gives an error with the status.
type Client struct {
	// Fill in.
}

// New returns a client for the PDS at baseURL (no trailing slash). hc may be
// nil, meaning http.DefaultClient. Callers bound each request with the
// context's deadline.
func New(baseURL string, hc *http.Client) *Client {
	panic("not implemented")
}

// ListDocuments returns every site.standard.document record in did's
// repository, following the cursor with limit=100 per request.
//
// A PDS returns a cursor even with its last records, so stop on a page with
// no records (or no cursor), not just on a missing cursor.
func (c *Client) ListDocuments(ctx context.Context, did string) ([]Record[Document], error) {
	panic("not implemented")
}

// ListPosts returns one page of did's app.bsky.feed.post records, newest
// first, starting after cursor ("" for the first page), and the cursor for
// the next page.
func (c *Client) ListPosts(ctx context.Context, did, cursor string, limit int) ([]Record[Post], string, error) {
	panic("not implemented")
}

// GetPost returns one app.bsky.feed.post record. A missing record is an error
// wrapping ErrNotFound.
func (c *Client) GetPost(ctx context.Context, did, rkey string) (Record[Post], error) {
	panic("not implemented")
}

// GetBlob streams a blob and returns its Content-Type as reported by the PDS.
// The caller closes the body. A missing blob is an error wrapping
// ErrNotFound (a PDS answers 400 InvalidRequest "Blob not found").
func (c *Client) GetBlob(ctx context.Context, did, cid string) (io.ReadCloser, string, error) {
	panic("not implemented")
}

// RKey returns the record key of an AT-URI: its last path segment
// ("at://did/collection/rkey" → "rkey").
func RKey(uri string) string {
	panic("not implemented")
}
