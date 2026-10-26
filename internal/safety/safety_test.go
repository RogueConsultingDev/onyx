package safety_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RogueConsultingDev/onyx/internal/config"
	"github.com/RogueConsultingDev/onyx/internal/format"
	"github.com/RogueConsultingDev/onyx/internal/safety"
)

func parse(t *testing.T, src string) func() (equivalent func(string) error) {
	t.Helper()

	return func() func(string) error {
		return func(other string) error {
			fset := token.NewFileSet()

			left, err := parser.ParseFile(fset, "a.go", src, parser.ParseComments)
			require.NoError(t, err)

			right, err := parser.ParseFile(fset, "b.go", other, parser.ParseComments)
			require.NoError(t, err)

			return safety.Equivalent(left, right)
		}
	}
}

func TestEquivalent(t *testing.T) {
	t.Parallel()

	const base = "package p\n\nvar x = (1 + 2) * 0xab\n\nfunc f() {\n\t;\n\tif (x) {\n\t}\n} // done\n"

	tests := map[string]struct {
		other string
		same  bool
	}{
		"identical": {base, true},
		"layout and parens": {
			"package p\nvar x = ((1+2))*0XAB\nfunc f(){\nif x {}\n} // done\n",
			true,
		},
		"comment spacing": {
			"package p\n\nvar x = (1 + 2) * 0xab\n\nfunc f() {\n\tif (x) {\n\t}\n} // done   \n",
			true,
		},
		"operator": {
			"package p\n\nvar x = (1 + 2) + 0xab\n\nfunc f() {\n\tif (x) {\n\t}\n} // done\n",
			false,
		},
		"grouping": {
			"package p\n\nvar x = 1 + 2*0xab\n\nfunc f() {\n\tif (x) {\n\t}\n} // done\n",
			false,
		},
		"comment text": {
			"package p\n\nvar x = (1 + 2) * 0xab\n\nfunc f() {\n\tif (x) {\n\t}\n} // other\n",
			false,
		},
		"missing comment": {
			"package p\n\nvar x = (1 + 2) * 0xab\n\nfunc f() {\n\tif (x) {\n\t}\n}\n",
			false,
		},
		"literal value": {
			"package p\n\nvar x = (1 + 2) * 0xac\n\nfunc f() {\n\tif (x) {\n\t}\n} // done\n",
			false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := parse(t, base)()(test.other)
			if test.same {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, "syntax tree changed")
		})
	}
}

func TestEquivalentShortInputs(t *testing.T) {
	t.Parallel()

	err := parse(t, "package a\n")()("package b\n")

	require.ErrorContains(t, err, "before:")
}

func TestSource(t *testing.T) {
	t.Parallel()

	settings := config.Defaults()

	for _, fast := range []bool{false, true} {
		out, err := safety.Source([]byte("package p\nvar  x=1\n"), "in.go", settings, fast)
		require.NoError(t, err)
		assert.Equal(t, "package p\n\nvar x = 1\n", string(out))
	}

	_, err := safety.Source([]byte("package p\nvar x = a + /* c */ b\n"), "in.go", settings, false)
	require.ErrorIs(t, err, format.ErrUnsupported)

	_, err = safety.Source([]byte("package"), "in.go", settings, false)
	require.Error(t, err)
}

func TestVerify(t *testing.T) {
	t.Parallel()

	settings := config.Defaults()

	const (
		src           = "package p\n\nvar x = 1\n"
		unformattable = "package p\n\nvar y = a + /* c */ b\n"
	)

	require.NoError(t, safety.Verify([]byte(src), []byte(src), "in.go", settings))

	tests := map[string]struct {
		src  string
		out  string
		want string
	}{
		"source does not parse": {"package", src, "error parsing"},
		"output does not parse": {src, "package", "does not parse after formatting"},
		"meaning changed":       {src, "package p\n\nvar x = 2\n", "syntax tree changed"},
		"not idempotent":        {src, "package p\nvar  x = 1\n", "not idempotent"},
		"cannot be reformatted": {unformattable, unformattable, "cannot be formatted twice"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := safety.Verify([]byte(test.src), []byte(test.out), "in.go", settings)
			require.ErrorContains(t, err, test.want)
		})
	}

	err := safety.Verify([]byte(unformattable), []byte(unformattable), "in.go", settings)
	require.ErrorIs(t, err, safety.ErrUnsafe)
	require.ErrorIs(t, err, format.ErrUnsupported)
}
