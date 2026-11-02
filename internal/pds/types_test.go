package pds

import (
	"encoding/json"
	"testing"
)

const blobJSON = `{"$type":"blob","ref":{"$link":"bafkreiimg"},"mimeType":"image/jpeg","size":1234}`

func decodePost(t *testing.T, raw string) Post {
	t.Helper()
	var p Post
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return p
}

func TestEmbedImages(t *testing.T) {
	p := decodePost(t, `{"text":"","embed":{"$type":"app.bsky.embed.images","images":[
		{"alt":"a cat","image":`+blobJSON+`,"aspectRatio":{"width":4,"height":3}},
		{"alt":"","image":`+blobJSON+`}]}}`)
	e := p.Embed
	if e == nil || e.Type != EmbedImages || len(e.Images) != 2 {
		t.Fatalf("embed = %+v", e)
	}
	img := e.Images[0]
	if img.Alt != "a cat" || img.Image.Ref.Link != "bafkreiimg" || img.Image.MimeType != "image/jpeg" ||
		img.AspectRatio == nil || img.AspectRatio.Width != 4 || img.AspectRatio.Height != 3 {
		t.Errorf("image = %+v", img)
	}
	if e.Images[1].AspectRatio != nil {
		t.Error("missing aspectRatio should be nil")
	}
	if e.External != nil || e.Quote != nil || e.Media != nil || e.Video != nil {
		t.Errorf("only Images should be set: %+v", e)
	}
}

func TestEmbedExternal(t *testing.T) {
	p := decodePost(t, `{"embed":{"$type":"app.bsky.embed.external","external":{
		"uri":"https://example.com/","title":"Example","description":"An example.","thumb":`+blobJSON+`}}}`)
	x := p.Embed.External
	if p.Embed.Type != EmbedExternal || x == nil || x.URI != "https://example.com/" || x.Title != "Example" ||
		x.Description != "An example." || x.Thumb == nil || x.Thumb.Ref.Link != "bafkreiimg" {
		t.Errorf("embed = %+v, external = %+v", p.Embed, x)
	}
}

func TestEmbedRecord(t *testing.T) {
	p := decodePost(t, `{"embed":{"$type":"app.bsky.embed.record","record":{
		"uri":"at://did:plc:other/app.bsky.feed.post/3mquoted","cid":"bafyreiquoted"}}}`)
	q := p.Embed.Quote
	if p.Embed.Type != EmbedRecord || q == nil || q.URI != "at://did:plc:other/app.bsky.feed.post/3mquoted" || q.CID != "bafyreiquoted" {
		t.Errorf("embed = %+v, quote = %+v", p.Embed, q)
	}
	if p.Embed.Media != nil {
		t.Error("Media should be nil")
	}
}

func TestEmbedRecordWithMedia(t *testing.T) {
	p := decodePost(t, `{"embed":{"$type":"app.bsky.embed.recordWithMedia",
		"record":{"$type":"app.bsky.embed.record","record":{"uri":"at://did:plc:other/app.bsky.feed.post/3mquoted","cid":"bafyreiquoted"}},
		"media":{"$type":"app.bsky.embed.images","images":[{"alt":"pic","image":`+blobJSON+`}]}}}`)
	e := p.Embed
	if e.Type != EmbedRecordWithMedia {
		t.Fatalf("Type = %q", e.Type)
	}
	if e.Quote == nil || e.Quote.URI != "at://did:plc:other/app.bsky.feed.post/3mquoted" {
		t.Errorf("Quote = %+v", e.Quote)
	}
	if e.Media == nil || e.Media.Type != EmbedImages || len(e.Media.Images) != 1 || e.Media.Images[0].Alt != "pic" {
		t.Errorf("Media = %+v", e.Media)
	}
}

func TestEmbedVideo(t *testing.T) {
	p := decodePost(t, `{"embed":{"$type":"app.bsky.embed.video","video":`+blobJSON+`,"alt":"a clip","aspectRatio":{"width":16,"height":9}}}`)
	if p.Embed.Type != EmbedVideo || p.Embed.Video == nil || p.Embed.Video.Alt != "a clip" {
		t.Errorf("embed = %+v", p.Embed)
	}
}

func TestEmbedUnknownType(t *testing.T) {
	p := decodePost(t, `{"text":"hi","embed":{"$type":"app.bsky.embed.somethingNew","stuff":[1,2,3]}}`)
	e := p.Embed
	if e == nil || e.Type != "app.bsky.embed.somethingNew" || e.Images != nil || e.External != nil || e.Quote != nil || e.Video != nil {
		t.Errorf("embed = %+v", e)
	}
	if p.Text != "hi" {
		t.Error("the rest of the post should still decode")
	}
}

func TestEmbedMalformed(t *testing.T) {
	var p Post
	err := json.Unmarshal([]byte(`{"embed":{"$type":"app.bsky.embed.images","images":"not a list"}}`), &p)
	if err == nil {
		t.Error("want an error for an embed whose fields have the wrong shape")
	}
}

func TestNoEmbed(t *testing.T) {
	if p := decodePost(t, `{"text":"plain"}`); p.Embed != nil {
		t.Errorf("Embed = %+v, want nil", p.Embed)
	}
}

func TestFacetsAndReply(t *testing.T) {
	p := decodePost(t, `{"text":"hi @alice.test #go https://go.dev",
		"facets":[
			{"index":{"byteStart":3,"byteEnd":14},"features":[{"$type":"app.bsky.richtext.facet#mention","did":"did:plc:alice"}]},
			{"index":{"byteStart":15,"byteEnd":18},"features":[{"$type":"app.bsky.richtext.facet#tag","tag":"go"}]},
			{"index":{"byteStart":19,"byteEnd":33},"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://go.dev"}]}],
		"reply":{"root":{"uri":"at://x/app.bsky.feed.post/1","cid":"c"},"parent":{"uri":"at://x/app.bsky.feed.post/1","cid":"c"}}}`)
	if len(p.Facets) != 3 {
		t.Fatalf("facets = %+v", p.Facets)
	}
	m, tag, link := p.Facets[0], p.Facets[1].Features[0], p.Facets[2].Features[0]
	if m.Index.ByteStart != 3 || m.Index.ByteEnd != 14 || m.Features[0].Type != "app.bsky.richtext.facet#mention" || m.Features[0].DID != "did:plc:alice" {
		t.Errorf("mention = %+v", m)
	}
	if tag.Tag != "go" || link.URI != "https://go.dev" {
		t.Errorf("tag, link = %+v, %+v", tag, link)
	}
	if p.Reply == nil {
		t.Error("Reply should be set on a reply")
	}
	if q := decodePost(t, `{"text":"top level"}`); q.Reply != nil {
		t.Error("Reply should be nil on a top-level post")
	}
}
