package format

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RogueConsultingDev/onyx/internal/config"
)

type foreignSpec struct{ ast.Spec }

func (foreignSpec) Pos() token.Pos { return token.NoPos }

func TestUnexpectedNodes(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	newPrinter := func() *printer { return &printer{fset: fset, settings: config.Defaults()} }

	tests := map[string]struct {
		run  func(*printer)
		want string
	}{
		"declaration": {func(p *printer) { p.decl(&ast.BadDecl{}) }, "unexpected declaration"},
		"spec": {
			func(p *printer) { p.spec(foreignSpec{}, false, false) },
			"unexpected spec",
		},
		"expression": {func(p *printer) { p.expr(&ast.BadExpr{}, 1) }, "unexpected expression"},
		"statement":  {func(p *printer) { p.stmt(&ast.BadStmt{}) }, "unexpected statement"},
		"clause": {
			func(p *printer) { p.clause(&ast.BadStmt{}, token.NoPos) },
			"unexpected clause",
		},
		"else branch": {
			func(p *printer) {
				p.ifStmt(
					&ast.IfStmt{Cond: &ast.Ident{}, Body: &ast.BlockStmt{}, Else: &ast.BadStmt{}},
				)
			},
			"unexpected else branch",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recovered := func() (value any) {
				defer func() { value = recover() }()

				test.run(newPrinter())

				return nil
			}()

			failure, ok := recovered.(unsupported)
			require.True(t, ok, "recovered %v", recovered)
			require.ErrorIs(t, failure.err, ErrUnsupported)
			assert.ErrorContains(t, failure.err, test.want)
		})
	}
}

func TestFileRepanicsForeignPanics(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "in.go", "package p\n", parser.ParseComments)
	require.NoError(t, err)

	file.Name = nil

	assert.Panics(t, func() { _, _ = File(fset, file, nil, config.Defaults()) })
}
