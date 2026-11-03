package render

import (
	"github.com/wescran/cranlab/internal/pds"
)

// Embed is what a post's embed looks like on a page. Templates escape every
// field.
type Embed struct {
	Images   []ImageView
	External *ExternalView
	// QuoteURL links to a quoted post on bsky.app.
	QuoteURL string
	// VideoURL links to the post itself on bsky.app, where its video plays.
	VideoURL string
	VideoAlt string
}

// ImageView is one image. Width and Height are its aspect ratio (0 if
// unknown), for the <img> attributes.
type ImageView struct {
	URL, Alt      string
	Width, Height int
}

// ExternalView is a link card. ThumbURL is empty without a thumbnail.
type ExternalView struct {
	URL, Title, Description, ThumbURL string
}

// EmbedView describes e for a page, or returns nil if there's nothing to
// show (a nil embed, an unknown type, or only unusable parts).
//
//   - Images: /media/<cid> URLs (path-escaped) with their alt text and aspect
//     ratio; images without a CID are skipped.
//   - External: only for http or https URIs; the thumbnail via /media/<cid>.
//   - Record: QuoteURL = PostURL of the quoted record (empty for things that
//     aren't posts, like lists).
//   - RecordWithMedia: the quote, plus the media embed's images, link card or
//     video, as if it were the only embed.
//   - Video: VideoURL = PostURL(postURI), the post itself, and VideoAlt.
//
// Images and thumbnails are blobs in the post author's repository, which is
// why /media can serve them.
func EmbedView(e *pds.Embed, postURI string) *Embed {
	panic("not implemented")
}
