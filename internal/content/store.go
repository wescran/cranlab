package content

import (
	"context"
	"errors"
)

// ErrNoSnapshot is distinct from an initialized snapshot containing no Articles.
var ErrNoSnapshot = errors.New("article snapshot unavailable")

// ContentStore reads a single, complete snapshot per query. A missing Article
// is reported as (zero, false, nil); missing Tags have an empty result.
type ContentStore interface {
	Articles(context.Context) ([]Article, error)
	FindArticle(context.Context, string) (Article, bool, error)
	Tags(context.Context) ([]TagCount, error)
	WithTag(context.Context, string) ([]Article, error)
}
