package doc_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RogueConsultingDev/onyx/internal/doc"
)

func call(args ...string) doc.Doc {
	parts := make([]doc.Doc, 0, len(args))
	for _, arg := range args {
		parts = append(parts, doc.Text(arg))
	}

	return doc.Group(
		doc.Text("f("),
		doc.Indent(doc.SoftLine(), doc.Join(doc.Concat(doc.Text(","), doc.Line()), parts)),
		doc.IfBreak(doc.Text(","), doc.Text("")),
		doc.SoftLine(),
		doc.Text(")"),
	)
}

func TestPrint(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input   doc.Doc
		options doc.Options
		want    string
	}{
		"flat": {call("a", "b"), doc.Options{LineLength: 20, TabWidth: 4}, "f(a, b)"},
		"broken": {
			call("aaaa", "bbbb"),
			doc.Options{LineLength: 8, TabWidth: 4},
			"f(\n\taaaa,\n\tbbbb,\n)",
		},
		"exact": {call("aa", "b"), doc.Options{LineLength: 8, TabWidth: 4}, "f(aa, b)"},
		"one over": {
			call("aa", "b"),
			doc.Options{LineLength: 6, TabWidth: 4},
			"f(\n\taa,\n\tb,\n)",
		},
		"hardline forces break": {
			doc.Group(
				doc.Text("{"),
				doc.Indent(doc.Line(), doc.Text("a"), doc.HardLine(), doc.Text("b")),
				doc.Line(),
				doc.Text("}"),
			),
			doc.Options{LineLength: 100, TabWidth: 4},
			"{\n\ta\n\tb\n}",
		},
		"hardline in ifbreak flat branch": {
			doc.Group(doc.IfBreak(doc.Text("x"), doc.HardLine()), doc.Text("y")),
			doc.Options{LineLength: 100, TabWidth: 4},
			"x" + "y",
		},
		"tab width counts": {
			doc.Indent(doc.HardLine(), call("aaaa", "bbbb")),
			doc.Options{LineLength: 16, TabWidth: 4},
			"\n\tf(\n\t\taaaa,\n\t\tbbbb,\n\t)",
		},
		"tab width counts fits": {
			doc.Indent(doc.HardLine(), call("aaaa", "bbbb")),
			doc.Options{LineLength: 17, TabWidth: 4},
			"\n\tf(aaaa, bbbb)",
		},
		"trailing text counts": {
			doc.Concat(call("a", "b"), doc.Text(strings.Repeat("x", 10))),
			doc.Options{LineLength: 12, TabWidth: 4},
			"f(\n\ta,\n\tb,\n)xxxxxxxxxx",
		},
		"trailing whitespace trimmed": {
			doc.Concat(doc.Text("a "), doc.HardLine(), doc.Text("b")),
			doc.Options{LineLength: 10, TabWidth: 4},
			"a\nb",
		},
		"soft line flat": {
			doc.Group(doc.Text("a"), doc.SoftLine(), doc.Text("b")),
			doc.Options{LineLength: 10, TabWidth: 4},
			"ab",
		},
		"rest hard line ends measure": {
			doc.Concat(call("a", "b"), doc.HardLine(), doc.Text("zzzzzzzzzzzzzzzz")),
			doc.Options{LineLength: 8, TabWidth: 4},
			"f(a, b)\nzzzzzzzzzzzzzzzz",
		},
		"rest ifbreak and softline in break mode": {
			doc.Concat(
				call("a", "b"),
				doc.IfBreak(doc.Text(";"), doc.Text("")),
				doc.SoftLine(),
				doc.Text("zzzzzzzzzzzzzzzz"),
			),
			doc.Options{LineLength: 8, TabWidth: 4},
			"f(a, b);\nzzzzzzzzzzzzzzzz",
		},
		"rest line in break mode": {
			doc.Concat(call("a", "b"), doc.Line(), doc.Text("zzzzzzzzzzzzzzzz")),
			doc.Options{LineLength: 8, TabWidth: 4},
			"f(a, b)\nzzzzzzzzzzzzzzzz",
		},
		"nested group with hardline in ifbreak": {
			doc.Group(doc.Group(doc.IfBreak(doc.Text("x"), doc.HardLine()))),
			doc.Options{LineLength: 8, TabWidth: 4},
			"x",
		},
		"unicode width": {
			doc.Group(doc.Text("é"), doc.Line(), doc.Text("é")),
			doc.Options{LineLength: 3, TabWidth: 4},
			"é é",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, doc.Print(test.input, test.options))
		})
	}
}
