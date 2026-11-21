package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
)

// static serves files under /static/ with URLs that change whenever a file's
// content does, so they can be cached forever.
type static struct {
	fsys   fs.FS
	hashes map[string]string
}

// newStatic hashes every file in fsys.
func newStatic(fsys fs.FS) (*static, error) {
	s := &static{
		fsys:   fsys,
		hashes: make(map[string]string),
	}
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fileBytes, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(fileBytes)
		encoded := hex.EncodeToString(hash[:])
		s.hashes[path] = encoded[:12]
		return nil
	}); err != nil {
		return nil, err
	}
	return s, nil
}

// url returns the file's versioned URL, "/static/<name>?v=<hash>". An unknown
// name is an error, so a typo in a template fails rendering.
func (s *static) url(name string) (string, error) {
	if hash, ok := s.hashes[name]; ok {
		return fmt.Sprintf("/static/%s?v=%s", name, hash), nil
	}
	return "", fmt.Errorf("static: unknown file %q", name)
}

// ServeHTTP caches an asset as immutable only under its current content version.
func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rPath := r.PathValue("path")
	if hash, ok := s.hashes[rPath]; ok {
		fileBytes, err := fs.ReadFile(s.fsys, rPath)
		if err != nil {
			http.Error(w, "Asset unavailable", http.StatusServiceUnavailable)
			return
		}
		var contentType string
		extension := path.Ext(rPath)
		// Pinned rather than left to mime.TypeByExtension, which varies by OS.
		switch extension {
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".woff2":
			contentType = "font/woff2"
		case ".svg":
			contentType = "image/svg+xml"
		default:
			contentType = mime.TypeByExtension(path.Ext(rPath))
		}
		w.Header().Set("Content-Type", contentType)
		if r.URL.Query().Get("v") == hash {
			w.Header().Set("Cache-Control", immutableCache)
		}
		_, _ = w.Write(fileBytes)
		return
	}
	http.NotFound(w, r)
}
