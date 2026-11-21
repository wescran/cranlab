package render

import (
	"reflect"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

const postURI = "at://did:plc:ogl52fc2rdooori2ayf74can/app.bsky.feed.post/3mxc6hxwdnc2s"

func blob(cid string) pds.Blob {
	var b pds.Blob
	b.Ref.Link = cid
	b.MIMEType = "image/jpeg"
	return b
}

func TestEmbedViewNil(t *testing.T) {
	if v := EmbedView(nil, postURI); v != nil {
		t.Errorf("EmbedView(nil) = %+v", v)
	}
	if v := EmbedView(&pds.Embed{Type: "app.bsky.embed.somethingNew"}, postURI); v != nil {
		t.Errorf("unknown embed = %+v, want nil", v)
	}
}

func TestEmbedViewImages(t *testing.T) {
	e := &pds.Embed{Type: pds.EmbedImages, Images: []pds.Image{
		{Alt: "a cat", Image: blob("bafkreicat"), AspectRatio: &pds.AspectRatio{Width: 4, Height: 3}},
		{Alt: "", Image: blob("bafkreidog")},
	}}
	got := EmbedView(e, postURI)
	want := &Embed{Images: []ImageView{
		{URL: "/media/bafkreicat", Alt: "a cat", Width: 4, Height: 3},
		{URL: "/media/bafkreidog"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EmbedView =\n%+v\nwant\n%+v", got, want)
	}
}

func TestEmbedViewExternal(t *testing.T) {
	thumb := blob("bafkreithumb")
	e := &pds.Embed{Type: pds.EmbedExternal, External: &pds.External{
		URI: "https://example.com/post", Title: "<script>alert(1)</script>", Description: "A page.", Thumb: &thumb,
	}}
	got := EmbedView(e, postURI)
	want := &Embed{External: &ExternalView{
		URL: "https://example.com/post", Title: "<script>alert(1)</script>", Description: "A page.", ThumbURL: "/media/bafkreithumb",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EmbedView =\n%+v\nwant\n%+v (text is left for the template to escape)", got, want)
	}
	noThumb := EmbedView(&pds.Embed{Type: pds.EmbedExternal, External: &pds.External{URI: "https://example.com/"}}, postURI)
	if noThumb == nil || noThumb.External == nil || noThumb.External.ThumbURL != "" {
		t.Errorf("without a thumb = %+v", noThumb)
	}
}

func TestEmbedViewExternalRejectsNonWebURL(t *testing.T) {
	for _, uri := range []string{"javascript:alert(1)", "http:/path", "https://:443/path"} {
		e := &pds.Embed{Type: pds.EmbedExternal, External: &pds.External{URI: uri}}
		if v := EmbedView(e, postURI); v != nil {
			t.Errorf("link card for %q = %+v, want nil", uri, v)
		}
	}
}

func TestEmbedViewQuote(t *testing.T) {
	e := &pds.Embed{Type: pds.EmbedRecord, Quote: &pds.StrongRef{URI: "at://did:plc:other/app.bsky.feed.post/3mquoted", CID: "bafyrei"}}
	got := EmbedView(e, postURI)
	if got == nil || got.QuoteURL != "https://bsky.app/profile/did:plc:other/post/3mquoted" {
		t.Errorf("EmbedView = %+v", got)
	}
	// Quoting something that isn't a post (a list, a feed) has nothing to link.
	other := &pds.Embed{Type: pds.EmbedRecord, Quote: &pds.StrongRef{URI: "at://did:plc:other/app.bsky.graph.list/3mlist"}}
	if v := EmbedView(other, postURI); v != nil {
		t.Errorf("quote of a list = %+v, want nil", v)
	}
}

func TestEmbedViewRecordWithMedia(t *testing.T) {
	e := &pds.Embed{
		Type:  pds.EmbedRecordWithMedia,
		Quote: &pds.StrongRef{URI: "at://did:plc:other/app.bsky.feed.post/3mquoted"},
		Media: &pds.Embed{Type: pds.EmbedImages, Images: []pds.Image{{Alt: "pic", Image: blob("bafkreipic")}}},
	}
	got := EmbedView(e, postURI)
	want := &Embed{
		Images:   []ImageView{{URL: "/media/bafkreipic", Alt: "pic"}},
		QuoteURL: "https://bsky.app/profile/did:plc:other/post/3mquoted",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EmbedView =\n%+v\nwant\n%+v", got, want)
	}
}

func TestEmbedViewVideo(t *testing.T) {
	got := EmbedView(&pds.Embed{Type: pds.EmbedVideo, Video: &pds.Video{Alt: "a clip"}}, postURI)
	want := &Embed{VideoURL: "https://bsky.app/profile/did:plc:ogl52fc2rdooori2ayf74can/post/3mxc6hxwdnc2s", VideoAlt: "a clip"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EmbedView = %+v, want %+v", got, want)
	}
}
