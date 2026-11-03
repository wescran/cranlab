// Package pds reads records and blobs from an atproto PDS over its public
// XRPC endpoints.
package pds

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var rkeyRe = regexp.MustCompile(`^at://[^/]+/[^/]+/([^/]+)$`)

// ErrNotFound is returned (wrapped) when a record or blob doesn't exist.
var ErrNotFound = errors.New("pds: not found")

// Client reads from one PDS through its public XRPC endpoints. It never logs
// in.
type Client struct {
	baseURL string
	hc      *http.Client
}

// New returns a client for the PDS at baseURL. A nil hc means
// http.DefaultClient; requests are bounded by their context.
func New(baseURL string, hc *http.Client) *Client {
	base := strings.TrimRight(baseURL, "/")
	if hc == nil {
		hc = http.DefaultClient
	}

	return &Client{
		baseURL: base,
		hc:      hc,
	}
}

// get sends an XRPC query and returns the response if it succeeded. Error
// responses become errors with the status and the XRPC error name; a missing
// record or blob wraps ErrNotFound.
func (c *Client) get(ctx context.Context, method string, q url.Values) (*http.Response, error) {
	reqURL := c.baseURL + "/xrpc/" + method + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	defer resp.Body.Close()
	var xrpcErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.UnmarshalRead(resp.Body, &xrpcErr)
	err = fmt.Errorf("%s: %d %s: %s", method, resp.StatusCode, xrpcErr.Error, xrpcErr.Message)
	// A PDS answers a missing blob with 400 InvalidRequest.
	if xrpcErr.Error == "RecordNotFound" || (xrpcErr.Error == "InvalidRequest" && method == "com.atproto.sync.getBlob") {
		err = fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return nil, err
}

// decodeRecord builds a Record, keeping the value's raw JSON.
func decodeRecord[T any](uri, cid string, value jsontext.Value) (Record[T], error) {
	record := Record[T]{
		URI: uri,
		CID: cid,
		Raw: value,
	}
	if err := json.Unmarshal(record.Raw, &record.Value); err != nil {
		return Record[T]{}, fmt.Errorf("decode %s: %w", uri, err)
	}

	return record, nil
}

// listPage fetches one page of a collection with listRecords.
func listPage[T any](ctx context.Context, c *Client, did, collection, cursor string, limit int) ([]Record[T], string, error) {
	query := url.Values{
		"repo":       {did},
		"collection": {collection},
		"limit":      {strconv.Itoa(limit)},
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	resp, err := c.get(ctx, "com.atproto.repo.listRecords", query)
	if err != nil {
		return nil, "", fmt.Errorf("list: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Cursor  string `json:"cursor"`
		Records []struct {
			URI   string         `json:"uri"`
			CID   string         `json:"cid"`
			Value jsontext.Value `json:"value"`
		} `json:"records"`
	}
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		return nil, "", fmt.Errorf("list: %w", err)
	}

	records := make([]Record[T], 0, len(out.Records))
	for _, rec := range out.Records {
		record, err := decodeRecord[T](rec.URI, rec.CID, rec.Value)
		if err != nil {
			return nil, "", fmt.Errorf("list: %w", err)
		}
		records = append(records, record)
	}

	return records, out.Cursor, nil
}

// ListDocuments returns every site.standard.document record in did's
// repository.
//
// A PDS may return a cursor with its last page, so the listing ends on an
// empty page or a missing cursor.
func (c *Client) ListDocuments(ctx context.Context, did string) ([]Record[Document], error) {
	allRecords := make([]Record[Document], 0)
	cursor := ""
	for {
		records, nextCursor, err := listPage[Document](ctx, c, did, DocumentCollection, cursor, 100)
		if err != nil {
			return nil, fmt.Errorf("ListDocuments: %w", err)
		}
		allRecords = append(allRecords, records...)
		if len(records) == 0 || nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allRecords, nil
}

// ListPosts returns one page of did's app.bsky.feed.post records, newest
// first, starting after cursor ("" for the first page), and the cursor for
// the next page.
func (c *Client) ListPosts(ctx context.Context, did, cursor string, limit int) ([]Record[Post], string, error) {
	return listPage[Post](ctx, c, did, PostCollection, cursor, limit)
}

// GetPost returns one app.bsky.feed.post record. A missing record is an error
// wrapping ErrNotFound.
func (c *Client) GetPost(ctx context.Context, did, rkey string) (Record[Post], error) {
	query := url.Values{
		"repo":       {did},
		"collection": {PostCollection},
		"rkey":       {rkey},
	}
	resp, err := c.get(ctx, "com.atproto.repo.getRecord", query)
	if err != nil {
		return Record[Post]{}, fmt.Errorf("GetPost: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		URI   string         `json:"uri"`
		CID   string         `json:"cid"`
		Value jsontext.Value `json:"value"`
	}
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		return Record[Post]{}, fmt.Errorf("GetPost: %w", err)
	}

	record, err := decodeRecord[Post](out.URI, out.CID, out.Value)
	if err != nil {
		return Record[Post]{}, fmt.Errorf("GetPost: %w", err)
	}

	return record, nil
}

// GetBlob streams a blob and returns its Content-Type as reported by the PDS.
// The caller closes the body. A missing blob is an error wrapping ErrNotFound.
func (c *Client) GetBlob(ctx context.Context, did, cid string) (io.ReadCloser, string, error) {
	query := url.Values{
		"did": {did},
		"cid": {cid},
	}
	resp, err := c.get(ctx, "com.atproto.sync.getBlob", query)
	if err != nil {
		return nil, "", fmt.Errorf("GetBlob: %w", err)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// RKey returns the record key of an AT-URI, or "" if uri isn't one.
func RKey(uri string) string {
	matches := rkeyRe.FindStringSubmatch(uri)
	if matches == nil {
		return ""
	}
	return matches[1]
}
