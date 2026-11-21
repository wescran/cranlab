package pds

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Collections the site reads.
const (
	DocumentCollection = "site.standard.document"
	PostCollection     = "app.bsky.feed.post"
)

// Record is a record as the PDS returns it.
type Record[T any] struct {
	URI   string
	CID   string
	Value T
	// Raw is the record's value exactly as received, including fields the
	// site doesn't know about.
	Raw jsontext.Value
}

// Document is a site.standard.document record (an article).
type Document struct {
	Site        string         `json:"site"`
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	PublishedAt string         `json:"publishedAt"`
	UpdatedAt   string         `json:"updatedAt"`
	TextContent string         `json:"textContent"`
	Tags        []string       `json:"tags"`
	Content     jsontext.Value `json:"content"`
	CoverImage  *Blob          `json:"coverImage"`
	BskyPostRef *StrongRef     `json:"bskyPostRef"`
}

// Markdown returns authored Markdown only for at.markpub.markdown content.
func (d Document) Markdown() *string {
	var body struct {
		Type string `json:"$type"`
		Text struct {
			Markdown *string `json:"markdown"`
		} `json:"text"`
	}
	if json.Unmarshal(d.Content, &body) != nil || body.Type != "at.markpub.markdown" {
		return nil
	}
	return body.Text.Markdown
}

// Post is an app.bsky.feed.post record.
type Post struct {
	Text      string   `json:"text"`
	CreatedAt string   `json:"createdAt"`
	Facets    []Facet  `json:"facets"`
	Embed     *Embed   `json:"embed"`
	Langs     []string `json:"langs"`
	// Reply is set on replies; the site only needs to know whether it's there.
	Reply jsontext.Value `json:"reply"`
}

// Facet annotates a byte range of a post's UTF-8 text.
type Facet struct {
	Index    ByteSlice `json:"index"`
	Features []Feature `json:"features"`
}

// ByteSlice is a half-open range of UTF-8 byte offsets.
type ByteSlice struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

// Feature is what a facet means: a link (URI), a mention (DID) or a hashtag
// (Tag), told apart by Type.
type Feature struct {
	Type string `json:"$type"` // app.bsky.richtext.facet#link, #mention or #tag
	URI  string `json:"uri"`
	DID  string `json:"did"`
	Tag  string `json:"tag"`
}

// Embed types.
const (
	EmbedImages          = "app.bsky.embed.images"
	EmbedExternal        = "app.bsky.embed.external"
	EmbedRecord          = "app.bsky.embed.record"
	EmbedRecordWithMedia = "app.bsky.embed.recordWithMedia"
	EmbedVideo           = "app.bsky.embed.video"
)

// Embed is a post's embed, of any type. Type says which fields are set:
//
//   - EmbedImages: Images
//   - EmbedExternal: External
//   - EmbedRecord: Quote (the quoted post)
//   - EmbedRecordWithMedia: Quote, and Media (an images, video or external
//     embed)
//   - EmbedVideo: Video
//   - anything else: only Type
type Embed struct {
	Type     string
	Images   []Image
	External *External
	Quote    *StrongRef
	Media    *Embed
	Video    *Video
}

// Image is one image of an images embed.
type Image struct {
	Alt         string       `json:"alt"`
	Image       Blob         `json:"image"`
	AspectRatio *AspectRatio `json:"aspectRatio"`
}

// AspectRatio is an image's or video's width and height, in any unit.
type AspectRatio struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// External is a link card.
type External struct {
	URI         string `json:"uri"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumb       *Blob  `json:"thumb"`
}

// Video is a video embed. The site links to it rather than playing it.
type Video struct {
	Alt string
}

// Blob is a reference to an uploaded file (image).
type Blob struct {
	Ref struct {
		Link string `json:"$link"` // the blob's CID
	} `json:"ref"`
	MIMEType string `json:"mimeType"`
	Size     int    `json:"size"`
}

// StrongRef points at a specific version of a record.
type StrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// UnmarshalJSON decodes any embed type into Embed. "record" holds a strong
// ref in a record embed but a whole record embed in recordWithMedia. Unknown
// types set only Type.
func (e *Embed) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type     string         `json:"$type"`
		Images   []Image        `json:"images"`
		External *External      `json:"external"`
		Record   jsontext.Value `json:"record"`
		Media    *Embed         `json:"media"`
		Alt      string         `json:"alt"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*e = Embed{Type: raw.Type}

	switch raw.Type {
	case EmbedImages:
		e.Images = raw.Images
	case EmbedExternal:
		e.External = raw.External
	case EmbedRecord:
		if len(raw.Record) > 0 {
			var ref StrongRef
			if err := json.Unmarshal(raw.Record, &ref); err != nil {
				return err
			}
			e.Quote = &ref
		}
	case EmbedRecordWithMedia:
		if len(raw.Record) > 0 {
			var emRec struct {
				Record StrongRef `json:"record"`
			}
			if err := json.Unmarshal(raw.Record, &emRec); err != nil {
				return err
			}
			e.Quote = &emRec.Record
			e.Media = raw.Media
		}
	case EmbedVideo:
		e.Video = &Video{Alt: raw.Alt}
	}

	return nil
}
