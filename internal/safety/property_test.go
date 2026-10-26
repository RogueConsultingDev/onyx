package safety_test

import (
	"bytes"
	"errors"
	"fmt"
	gofmt "go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RogueConsultingDev/onyx/internal/config"
	"github.com/RogueConsultingDev/onyx/internal/format"
	"github.com/RogueConsultingDev/onyx/internal/safety"
)

// corpusPackages are standard library packages that exercise most syntax.
var corpusPackages = []string{
	"bufio",
	"bytes",
	"container/heap",
	"container/list",
	"context",
	"encoding/json",
	"errors",
	"flag",
	"fmt",
	"go/ast",
	"go/parser",
	"io",
	"path",
	"sort",
	"strconv",
	"strings",
	"sync",
	"text/template",
}

func settingsMatrix() map[string]config.Settings {
	defaults := config.Defaults()

	narrow := defaults
	narrow.LineLength = 60

	wide := defaults
	wide.LineLength = 140
	wide.TabWidth = 8

	plain := defaults
	plain.MagicTrailingComma = false

	return map[string]config.Settings{
		"defaults": defaults,
		"narrow":   narrow,
		"wide":     wide,
		"plain":    plain,
	}
}

func corpusFiles(t *testing.T) []string {
	t.Helper()

	root, err := exec.CommandContext(t.Context(), "go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("GOROOT is unavailable: %v", err)
	}

	var files []string

	for _, pkg := range corpusPackages {
		found, err := filepath.Glob(
			filepath.Join(strings.TrimSpace(string(root)), "src", pkg, "*.go"),
		)
		require.NoError(t, err)

		files = append(files, found...)
	}

	if len(files) == 0 {
		t.Skip("the standard library sources are unavailable")
	}

	return files
}

// checkProperties returns the first violated invariant of a formatting, or
// nil. safety.Source already verifies equivalence and idempotency.
func checkProperties(
	src []byte,
	name string,
	settings config.Settings,
) (supported bool, err error) {
	out, err := safety.Source(src, name, settings, false)
	if errors.Is(err, format.ErrUnsupported) {
		return false, nil
	}

	if err != nil {
		return true, err
	}

	if !strings.HasSuffix(string(out), "\n") {
		return true, errors.New("no final newline")
	}

	if settings.MagicTrailingComma {
		stable, err := gofmt.Source(out)
		if err != nil {
			return true, fmt.Errorf("output does not parse: %w", err)
		}

		if !bytes.Equal(out, stable) {
			return true, errors.New("not gofmt-stable")
		}
	}

	return true, nil
}

// maxBadPercent is the share of the corpus allowed to violate an invariant:
// trailing comment alignment is a known limitation.
const maxBadPercent = 2

func TestCorpusProperties(t *testing.T) {
	t.Parallel()

	files := corpusFiles(t)

	for label, settings := range settingsMatrix() {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			bad, total := 0, 0

			for _, file := range files {
				src, err := os.ReadFile(file)
				require.NoError(t, err)

				supported, err := checkProperties(src, filepath.Base(file), settings)
				if !supported {
					continue
				}

				total++

				if err != nil {
					bad++

					t.Logf("%s: %v", file, err)
				}
			}

			t.Logf("checked %d files, %d violations", total, bad)
			assert.LessOrEqual(t, bad*100, total*maxBadPercent)
		})
	}
}

var fuzzSeeds = []string{
	"package p\n",
	"package p\n\nvar x = 1\n",
	"package p\n\nfunc f(a, b int) (int, error) {\n\treturn a + b, nil\n}\n",
	"package p\n\nvar x = f(\n\ta,\n\tb,\n)\n",
	"package p\n\ntype T struct {\n\tA int // a\n\tB string\n}\n",
	"package p\n\nfunc f() {\n\tfor i := range 10 {\n\t\tif i%2 == 0 {\n\t\t\tcontinue\n\t\t}\n\t}\n}\n",
	"package p\n\nvar m = map[string][]int{\n\t\"a\": {1, 2},\n\t\"bb\": {3},\n}\n",
	"package p\n\nfunc f() {\n\tx.a().b().c(func() {\n\t})\n\tswitch v := y.(type) {\n\tcase int:\n\t}\n}\n",
	"package p\n\n// Doc.\nconst (\n\ta = iota // first\n\tb\n)\n",
}

func FuzzSource(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		_, err := safety.Source([]byte(src), "fuzz.go", config.Defaults(), false)
		if err == nil || errors.Is(err, format.ErrUnsupported) {
			return
		}

		// Inputs that do not parse are not interesting; anything else is a bug.
		require.NotErrorIs(t, err, safety.ErrUnsafe, "input: %q", src)
	})
}
