package web

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/pds/pdstest"
)

func (f *fixture) get(t *testing.T, target string) (int, string) {
	t.Helper()
	rec := get(t, f.handler, target)
	return rec.Code, rec.Body.String()
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
}

func mustNotContain(t *testing.T, body string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(body, u) {
			t.Errorf("page should not contain %q", u)
		}
	}
}

func TestCurrentNavigation(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct{ path, link, current string }{
		{"/", "/", "page"}, {"/about", "/about", "page"},
		{"/articles", "/articles", "page"}, {"/articles/welcome", "/articles", "location"},
		{"/notes/3n0025", "/notes", "location"}, {"/tags/writing", "/tags", "location"},
		{"/articles/missing", "", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			_, body := f.get(t, tc.path)
			if tc.current == "" {
				mustNotContain(t, body, "aria-current=")
				return
			}
			mustContain(t, body, `href="`+tc.link+`" aria-current="`+tc.current+`"`)
			if strings.Count(body, "aria-current=") != 1 {
				t.Error("want exactly one current navigation link")
			}
		})
	}
}

func TestArticlesPage(t *testing.T) {
	code, body := newFixture(t).get(t, "/articles")
	if code != http.StatusOK {
		t.Fatalf("GET /articles = %d", code)
	}
	mustContain(t, body,
		"<title>Articles | CranLab</title>",
		`href="/articles/homelab"`, "My Homelab",
		`href="/articles/welcome"`, "Welcome", "Hello there.",
		`href="/tags/writing"`, `href="/tags/homelab"`,
		`datetime="2026-10-15T12:00:00Z"`, "October 15, 2026",
	)
	mustNotContain(t, body, "Not Mine")
	if strings.Index(body, "My Homelab") > strings.Index(body, ">Welcome<") {
		t.Error("articles should be newest first")
	}
	// No description: an excerpt of the text.
	mustContain(t, body, "Racks and cables.")
}

func TestArticleSummaryReadingLayout(t *testing.T) {
	_, page := newFixture(t).get(t, "/articles")
	for _, entry := range regexp.MustCompile(`(?s)<article\b.*?</article>`).FindAllString(page, -1) {
		if !strings.Contains(entry, `href="/articles/welcome"`) {
			continue
		}
		mustContain(t, entry, "September 28, 2026", "1 min read", "Hello there.")
		if !regexp.MustCompile(`<h2[^>]*>\s*<a[^>]*>Welcome</a>\s*</h2>`).MatchString(entry) {
			t.Error("summary title should be a linked heading")
		}
		return
	}
	t.Fatal("Welcome summary missing")
}

func TestArticlePage(t *testing.T) {
	code, body := newFixture(t).get(t, "/articles/welcome")
	if code != http.StatusOK {
		t.Fatalf("GET /articles/welcome = %d", code)
	}
	mustContain(t, body,
		"<title>Welcome | CranLab</title>",
		`<meta name="description" content="Hello there.">`,
		`<h1 class="p-name">Welcome</h1>`,
		"<h2>Hi</h2>",
		`<img src="/media/`+catCID+`" alt="a cat">`,
		"September 28, 2026", "1 min read",
		`href="/tags/writing"`,
		`<a class="u-syndication" href="https://bsky.app/profile/`+testDID+`/post/3mwelcome">Also on Bluesky</a>`,
	)
}

func TestArticlePageWithoutAnnouncement(t *testing.T) {
	_, body := newFixture(t).get(t, "/articles/homelab")
	mustContain(t, body, "My Homelab", "Racks and cables.")
	mustNotContain(t, body, "Also on Bluesky")
}

func TestArticleNotFound(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/articles/missing", "/articles/foreign", "/articles/Welcome"} {
		code, body := f.get(t, path)
		if code != http.StatusNotFound || !strings.Contains(body, "Page Not Found") {
			t.Errorf("GET %s = %d, want the 404 page", path, code)
		}
	}
}

var noteLink = regexp.MustCompile(`href="/notes/(3n\d{4})"`)

func noteKeys(body string) []string {
	var keys []string
	for _, m := range noteLink.FindAllStringSubmatch(body, -1) {
		keys = append(keys, m[1])
	}
	return keys
}

func TestNotesPage(t *testing.T) {
	f := newFixture(t)
	code, body := f.get(t, "/notes")
	if code != http.StatusOK {
		t.Fatalf("GET /notes = %d", code)
	}
	keys := noteKeys(body)
	if len(keys) != 20 || keys[0] != "3n0025" || keys[19] != "3n0006" {
		t.Fatalf("first page notes = %v", keys)
	}
	mustContain(t, body, "<title>Notes | CranLab</title>")
	mustNotContain(t, body, "A reply to someone", "New article: Welcome")

	older := regexp.MustCompile(`<a href="(/notes\?cursor=[^"]+)" rel="next">Older notes</a>`).FindStringSubmatch(body)
	if older == nil {
		t.Fatal("no Older notes link")
	}
	code, body = f.get(t, strings.ReplaceAll(older[1], "&amp;", "&"))
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d", older[1], code)
	}
	keys = noteKeys(body)
	if len(keys) != 5 || keys[0] != "3n0005" || keys[4] != "3n0001" {
		t.Errorf("second page notes = %v", keys)
	}
	mustNotContain(t, body, "Older notes")
}

func TestNotesBadCursor(t *testing.T) {
	code, body := newFixture(t).get(t, "/notes?cursor=%00%3Cscript%3E")
	if code != http.StatusOK {
		t.Fatalf("GET /notes with a junk cursor = %d, want the first page", code)
	}
	if keys := noteKeys(body); len(keys) == 0 || keys[0] != "3n0025" {
		t.Errorf("junk cursor should give the first page, got %v", keys)
	}
}

