package format_test

import (
	"go/format"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RogueConsultingDev/onyx/internal/config"
	onyx "github.com/RogueConsultingDev/onyx/internal/format"
)

const header = "package p\n\n"

func TestSource(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  string
	}{
		"keeps a raw tab inside a rune literal": {
			"var r = '\t'\n",
			"var r = '\t'\n",
		},
		"keeps a comment after an opening brace": {
			"func f() { // note\n\ta()\n}\n",
			"func f() { // note\n\ta()\n}\n",
		},
		"splits a call whose trailing comment overflows": {
			"var x = f(aaaaaaaaaaaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbbbbbbbb, cccccccccccccccccccccc) // note\n",
			"var x = f(\n\taaaaaaaaaaaaaaaaaaaaaaaaaaaaa,\n\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbb,\n\tcccccccccccccccccccccc,\n) // note\n", //nolint:lll // long golden literal
		},
		"moves an overflowing brace comment to the last split line": {
			"var h = func(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb int) error { // note\n\treturn nil\n}\n",        //nolint:lll // long golden literal
			"var h = func(\n\taaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb int,\n) error { // note\n\treturn nil\n}\n", //nolint:lll // long golden literal
		},
		"leaves an unsplittable line with a long comment": {
			"var x = 1 // a very long comment that cannot be split anywhere because there is no place to split it ok\n",
			"var x = 1 // a very long comment that cannot be split anywhere because there is no place to split it ok\n",
		},
		"keeps a comment after an opening parenthesis": {
			"func (c *C) HttpUrl( //nolint:staticcheck  // HttpUrl is readable, HTTPURL is not.\n\tctx context.Context,\n\tport uint16,\n) (string, error) {\n\treturn \"\", nil\n}\n", //nolint:lll // long golden literal
			"func (c *C) HttpUrl( //nolint:staticcheck  // HttpUrl is readable, HTTPURL is not.\n\tctx context.Context,\n\tport uint16,\n) (string, error) {\n\treturn \"\", nil\n}\n", //nolint:lll // long golden literal
		},
		"keeps a comment after the opening of a call": {
			"var x = f( // note\n\ta,\n\tb,\n)\n",
			"var x = f( // note\n\ta,\n\tb,\n)\n",
		},
		"keeps a comment after the opening of a literal": {
			"var x = T{ // note\n\tA: 1,\n}\n",
			"var x = T{ // note\n\tA: 1,\n}\n",
		},
		"aligns values after a leading comment": {
			"var x = T{\n\tA: 1,\n\t// note\n\tTimestamp: event.Timestamp,\n\tTraceID:   event.TraceID,\n}\n",
			"var x = T{\n\tA: 1,\n\t// note\n\tTimestamp: event.Timestamp,\n\tTraceID:   event.TraceID,\n}\n",
		},
		"collapses blank lines": {
			"func f() {\n\ta()\n\n\n\n\tb()\n}\n",
			"func f() {\n\ta()\n\n\tb()\n}\n",
		},
		"blank before return": {
			"func f() int {\n\ta()\n\treturn 1\n}\n",
			"func f() int {\n\ta()\n\n\treturn 1\n}\n",
		},
		"no blank before a leading return": {
			"func f() int {\n\n\treturn 1\n\n}\n",
			"func f() int {\n\treturn 1\n}\n",
		},
		"blank after a closing block": {
			"func f() {\n\tif a {\n\t\tb()\n\t}\n\tc()\n}\n",
			"func f() {\n\tif a {\n\t\tb()\n\t}\n\n\tc()\n}\n",
		},
		"empty statements dropped": {
			"func f() {\n\ta();\n\t;\n}\n",
			"func f() {\n\ta()\n}\n",
		},
		"empty function body": {"func f() {\n}\n", "func f() {}\n"},
		"empty control body": {
			"func f() {\n\tif a {}\n\tfor {}\n}\n",
			"func f() {\n\tif a {\n\t}\n\n\tfor {\n\t}\n}\n",
		},
		"call fits on one line": {
			"var x = f(\n\ta,\n\tb)\n",
			"var x = f(a, b)\n",
		},
		"call arguments one per line": {
			"var x = fffffffffffff(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb, ccccccccccccccccccccccc)\n",              //nolint:lll // long golden literal
			"var x = fffffffffffff(\n\taaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,\n\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb,\n\tccccccccccccccccccccccc,\n)\n", //nolint:lll // long golden literal
		},
		"magic trailing comma": {
			"var x = f(a, b,\n)\n",
			"var x = f(\n\ta,\n\tb,\n)\n",
		},
		"composite literal": {
			"var x = []int{1, 2,\n\t3}\n",
			"var x = []int{1, 2, 3}\n",
		},
		"key value alignment": {
			"var x = T{\n\ta: 1,\n\tbbb: 2,\n}\n",
			"var x = T{\n\ta:   1,\n\tbbb: 2,\n}\n",
		},
		"struct fields align": {
			"type T struct {\n\ta int\n\tbbb string `json:\"b\"`\n}\n",
			"type T struct {\n\ta   int\n\tbbb string `json:\"b\"`\n}\n",
		},
		"interface": {
			"type I interface {\n\tM()\n\tfmt.Stringer\n}\n",
			"type I interface {\n\tM()\n\tfmt.Stringer\n}\n",
		},
		"empty struct": {"type T struct {\n}\n", "type T struct{}\n"},
		"declaration groups": {
			"const (\n\ta = 1\n\tbbb = 2\n)\nvar ()\n",
			"const (\n\ta   = 1\n\tbbb = 2\n)\n\nvar ()\n",
		},
		"imports": {
			"import \"a\"\nimport \"b\"\nfunc f() {}\n",
			"import \"a\"\nimport \"b\"\n\nfunc f() {}\n",
		},
		"binary expression breaks after operators": {
			"var x = aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa + bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb + cccccccccc\n",
			"var x = aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa +\n\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb +\n\tcccccccccc\n",
		},
		"operator spacing": {
			"var x = a*b + c/d - (e + f)\nvar y = a / *p\nvar z = a & &b\nvar w = a - -b\nvar v = - -a\n",
			"var x = a*b + c/d - (e + f)\n\nvar y = a / *p\n\nvar z = a & &b\n\nvar w = a - -b\n\nvar v = - -a\n",
		},
		"nested parentheses": {
			"var x = ((a + b)) * c\n",
			"var x = (a + b) * c\n",
		},
		"method chain": {
			"var x = aaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbbbbbbb().cccccccccccccccccccc().dddddddddddddddddddddd().eeeeeeeeeeeeee()\n",             //nolint:lll // long golden literal
			"var x = aaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbbbbbbb().\n\tcccccccccccccccccccc().\n\tdddddddddddddddddddddd().\n\teeeeeeeeeeeeee()\n", //nolint:lll // long golden literal
		},
		"function signature": {
			"func f(aaaaaaaaaaaaaaaaaaaaaaaa int, bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb string, ccccccccccccccccccccc bool) (int, error) {\n\treturn 0, nil\n}\n",              //nolint:lll // long golden literal
			"func f(\n\taaaaaaaaaaaaaaaaaaaaaaaa int,\n\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb string,\n\tccccccccccccccccccccc bool,\n) (int, error) {\n\treturn 0, nil\n}\n", //nolint:lll // long golden literal
		},
		"generics": {
			"func f[T any, U comparable](a T, b U) {}\ntype L[T any] []T\n",
			"func f[T any, U comparable](a T, b U) {}\n\ntype L[T any] []T\n",
		},
		"type switch with init": {
			"func f() {\n\tswitch x := g(); v := x.(type) {\n\tcase int, string:\n\t\ta(v)\n\tdefault:\n\t}\n}\n",
			"func f() {\n\tswitch x := g(); v := x.(type) {\n\tcase int, string:\n\t\ta(v)\n\tdefault:\n\t}\n}\n",
		},
		"switch with init and no tag": {
			"func f() {\n\tswitch x := g(); {\n\tcase x > 1:\n\t}\n\tswitch {\n\t}\n}\n",
			"func f() {\n\tswitch x := g(); {\n\tcase x > 1:\n\t}\n\n\tswitch {\n\t}\n}\n",
		},
		"select": {
			"func f() {\n\tselect {\n\tcase v := <-c:\n\t\ta(v)\n\tcase c <- 1:\n\tdefault:\n\t}\n\tselect {}\n}\n",
			"func f() {\n\tselect {\n\tcase v := <-c:\n\t\ta(v)\n\tcase c <- 1:\n\tdefault:\n\t}\n\n\tselect {}\n}\n",
		},
		"labels": {
			"func f() {\nL:\n\tfor {\n\t\tbreak L\n\t}\nM:\n}\n",
			"func f() {\nL:\n\tfor {\n\t\tbreak L\n\t}\n\nM:\n}\n",
		},
		"control parentheses stripped": {
			"func f() {\n\tif (a) {\n\t}\n\tfor ((b)) {\n\t}\n\tif (T{}) == x {\n\t}\n\tfor i := 0; (i < 1); i++ {\n\t}\n}\n",
			"func f() {\n\tif a {\n\t}\n\n\tfor b {\n\t}\n\n\tif (T{}) == x {\n\t}\n\n\tfor i := 0; i < 1; i++ {\n\t}\n}\n",
		},
		"else chain": {
			"func f() {\n\tif a {\n\t\tb()\n\t} else if c {\n\t\td()\n\t} else {\n\t\te()\n\t}\n}\n",
			"func f() {\n\tif a {\n\t\tb()\n\t} else if c {\n\t\td()\n\t} else {\n\t\te()\n\t}\n}\n",
		},
		"comments": {
			"// Package p.\n\n// F does.\nfunc f() {\n\t// lead\n\ta() // trail\n\t/* block */\n}\n",
			"// Package p.\n\n// F does.\nfunc f() {\n\t// lead\n\ta() // trail\n\t/* block */\n}\n",
		},
		"raw strings and numbers": {
			"var x = `a\n\tb`\nvar y = 0XFF + 1E3 + 0B1 + 0O7\n",
			"var x = `a\n\tb`\n\nvar y = 0xFF + 1e3 + 0b1 + 0o7\n",
		},
		"statements": {
			"func f() {\n\tx++\n\tc <- 1\n\tgo g()\n\tdefer h()\n\tvar a, b = 1, 2\n\ta, b = b, a\n\tfor range c {\n\t}\n\tfor k, v := range m {\n\t\t_, _ = k, v\n\t}\n\tgoto L\n\tfunc() {}()\n\ttype T int\n}\n",     //nolint:lll // long golden literal
			"func f() {\n\tx++\n\tc <- 1\n\tgo g()\n\tdefer h()\n\tvar a, b = 1, 2\n\ta, b = b, a\n\tfor range c {\n\t}\n\n\tfor k, v := range m {\n\t\t_, _ = k, v\n\t}\n\n\tgoto L\n\tfunc() {}()\n\ttype T int\n}\n", //nolint:lll // long golden literal
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := onyx.Source([]byte(header+test.input), "in.go", config.Defaults())
			require.NoError(t, err)
			assert.Equal(t, header+test.want, string(got))

			stable, err := format.Source(got)
			require.NoError(t, err)
			assert.Equal(t, string(got), string(stable), "output is not gofmt-stable")

			again, err := onyx.Source(got, "in.go", config.Defaults())
			require.NoError(t, err)
			assert.Equal(t, string(got), string(again), "output is not idempotent")
		})
	}
}

