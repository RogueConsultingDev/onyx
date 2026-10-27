// Package cli implements the onyx command line.
package cli

import (
	"fmt"
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

func ParsePositive(value string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", value)
	}

	if number <= 0 {
		return 0, fmt.Errorf("%d is not positive", number)
	}

	return number, nil
}
