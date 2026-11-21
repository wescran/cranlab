package render

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

const ownerDID = "did:plc:ogl52fc2rdooori2ayf74can"

var opts = Options{DID: ownerDID, PDSPublicURL: "https://weslandia.ca"}

func markpub(t *testing.T, markdown string) pds.Document {
	t.Helper()
	content, err := json.Marshal(map[string]any{
		"$type":  "at.markpub.markdown",
		"flavor": "gfm",
		"text":   map[string]string{"$type": "at.markpub.text", "markdown": markdown},
	})
	if err != nil {
		t.Fatal(err)
	}
	return pds.Document{Content: jsontext.Value(content), TextContent: "plain fallback"}
}

func body(t *testing.T, markdown string) string {
	t.Helper()
	doc := markpub(t, markdown)
	html, err := ArticleBody(doc.Markdown(), doc.TextContent, opts)
	if err != nil {
		t.Fatal(err)
	}
	return string(html)
}

func assertContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
}

func TestArticleBodyGFM(t *testing.T) {
	got := body(t, "## Hello\n\nSome **bold** and ~~gone~~.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- [x] done\n- [ ] todo\n")
	assertContains(t, got,
		"<h2>Hello</h2>",
		"<strong>bold</strong>",
		"<del>gone</del>",
		"<table>", "<td>1</td>",
		`<input checked="" disabled="" type="checkbox"`,
	)
}

func TestArticleBodyHighlightsFencedCode(t *testing.T) {
	got := body(t, "```go\nfunc main() {}\n```\n")
	assertContains(t, got, `class="chroma"`, `<span class="kd">func</span>`)
	if strings.Contains(got, `style="`) {
		t.Error("code colors should follow theme classes, not inline styles")
	}
}

func TestArticleBodyKeepsRawHTML(t *testing.T) {
	got := body(t, "Before\n\n<details><summary>More</summary>Hidden</details>\n")
	assertContains(t, got, "<details><summary>More</summary>Hidden</details>")
}

func TestArticleBodyRewritesOwnBlobs(t *testing.T) {
	cid := "bafkreidtpdyu4mrdvq4mdvmsauf7gveyfjfw5txhxxs6rroeqg5kg2qlg4"
	got := body(t, "![A painting](https://weslandia.ca/xrpc/com.atproto.sync.getBlob?did=did%3Aplc%3Aogl52fc2rdooori2ayf74can&cid="+cid+")\n\n"+
		"[download](https://weslandia.ca/xrpc/com.atproto.sync.getBlob?did="+ownerDID+"&cid="+cid+")\n")
	assertContains(t, got, `<img src="/media/`+cid+`" alt="A painting">`, `<a href="/media/`+cid+`">download</a>`)
}

func TestArticleBodyLeavesOtherURLs(t *testing.T) {
	other := "https://weslandia.ca/xrpc/com.atproto.sync.getBlob?did=did%3Aplc%3Asomeoneelse&cid=bafkreiother"
	elsewhere := "https://example.com/xrpc/com.atproto.sync.getBlob?did=did%3Aplc%3Aogl52fc2rdooori2ayf74can&cid=bafkreiother"
	got := body(t, "![x]("+other+")\n\n![y]("+elsewhere+")\n\n[plain](https://example.com/page)\n")
	if strings.Contains(got, "/media/") {
		t.Errorf("rewrote a URL that isn't the owner's blob on their PDS:\n%s", got)
	}
	assertContains(t, got, `href="https://example.com/page"`)
}

func TestArticleBodyCallout(t *testing.T) {
	got := body(t, "> [!note] Heads up\n> The body text.\n>\n> Second paragraph.\n")
	assertContains(t, got,
		`<div class="callout callout-note">`,
		`<p class="callout-title">Heads up</p>`,
		"<p>The body text.</p>",
		"<p>Second paragraph.</p>",
		"</div>",
	)
	if strings.Contains(got, "<blockquote>") || strings.Contains(got, "[!note]") {
		t.Errorf("callout still rendered as a blockquote:\n%s", got)
	}
}

func TestArticleBodyCalloutDefaultTitle(t *testing.T) {
	got := body(t, "> [!WARNING]\n> Careful.\n")
	assertContains(t, got, `<div class="callout callout-warning">`, `<p class="callout-title">Warning</p>`, "<p>Careful.</p>")
}

func TestArticleBodyCalloutTitleOnly(t *testing.T) {
	got := body(t, "> [!tip] Just a title\n")
	assertContains(t, got, `<div class="callout callout-tip">`, `<p class="callout-title">Just a title</p>`)
	if strings.Contains(got, "<p></p>") {
		t.Errorf("empty paragraph left behind:\n%s", got)
	}
}

func TestArticleBodyCalloutEscapesTitle(t *testing.T) {
	got := body(t, "> [!note] <b>Bold</b> & co\n> Body.\n")
	assertContains(t, got, `<p class="callout-title">&lt;b&gt;Bold&lt;/b&gt; &amp; co</p>`)
}

func TestArticleBodyFoldableCallout(t *testing.T) {
	got := body(t, "> [!faq]- Folded\n> Answer.\n")
	assertContains(t, got, `<div class="callout callout-faq">`, `<p class="callout-title">Folded</p>`)
}

func TestArticleBodyPlainBlockquote(t *testing.T) {
	got := body(t, "> Just a quote.\n")
	assertContains(t, got, "<blockquote>", "<p>Just a quote.</p>")
	if strings.Contains(got, "callout") {
		t.Errorf("plain blockquote became a callout:\n%s", got)
	}
}

func TestArticleBodyNestedCallout(t *testing.T) {
	got := body(t, "- item\n\n  > [!info] Inside a list\n  > Text.\n")
	assertContains(t, got, `<div class="callout callout-info">`, `<p class="callout-title">Inside a list</p>`)
}

func TestArticleBodyMathStaysLiteral(t *testing.T) {
	got := body(t, "Euler: $e^{i\\pi} + 1 = 0$\n")
	assertContains(t, got, "$e^{i\\pi} + 1 = 0$")
}

func TestArticleBodyFallsBackToTextContent(t *testing.T) {
	doc := pds.Document{
		Content:     jsontext.Value(`{"$type":"com.example.richtext","blocks":[]}`),
		TextContent: "First <para>.\n\nSecond para.",
	}
	got, err := ArticleBody(doc.Markdown(), doc.TextContent, opts)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "<p>First &lt;para&gt;.</p>\n<p>Second para.</p>\n" {
		t.Errorf("fallback = %q", got)
	}
	noContent, _ := ArticleBody(nil, "Only text.", opts)
	if string(noContent) != "<p>Only text.</p>\n" {
		t.Errorf("no content = %q", noContent)
	}
}
