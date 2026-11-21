// Package render turns record content into HTML for the site's templates.
package render

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Options configure ArticleBody.
type Options struct {
	// DID owns the articles; only its blob URLs are rewritten.
	DID string
	// PDSPublicURL is the PDS's public base URL, as it appears in getBlob URLs
	// (https://weslandia.ca).
	PDSPublicURL string
}

// textFallback renders text as escaped paragraphs, one per blank-line-separated
// block.
func textFallback(text string) string {
	split := strings.Split(text, "\n\n")
	result := make([]string, 0, len(split))
	for _, block := range split {
		newBlock := strings.TrimSpace(block)
		if newBlock == "" {
			continue
		}
		newBlock = "<p>" + template.HTMLEscapeString(newBlock) + "</p>\n"
		result = append(result, newBlock)
	}
	return strings.Join(result, "")
}

// ArticleBody renders a document's content as HTML. at.markpub.markdown
// content is rendered as GFM with raw HTML kept (only the owner's records are
// rendered), callouts, and the owner's blob URLs pointed at /media. Any other
// content falls back to the escaped textContent.
func ArticleBody(markdown *string, textContent string, o Options) (template.HTML, error) {
	if markdown == nil {
		return template.HTML(textFallback(textContent)), nil
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, callouts{}, highlighting.NewHighlighting(
			highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
		)),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(blobRewriter{o}, 100))),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(*markdown), &buf); err != nil {
		return template.HTML(""), err
	}
	return template.HTML(buf.String()), nil
}

// blobRewriter points the owner's getBlob URLs in images and links at
// /media/<cid>.
type blobRewriter struct{ o Options }

// Transform rewrites the destinations of images and links.
func (r blobRewriter) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Image:
			n.Destination = r.rewrite(n.Destination)
		case *ast.Link:
			n.Destination = r.rewrite(n.Destination)
		}
		return ast.WalkContinue, nil
	})
}

// rewrite returns /media/<cid> for a getBlob URL of the owner's on their PDS,
// and dest unchanged otherwise.
func (r blobRewriter) rewrite(dest []byte) []byte {
	u, err := url.Parse(string(dest))
	if err != nil {
		return dest
	}
	base, err := url.Parse(r.o.PDSPublicURL)
	if err != nil {
		return dest
	}
	if u.Scheme != base.Scheme || u.Host != base.Host || u.Path != "/xrpc/com.atproto.sync.getBlob" {
		return dest
	}
	q := u.Query()
	if q.Get("did") != r.o.DID || q.Get("cid") == "" {
		return dest
	}
	return []byte(mediaURL(q.Get("cid")))
}
