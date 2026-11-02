package pds

import "encoding/json/jsontext"

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
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
}

// StrongRef points at a specific version of a record.
type StrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// UnmarshalJSON decodes any embed type into the one Embed struct.
//
// The JSON shapes, by $type:
//
//	images:          {"$type", "images": [Image…]}
//	external:        {"$type", "external": External}
//	record:          {"$type", "record": StrongRef}
//	recordWithMedia: {"$type", "record": {"$type": "…embed.record", "record": StrongRef},
//	                  "media": <an images, video or external embed>}
//	video:           {"$type", "video": Blob, "alt": "…"}
//
// Note that "record" means different things in record and recordWithMedia:
// a strong ref in one, a whole record embed in the other. An unknown $type
// sets only Type, with no error, so new embed types never break a page.
func (e *Embed) UnmarshalJSON(data []byte) error {
	panic("not implemented")
}
