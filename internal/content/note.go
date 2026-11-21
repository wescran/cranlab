package content

import (
	"net/url"
	"strings"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/weburl"
)

// Facet links a half-open range of UTF-8 bytes in a Note's text.
type Facet struct {
	Start, End int
	URL        string
}

// NoteEmbed contains the public media and quoted Note, which can coexist.
type NoteEmbed struct {
	Images   []NoteImage
	External *LinkCard
	QuoteURI string
	VideoAlt *string // non-nil for a video, even one without alt text
}

type NoteImage struct {
	CID, Alt      string
	Width, Height int
}

type LinkCard struct {
	URL, Title, Description, ThumbCID string
}

// ProjectFacets selects the first usable feature in each annotated range.
func ProjectFacets(src []pds.Facet) []Facet {
	var out []Facet
	for _, f := range src {
		if href := featureURL(f.Features); href != "" {
			out = append(out, Facet{Start: f.Index.ByteStart, End: f.Index.ByteEnd, URL: href})
		}
	}
	return out
}

func featureURL(features []pds.Feature) string {
	for _, f := range features {
		switch f.Type {
		case "app.bsky.richtext.facet#link":
			if weburl.Absolute(f.URI) {
				return f.URI
			}
		case "app.bsky.richtext.facet#mention":
			if strings.HasPrefix(f.DID, "did:") {
				return "https://bsky.app/profile/" + f.DID
			}
		case "app.bsky.richtext.facet#tag":
			if f.Tag != "" {
				return "https://bsky.app/hashtag/" + url.PathEscape(f.Tag)
			}
		}
	}
	return ""
}

// ProjectEmbed extracts media independently of the record-with-media envelope.
func ProjectEmbed(e *pds.Embed) *NoteEmbed {
	if e == nil {
		return nil
	}
	var out NoteEmbed
	if e.Type == pds.EmbedRecordWithMedia || e.Type == pds.EmbedRecord {
		if e.Quote != nil {
			out.QuoteURI = e.Quote.URI
		}
		if e.Type == pds.EmbedRecordWithMedia && e.Media != nil {
			e = e.Media
		}
	}
	switch e.Type {
	case pds.EmbedImages:
		for _, img := range e.Images {
			if img.Image.Ref.Link == "" {
				continue
			}
			i := NoteImage{CID: img.Image.Ref.Link, Alt: img.Alt}
			if img.AspectRatio != nil {
				i.Width, i.Height = img.AspectRatio.Width, img.AspectRatio.Height
			}
			out.Images = append(out.Images, i)
		}
	case pds.EmbedExternal:
		if x := e.External; x != nil && weburl.Absolute(x.URI) {
			out.External = &LinkCard{URL: x.URI, Title: x.Title, Description: x.Description}
			if x.Thumb != nil {
				out.External.ThumbCID = x.Thumb.Ref.Link
			}
		}
	case pds.EmbedVideo:
		alt := ""
		if e.Video != nil {
			alt = e.Video.Alt
		}
		out.VideoAlt = &alt
	}
	if len(out.Images) == 0 && out.External == nil && out.QuoteURI == "" && out.VideoAlt == nil {
		return nil
	}
	return &out
}
