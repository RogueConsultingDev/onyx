package format

import (
	"errors"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RogueConsultingDev/onyx/internal/config"
)

func TestStdlibCorpus(t *testing.T) {
	bad, unsupported, total := 0, 0, 0
	root := filepath.Join(goroot(t), "src")
	_ = filepath.WalkDir(root, func(name string, entry fs.DirEntry, _ error) error {
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.Contains(name, "testdata") {
			return nil
		}
		if filepath.Dir(filepath.Dir(name)) != root {
			return nil
		}
		src, _ := os.ReadFile(name)
		total++
		out, err := Source(src, name, config.Defaults())
		if err != nil {
			if errors.Is(err, ErrUnsupported) {
				unsupported++
			} else {
				bad++
				t.Logf("%s: %v", name, err)
			}
			return nil
		}
		g, err := format.Source(out)
		if err != nil || string(g) != string(out) {
			bad++
			t.Logf("%s: not gofmt-stable (%v)", name, err)
			return nil
		}
		again, err := Source(out, name, config.Defaults())
		if err != nil || string(again) != string(out) {
			bad++
			t.Logf("%s: not idempotent (%v)", name, err)
		}
		return nil
	})
	t.Logf("files=%d bad=%d unsupported=%d", total, bad, unsupported)

	if total > 0 {
		require.LessOrEqual(t, bad*100, total, "more than 1%% of the corpus is unstable")
	}
}

func goroot(t *testing.T) string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "go", "env", "GOROOT").Output()
	if err != nil {
		t.Skip("go toolchain unavailable")
	}

	return strings.TrimSpace(string(out))
}
