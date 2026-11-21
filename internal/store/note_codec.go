package store

import (
	"bytes"
	"encoding/gob"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/pds"
)

// Legacy rows begin with a nonzero gob message length. A zero marker followed
// by domain JSON allows old and new rows to coexist during recent refreshes.
func encodeNote(n content.Note) ([]byte, error) {
	b, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("encode Note: %w", err)
	}
	return append([]byte{0}, b...), nil
}

func decodeNote(raw []byte) (content.Note, error) {
	if len(raw) > 0 && raw[0] == 0 {
		var n content.Note
		if err := json.Unmarshal(raw[1:], &n); err != nil {
			return n, fmt.Errorf("decode Note: %w", err)
		}
		return n, nil
	}
	// Read the last-good snapshot even if the first post-upgrade refresh fails.
	// A successful full refresh replaces all legacy rows atomically.
	var old struct {
		RKey, URI, Text string
		Published       *time.Time
		Facets          []pds.Facet
		Embed           *pds.Embed
	}
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&old); err != nil {
		return content.Note{}, fmt.Errorf("decode legacy Note: %w", err)
	}
	return content.Note{
		RKey: old.RKey, URI: old.URI, Text: old.Text, Published: old.Published,
		Facets: content.ProjectFacets(old.Facets),
		Embed:  content.ProjectEmbed(old.Embed),
	}, nil
}
