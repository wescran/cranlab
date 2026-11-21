// Package webmention reads public jf2 Interactions from webmention.io.
package webmention

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/weburl"
)

// Client reads verified Webmentions for a target from a public jf2 endpoint.
type Client struct {
	origin string
	http   *http.Client
}

// New uses the webmention.io API at origin with the supplied HTTP client.
func New(origin string, client *http.Client) *Client {
	return &Client{origin: origin, http: client}
}

type child struct {
	Property string `json:"wm-property"`
	Source   string `json:"wm-source"`
	Author   struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Photo string `json:"photo"`
	} `json:"author"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
	RSVP string `json:"rsvp"`
}

var kinds = map[string]content.InteractionKind{
	"like-of": content.Like, "repost-of": content.Repost, "in-reply-to": content.Reply,
	"mention-of": content.Mention, "bookmark-of": content.Bookmark, "rsvp": content.RSVP,
}

// Interactions merges both spellings of a canonical target under one deadline.
// A failed lookup never returns a partial list.
func (c *Client) Interactions(ctx context.Context, canonical string) ([]content.Interaction, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	target := strings.TrimSuffix(canonical, "/")
	seen := make(map[string]bool)
	var result []content.Interaction
	for _, variant := range []string{target, target + "/"} {
		children, err := c.children(ctx, variant)
		if err != nil {
			return nil, err
		}
		for _, m := range children {
			kind, ok := kinds[m.Property]
			if !ok || !weburl.Absolute(m.Source) || seen[m.Source] {
				continue
			}
			if kind == content.Reply && m.RSVP != "" {
				kind = content.RSVP
			}
			seen[m.Source] = true
			item := content.Interaction{Kind: kind, SourceURL: m.Source, AuthorName: m.Author.Name, Text: m.Content.Text, RSVP: m.RSVP}
			if weburl.Absolute(m.Author.Photo) {
				item.PhotoURL = m.Author.Photo
			}
			if weburl.Absolute(m.Author.URL) {
				item.AuthorURL = m.Author.URL
			}
			result = append(result, item)
		}
	}
	return result, nil
}

func (c *Client) children(ctx context.Context, target string) ([]child, error) {
	q := url.Values{"target": {target}, "per-page": {"1000"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.origin+"/api/mentions.jf2?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webmention: HTTP %d", res.StatusCode)
	}
	const maxBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, fmt.Errorf("webmention: response exceeds %d bytes", maxBytes)
	}
	var feed struct {
		Children []child `json:"children"`
	}
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	return feed.Children, nil
}