func TestSettings(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mutate func(*config.Settings)
		input  string
		want   string
	}{
		"line length": {
			func(settings *config.Settings) { settings.LineLength = 20 },
			"var x = f(aaaaaaaaaa, bbbbbbbbbb)\n",
			"var x = f(\n\taaaaaaaaaa,\n\tbbbbbbbbbb,\n)\n",
		},
		"tab width": {
			func(settings *config.Settings) { settings.LineLength = 20; settings.TabWidth = 8 },
			"func g() {\n\tx = f(aaaaaaaa, bb)\n}\n",
			"func g() {\n\tx = f(\n\t\taaaaaaaa,\n\t\tbb,\n\t)\n}\n",
		},
		"magic trailing comma keeps a short call exploded": {
			func(*config.Settings) {},
			"var x = f(a, b,\n)\n",
			"var x = f(\n\ta,\n\tb,\n)\n",
		},
		"magic trailing comma keeps a short signature exploded": {
			func(*config.Settings) {},
			"func g(a int, b int,\n) {}\n",
			"func g(\n\ta int,\n\tb int,\n) {}\n",
		},
		"magic trailing comma disabled": {
			func(settings *config.Settings) { settings.MagicTrailingComma = false },
			"var x = f(a, b,\n)\nvar y = []int{1,\n}\n",
			"var x = f(a, b)\n\nvar y = []int{1}\n",
		},
		"tiers without magic trailing comma": {
			func(settings *config.Settings) { settings.MagicTrailingComma = false },
			"var x = fffffffffffff(aaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbb, ccccccccccccccccccccccc, dd, eeeeeeeeee)\n",
			"var x = fffffffffffff(\n\taaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbb, ccccccccccccccccccccccc, dd, eeeeeeeeee,\n)\n", //nolint:lll // long golden literal
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := config.Defaults()
			test.mutate(&settings)

			got, err := onyx.Source([]byte(header+test.input), "in.go", settings)
			require.NoError(t, err)
			assert.Equal(t, header+test.want, string(got))
		})
	}
}

