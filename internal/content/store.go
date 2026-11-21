package content

import (
	"context"
	"errors"
	"regexp"
	"time"
)

// Note contains the fields needed for a public permalink, not the XRPC envelope.
type Note struct {
	RKey, URI, Text string
	Published       *time.Time
	Facets          []Facet
	Embed           *NoteEmbed
}

var recordKey = regexp.MustCompile(`^[A-Za-z0-9._:~-]{1,512}$`)

func ValidRKey(s string) bool { return s != "." && s != ".." && recordKey.MatchString(s) }

// ErrNoSnapshot is distinct from an initialized snapshot containing no Articles.
var ErrNoSnapshot = errors.New("content snapshot unavailable")

// ContentStore reads a single, complete snapshot per query. A missing Article
// is reported as (zero, false, nil); missing Tags have an empty result.
type ContentStore interface {
	Notes(context.Context, string, int) ([]Note, string, error)
	FindNote(context.Context, string) (Note, bool, error)
	Articles(context.Context) ([]Article, error)
	FindArticle(context.Context, string) (Article, bool, error)
	Tags(context.Context) ([]TagCount, error)
	WithTag(context.Context, string) ([]Article, error)
}
