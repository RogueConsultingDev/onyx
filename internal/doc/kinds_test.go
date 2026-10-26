package doc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

func TestPrintKinds(t *testing.T) {
	t.Parallel()

	options := doc.Options{LineLength: 20, TabWidth: 4}
	cell := doc.Text(doc.CellSeparator)

	tests := map[string]struct {
		input   doc.Doc
		options doc.Options
		want    string
	}{
		"verbatim single line": {
			doc.Concat(doc.Text("a "), doc.Verbatim("`x`")),
			options,
			"a `x`",
		},
		"verbatim multi line keeps content": {
			doc.Concat(doc.Verbatim("`a\n\tb`"), doc.Text(" c")),
			options,
			"`a\n\tb` c",
		},
		"verbatim fits on its first line": {
			doc.Group(
				doc.Verbatim("`a\nbbbbbbbbbbbbbbbbbbbbbbbbbbbbb`"),
				doc.Line(),
				doc.Text("c"),
			),
			options,
			"`a\nbbbbbbbbbbbbbbbbbbbbbbbbbbbbb` c",
		},
		"line suffix flushed at newline": {
			doc.Concat(
				doc.Text("a"),
				doc.LineSuffix(doc.Text(" // c")),
				doc.HardLine(),
				doc.Text("b"),
			),
			options,
			"a // c\nb",
		},
		"line suffix flushed at end": {
			doc.Concat(doc.Text("a"), doc.LineSuffix(doc.Text(" // c"))),
			options,
			"a // c",
		},
		"line suffix counted by fits": {
			doc.Group(
				doc.Text("aaaa"),
				doc.LineSuffix(doc.Text(" // long comment here")),
				doc.Line(),
				doc.Text("b"),
			),
			options,
			"aaaa // long comment here\nb",
		},
		"sectioned line breaks like a line": {
			doc.Group(doc.Text("a"), doc.SectionedLine(), doc.Text("b")),
			options,
			"a b",
		},
		"sectioned line after a multi line item": {
			doc.Concat(
				doc.Mark(), doc.Text("a"), doc.HardLine(), doc.Text("b"),
				doc.SectionedLine(), doc.Mark(), doc.Text("c"),
			),
			options,
			"a\nb\nc",
		},
		"break parent forces group": {
			doc.Group(doc.Text("a"), doc.BreakParent(), doc.Line(), doc.Text("b")),
			options,
			"a\nb",
		},
		"form feed line breaks only in break mode": {
			doc.Concat(
				doc.Group(doc.Text("a"), doc.FormFeedLine(), doc.Text("b")),
				doc.Text(" "),
				doc.Group(doc.Text("c"), doc.BreakParent(), doc.FormFeedLine(), doc.Text("d")),
			),
			options,
			"a b c\nd",
		},
		"form feed soft line": {
			doc.Concat(
				doc.Group(doc.Text("a"), doc.FormFeedSoftLine(), doc.Text("b")),
				doc.Group(doc.Text("c"), doc.BreakParent(), doc.FormFeedSoftLine(), doc.Text("d")),
			),
			options,
			"abc\nd",
		},
		"dedent": {
			doc.Indent(
				doc.Indent(
					doc.HardLine(),
					doc.Text("a"),
					doc.Dedent(doc.HardLine(), doc.Text("b")),
				),
			),
			options,
			"\n\t\ta\n\tb",
		},
		"dedent floors at zero": {
			doc.Dedent(doc.HardLine(), doc.Text("a")),
			options,
			"\na",
		},
		"columns align": {
			doc.Concat(
				doc.Text("a"), cell, doc.Text("x"), doc.HardLine(),
				doc.Text("bbb"), cell, doc.Text("y"),
			),
			options,
			"a   x\nbbb y",
		},
		"form feed ends column block": {
			doc.Concat(
				doc.Text("a"), cell, doc.Text("x"), doc.FormFeed(),
				doc.Text("bbb"), cell, doc.Text("y"),
			),
			options,
			"a x\nbbb y",
		},
		"cell fit aligns when it fits": {
			doc.Concat(
				doc.Text("a:"), doc.CellFit(doc.Text("x")), doc.HardLine(),
				doc.Text("bbb:"), doc.CellFit(doc.Text("y")),
			),
			options,
			"a:   x\nbbb: y",
		},
		"cell fit drops alignment when too long": {
			doc.Concat(
				doc.Text("a:"),
				doc.CellFit(doc.Group(
					doc.Text("xxxxxxxxxxxxxxx"), doc.Line(), doc.Text("xxxxxxxxxxxxxxx"),
				)),
				doc.HardLine(),
				doc.Text("bbb:"), doc.CellFit(doc.Text("y")),
			),
			options,
			"a: xxxxxxxxxxxxxxx\nxxxxxxxxxxxxxxx\nbbb: y",
		},
		"cell fit keeps alignment for unbreakable values": {
			doc.Concat(
				doc.Text("a:"), doc.CellFit(doc.Text("xxxxxxxxxxxxxxxxxxxxxxxxxx")), doc.HardLine(),
				doc.Text("bbb:"), doc.CellFit(doc.Text("y")),
			),
			options,
			"a:   xxxxxxxxxxxxxxxxxxxxxxxxxx\nbbb: y",
		},
		"cell fit counted by an enclosing group": {
			doc.Group(doc.Text("a:"), doc.CellFit(doc.Text("x")), doc.Line(), doc.Text("y")),
			doc.Options{LineLength: 5, TabWidth: 4},
			"a: x\ny",
		},
		"cell fit with multi-line verbatim": {
			doc.Concat(doc.Text("a:"), doc.CellFit(doc.Verbatim("`x\ny`"), doc.Verbatim("`z`"))),
			options,
			"a: `x\ny``z`",
		},
		"cell fit with multi-line value": {
			doc.Concat(doc.Text("a:"), doc.CellFit(doc.Text("x"), doc.HardLine(), doc.Text("y"))),
			options,
			"a: x\ny",
		},
		"cell fit with breakable value that fits": {
			doc.Concat(
				doc.Text("a:"), doc.CellFit(doc.Group(doc.Text("x"), doc.Line(), doc.Text("y"))),
				doc.HardLine(),
				doc.Text("bbb:"), doc.CellFit(doc.Text("z")),
			),
			options,
			"a:   x y\nbbb: z",
		},
		"cell fit with if break and line suffix": {
			doc.Concat(
				doc.Text("a:"),
				doc.CellFit(
					doc.IfBreak(doc.Text("p"), doc.Text("q")),
					doc.LineSuffix(doc.Text("//")),
				),
			),
			options,
			"a: p//",
		},
		"section line without a mark": {
			doc.Concat(doc.Text("a"), doc.SectionLine(), doc.Text("b")),
			options,
			"a\nb",
		},
		"section line after a multi line item": {
			doc.Concat(
				doc.Mark(), doc.Text("a"), doc.HardLine(), doc.Text("b"),
				doc.SectionLine(), doc.Mark(), doc.Text("c"),
			),
			options,
			"a\nb\nc",
		},
		"blank line after a multi line item": {
			doc.Concat(
				doc.Mark(), doc.Text("a"), doc.HardLine(), doc.Text("b"),
				doc.BlankIfMulti(), doc.SectionLine(), doc.Text("c"),
			),
			options,
			"a\nb\n\nc",
		},
		"no blank line after a single line item": {
			doc.Concat(
				doc.Mark(),
				doc.Text("a"),
				doc.BlankIfMulti(),
				doc.SectionLine(),
				doc.Text("c"),
			),
			options,
			"a\nc",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, doc.Print(test.input, test.options))
		})
	}
}

func TestInspect(t *testing.T) {
	t.Parallel()

	assert.True(t, doc.HasForcedBreak(doc.Concat(doc.Text("a"), doc.HardLine())))
	assert.True(t, doc.HasForcedBreak(doc.Group(doc.BreakParent())))
	assert.False(t, doc.HasForcedBreak(doc.Group(doc.Text("a"), doc.Line())))
	assert.False(t, doc.HasForcedBreak(doc.IfBreak(doc.HardLine(), doc.Text("a"))))

	assert.Equal(
		t,
		"}",
		doc.LastText(doc.Concat(doc.Text("{"), doc.Text("}"), doc.LineSuffix(doc.Text("//")))),
	)
	assert.Equal(t, "x", doc.LastText(doc.Concat(doc.Text("x"), doc.Line(), doc.Concat())))
	assert.Equal(t, "`v`", doc.LastText(doc.Verbatim("`v`")))
	assert.Equal(t, "f", doc.LastText(doc.IfBreak(doc.Text("b"), doc.Text("f"))))
	assert.Empty(t, doc.LastText(doc.Concat(doc.Line(), doc.BreakParent(), doc.Mark())))
}
