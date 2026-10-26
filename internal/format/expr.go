package format

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

func text(value string) doc.Doc { return doc.Text(value) }

// commaList lays out comma-separated expressions that may continue on
// indented lines.
func commaList(items []doc.Doc) doc.Doc {
	if len(items) == 1 {
		return items[0]
	}

	rest := make([]doc.Doc, 0, len(items)*3)
	for _, item := range items[1:] {
		rest = append(rest, text(","), doc.Line(), item)
	}

	return doc.Group(doc.Indent(append([]doc.Doc{items[0]}, rest...)...))
}

func (p *printer) exprs(list []ast.Expr, depth int) []doc.Doc {
	docs := make([]doc.Doc, 0, len(list))
	for _, item := range list {
		docs = append(docs, p.expr(item, depth))
	}

	return docs
}

func (p *printer) idents(list []*ast.Ident) doc.Doc {
	names := make([]doc.Doc, 0, len(list))
	for _, name := range list {
		names = append(names, text(name.Name))
	}

	return doc.Join(text(", "), names)
}

// expr prints an expression. depth follows gofmt's convention, where it
// controls the spacing around binary operators.
func (p *printer) expr(node ast.Expr, depth int) doc.Doc {
	switch typed := node.(type) {
	case *ast.BinaryExpr:
		return p.binary(typed, cutoff(typed, depth), depth)
	case *ast.CallExpr, *ast.SelectorExpr:
		return p.chain(typed, depth)
	case *ast.Ident:
		return text(typed.Name)
	case *ast.BasicLit:
		return basicLit(typed)
	case *ast.ParenExpr:
		if _, nested := typed.X.(*ast.ParenExpr); nested {
			return p.expr(typed.X, depth)
		}

		return doc.Concat(text("("), p.expr(typed.X, reduceDepth(depth)), text(")"))
	case *ast.StarExpr:
		return doc.Concat(text("*"), p.expr(typed.X, 1))
	case *ast.UnaryExpr:
		return p.unary(typed, depth)
	default:
		return p.expr2(node, depth)
	}
}

func (p *printer) expr2(node ast.Expr, depth int) doc.Doc {
	switch typed := node.(type) {
	case *ast.CompositeLit:
		return p.composite(typed)
	case *ast.FuncLit:
		return doc.Concat(text("func"), p.signature(typed.Type), text(" "), p.block(typed.Body))
	case *ast.IndexExpr:
		return doc.Concat(
			p.expr(typed.X, depth),
			text("["),
			p.expr(typed.Index, depth+1),
			text("]"),
		)
	case *ast.IndexListExpr:
		return doc.Concat(
			p.expr(typed.X, depth),
			text("["),
			joinComma0(p.exprs(typed.Indices, depth+1)),
			text("]"),
		)
	case *ast.SliceExpr:
		return p.slice(typed, depth)
	case *ast.TypeAssertExpr:
		return p.typeAssert(typed, depth)
	default:
		return p.typeExpr(node, depth)
	}
}

func (p *printer) typeExpr(node ast.Expr, depth int) doc.Doc {
	switch typed := node.(type) {
	case *ast.ArrayType:
		return doc.Concat(p.arrayLen(typed), p.expr(typed.Elt, depth))
	case *ast.MapType:
		return doc.Concat(
			text("map["),
			p.expr(typed.Key, depth),
			text("]"),
			p.expr(typed.Value, depth),
		)
	case *ast.ChanType:
		return p.chanType(typed, depth)
	case *ast.Ellipsis:
		return p.ellipsis(typed, depth)
	case *ast.FuncType:
		return doc.Concat(text("func"), p.signature(typed))
	case *ast.StructType:
		return p.structType(typed)
	case *ast.InterfaceType:
		return p.interfaceType(typed)
	default:
		panic(p.unsupportedf(node.Pos(), "unexpected expression %T", node))
	}
}

func joinComma0(items []doc.Doc) doc.Doc { return doc.Join(text(", "), items) }

