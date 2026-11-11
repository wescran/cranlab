package web

import (
	"net/http"
	"regexp"
)

// cidPattern accepts base32 CIDv1 strings, which is what blob CIDs are.
var cidPattern = regexp.MustCompile(`^b[a-z2-7]{8,128}$`)

// imageTypes are the content types /media passes through; anything else is
// served as application/octet-stream.
var imageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"image/avif": true,
}

// media serves one of the owner's blobs from the PDS, with the PDS's
// protective headers, so an uploaded file can never run as a page on the
// site.
//
//   - {cid} must match cidPattern, otherwise a plain 404 (http.NotFound)
//     without asking the PDS.
//   - A missing blob (pds.ErrNotFound) is a plain 404; another error is a
//     plain 503 (http.Error), logged.
//   - Headers: Content-Type from the PDS if it's in imageTypes, otherwise
//     application/octet-stream; Content-Security-Policy "default-src 'none';
//     sandbox"; X-Content-Type-Options "nosniff"; Cache-Control "public,
//     max-age=31536000, immutable". Then stream the body (io.Copy) and close
//     it.
func (s *server) media(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}
