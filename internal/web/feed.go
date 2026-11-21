package web

import (
	"encoding/xml"
	"net/http"
	"net/url"

	"github.com/wescran/cranlab/internal/content"
	"github.com/wescran/cranlab/internal/render"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate,omitempty"`
	Description string `xml:"description"`
}

func (s *server) feed(w http.ResponseWriter, r *http.Request) {
	articles, err := s.deps.Store.Articles(r.Context())
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	channel := rssChannel{Title: SiteTitle, Link: s.cfg.SiteURL + "/", Description: "Articles by Wesley Cranston"}
	for _, a := range articles {
		link := s.cfg.SiteURL + content.ArticlePrefix + url.PathEscape(a.Slug)
		item := rssItem{Title: a.Title, Link: link, GUID: link, Description: a.Description}
		if item.Description == "" {
			item.Description = render.FirstCharacters(a.TextContent, 300)
		}
		if a.HasPublished {
			item.PubDate = a.Published.UTC().Format(http.TimeFormat)
		}
		channel.Items = append(channel.Items, item)
	}
	data, err := xml.Marshal(rssFeed{Version: "2.0", Channel: channel})
	if err != nil {
		s.unavailable(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write(data)
}
