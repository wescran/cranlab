// Package render turns record content into HTML for the site's templates.
package render

import (
	"html/template"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/wescran/cranlab/internal/pds"
)

// Options configure ArticleBody.
type Options struct {
	// DID owns the articles; only its blob URLs are rewritten.
	DID string
	// PDSPublicURL is the PDS's public base URL, as it appears in getBlob URLs
	// (https://weslandia.ca).
	PDSPublicURL string
}

// ArticleBody renders a document's content as HTML.
//
// at.markpub.markdown content is GFM, rendered with goldmark. Raw HTML is
// kept (only the site owner's records are rendered), callouts become styled
// boxes, and the owner's getBlob image and link URLs become /media/<cid>.
// Any other content type falls back to the document's textContent, escaped,
// one paragraph per blank-line-separated block.
//
// Steps:
//   - Decode doc.Content's "$type" and "text"."markdown". Empty, undecodable
//     or another $type: the textContent fallback.
//   - goldmark.New with extension.GFM and the callouts extension, the
//     blobRewriter as an AST transformer (parser.WithASTTransformers with
//     util.Prioritized), and html.WithUnsafe() so raw HTML is kept.
//   - Convert into a bytes.Buffer.
func ArticleBody(doc pds.Document, o Options) (template.HTML, error) {
	panic("not implemented")
}

// blobRewriter points the owner's getBlob URLs in images and links at
// /media/<cid>.
type blobRewriter struct{ o Options }

// Transform walks the document (ast.Walk), rewriting the Destination of every
// *ast.Image and *ast.Link.
func (r blobRewriter) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	panic("not implemented")
}

// rewrite returns /media/<cid> for
// <PDSPublicURL>/xrpc/com.atproto.sync.getBlob?did=<DID>&cid=<cid> (same
// scheme and host as PDSPublicURL, that exact path, the owner's DID, a
// non-empty cid; the query may be in any order and escaped), and dest
// unchanged otherwise.
func (r blobRewriter) rewrite(dest []byte) []byte {
	panic("not implemented")
}
