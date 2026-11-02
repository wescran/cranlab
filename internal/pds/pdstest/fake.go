// Package pdstest provides an in-memory fake PDS for tests. It serves the
// read endpoints the site uses, with the same responses and error shapes as
// a real PDS.
package pdstest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Server is a fake PDS hosting one repository.
type Server struct {
	*httptest.Server
	DID string

	mu       sync.Mutex
	records  map[string]map[string]record // collection → rkey → record
	blobs    map[string]blob              // cid → blob
	failWith int
	delay    time.Duration
	requests []url.URL
}

type record struct {
	cid   string
	value json.RawMessage
}

type blob struct {
	mimeType string
	data     []byte
}

// New starts a fake PDS for did, stopped when the test ends.
func New(t testing.TB, did string) *Server {
	s := &Server{
		DID:     did,
		records: map[string]map[string]record{},
		blobs:   map[string]blob{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /xrpc/com.atproto.repo.listRecords", s.listRecords)
	mux.HandleFunc("GET /xrpc/com.atproto.repo.getRecord", s.getRecord)
	mux.HandleFunc("GET /xrpc/com.atproto.sync.getBlob", s.getBlob)
	s.Server = httptest.NewServer(s.wrap(mux))
	t.Cleanup(s.Close)
	return s
}

// Put stores a record (value is marshalled to JSON, or used as-is if it is
// a json.RawMessage or []byte) and returns its CID.
func (s *Server) Put(collection, rkey string, value any) string {
	var raw []byte
	switch v := value.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	default:
		var err error
		if raw, err = json.Marshal(v); err != nil {
			panic(err)
		}
	}
	sum := sha256.Sum256(append([]byte(collection+"/"+rkey+"\n"), raw...))
	cid := "bafyrei" + hex.EncodeToString(sum[:])[:40]
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[collection] == nil {
		s.records[collection] = map[string]record{}
	}
	s.records[collection][rkey] = record{cid: cid, value: raw}
	return cid
}

// PutBlob stores a blob under cid.
func (s *Server) PutBlob(cid, mimeType string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blobs[cid] = blob{mimeType: mimeType, data: data}
}

// FailWith makes every request fail with this HTTP status (0 restores normal
// behaviour).
func (s *Server) FailWith(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failWith = status
}

// Delay makes every request wait this long before answering (or until the
// client gives up).
func (s *Server) Delay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delay = d
}

// Requests returns the URLs requested so far, in order.
func (s *Server) Requests() []url.URL {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.URL(nil), s.requests...)
}

func (s *Server) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, *r.URL)
		failWith, delay := s.failWith, s.delay
		s.mu.Unlock()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		if failWith != 0 {
			xrpcError(w, failWith, "InternalServerError", "Internal Server Error")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// listRecords returns records newest first (descending record key), like a
// real PDS. Like a real PDS, it returns a cursor whenever it returned any
// records, even on the last page; the end is an empty page.
func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("repo") != s.DID {
		xrpcError(w, http.StatusBadRequest, "RepoNotFound", "Could not find repo: "+q.Get("repo"))
		return
	}
	limit := 50
	if l := q.Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > 100 {
			xrpcError(w, http.StatusBadRequest, "InvalidRequest", "limit must be 1-100")
			return
		}
		limit = n
	}
	cursor := q.Get("cursor")
	collection := q.Get("collection")

	s.mu.Lock()
	recs := s.records[collection]
	rkeys := make([]string, 0, len(recs))
	for rkey := range recs {
		if cursor == "" || rkey < cursor {
			rkeys = append(rkeys, rkey)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(rkeys)))
	if len(rkeys) > limit {
		rkeys = rkeys[:limit]
	}
	type item struct {
		URI   string          `json:"uri"`
		CID   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	}
	out := struct {
		Cursor  string `json:"cursor,omitempty"`
		Records []item `json:"records"`
	}{Records: []item{}}
	for _, rkey := range rkeys {
		rec := recs[rkey]
		out.Records = append(out.Records, item{URI: s.uri(collection, rkey), CID: rec.cid, Value: rec.value})
	}
	s.mu.Unlock()
	if len(rkeys) > 0 {
		out.Cursor = rkeys[len(rkeys)-1]
	}
	writeJSON(w, out)
}

func (s *Server) getRecord(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	collection, rkey := q.Get("collection"), q.Get("rkey")
	s.mu.Lock()
	rec, ok := s.records[collection][rkey]
	s.mu.Unlock()
	if q.Get("repo") != s.DID || !ok {
		xrpcError(w, http.StatusBadRequest, "RecordNotFound", "Could not locate record: "+s.uri(collection, rkey))
		return
	}
	writeJSON(w, struct {
		URI   string          `json:"uri"`
		CID   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	}{s.uri(collection, rkey), rec.cid, rec.value})
}

func (s *Server) getBlob(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	b, ok := s.blobs[q.Get("cid")]
	s.mu.Unlock()
	if q.Get("did") != s.DID || !ok {
		// What a real PDS answers for a missing blob.
		xrpcError(w, http.StatusBadRequest, "InvalidRequest", "Blob not found")
		return
	}
	w.Header().Set("Content-Type", b.mimeType)
	_, _ = w.Write(b.data)
}

func (s *Server) uri(collection, rkey string) string {
	return "at://" + s.DID + "/" + collection + "/" + rkey
}

func xrpcError(w http.ResponseWriter, status int, name, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": name, "message": message})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
