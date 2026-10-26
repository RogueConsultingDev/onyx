package format

import (
	"go/ast"
	"go/token"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

func (p *printer) fieldItems(list []*ast.Field, closing token.Pos) ([]listItem, []doc.Doc, bool) {
	return p.listItems(
		len(list),
		func(index int) ast.Node { return list[index] },
		func(index int) doc.Doc { return p.param(list[index]) },
		closing,
	)
}

func (p *printer) param(field *ast.Field) doc.Doc {
	if len(field.Names) == 0 {
		return p.expr(field.Type, 1)
	}

	return doc.Concat(p.idents(field.Names), text(" "), p.expr(field.Type, 1))
}

func (p *printer) fieldList(list *ast.FieldList, open, closeToken string) doc.Doc {
	items, tail, forced := p.fieldItems(list.List, list.Closing)

	if p.hasTrailingComma(lastOf(list.List), list.Closing) {
		forced = true
	}

	return p.delimited(open, closeToken, items, tail, forced)
}

// signature prints type parameters, parameters and results.
func (p *printer) signature(node *ast.FuncType) doc.Doc {
	parts := []doc.Doc{}

	if node.TypeParams != nil {
		parts = append(parts, p.fieldList(node.TypeParams, "[", "]"))
	}

	parts = append(parts, p.fieldList(node.Params, "(", ")"))

	results := node.Results
	switch {
	case results == nil || len(results.List) == 0:
	case len(results.List) == 1 && len(results.List[0].Names) == 0:
		parts = append(parts, text(" "), p.expr(results.List[0].Type, 1))
	default:
		parts = append(parts, text(" "), p.fieldList(results, "(", ")"))
	}

	return doc.Concat(parts...)
}

func (p *printer) structType(node *ast.StructType) doc.Doc {
	entries := make([]entry, 0, len(node.Fields.List))

	for _, field := range node.Fields.List {
		entries = append(entries, entry{
			pos: field.Pos(), end: field.End(), tabs: fieldTabs(field),
			render: func() doc.Doc { return p.structField(field) },
		})
	}

	return p.braced("struct", entries, node.Fields.Closing)
}

// braced prints keyword { entries }, or keyword{} when there is nothing
// inside.
func (p *printer) braced(keyword string, entries []entry, closing token.Pos) doc.Doc {
	body, any := p.sequence(entries, closing, sequenceOptions{})
	if !any {
		return text(keyword + "{}")
	}

	return doc.Concat(
		text(keyword+" {"),
		doc.Indent(doc.FormFeed(), body),
		doc.FormFeed(),
		text("}"),
	)
}

// fieldTabs is the number of cells before a trailing comment on a struct
// field, as in gofmt.
func fieldTabs(field *ast.Field) int {
	switch {
	case field.Tag != nil:
		return 0
	case len(field.Names) > 0:
		return 1
	default:
		return 2
	}
}

// structField prints a field with gofmt's alignment cells.
func (p *printer) structField(field *ast.Field) doc.Doc {
	var parts []doc.Doc

	if len(field.Names) > 0 {
		parts = []doc.Doc{p.idents(field.Names), text(doc.CellSeparator), p.expr(field.Type, 1)}
	} else {
		parts = []doc.Doc{p.expr(field.Type, 1)}
	}

	if field.Tag != nil {
		if len(field.Names) > 0 {
			parts = append(parts, text(doc.CellSeparator))
		}

		parts = append(parts, text(doc.CellSeparator), basicLit(field.Tag))
	}

	return doc.Concat(parts...)
}

func (p *printer) interfaceType(node *ast.InterfaceType) doc.Doc {
	entries := make([]entry, 0, len(node.Methods.List))

	for _, field := range node.Methods.List {
		entries = append(entries, entry{
			pos: field.Pos(), end: field.End(),
			render: func() doc.Doc { return p.interfaceElem(field) },
		})
	}

	return p.braced("interface", entries, node.Methods.Closing)
}

func (p *printer) interfaceElem(field *ast.Field) doc.Doc {
	signature, isMethod := field.Type.(*ast.FuncType)
	if len(field.Names) == 0 || !isMethod {
		return p.expr(field.Type, 1)
	}

	return doc.Concat(text(field.Names[0].Name), p.signature(signature))
}
