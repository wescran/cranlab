package render

import (
	"bytes"
	"regexp"
	"strings"

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

// calloutMarker matches a callout's first line: [!type], an optional + or -
// (foldable), and an optional title.
var calloutMarker = regexp.MustCompile(`^\[!([A-Za-z][\w-]*)\][+-]?(?:[ \t]+(.*))?$`)

type calloutTransformer struct{}

// Transform replaces each blockquote whose first line is a callout marker
// with a Callout holding the rest of its contents. Blockquotes are collected
// before any are changed, because editing the tree while walking it skips
// nodes.
func (calloutTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var blockQuotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if bq, ok := n.(*ast.Blockquote); ok {
			blockQuotes = append(blockQuotes, bq)
		}
		return ast.WalkContinue, nil
	})
	for _, bq := range blockQuotes {
		para, ok := bq.FirstChild().(*ast.Paragraph)
		if !ok || para.Lines().Len() == 0 {
			continue
		}
		first := para.Lines().At(0)
		line := first.Value(reader.Source())
		line = bytes.TrimRight(line, "\r\n")
		matches := calloutMarker.FindSubmatch(line)
		if matches == nil {
			continue
		}
		variant, title := strings.ToLower(string(matches[1])), string(matches[2])
		if title == "" {
			title = strings.ToUpper(variant[:1]) + variant[1:]
		}
		c := &Callout{
			Variant: variant,
			Title:   title,
		}
		dropFirstLine(para)
		if para.ChildCount() == 0 {
			bq.RemoveChild(bq, para)
		}
		for child := bq.FirstChild(); child != nil; {
			next := child.NextSibling()
			c.AppendChild(c, child)
			child = next
		}
		bq.Parent().ReplaceChild(bq.Parent(), bq, c)
	}
}

// dropFirstLine removes a paragraph's inline nodes up to and including the
// end of its first line. The marker may span several text nodes.
func dropFirstLine(para *ast.Paragraph) {
	for n := para.FirstChild(); n != nil; {
		next := n.NextSibling()
		para.RemoveChild(para, n)
		if t, ok := n.(*ast.Text); ok && (t.SoftLineBreak() || t.HardLineBreak()) {
			return
		}
		n = next
	}
}

type calloutRenderer struct{}

func (calloutRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(KindCallout, renderCallout)
}

// renderCallout writes a callout as
// <div class="callout callout-<variant>"><p class="callout-title">…</p>…</div>.
func renderCallout(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	c := n.(*Callout)
	if entering {
		_, _ = w.WriteString(`<div class="callout callout-` + string(util.EscapeHTML([]byte(c.Variant))) + `">` + "\n")
		_, _ = w.WriteString(`<p class="callout-title">` + string(util.EscapeHTML([]byte(c.Title))) + "</p>\n")
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}
