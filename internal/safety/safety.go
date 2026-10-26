package safety

import (
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"

	"github.com/RogueConsultingDev/onyx/internal/config"
	"github.com/RogueConsultingDev/onyx/internal/format"
)

// ErrUnsafe wraps every failed verification. Seeing it means onyx has a bug
// and the original source must be left untouched.
var ErrUnsafe = errors.New("onyx produced unsafe output")

// Source formats src and, unless fast is set, verifies that the output has
// the same syntax tree as the input and that formatting it again changes
// nothing. The output is only meaningful when the error is nil.
func Source(src []byte, filename string, settings config.Settings, fast bool) ([]byte, error) {
	out, err := format.Source(src, filename, settings)
	if err != nil {
		return nil, fmt.Errorf("error formatting %s: %w", filename, err)
	}

	if fast {
		return out, nil
	}

	return out, Verify(src, out, filename, settings)
}

// Verify checks that out is an equivalent, stable rendering of src.
func Verify(src, out []byte, filename string, settings config.Settings) error {
	fset := token.NewFileSet()

	before, err := parser.ParseFile(
		fset,
		filename,
		src,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return fmt.Errorf("error parsing %s: %w", filename, err)
	}

	after, err := parser.ParseFile(
		fset,
		filename,
		out,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return fmt.Errorf("%w: %s does not parse after formatting: %w", ErrUnsafe, filename, err)
	}

	err = Equivalent(before, after)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrUnsafe, filename, err)
	}

	again, err := format.Source(out, filename, settings)
	if err != nil {
		return fmt.Errorf("%w: %s cannot be formatted twice: %w", ErrUnsafe, filename, err)
	}

	if !bytes.Equal(out, again) {
		return fmt.Errorf("%w: %s is not idempotent", ErrUnsafe, filename)
	}

	return nil
}
