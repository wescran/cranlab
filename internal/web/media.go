package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"

	"github.com/wescran/cranlab/internal/pds"
)

// BlobReader opens a PDS blob; the caller closes the returned body.
type BlobReader interface {
	GetBlob(ctx context.Context, did, cid string) (io.ReadCloser, string, error)
}

// AT Protocol blob CIDs are lowercase, unpadded base32 over the four bytes
// 01 55 12 20 (CIDv1, raw, sha2-256, 32 bytes) and the 256-bit digest.
// The seventh base32 digit straddles the prefix; the last has two zero bits.
var cidPattern = regexp.MustCompile(`^bafkrei[a-h][a-z2-7]{50}[aeimquy4]$`)

// imageTypes are the content types /media passes through; anything else is
// served as application/octet-stream.
var imageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"image/avif": true,
}

// media streams one of the owner's blobs without letting it run as a page.
func (s *server) media(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	if !cidPattern.MatchString(cid) {
		http.NotFound(w, r)
		return
	}
	if s.deps.Blobs == nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	body, contentType, err := s.deps.Blobs.GetBlob(r.Context(), s.cfg.DID, cid)
	if errors.Is(err, pds.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("get media", "cid", cid, "err", err)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	defer body.Close()
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !imageTypes[mimeType] {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	n, err := io.Copy(w, body)
	if err != nil {
		slog.Error("stream media", "cid", cid, "err", err)
		if n == 0 {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		// Status is committed. Never let a cache treat a partial blob as complete.
		panic(http.ErrAbortHandler)
	}
}
