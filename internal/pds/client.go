// Package pds accesses records and blobs on an atproto PDS over XRPC.
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
	"time"

	"github.com/coder/websocket"
)

var rkeyRe = regexp.MustCompile(`^at://[^/]+/[^/]+/([^/]+)$`)

// ErrNotFound is returned (wrapped) when a record or blob doesn't exist.
var ErrNotFound = errors.New("pds: not found")

// Client accesses one PDS. Public reads do not need a session.
type Client struct {
	baseURL string
	hc      *http.Client
}

// New returns a client for the PDS at baseURL. A nil hc means
// http.DefaultClient. Each request, including body reads, takes at most five
// seconds (less if hc or the calling context has a tighter bound).
func New(baseURL string, hc *http.Client) *Client {
	base := strings.TrimRight(baseURL, "/")
	if hc == nil {
		hc = http.DefaultClient
	}

	bounded := *hc
	const readTimeout = 5 * time.Second
	if bounded.Timeout <= 0 || bounded.Timeout > readTimeout {
		bounded.Timeout = readTimeout
	}
	return &Client{
		baseURL: base,
		hc:      &bounded,
	}
}

// SubscribeRepos calls wake once per message until interrupted. The stream
// is only a bell: discard payloads without buffering or decoding commits.
func (c *Client) SubscribeRepos(ctx context.Context, wake func()) error {
	conn, _, err := websocket.Dial(ctx, c.baseURL+"/xrpc/com.atproto.sync.subscribeRepos", &websocket.DialOptions{HTTPClient: c.hc})
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(-1) // firehose commits can exceed the default 32 KiB
	for {
		_, r, err := conn.Reader(ctx)
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return err
		}
		wake()
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
	lookup := method == "com.atproto.repo.getRecord" || method == "com.atproto.sync.getBlob"
	if lookup && resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %d: %w", method, resp.StatusCode, ErrNotFound)
	}
	if err := json.UnmarshalRead(resp.Body, &xrpcErr); err != nil {
		return nil, fmt.Errorf("%s: %d: %w", method, resp.StatusCode, err)
	}
	err = fmt.Errorf("%s: %d %s: %s", method, resp.StatusCode, xrpcErr.Error, xrpcErr.Message)
	if resp.StatusCode == http.StatusBadRequest {
		switch method {
		case "com.atproto.repo.getRecord":
			if xrpcErr.Error == "RecordNotFound" {
				return nil, fmt.Errorf("%w: %w", ErrNotFound, err)
			}
		case "com.atproto.sync.getBlob":
			if xrpcErr.Error == "BlobNotFound" || (xrpcErr.Error == "InvalidRequest" && xrpcErr.Message == "Blob not found") {
				return nil, fmt.Errorf("%w: %w", ErrNotFound, err)
			}
		}
	}
	return nil, err
}

// decodeRecord builds a Record, keeping the value's raw JSON.
func decodeRecord[T any](uri, cid string, value jsontext.Value) (Record[T], error) {
	if RKey(uri) == "" || cid == "" || value.Kind() != '{' {
		return Record[T]{}, errors.New("malformed record envelope")
	}
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
		Cursor  jsontext.Value `json:"cursor"`
		Records []struct {
			URI   string         `json:"uri"`
			CID   string         `json:"cid"`
			Value jsontext.Value `json:"value"`
		} `json:"records"`
	}
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		return nil, "", fmt.Errorf("list: %w", err)
	}

	if out.Records == nil {
		return nil, "", errors.New("list: expected records array")
	}
	var next string
	if out.Cursor != nil {
		if out.Cursor.Kind() != '"' {
			return nil, "", errors.New("list: expected string cursor")
		}
		if err := json.Unmarshal(out.Cursor, &next); err != nil {
			return nil, "", fmt.Errorf("list cursor: %w", err)
		}
	}
	records := make([]Record[T], 0, len(out.Records))
	for _, rec := range out.Records {
		if !strings.HasPrefix(rec.URI, "at://"+did+"/"+collection+"/") {
			return nil, "", errors.New("list: record outside requested collection")
		}
		record, err := decodeRecord[T](rec.URI, rec.CID, rec.Value)
		if err != nil {
			return nil, "", fmt.Errorf("list: %w", err)
		}
		records = append(records, record)
	}

	return records, next, nil
}

// DocumentListing can be used to replace a snapshot only when Exhausted.
type DocumentListing struct {
	Records   []Record[Document]
	Exhausted bool
}

// ListDocuments returns every site.standard.document record in did's
// repository.
//
// Follow every cursor, even when the next page may be empty.
func (c *Client) ListDocuments(ctx context.Context, did string) (DocumentListing, error) {
	allRecords := make([]Record[Document], 0)
	seenURIs := make(map[string]bool)
	seen := make(map[string]bool)
	cursor := ""
	for {
		records, nextCursor, err := listPage[Document](ctx, c, did, DocumentCollection, cursor, 100)
		if err != nil {
			return DocumentListing{}, fmt.Errorf("ListDocuments: %w", err)
		}
		for _, record := range records {
			if seenURIs[record.URI] {
				return DocumentListing{}, errors.New("ListDocuments: repeated record URI")
			}
			seenURIs[record.URI] = true
		}
		allRecords = append(allRecords, records...)
		if nextCursor == "" {
			break
		}
		if seen[nextCursor] {
			return DocumentListing{}, errors.New("ListDocuments: repeated cursor")
		}
		seen[nextCursor] = true
		cursor = nextCursor
	}

	return DocumentListing{Records: allRecords, Exhausted: true}, nil
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
	return getRecord[Post](ctx, c, did, PostCollection, rkey)
}

// GetDocument includes the raw document value for a lossless conditional update.
func (c *Client) GetDocument(ctx context.Context, did, rkey string) (Record[Document], error) {
	return getRecord[Document](ctx, c, did, DocumentCollection, rkey)
}

func getRecord[T any](ctx context.Context, c *Client, did, collection, rkey string) (Record[T], error) {
	query := url.Values{
		"repo":       {did},
		"collection": {collection},
		"rkey":       {rkey},
	}
	resp, err := c.get(ctx, "com.atproto.repo.getRecord", query)
	if err != nil {
		return Record[T]{}, fmt.Errorf("getRecord: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		URI   string         `json:"uri"`
		CID   string         `json:"cid"`
		Value jsontext.Value `json:"value"`
	}
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		return Record[T]{}, fmt.Errorf("getRecord: %w", err)
	}
	if out.URI != "at://"+did+"/"+collection+"/"+rkey {
		return Record[T]{}, errors.New("getRecord: unexpected record URI")
	}
	return decodeRecord[T](out.URI, out.CID, out.Value)
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
