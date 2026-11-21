package render

import (
	"net/url"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/weburl"
)

// Embed is what a Note's embed looks like on a page. Templates escape every field.
type Embed struct {
	Images   []ImageView
	External *ExternalView
	QuoteURL string
	VideoURL string
	VideoAlt string
}

// ImageView is one image. Width and Height are its aspect ratio (0 if unknown).
type ImageView struct {
	URL, Alt      string
	Width, Height int
}

// ExternalView is a link card. ThumbURL is empty without a thumbnail.
type ExternalView struct {
	URL, Title, Description, ThumbURL string
}

// mediaURL is the site's URL for one of the owner's blobs.
func mediaURL(cid string) string { return "/media/" + url.PathEscape(cid) }

// EmbedView links media locally and quoted posts and videos on Bluesky.
func EmbedView(e *content.NoteEmbed, postURI string) *Embed {
	if e == nil {
		return nil
	}
	v := &Embed{QuoteURL: PostURL(e.QuoteURI)}
	for _, i := range e.Images {
		v.Images = append(v.Images, ImageView{URL: mediaURL(i.CID), Alt: i.Alt, Width: i.Width, Height: i.Height})
	}
	if x := e.External; x != nil && weburl.Absolute(x.URL) {
		v.External = &ExternalView{URL: x.URL, Title: x.Title, Description: x.Description}
		if x.ThumbCID != "" {
			v.External.ThumbURL = mediaURL(x.ThumbCID)
		}
	}
	if e.VideoAlt != nil {
		v.VideoURL = PostURL(postURI)
		v.VideoAlt = *e.VideoAlt
	}
	if len(v.Images) == 0 && v.External == nil && v.QuoteURL == "" && v.VideoURL == "" {
		return nil
	}
	return v
}
