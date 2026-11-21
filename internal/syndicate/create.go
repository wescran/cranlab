package syndicate

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/render"
	assets "github.com/wescran/cranlab/web"
)

func eligible(date, now time.Time) bool {
	return !date.Before(now.Add(-7*24*time.Hour)) && !date.After(now)
}

func (r *run) thumbnail(ctx context.Context, cover *pds.Blob) (pds.Blob, error) {
	// The Article lives in this repository already. An external card can use
	// its cover blob directly if it fits the thumbnail lexicon limit.
	if cover != nil && cover.Type == "blob" && cover.Ref.Link != "" && strings.HasPrefix(cover.MIMEType, "image/") && cover.Size > 0 && cover.Size <= 1_000_000 {
		return *cover, nil
	}
	data, err := assets.FS.ReadFile("static/images/card-default.png")
	if err != nil {
		return pds.Blob{}, errors.New("missing default thumbnail")
	}
	thumb, err := r.client.UploadBlob(ctx, r.token, "image/png", data)
	if err != nil {
		return pds.Blob{}, errors.New("thumbnail upload failed")
	}
	return thumb, nil
}

func (r *run) create(ctx context.Context, key string, doc pds.Document, canonical string) (pds.StrongRef, error) {
	thumb, err := r.thumbnail(ctx, doc.CoverImage)
	if err != nil {
		return pds.StrongRef{}, err
	}
	description := doc.Description
	if strings.TrimSpace(description) == "" {
		description = render.FirstCharacters(doc.TextContent, 300)
	}
	prefix := doc.Title + "\n\n"
	text := prefix + canonical
	post := map[string]any{
		"$type":     pds.PostCollection,
		"text":      text,
		"createdAt": r.at.UTC().Format(time.RFC3339Nano),
		"langs":     []string{"en"},
		"facets": []pds.Facet{{
			Index:    pds.ByteSlice{ByteStart: len(prefix), ByteEnd: len(text)},
			Features: []pds.Feature{{Type: "app.bsky.richtext.facet#link", URI: canonical}},
		}},
		"embed": map[string]any{
			"$type":    pds.EmbedExternal,
			"external": pds.External{URI: canonical, Title: doc.Title, Description: description, Thumb: &thumb},
		},
	}
	// A timeout may hide a successful write. Count attempts before the call,
	// independent of its response or whether the later document link succeeds.
	r.remaining--
	ref, err := r.client.CreatePost(ctx, r.token, r.did, key, post)
	if err != nil {
		return pds.StrongRef{}, errors.New("Announcement creation failed")
	}
	return ref, nil
}
