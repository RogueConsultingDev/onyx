package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/RogueConsultingDev/onyx/internal/config"
	"github.com/RogueConsultingDev/onyx/internal/safety"
)

// Exit statuses.
const (
	ExitOK      = 0
	ExitError   = 1
	ExitChanges = 2
)

const filePermissions = 0o644

type runner struct {
	options Options
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	changed bool
	failed  bool
}

// Run executes onyx with the given arguments and returns the exit status.
func Run(options Options, stdin io.Reader, stdout, stderr io.Writer) int {
	run := &runner{options: options, stdin: stdin, stdout: stdout, stderr: stderr}

	return run.execute()
}

func (r *runner) execute() int {
	for _, path := range r.options.Paths {
		if path == StdinPath {
			r.report(r.formatStdin())
		}
	}

	var paths []string

	for _, path := range r.options.Paths {
		if path != StdinPath {
			paths = append(paths, path)
		}
	}

	files, err := collect(paths)
	r.report(err)

	for _, file := range files {
		r.report(r.formatFile(file))
	}

	switch {
	case r.failed:
		return ExitError
	case r.changed && r.options.Check:
		return ExitChanges
	default:
		return ExitOK
	}
}

func (r *runner) report(err error) {
	if err == nil {
		return
	}

	r.failed = true

	_, _ = fmt.Fprintf(r.stderr, "error: %v\n", err)
}

func (r *runner) format(path string, src []byte) ([]byte, error) {
	settings, err := config.Resolve(path, r.options.Config, r.options.Overrides)
	if err != nil {
		return nil, fmt.Errorf("error resolving settings for %s: %w", path, err)
	}

	return safety.Source(src, path, settings, r.options.Fast)
}

func (r *runner) formatStdin() error {
	src, err := io.ReadAll(r.stdin)
	if err != nil {
		return fmt.Errorf("error reading standard input: %w", err)
	}

	out, err := r.format("<stdin>", src)
	if err != nil {
		return err
	}

	if !bytes.Equal(src, out) {
		r.changed = true
	}

	switch {
	case r.options.Diff:
		return r.printDiff("<stdin>", src, out)
	case r.options.Check:
		return nil
	default:
		_, err = r.stdout.Write(out)
		if err != nil {
			return fmt.Errorf("error writing standard output: %w", err)
		}

		return nil
	}
}

func (r *runner) formatFile(path string) error {
	src, err := os.ReadFile(path) //nolint:gosec // paths come from the command line
	if err != nil {
		return fmt.Errorf("error reading %s: %w", path, err)
	}

	out, err := r.format(path, src)
	if err != nil {
		return err
	}

	if bytes.Equal(src, out) {
		return nil
	}

	r.changed = true

	switch {
	case r.options.Diff:
		return r.printDiff(path, src, out)
	case r.options.Check:
		_, _ = fmt.Fprintf(r.stderr, "would reformat %s\n", path)

		return nil
	default:
		return r.write(path, out)
	}
}

func (r *runner) write(path string, out []byte) error {
	mode := os.FileMode(filePermissions)

	info, err := os.Stat(path)
	if err == nil {
		mode = info.Mode().Perm()
	}

	err = os.WriteFile(path, out, mode) //nolint:gosec // paths come from the command line
	if err != nil {
		return fmt.Errorf("error writing %s: %w", path, err)
	}

	_, _ = fmt.Fprintf(r.stderr, "reformatted %s\n", path)

	return nil
}

func (r *runner) printDiff(path string, src, out []byte) error {
	text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(src)),
		B:        difflib.SplitLines(string(out)),
		FromFile: path,
		ToFile:   path + " (formatted)",
		Context:  3,
	})
	if err != nil {
		return fmt.Errorf("error diffing %s: %w", path, err)
	}

	_, err = io.WriteString(r.stdout, text)
	if err != nil {
		return fmt.Errorf("error writing the diff of %s: %w", path, err)
	}

	return nil
}