func (p *printer) arrayLen(node *ast.ArrayType) doc.Doc {
	if node.Len == nil {
		return text("[]")
	}

	return doc.Concat(text("["), p.expr(node.Len, 1), text("]"))
}

func (p *printer) ellipsis(node *ast.Ellipsis, depth int) doc.Doc {
	if node.Elt == nil {
		return text("...")
	}

	return doc.Concat(text("..."), p.expr(node.Elt, depth))
}

func (p *printer) chanType(node *ast.ChanType, depth int) doc.Doc {
	prefix := "chan "

	switch node.Dir {
	case ast.SEND:
		prefix = "chan<- "
	case ast.RECV:
		prefix = "<-chan "
	case ast.SEND | ast.RECV:
	}

	return doc.Concat(text(prefix), p.expr(node.Value, depth))
}

func (p *printer) typeAssert(node *ast.TypeAssertExpr, depth int) doc.Doc {
	target := text("type")
	if node.Type != nil {
		target = p.expr(node.Type, 1)
	}

	return doc.Concat(p.expr(node.X, depth), text(".("), target, text(")"))
}

func (p *printer) unary(node *ast.UnaryExpr, depth int) doc.Doc {
	operand := p.expr(node.X, depth)

	if inner, ok := node.X.(*ast.UnaryExpr); ok && inner.Op == node.Op &&
		(node.Op == token.ADD || node.Op == token.SUB) {
		return doc.Concat(text(node.Op.String()+" "), operand)
	}

	return doc.Concat(text(node.Op.String()), operand)
}

func (p *printer) slice(node *ast.SliceExpr, depth int) doc.Doc {
	indices := []ast.Expr{node.Low, node.High}
	if node.Slice3 {
		indices = append(indices, node.Max)
	}

	needsBlanks := false

	if depth <= 1 {
		count, hasBinary := 0, false

		for _, index := range indices {
			if index == nil {
				continue
			}

			count++

			if _, isBinary := index.(*ast.BinaryExpr); isBinary {
				hasBinary = true
			}
		}

		needsBlanks = count > 1 && hasBinary
	}

	parts := []doc.Doc{p.expr(node.X, depth), text("[")}

	for position, index := range indices {
		if position > 0 {
			if indices[position-1] != nil && needsBlanks {
				parts = append(parts, text(" "))
			}

			parts = append(parts, text(":"))

			if index != nil && needsBlanks {
				parts = append(parts, text(" "))
			}
		}

		if index != nil {
			parts = append(parts, p.expr(index, depth+1))
		}
	}

	return doc.Concat(append(parts, text("]"))...)
}

func basicLit(node *ast.BasicLit) doc.Doc {
	value := node.Value

	switch node.Kind {
	case token.INT, token.FLOAT, token.IMAG:
		value = normalizeNumber(value)
	default:
	}

	// Tabs and newlines inside literals must not be seen by the tab writer.
	if strings.ContainsAny(value, "\n\t") {
		return doc.Verbatim(strings.ReplaceAll(value, "\r", ""))
	}

	return text(value)
}

// normalizeNumber lowercases radix prefixes and exponents like gofmt does.
func normalizeNumber(value string) string {
	if len(value) < 2 {
		return value
	}

	lower := strings.ToLower(value)

	switch {
	case strings.HasPrefix(lower, "0x"):
		mantissa, exponent, found := strings.Cut(lower[2:], "p")
		if !found {
			return "0x" + strings.ToUpper(mantissa)
		}

		return "0x" + strings.ToUpper(mantissa) + "p" + exponent
	case strings.HasPrefix(lower, "0b"), strings.HasPrefix(lower, "0o"):
		return lower
	default:
		return lower
	}
}

func reduceDepth(depth int) int {
	return max(depth-1, 1)
}

func cutoff(node *ast.BinaryExpr, depth int) int {
	has4, has5, maxProblem := walkBinary(node)
	if maxProblem > 0 {
		return maxProblem + 1
	}

	if has4 && has5 {
		if depth == 1 {
			return 5
		}

		return 4
	}

	if depth == 1 {
		return 6
	}

	return 4
}

