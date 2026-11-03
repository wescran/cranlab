package render

import (
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindCallout is the AST node kind of a callout.
var KindCallout = ast.NewNodeKind("Callout")

// Callout is an Obsidian callout: a blockquote whose first line is
// "[!type] Title". Its children are the blockquote's contents after that line.
type Callout struct {
	ast.BaseBlock
	// Variant is the callout type in lower case ("note", "warning", …). (Not
	// "Type": ast.Node already has a Type method.)
	Variant string
	// Title is the text after the marker, or the type capitalised.
	Title string
}

// Kind implements ast.Node.
func (n *Callout) Kind() ast.NodeKind { return KindCallout }

// Dump implements ast.Node.
func (n *Callout) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Variant": n.Variant, "Title": n.Title}, nil)
}

// callouts is the goldmark extension: a transformer that turns matching
// blockquotes into Callout nodes, and a renderer for them.
type callouts struct{}

func (callouts) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(util.Prioritized(calloutTransformer{}, 100)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(calloutRenderer{}, 100)))
}

var calloutMarker = regexp.MustCompile(`^\[!([A-Za-z][\w-]*)\][+-]?(?:[ \t]+(.*))?$`)

type calloutTransformer struct{}

// Transform replaces each blockquote whose first line is a callout marker
// with a Callout holding the blockquote's other contents.
//
//   - Collect the blockquotes first (ast.Walk), then change them: editing the
//     tree while walking it skips nodes.
//   - The marker is on the first line of the blockquote's first child, which
//     must be a paragraph. Read that line from the source:
//     para.Lines().At(0).Value(reader.Source()), without its newline, and
//     match it against calloutMarker. "[!NOTE]- Title" gives type "note" and
//     title "Title"; a "+" or "-" after "]" (a foldable callout) is ignored.
//   - Without a title, use the type with its first letter upper-cased.
//   - Remove the marker line from the paragraph (dropFirstLine), and the
//     paragraph itself if nothing is left.
//   - Move the blockquote's remaining children into the Callout, in order, and
//     put the Callout where the blockquote was (ReplaceChild on its parent).
//     Callouts can be nested in lists, so use the blockquote's own parent.
func (calloutTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	panic("not implemented")
}

// dropFirstLine removes a paragraph's inline nodes up to and including the
// end of its first line: the first *ast.Text whose SoftLineBreak() or
// HardLineBreak() is true. (The marker may be split across several nodes.)
func dropFirstLine(para *ast.Paragraph) {
	panic("not implemented")
}

type calloutRenderer struct{}

func (calloutRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(KindCallout, renderCallout)
}

// renderCallout writes, escaping the variant and title (util.EscapeHTML):
//
//	<div class="callout callout-<variant>">
//	<p class="callout-title">Title</p>
//	…children (goldmark renders them between entering and leaving)…
//	</div>
//
// goldmark calls it twice per node: with entering true (write the opening
// tags) and false (write the closing tag).
func renderCallout(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	panic("not implemented")
}
