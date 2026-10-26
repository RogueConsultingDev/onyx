package format

import (
	"go/ast"
	"go/token"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

func (p *printer) file(node *ast.File) doc.Doc {
	entries := []entry{{
		pos: node.Package,
		end: node.Name.End(),
		render: func() doc.Doc {
			return text("package " + node.Name.Name)
		},
	}}

	for _, decl := range node.Decls {
		entries = append(entries, entry{
			pos:    decl.Pos(),
			end:    decl.End(),
			render: func() doc.Doc { return p.decl(decl) },
		})
	}

	body, _ := p.sequence(entries, node.FileEnd+1, sequenceOptions{
		blankBefore: func(index int, _ doc.Doc, _ bool) (bool, bool) {
			return !isImport(node.Decls, index-1) || !isImport(node.Decls, index-2), false
		},
	})

	return body
}

func isImport(decls []ast.Decl, index int) bool {
	if index < 0 || index >= len(decls) {
		return false
	}

	gen, ok := decls[index].(*ast.GenDecl)

	return ok && gen.Tok == token.IMPORT
}

func (p *printer) decl(node ast.Decl) doc.Doc {
	switch typed := node.(type) {
	case *ast.GenDecl:
		return p.genDecl(typed)
	case *ast.FuncDecl:
		return p.funcDecl(typed)
	default:
		panic(p.unsupportedf(node.Pos(), "unexpected declaration %T", node))
	}
}

func (p *printer) funcDecl(node *ast.FuncDecl) doc.Doc {
	parts := []doc.Doc{text("func ")}

	if node.Recv != nil {
		parts = append(parts, p.fieldList(node.Recv, "(", ")"), text(" "))
	}

	parts = append(parts, text(node.Name.Name), p.signature(node.Type))

	if node.Body != nil {
		parts = append(parts, text(" "), p.block(node.Body))
	}

	return doc.Concat(parts...)
}

func (p *printer) genDecl(node *ast.GenDecl) doc.Doc {
	keyword := node.Tok.String()

	if !node.Lparen.IsValid() {
		return doc.Concat(text(keyword+" "), p.spec(node.Specs[0], false, false))
	}

	keepType := keepTypeColumn(node.Specs)
	entries := make([]entry, 0, len(node.Specs))

	for index, spec := range node.Specs {
		entries = append(entries, entry{
			pos:    spec.Pos(),
			end:    spec.End(),
			tabs:   specTabs(spec, keepType[index]),
			render: func() doc.Doc { return p.spec(spec, true, keepType[index]) },
		})
	}

	body, any := p.sequence(entries, node.Rparen, sequenceOptions{})
	if !any {
		return text(keyword + " ()")
	}

	return doc.Concat(
		text(keyword+" ("),
		doc.Indent(doc.FormFeed(), body),
		doc.FormFeed(),
		text(")"),
	)
}

func (p *printer) spec(node ast.Spec, grouped, keepType bool) doc.Doc {
	switch typed := node.(type) {
	case *ast.ImportSpec:
		return p.importSpec(typed)
	case *ast.ValueSpec:
		return p.valueSpec(typed, grouped, keepType)
	case *ast.TypeSpec:
		return p.typeSpec(typed, grouped)
	default:
		panic(p.unsupportedf(node.Pos(), "unexpected spec %T", node))
	}
}

func (p *printer) importSpec(node *ast.ImportSpec) doc.Doc {
	if node.Name == nil {
		return text(node.Path.Value)
	}

	return text(node.Name.Name + " " + node.Path.Value)
}

func (p *printer) typeSpec(node *ast.TypeSpec, grouped bool) doc.Doc {
	parts := []doc.Doc{text(node.Name.Name)}

	if node.TypeParams != nil {
		parts = append(parts, p.fieldList(node.TypeParams, "[", "]"))
	}

	if grouped {
		parts = append(parts, text(doc.CellSeparator))
	} else {
		parts = append(parts, text(" "))
	}

	if node.Assign.IsValid() {
		parts = append(parts, text("= "))
	}

	return doc.Concat(append(parts, p.expr(node.Type, 1))...)
}

// specTabs is the number of cells before a trailing comment on a value spec.
func specTabs(node ast.Spec, keepType bool) int {
	value, ok := node.(*ast.ValueSpec)
	if !ok {
		return 1
	}

	tabs := 3
	if value.Type != nil || keepType {
		tabs--
	}

	if value.Values != nil {
		tabs--
	}

	return tabs
}

func (p *printer) valueSpec(node *ast.ValueSpec, grouped, keepType bool) doc.Doc {
	parts := []doc.Doc{p.idents(node.Names)}

	showType := node.Type != nil || (grouped && keepType)

	switch {
	case !grouped && node.Type != nil:
		parts = append(parts, text(" "), p.expr(node.Type, 1))
	case grouped && showType:
		parts = append(parts, text(doc.CellSeparator))

		if node.Type != nil {
			parts = append(parts, p.expr(node.Type, 1))
		}
	}

	if node.Values != nil {
		separator := " "
		if grouped {
			separator = doc.CellSeparator
		}

		parts = append(parts, text(separator+"= "), commaList(p.exprs(node.Values, 1)))
	}

	return doc.Concat(parts...)
}

// keepTypeColumn reports, for each spec of a const or var block, whether the
// type column must be kept even if the spec has no type, as gofmt does for
// runs of specs with values.
func keepTypeColumn(specs []ast.Spec) []bool {
	keep := make([]bool, len(specs))

	populate := func(from, to int, keepType bool) {
		if keepType {
			for index := from; index < to; index++ {
				keep[index] = true
			}
		}
	}

	start, keepType := -1, false

	for index, spec := range specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		switch {
		case value.Values != nil:
			if start < 0 {
				start, keepType = index, false
			}

			if value.Type != nil {
				keepType = true
			}
		case start >= 0:
			populate(start, index, keepType)

			start = -1
		}
	}

	if start >= 0 {
		populate(start, len(specs), keepType)
	}

	return keep
}
