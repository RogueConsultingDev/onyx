// Package cli implements the onyx command line.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/RogueConsultingDev/onyx/internal/config"
)

// StdinPath is the path argument that selects standard input.
const StdinPath = "-"

// Options are the parsed command line arguments.
type Options struct {
	Check     bool
	Diff      bool
	Fast      bool
	Config    string
	Overrides config.Overrides
	Paths     []string
}

// ErrHelp is returned when the usage was requested.
var ErrHelp = flag.ErrHelp

// ParseArgs parses the command line arguments, without the program name.
func ParseArgs(args []string, output io.Writer) (Options, error) {
	var options Options

	flags := flag.NewFlagSet("onyx", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(output, "usage: onyx [flags] [path ...]")
		_, _ = fmt.Fprintln(output, "Paths are files or directories; \"-\" reads standard input.")

		flags.PrintDefaults()
	}

	flags.BoolVar(
		&options.Check,
		"check",
		false,
		"do not write files, exit with status 1 if any would change",
	)
	flags.BoolVar(&options.Diff, "diff", false, "print a unified diff instead of writing files")
	flags.BoolVar(
		&options.Fast,
		"fast",
		false,
		"skip the equivalence and idempotency safety checks",
	)
	flags.StringVar(
		&options.Config,
		"config",
		"",
		"path of a configuration file, replaces the discovery",
	)
	flags.Func("line-length", "maximum line length (default 100)", func(value string) error {
		number, err := parsePositive(value)
		options.Overrides.LineLength = &number

		return err
	})
	flags.Func(
		"tab-width",
		"width of a tab for measuring lines (default 4)",
		func(value string) error {
			number, err := parsePositive(value)
			options.Overrides.TabWidth = &number

			return err
		},
	)
	flags.BoolFunc(
		"magic-trailing-comma",
		"keep collections with a trailing comma exploded (default true)",
		func(value string) error {
			enabled, err := strconv.ParseBool(value)
			options.Overrides.MagicTrailingComma = &enabled

			return err
		},
	)

	err := flags.Parse(args)
	if err != nil {
		return options, err
	}

	options.Paths = flags.Args()
	if len(options.Paths) == 0 {
		return options, errors.New("no paths given")
	}

	return options, nil
}

func parsePositive(value string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", value)
	}

	if number <= 0 {
		return 0, fmt.Errorf("%d is not positive", number)
	}

	return number, nil
}