func walkBinary(node *ast.BinaryExpr) (has4, has5 bool, maxProblem int) {
	switch node.Op.Precedence() {
	case 4:
		has4 = true
	case 5:
		has5 = true
	}

	if left, ok := node.X.(*ast.BinaryExpr); ok && left.Op.Precedence() >= node.Op.Precedence() {
		left4, left5, problem := walkBinary(left)
		has4, has5, maxProblem = has4 || left4, has5 || left5, max(maxProblem, problem)
	}

	if right, ok := node.Y.(*ast.BinaryExpr); ok {
		if right.Op.Precedence() > node.Op.Precedence() {
			right4, right5, problem := walkBinary(right)
			has4, has5, maxProblem = has4 || right4, has5 || right5, max(maxProblem, problem)
		}
	} else {
		maxProblem = max(maxProblem, operandProblem(node.Op, node.Y))
	}

	return has4, has5, maxProblem
}

// operandProblem returns the precedence at which printing op next to right
// without blanks would form a different token sequence.
func operandProblem(op token.Token, right ast.Expr) int {
	switch right := right.(type) {
	case *ast.StarExpr:
		if op == token.QUO {
			return 5
		}
	case *ast.UnaryExpr:
		switch op.String() + right.Op.String() {
		case "/*", "&&", "&^":
			return 5
		case "++", "--":
			return 4
		}
	}

	return 0
}

type binaryLink struct {
	op token.Token
	y  ast.Expr
}

// binary prints a chain of binary operations of equal precedence. Lines
// break after the operators.
func (p *printer) binary(node *ast.BinaryExpr, cutoffPrec, depth int) doc.Doc {
	prec := node.Op.Precedence()
	var links []binaryLink

	var left ast.Expr = node

	for {
		binary, ok := left.(*ast.BinaryExpr)
		if !ok || binary.Op.Precedence() != prec {
			break
		}

		links = append(links, binaryLink{binary.Op, binary.Y})
		left = binary.X
	}

	blanks := prec < cutoffPrec

	if merged, ok := mergedStrings(left, links); ok {
		return p.stringChain(left, links, merged, blanks, depth)
	}

	first := p.expr(left, depth+1)

	rest := make([]doc.Doc, 0, len(links)*3)

	for _, link := range slices.Backward(links) {
		operand := p.expr(link.y, depth+1)

		if blanks {
			rest = append(rest, text(" "+link.op.String()), doc.FormFeedLine(), operand)
		} else {
			rest = append(rest, text(link.op.String()), operand)
		}
	}

	if !blanks {
		return doc.Concat(first, doc.Concat(rest...))
	}

	return doc.Group(first, doc.Indent(rest...))
}

// stringChain prints a concatenation of string literals as a single literal
// when it fits, and one literal per line otherwise.
func (p *printer) stringChain(
	left ast.Expr, links []binaryLink, merged string, blanks bool, depth int,
) doc.Doc {
	operator := "+"
	if blanks {
		operator = " +"
	}

	parts := []doc.Doc{p.expr(left, depth+1)}

	for _, link := range slices.Backward(links) {
		parts = append(parts, text(operator), doc.Indent(doc.Line(), p.expr(link.y, depth+1)))
	}

	return doc.Group(doc.IfBreak(doc.Concat(parts...), text(merged)))
}

// mergedStrings returns the source of a single literal equivalent to a
// concatenation made only of string literals.
func mergedStrings(left ast.Expr, links []binaryLink) (string, bool) {
	literals := []*ast.BasicLit{}

	if first, ok := left.(*ast.BasicLit); ok {
		literals = append(literals, first)
	}

	for _, link := range slices.Backward(links) {
		if literal, ok := link.y.(*ast.BasicLit); ok {
			literals = append(literals, literal)
		}
	}

	if len(literals) != len(links)+1 || links[0].op != token.ADD {
		return "", false
	}

	var values strings.Builder

	cooked := true

	for _, literal := range literals {
		value, err := strconv.Unquote(literal.Value)
		if literal.Kind != token.STRING || err != nil {
			return "", false
		}

		values.WriteString(value)

		cooked = cooked && !strings.HasPrefix(literal.Value, "`")
	}

	value := values.String()
	if cooked || strings.ContainsAny(value, "`\r") || !utf8.ValidString(value) {
		return strconv.Quote(value), true
	}

	return "`" + value + "`", true
}