func TestSourceErrors(t *testing.T) {
	t.Parallel()

	_, err := onyx.Source([]byte("package"), "bad.go", config.Defaults())
	require.ErrorContains(t, err, "error parsing bad.go")

	_, err = onyx.Source(
		[]byte(header+"var x = a + /* inside */ b\n"),
		"in.go",
		config.Defaults(),
	)
	require.ErrorIs(t, err, onyx.ErrUnsupported)
	assert.ErrorContains(t, err, "comment inside an expression")
}

// gofmt indents the line after a trailing comment on a dot far to the right;
// onyx deliberately does not.
func TestDotComments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ input, want string }{
		"call chain": {
			"func f() {\n\tlogEvt = logger.Error(). //nolint:zerologlint // manual\n\t\t\t\t\tErr(err)\n}\n",
			"func f() {\n\tlogEvt = logger.Error(). //nolint:zerologlint // manual\n\t\tErr(err)\n}\n",
		},
		"selector": {
			"func f() {\n\tx = a.b. //note\n\t\t\td()\n}\n",
			"func f() {\n\tx = a.b. //note\n\t\td()\n}\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := onyx.Source([]byte(header+test.input), "in.go", config.Defaults())
			require.NoError(t, err)
			assert.Equal(t, header+test.want, string(got))

			again, err := onyx.Source(got, "in.go", config.Defaults())
			require.NoError(t, err)
			assert.Equal(t, string(got), string(again))
		})
	}
}
