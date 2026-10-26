// Command onyx is an uncompromising Go code formatter.
package main

import (
	"os"

	"github.com/RogueConsultingDev/onyx/internal/cli"
)

func main() {
	// notest
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
