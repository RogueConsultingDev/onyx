// Command onyx is an uncompromising Go code formatter.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/RogueConsultingDev/onyx/internal/cli"
)

var runOptions cli.Options

var rootCmd = &cobra.Command{
	Use:   "onyx [flags] <files>...",
	Short: "onyx is an uncompromising Go code formatter",
	Args:  cobra.MinimumNArgs(1),

	Run: func(cmd *cobra.Command, args []string) {
		runOptions.Paths = args
		status := cli.Run(runOptions, os.Stdin, os.Stdout, os.Stderr)
		os.Exit(status)
	},
}

func main() { // notest
	rootCmd.Flags().
		BoolVar(&runOptions.Check, "check", false, "do not write files, exit with status 2 if any would change")
	rootCmd.Flags().
		BoolVar(&runOptions.Diff, "diff", false, "print a unified diff instead of writing files")
	rootCmd.Flags().
		BoolVar(&runOptions.Fast, "fast", false, "skip the equivalence and idempotency safety checks")
	rootCmd.Flags().
		StringVar(&runOptions.Config, "config", "", "path of a configuration file, replaces the discovery")
	rootCmd.Flags().
		Func("line-length", "maximum line length (default 100)", func(s string) error {
			n, err := cli.ParsePositive(s)
			if err != nil {
				return err
			}

			runOptions.Overrides.LineLength = &n
			return nil
		})

	rootCmd.Flags().
		Func("tab-width", "width of a tab for measuring lines (default 4)", func(s string) error {
			n, err := cli.ParsePositive(s)
			if err != nil {
				return err
			}

			runOptions.Overrides.TabWidth = &n
			return nil
		})

	rootCmd.Flags().
		BoolFunc("magic-trailing-comma", "keep collections with a trailing comma exploded (default true)", func(s string) error {
			enabled := true
			if s == "false" {
				enabled = false
			}

			runOptions.Overrides.MagicTrailingComma = &enabled

			return nil
		})

	err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithColorSchemeFunc(fang.AnsiColorScheme),
	)
	if err != nil {
		os.Exit(1)
	}
}
