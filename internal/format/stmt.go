package format

import (
	"go/ast"
	"go/token"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

// block prints a braced statement block.
func (p *printer) block(node *ast.BlockStmt) doc.Doc {
	open := p.openLine(node.List)
	body, any := p.stmtList(node.List, node.Rbrace)
	if !any {
		return text("{}")
	}

	return doc.Concat(text("{"), doc.Indent(open, body), doc.FormFeed(), text("}"))
}

// stmtBlock prints a block of a statement; gofmt never collapses these.
func (p *printer) stmtBlock(node *ast.BlockStmt) doc.Doc {
	open := p.openLine(node.List)
	body, any := p.stmtList(node.List, node.Rbrace)
	if !any {
		return doc.Concat(text("{"), doc.FormFeed(), text("}"))
	}

	return doc.Concat(text("{"), doc.Indent(open, body), doc.FormFeed(), text("}"))
}

func (p *printer) stmtList(list []ast.Stmt, closing token.Pos) (doc.Doc, bool) {
	entries := make([]entry, 0, len(list))

	for _, stmt := range list {
		if _, empty := stmt.(*ast.EmptyStmt); empty {
			continue
		}

		entries = append(entries, entry{
			pos:     stmt.Pos(),
			end:     stmt.End(),
			outdent: isLabeled(stmt),
			render:  func() doc.Doc { return p.stmt(stmt) },
		})
	}

	kinds := make([]ast.Stmt, 0, len(entries))

	for _, stmt := range list {
		if _, empty := stmt.(*ast.EmptyStmt); !empty {
			kinds = append(kinds, stmt)
		}
	}

	return p.sequence(entries, closing, sequenceOptions{
		blankBefore: func(index int, previous doc.Doc, hadBlank bool) (bool, bool) {
			if _, isReturn := kinds[index].(*ast.ReturnStmt); isReturn {
				return true, false
			}

			return hadBlank, !hadBlank && doc.LastText(previous) == "}"
		},
	})
}

func (p *printer) stmt(node ast.Stmt) doc.Doc {
	switch typed := node.(type) {
	case *ast.ExprStmt:
		return p.expr(typed.X, 1)
	case *ast.AssignStmt:
		return p.assign(typed)
	case *ast.ReturnStmt:
		return p.returnStmt(typed)
	case *ast.BlockStmt:
		return p.stmtBlock(typed)
	case *ast.IfStmt:
		return p.ifStmt(typed)
	case *ast.ForStmt:
		return p.forStmt(typed)
	case *ast.RangeStmt:
		return p.rangeStmt(typed)
	case *ast.DeclStmt:
		return p.decl(typed.Decl)
	default:
		return p.stmt2(node)
	}
}

func (p *printer) stmt2(node ast.Stmt) doc.Doc {
	switch typed := node.(type) {
	case *ast.IncDecStmt:
		return doc.Concat(p.expr(typed.X, 1), text(typed.Tok.String()))
	case *ast.SendStmt:
		return doc.Concat(p.expr(typed.Chan, 1), text(" <- "), p.expr(typed.Value, 1))
	case *ast.GoStmt:
		return doc.Concat(text("go "), p.expr(typed.Call, 1))
	case *ast.DeferStmt:
		return doc.Concat(text("defer "), p.expr(typed.Call, 1))
	case *ast.BranchStmt:
		if typed.Label != nil {
			return text(typed.Tok.String() + " " + typed.Label.Name)
		}

		return text(typed.Tok.String())
	case *ast.LabeledStmt:
		return p.labeled(typed)
	case *ast.SwitchStmt:
		return p.switchStmt(typed)
	case *ast.TypeSwitchStmt:
		return p.typeSwitchStmt(typed)
	case *ast.SelectStmt:
		if len(typed.Body.List) == 0 &&
			(len(p.comments) == 0 || p.comments[0].Pos() > typed.Body.Rbrace) {
			return text("select {}")
		}

		return doc.Concat(text("select "), p.clauses(typed.Body))
	default:
		panic(p.unsupportedf(node.Pos(), "unexpected statement %T", node))
	}
}

func (p *printer) labeled(node *ast.LabeledStmt) doc.Doc {
	label := text(node.Label.Name + ":")

	if _, empty := node.Stmt.(*ast.EmptyStmt); empty {
		return label
	}

	return doc.Concat(label, doc.HardLine(), p.stmt(node.Stmt))
}

func (p *printer) assign(node *ast.AssignStmt) doc.Doc {
	depth := 1
	if len(node.Lhs) > 1 && len(node.Rhs) > 1 {
		depth = 2
	}

	return doc.Concat(
		commaList(p.exprs(node.Lhs, depth)),
		text(" "+node.Tok.String()+" "),
		commaList(p.exprs(node.Rhs, depth)),
	)
}

func (p *printer) returnStmt(node *ast.ReturnStmt) doc.Doc {
	if len(node.Results) == 0 {
		return text("return")
	}

	return doc.Concat(text("return "), commaList(p.exprs(node.Results, 1)))
}

// header prints the init statement and condition of a control clause.
func (p *printer) header(init ast.Stmt, cond ast.Expr) doc.Doc {
	parts := []doc.Doc{}

	if init != nil {
		parts = append(parts, p.stmt(init), text("; "))
	}

	if cond != nil {
		parts = append(parts, p.expr(stripParens(cond), 1))
	}

	return doc.Concat(parts...)
}

func (p *printer) ifStmt(node *ast.IfStmt) doc.Doc {
	parts := []doc.Doc{
		text("if "),
		p.header(node.Init, node.Cond),
		text(" "),
		p.stmtBlock(node.Body),
	}

	switch alternative := node.Else.(type) {
	case nil:
	case *ast.IfStmt:
		parts = append(parts, text(" else "), p.ifStmt(alternative))
	case *ast.BlockStmt:
		parts = append(parts, text(" else "), p.stmtBlock(alternative))
	default:
		panic(p.unsupportedf(node.Else.Pos(), "unexpected else branch %T", node.Else))
	}

	return doc.Concat(parts...)
}

func (p *printer) forStmt(node *ast.ForStmt) doc.Doc {
	parts := []doc.Doc{text("for ")}

	if node.Init == nil && node.Post == nil {
		if node.Cond != nil {
			parts = append(parts, p.expr(stripParens(node.Cond), 1), text(" "))
		}

		return doc.Concat(append(parts, p.stmtBlock(node.Body))...)
	}

	if node.Init != nil {
		parts = append(parts, p.stmt(node.Init))
	}

	parts = append(parts, text("; "))

	if node.Cond != nil {
		parts = append(parts, p.expr(stripParens(node.Cond), 1))
	}

	parts = append(parts, text(";"))

	if node.Post != nil {
		parts = append(parts, text(" "), p.stmt(node.Post))
	}

	return doc.Concat(append(parts, text(" "), p.stmtBlock(node.Body))...)
}

func (p *printer) rangeStmt(node *ast.RangeStmt) doc.Doc {
	parts := []doc.Doc{text("for ")}

	if node.Key != nil {
		keys := []doc.Doc{p.expr(node.Key, 1)}
		if node.Value != nil {
			keys = append(keys, p.expr(node.Value, 1))
		}

		parts = append(parts, doc.Join(text(", "), keys), text(" "+node.Tok.String()+" "))
	}

	return doc.Concat(
		append(parts, text("range "), p.expr(node.X, 1), text(" "), p.stmtBlock(node.Body))...)
}

func (p *printer) switchStmt(node *ast.SwitchStmt) doc.Doc {
	parts := []doc.Doc{text("switch ")}

	switch {
	case node.Tag != nil:
		parts = append(parts, p.header(node.Init, node.Tag), text(" "))
	case node.Init != nil:
		parts = append(parts, p.stmt(node.Init), text("; "))
	}

	return doc.Concat(append(parts, p.clauses(node.Body))...)
}

func (p *printer) typeSwitchStmt(node *ast.TypeSwitchStmt) doc.Doc {
	parts := []doc.Doc{text("switch ")}

	if node.Init != nil {
		parts = append(parts, p.stmt(node.Init), text("; "))
	}

	return doc.Concat(append(parts, p.stmt(node.Assign), text(" "), p.clauses(node.Body))...)
}

// clauses prints the braced list of case clauses of a switch or select.
func (p *printer) clauses(body *ast.BlockStmt) doc.Doc {
	entries := make([]entry, 0, len(body.List))

	for index, clause := range body.List {
		closing := clause.End()
		if index == len(body.List)-1 {
			closing = body.Rbrace
		}

		entries = append(entries, entry{
			pos:    clause.Pos(),
			end:    clause.End(),
			render: func() doc.Doc { return p.clause(clause, closing) },
		})
	}

	seq, any := p.sequence(entries, body.Rbrace, sequenceOptions{})
	if !any {
		return doc.Concat(text("{"), doc.FormFeed(), text("}"))
	}

	return doc.Concat(text("{"), doc.FormFeed(), seq, doc.FormFeed(), text("}"))
}

func (p *printer) clause(node ast.Stmt, closing token.Pos) doc.Doc {
	var (
		head doc.Doc
		body []ast.Stmt
	)

	switch typed := node.(type) {
	case *ast.CaseClause:
		head, body = p.caseHead(typed), typed.Body
	case *ast.CommClause:
		head, body = p.commHead(typed), typed.Body
	default:
		panic(p.unsupportedf(node.Pos(), "unexpected clause %T", node))
	}

	open := p.openLine(body)

	list, any := p.stmtList(body, closing)
	if !any {
		return head
	}

	return doc.Concat(head, doc.Indent(open, list))
}

func (p *printer) caseHead(node *ast.CaseClause) doc.Doc {
	if node.List == nil {
		return text("default:")
	}

	return doc.Concat(text("case "), commaList(p.exprs(node.List, 1)), text(":"))
}

func (p *printer) commHead(node *ast.CommClause) doc.Doc {
	if node.Comm == nil {
		return text("default:")
	}

	return doc.Concat(text("case "), p.stmt(node.Comm), text(":"))
}

// stripParens removes redundant parentheses around a control clause
// expression, unless it holds a composite literal that needs them.
func stripParens(node ast.Expr) ast.Expr {
	paren, ok := node.(*ast.ParenExpr)
	if !ok {
		return node
	}

	hasLiteral := false

	ast.Inspect(paren.X, func(inner ast.Node) bool {
		if _, isLiteral := inner.(*ast.CompositeLit); isLiteral {
			hasLiteral = true
		}

		return !hasLiteral
	})

	if hasLiteral {
		return node
	}

	return stripParens(paren.X)
}

func isLabeled(node ast.Stmt) bool {
	_, ok := node.(*ast.LabeledStmt)

	return ok
}

// openLine is the line break before the first statement of a body; a leading
// label sits one level to the left.
func (p *printer) openLine(list []ast.Stmt) doc.Doc {
	commented := len(p.comments) > 0 && len(list) > 0 && p.comments[0].Pos() < list[0].Pos()
	if len(list) > 0 && isLabeled(list[0]) && !commented {
		return doc.Dedent(doc.FormFeed())
	}

	return doc.FormFeed()
}
