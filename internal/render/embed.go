package render

import (
	"net/url"

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

// addEmbed adds one embed's images, link card, quote or video to v. It handles
// recordWithMedia by adding its media embed too.
func addEmbed(v *Embed, e *pds.Embed, postURI string) {
	switch e.Type {
	case pds.EmbedImages:
		for _, img := range e.Images {
			cid := img.Image.Ref.Link
			if cid == "" {
				continue
			}
			view := ImageView{URL: mediaURL(cid), Alt: img.Alt}
			if img.AspectRatio != nil {
				view.Width = img.AspectRatio.Width
				view.Height = img.AspectRatio.Height
			}
			v.Images = append(v.Images, view)
		}
	case pds.EmbedExternal:
		if x := e.External; x != nil && isWebURL(x.URI) {
			v.External = &ExternalView{URL: x.URI, Title: x.Title, Description: x.Description}
			if x.Thumb != nil && x.Thumb.Ref.Link != "" {
				v.External.ThumbURL = mediaURL(x.Thumb.Ref.Link)
			}
		}
	case pds.EmbedVideo:
		v.VideoURL = PostURL(postURI)
		if e.Video != nil {
			v.VideoAlt = e.Video.Alt
		}
	case pds.EmbedRecord, pds.EmbedRecordWithMedia:
		if e.Quote != nil {
			v.QuoteURL = PostURL(e.Quote.URI)
		}
		if e.Type == pds.EmbedRecordWithMedia && e.Media != nil {
			addEmbed(v, e.Media, postURI)
		}
	}
}

// mediaURL is the site's URL for one of the owner's blobs.
func mediaURL(cid string) string {
	return "/media/" + url.PathEscape(cid)
}

// isWebURL reports whether s is an absolute http or https URL.
func isWebURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// EmbedView describes e for a page: images and link-card thumbnails as
// /media/<cid> URLs, link cards with http(s) URLs only, quoted posts as
// bsky.app links, and videos as a link to the post itself (postURI). It
// returns nil when there's nothing to show.
func EmbedView(e *pds.Embed, postURI string) *Embed {
	if e == nil {
		return nil
	}
	v := &Embed{}
	addEmbed(v, e, postURI)
	if len(v.Images) == 0 && v.External == nil && v.QuoteURL == "" && v.VideoURL == "" {
		return nil
	}
	return v
}