// link is one step of a selector/call chain.
type link struct {
	selector *ast.SelectorExpr
	call     *ast.CallExpr
}

// chain prints selectors and calls. Chains of several calls break after the
// dots when they do not fit.
func (p *printer) chain(node ast.Expr, depth int) doc.Doc {
	var reversed []link

	current := node

loop:
	for {
		switch typed := current.(type) {
		case *ast.CallExpr:
			reversed = append(reversed, link{call: typed})
			current = typed.Fun
		case *ast.SelectorExpr:
			reversed = append(reversed, link{selector: typed})
			current = typed.X
		default:
			break loop
		}
	}

	links := make([]link, 0, len(reversed))
	for _, item := range slices.Backward(reversed) {
		links = append(links, item)
	}

	head := p.expr(current, depth)

	calls := 0

	firstCall := len(links)

	for index, item := range links {
		if item.call != nil {
			calls++

			firstCall = min(firstCall, index)
		}
	}

	if calls < 2 || links[firstCall+1].selector == nil {
		return doc.Concat(head, p.links(links, depth))
	}

	headLinks, rest := links[:firstCall+1], links[firstCall+1:]
	segments := p.segments(rest, depth)

	parts := make([]doc.Doc, 0, len(segments)*3)
	for _, segment := range segments {
		parts = append(parts, text("."))

		if segment.comment != nil {
			parts = append(parts, plainTrailing(segment.comment), doc.BreakParent())
		}

		parts = append(parts, doc.FormFeedSoftLine(), segment.doc)
	}

	return doc.Group(head, p.links(headLinks, depth), doc.Indent(parts...))
}

func (p *printer) links(links []link, depth int) doc.Doc {
	parts := make([]doc.Doc, 0, len(links))
	for _, item := range links {
		parts = append(parts, p.link(item, depth))
	}

	return doc.Concat(parts...)
}

func (p *printer) link(item link, depth int) doc.Doc {
	if item.selector != nil {
		comment := p.dotComment(item.selector)
		if comment == nil {
			return text("." + item.selector.Sel.Name)
		}

		return doc.Concat(
			text("."), plainTrailing(comment),
			doc.Indent(doc.HardLine()), text(item.selector.Sel.Name),
		)
	}

	return p.callArgs(item.call, depth)
}

// plainTrailing places a comment after a space instead of an alignment cell,
// so the continuation line is not pushed to the right of it.
func plainTrailing(comment *ast.Comment) doc.Doc {
	return doc.LineSuffix(text(" "), commentDoc(comment))
}

// dotComment takes the comment that follows the dot of a selector on its line.
func (p *printer) dotComment(selector *ast.SelectorExpr) *ast.Comment {
	return p.takeTrailing(selector.X.End(), selector.Sel.Pos())
}

type segment struct {
	doc     doc.Doc
	comment *ast.Comment
}

// segments groups the links after the first call by selector. The leading
// dot of each segment is not part of the document.
func (p *printer) segments(links []link, depth int) []segment {
	var (
		segments []segment
		current  []doc.Doc
		comment  *ast.Comment
	)

	for _, item := range links {
		if item.selector != nil {
			if current != nil {
				segments = append(segments, segment{doc.Concat(current...), comment})
			}

			comment = p.dotComment(item.selector)
			current = []doc.Doc{text(item.selector.Sel.Name)}

			continue
		}

		current = append(current, p.callArgs(item.call, depth))
	}

	return append(segments, segment{doc.Concat(current...), comment})
}
