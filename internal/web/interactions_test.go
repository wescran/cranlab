package web

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/webmention"
	"golang.org/x/net/html"
	"willnorris.com/go/microformats"
)

func TestDetailEntryInteractions(t *testing.T) {
	const article = "/articles/welcome"
	const note = "/notes/3n0025"
	const untrusted = "<script>thirdParty()</script>"
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var children []map[string]any
		switch r.URL.Query().Get("target") {
		case "https://cranlab.ca" + article, "https://cranlab.ca" + note:
			for _, g := range []struct {
				property string
				count    int
			}{{"like-of", 5}, {"repost-of", 6}, {"in-reply-to", 1}, {"mention-of", 1}, {"bookmark-of", 1}, {"rsvp", 1}} {
				if g.property == "rsvp" && strings.HasSuffix(r.URL.Query().Get("target"), note) {
					continue
				}
				for i := range g.count {
					child := map[string]any{
						"wm-property": g.property, "wm-source": fmt.Sprintf("https://reader.test/%s/%d", g.property, i),
						"author":  map[string]string{"name": "Lee", "url": "https://reader.test/", "photo": "https://reader.test/me.png"},
						"content": map[string]string{"text": untrusted, "html": "<script>injected()</script>"},
					}
					if g.property == "rsvp" {
						child["wm-property"] = "in-reply-to"
						child["rsvp"] = "yes"
					}
					children = append(children, child)
				}
			}
		case "https://cranlab.ca" + article + "/", "https://cranlab.ca" + note + "/":
		default:
			t.Errorf("unexpected target %s", r.URL)
			w.WriteHeader(400)
			return
		}
		if err := json.MarshalWrite(w, map[string]any{"children": children}); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	f := newFixture(t)
	h, err := New(testConfig(), Deps{Store: f.store, Interactions: webmention.New(api.URL, api.Client())})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{article, note} {
		t.Run(path, func(t *testing.T) {
			rec := get(t, h, "http://preview.test"+path+"?ignored=yes")
			body := rec.Body.String()
			entry := findItem(parseMF(t, body), "h-entry")
			if rec.Code != 200 || entry == nil {
				t.Fatalf("missing h-entry: %d", rec.Code)
			}
			for prop, want := range map[string]int{"like": 5, "repost": 6, "comment": 1} {
				if n := len(entry.Properties[prop]); n != want {
					t.Errorf("%s count %d, want %d", prop, n, want)
				}
			}
			for _, prop := range []string{"comment", "mention"} {
				values := entry.Properties[prop]
				if len(values) == 0 {
					t.Fatalf("no %s", prop)
				}
				cite, ok := values[0].(*microformats.Microformat)
				if !ok || !slices.Contains(cite.Type, "h-cite") {
					t.Fatalf("%s is not a citation: %v", prop, values[0])
				}
			}
			authors := entry.Properties["author"]
			if len(authors) != 1 {
				t.Fatalf("entry authors: %v", authors)
			}
			author, ok := authors[0].(*microformats.Microformat)
			if !ok || property(author, "url") != "https://cranlab.ca/" {
				t.Errorf("entry author = %v", authors)
			}
			key := "3mwelcome"
			if path == note {
				key = "3n0025"
			}
			if property(entry, "syndication") != "https://bsky.app/profile/"+testDID+"/post/"+key {
				t.Error("lost entry syndication")
			}
			mustContain(t, body, `class="facepile"`, `src="https://reader.test/me.png"`, `&lt;script&gt;thirdParty()&lt;/script&gt;`)
			mustNotContain(t, body, untrusted, "injected()")
			groups := detailsGroups(t, body)
			want := map[string]bool{"Likes (5)": true, "Reposts (6)": false, "Replies (1)": true, "Mentions (1)": true, "Bookmarks (1)": true}
			if path == article {
				want["RSVPs (1)"] = true
			}
			if len(groups) != len(want) {
				t.Errorf("groups %v, want %v", groups, want)
			}
			for label, open := range want {
				if v, ok := groups[label]; !ok || v != open {
					t.Errorf("%s open=%v present=%v", label, v, ok)
				}
			}
		})
	}
}

func TestUnavailableInteractionsDoNotHideEntry(t *testing.T) {
	f := newFixture(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer api.Close()
	h, err := New(testConfig(), Deps{Store: f.store, Interactions: webmention.New(api.URL, api.Client())})
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"/articles/welcome": "Welcome", "/notes/3n0025": "Reading"} {
		rec := get(t, h, path)
		body := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(body, want) || strings.Contains(body, `class="interactions"`) {
			t.Errorf("GET %s on outage: %d, missing entry or unexpected interactions", path, rec.Code)
		}
	}
}

func detailsGroups(t *testing.T, body string) map[string]bool {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]bool{}
	for n := range doc.Descendants() {
		if n.Data != "summary" || n.Parent.Data != "details" {
			continue
		}
		var label strings.Builder
		for c := range n.Descendants() {
			if c.Type == html.TextNode {
				label.WriteString(c.Data)
			}
		}
		groups[strings.TrimSpace(label.String())] = slices.ContainsFunc(n.Parent.Attr, func(a html.Attribute) bool { return a.Key == "open" })
	}
	return groups
}
