package format

import (
	"go/ast"
	"go/token"
	"math"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

type listItem struct {
	doc   doc.Doc
	blank bool
	ff    bool
	// after is a comment that follows the opening bracket on its line; only
	// the first item carries one.
	after doc.Doc
}

// listItems renders the items of a delimited list together with their
// comments. It also returns the comments that dangle before closing and
// whether the list must be broken because of comments or blank lines.
func (p *printer) listItems(
	count int,
	at func(int) ast.Node,
	render func(int) doc.Doc,
	opening, closing token.Pos,
) (items []listItem, tail []doc.Doc, forced bool) {
	prevLine := 0

	var after doc.Doc

	if count > 0 {
		if comment := p.takeTrailing(opening+1, at(0).Pos()); comment != nil {
			after = trailingDoc(comment, 1)
			prevLine = p.endLine(comment.End())
			forced = true
		}
	}

	for index := range count {
		node := at(index)
		leading := p.takeBefore(node.Pos())

		lead := make([]doc.Doc, 0, len(leading)*2)
		for _, comment := range leading {
			lead = append(lead, commentDoc(comment), doc.HardLine())
			forced = true
		}

		firstLine := p.line(node.Pos())
		if len(leading) > 0 {
			firstLine = p.line(leading[0].Pos())
		}

		blank := index > 0 && firstLine-prevLine > 1
		forced = forced || blank

		body := render(index)
		p.checkConsumed(node.End())

		prevLine = p.endLine(node.End())

		full := doc.Concat(append(lead, doc.Mark(), body)...)

		trailing := p.takeTrailing(node.End(), closing)
		if trailing != nil {
			full = doc.Concat(full, trailingDoc(trailing, 1), doc.BreakParent())
			prevLine = p.endLine(trailing.End())
			forced = true
		}

		items = append(items, listItem{doc: full, blank: blank})
	}

	if len(items) > 0 {
		items[0].after = after
	}

	for _, comment := range p.takeBefore(closing) {
		tail = append(tail, doc.HardLine(), commentDoc(comment))
		forced = true
	}

	return items, tail, forced
}

func (p *printer) exprItems(
	exprs []ast.Expr,
	opening, closing token.Pos,
	depth int,
) ([]listItem, []doc.Doc, bool) {
	return p.listItems(
		len(exprs),
		func(index int) ast.Node { return exprs[index] },
		func(index int) doc.Doc { return p.expr(exprs[index], depth) },
		opening,
		closing,
	)
}

// delimited lays out a bracketed list: on one line if it fits, else with all
// items on one indented line when tiered, else one item per line with a
// trailing comma. A magic trailing comma cannot be told apart from a tier-two
// layout, so tiers are only used when magic trailing commas are disabled.
func (p *printer) delimited(
	open, closeToken string,
	items []listItem,
	tail []doc.Doc,
	force bool,
) doc.Doc {
	if len(items) == 0 && len(tail) == 0 {
		return text(open + closeToken)
	}

	if len(items) == 0 {
		return doc.Concat(
			append(append([]doc.Doc{text(open)}, tail...), doc.FormFeed(), text(closeToken))...)
	}

	parts := make([]doc.Doc, 0, len(items)*3)

	for index, item := range items {
		if index > 0 {
			parts = append(parts, text(","), separator(item))
		}

		parts = append(parts, item.doc)
	}

	inner := doc.Concat(parts...)
	if !p.settings.MagicTrailingComma {
		inner = doc.Group(parts...)
	}

	body := []doc.Doc{
		items[0].after,
		doc.FormFeedSoftLine(),
		inner,
		doc.IfBreak(text(","), text("")),
	}

	outer := []doc.Doc{
		text(open),
		doc.Indent(append(body, tail...)...),
		doc.FormFeedSoftLine(),
		text(closeToken),
	}
	if force {
		outer = append(outer, doc.BreakParent())
	}

	return doc.Group(outer...)
}

func separator(item listItem) doc.Doc {
	switch {
	case item.blank:
		return doc.Concat(doc.HardLine(), doc.HardLine())
	case item.ff:
		return doc.FormFeedLine()
	default:
		return doc.SectionedLine()
	}
}

func docs(items []listItem) []doc.Doc {
	out := make([]doc.Doc, 0, len(items))
	for _, item := range items {
		out = append(out, item.doc)
	}

	return out
}

// hasTrailingComma reports whether magic trailing commas are enabled and the
// list ends with a comma, detected by its closing token sitting on a later
// line than the last element.
func (p *printer) hasTrailingComma(last ast.Node, closing token.Pos) bool {
	return p.settings.MagicTrailingComma && last != nil &&
		p.line(closing) > p.endLine(last.End())
}

func lastOf[T ast.Node](nodes []T) ast.Node {
	if len(nodes) == 0 {
		return nil
	}

	return nodes[len(nodes)-1]
}

func (p *printer) callArgs(call *ast.CallExpr, depth int) doc.Doc {
	argDepth := depth
	if len(call.Args) > 1 {
		argDepth++
	}

	items, tail, forced := p.exprItems(call.Args, call.Lparen, call.Rparen, argDepth)

	if call.Ellipsis.IsValid() && len(items) > 0 {
		last := &items[len(items)-1]
		last.doc = doc.Concat(last.doc, text("..."))
	}

	if !forced && p.hugsLast(call.Args, items) {
		return doc.Concat(text("("), doc.Join(text(", "), docs(items)), text(")"))
	}

	if p.hasTrailingComma(lastOf(call.Args), call.Rparen) {
		forced = true
	}

	return p.delimited("(", ")", items, tail, forced)
}

// hugsLast reports whether the last argument is a function literal or a
// non-empty composite literal that stays attached to the call, which needs
// the other arguments to be short and single-line.
func (p *printer) hugsLast(args []ast.Expr, items []listItem) bool {
	if len(args) == 0 {
		return false
	}

	switch last := args[len(args)-1].(type) {
	case *ast.FuncLit:
	case *ast.CompositeLit:
		if len(last.Elts) == 0 {
			return false
		}
	default:
		return false
	}

	prefix := 0

	for _, item := range items[:len(items)-1] {
		if doc.HasForcedBreak(item.doc) {
			return false
		}

		prefix += len(doc.Print(item.doc, doc.Options{LineLength: math.MaxInt32})) + 2
	}

	return prefix <= p.settings.LineLength/2
}

func (p *printer) composite(node *ast.CompositeLit) doc.Doc {
	typeDoc := doc.Concat()
	if node.Type != nil {
		typeDoc = p.expr(node.Type, 1)
	}

	sizes := make([]int, len(node.Elts))
	items, tail, forced := p.listItems(
		len(node.Elts),
		func(index int) ast.Node { return node.Elts[index] },
		func(index int) doc.Doc { return p.element(node.Elts[index], len(node.Elts) > 1, &sizes[index]) },
		node.Lbrace,
		node.Rbrace,
	)

	alignSections(items, sizes)

	if p.hasTrailingComma(lastOf(node.Elts), node.Rbrace) {
		forced = true
	}

	return doc.Concat(typeDoc, p.delimited("{", "}", items, tail, forced))
}

// element prints a composite literal element. For single-line key-value
// pairs it records the key size so that values can be aligned.
func (p *printer) element(node ast.Expr, aligned bool, size *int) doc.Doc {
	pair, ok := node.(*ast.KeyValueExpr)
	if !ok {
		plain := p.expr(node, 1)
		if !p.isMultiline(plain) {
			*size = len([]rune(doc.Print(plain, doc.Options{LineLength: math.MaxInt32})))
		}

		return plain
	}

	keyDoc, valueDoc := p.expr(pair.Key, 1), p.expr(pair.Value, 1)
	if !aligned || p.isMultiline(doc.Concat(keyDoc, text(": "), valueDoc)) {
		return doc.Concat(keyDoc, text(": "), valueDoc)
	}

	*size = len([]rune(doc.Print(keyDoc, doc.Options{LineLength: math.MaxInt32})))

	return doc.Concat(keyDoc, text(":"), doc.CellFit(valueDoc))
}

// alignSections decides where alignment sections restart between elements,
// following gofmt's key size heuristic.
func alignSections(items []listItem, sizes []int) {
	const (
		smallSize = 40
		threshold = 2.5
	)

	var (
		prevSize int
		logSum   float64
		count    int
	)

	for index := range items {
		size := sizes[index]

		useFormFeed := true

		if prevSize > 0 && size > 0 {
			if prevSize <= smallSize && size <= smallSize {
				useFormFeed = false
			} else {
				ratio := float64(size) / math.Exp(logSum/float64(count))
				useFormFeed = threshold*ratio <= 1 || threshold <= ratio
			}
		}

		items[index].ff = useFormFeed

		if size > 0 {
			logSum += math.Log(float64(size))
			count++
		}

		prevSize = size
	}
}