func TestNotePage(t *testing.T) {
	f := newFixture(t)
	code, body := f.get(t, "/notes/3n0025")
	if code != http.StatusOK {
		t.Fatalf("GET /notes/3n0025 = %d", code)
	}
	mustContain(t, body,
		"<title>Reading go.dev today | CranLab</title>",
		`Reading <a href="https://go.dev">go.dev</a> today`,
		`<img src="/media/`+catCID+`" alt="a cat"`,
		`href="https://bsky.app/profile/`+testDID+`/post/3n0025"`,
		`datetime="2026-10-02T13:00:00Z"`,
	)
}

func TestNotePageEscapes(t *testing.T) {
	_, body := newFixture(t).get(t, "/notes/3n0024")
	mustContain(t, body,
		"&lt;b&gt;escape me&lt;/b&gt;",
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		`href="https://example.com/"`,
		`src="/media/`+thumbCID+`"`,
	)
	mustNotContain(t, body, "<b>escape me</b>", "<script>alert(1)</script>")
}

func TestNoteNotFound(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/notes/3mreply", "/notes/3mwelcome", "/notes/3n9999", "/notes/bad%20key"} {
		if code, _ := f.get(t, path); code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (replies, announcements, missing and invalid keys)", path, code)
		}
	}
}

func TestHomeShowsLatestArticleAndNotes(t *testing.T) {
	_, body := newFixture(t).get(t, "/")
	mustContain(t, body, "Recent Articles", `href="/articles/homelab"`, "My Homelab", "Recent Notes", `href="/notes"`)
	mustContain(t, body, `href="/articles/welcome"`)
	if keys := noteKeys(body); len(keys) != 5 || keys[0] != "3n0025" {
		t.Errorf("home notes = %v, want the 5 newest", keys)
	}
}

func TestTagsPages(t *testing.T) {
	f := newFixture(t)
	code, body := f.get(t, "/tags")
	if code != http.StatusOK {
		t.Fatalf("GET /tags = %d", code)
	}
	mustContain(t, body, `<a href="/tags/homelab">homelab <span>1</span></a>`, `<a href="/tags/writing">writing <span>2</span></a>`)

	code, body = f.get(t, "/tags/writing")
	if code != http.StatusOK {
		t.Fatalf("GET /tags/writing = %d", code)
	}
	mustContain(t, body, "<title>writing | CranLab</title>", `href="/articles/homelab"`, `href="/articles/welcome"`)

	_, body = f.get(t, "/tags/homelab")
	mustNotContain(t, body, `href="/articles/welcome"`)

	if code, _ := f.get(t, "/tags/nope"); code != http.StatusNotFound {
		t.Errorf("GET /tags/nope = %d, want 404", code)
	}
}

func TestMedia(t *testing.T) {
	f := newFixture(t)
	rec := get(t, f.handler, "/media/"+catCID)
	if rec.Code != http.StatusOK || rec.Body.String() != "png bytes" {
		t.Fatalf("GET /media/%s = %d %q", catCID, rec.Code, rec.Body.String())
	}
	want := map[string]string{
		"Content-Type":            "image/png",
		"Content-Security-Policy": "default-src 'none'; sandbox",
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "public, max-age=31536000, immutable",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}

	rec = get(t, f.handler, "/media/"+htmlCID)
	if ct := rec.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("a non-image blob's Content-Type = %q, want application/octet-stream", ct)
	}
}

func TestMediaRejectsBadCIDs(t *testing.T) {
	f := newFixture(t)
	before := len(f.pds.Requests())
	for _, path := range []string{"/media/not-a-cid", "/media/BAFKREIUPPER", "/media/bafk..%2Fxrpc", "/media/b1", "/media/bafkreitruncated"} {
		if rec := get(t, f.handler, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
	if got := f.pds.Requests()[before:]; len(got) != 0 {
		t.Errorf("invalid CIDs reached the PDS: %v", got)
	}
}

func TestMediaMissing(t *testing.T) {
	f := newFixture(t)
	const missing = "bafkreih7uy2yhx5gobvypuuexbvq22j2cypeqqfk2lc46225e7b3syq7pu"
	rec := get(t, f.handler, "/media/"+missing)
	if cc := rec.Result().Header.Get("Cache-Control"); rec.Code != http.StatusNotFound || cc != "public, max-age=60" {
		t.Errorf("missing blob = %d, Cache-Control %q; want 404 cached for 60s", rec.Code, cc)
	}
}

func TestPDSDown(t *testing.T) {
	fake := pdstest.New(t, testDID)
	fake.FailWith(http.StatusInternalServerError)
	f := startFixture(t, fake, false)
	for _, path := range []string{"/articles", "/articles/welcome", "/tags", "/tags/writing", "/", "/notes", "/notes/3n0025"} {
		t.Run("GET"+path, func(t *testing.T) {
			code, body := f.get(t, path)
			if code != http.StatusServiceUnavailable || !strings.Contains(strings.ToLower(body), "temporarily unavailable") {
				t.Errorf("GET %s with the PDS down = %d, want the 503 page", path, code)
			}
		})
	}
	t.Run("GET/media", func(t *testing.T) {
		if rec := get(t, f.handler, "/media/"+catCID); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("GET /media with the PDS down = %d, want 503 (not 404)", rec.Code)
		}
	})
	// Pages that don't use the PDS keep working.
	if code, _ := f.get(t, "/about"); code != http.StatusOK {
		t.Errorf("GET /about = %d", code)
	}
}
