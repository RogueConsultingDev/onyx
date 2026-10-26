// Package format turns Go source into its canonical onyx layout.
package format

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/RogueConsultingDev/onyx/internal/config"
	"github.com/RogueConsultingDev/onyx/internal/doc"
)

// ErrUnsupported is returned when the source holds a construct onyx cannot
// lay out without losing information, such as a comment in the middle of an
// expression.
var ErrUnsupported = errors.New("unsupported construct")

type unsupported struct{ err error }

// Source formats a Go source file.
func Source(src []byte, filename string, settings config.Settings) ([]byte, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(
		fset,
		filename,
		src,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", filename, err)
	}

	return File(fset, file, src, settings)
}

// File formats a parsed file whose comments were parsed. The source is the
// text the file was parsed from.
func File(
	fset *token.FileSet, file *ast.File, src []byte, settings config.Settings,
) (out []byte, err error) {
	state := &printer{fset: fset, settings: settings, src: src}

	for _, group := range file.Comments {
		state.comments = append(state.comments, group.List...)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		failure, ok := recovered.(unsupported)
		if !ok {
			panic(recovered)
		}

		out, err = nil, failure.err
	}()

	root := state.file(file)

	text := doc.Print(root, state.options())

	return append(bytes.TrimRight([]byte(text), "\n"), '\n'), nil
}

type printer struct {
	fset     *token.FileSet
	settings config.Settings
	comments []*ast.Comment
	src      []byte
}

func (p *printer) options() doc.Options {
	return doc.Options{LineLength: p.settings.LineLength, TabWidth: p.settings.TabWidth}
}

func (p *printer) unsupportedf(pos token.Pos, format string, args ...any) unsupported {
	where := p.fset.Position(pos)
	message := fmt.Sprintf(format, args...)

	return unsupported{fmt.Errorf("%w at %s: %s", ErrUnsupported, where, message)}
}

func (p *printer) line(pos token.Pos) int {
	return p.fset.Position(pos).Line
}

func (p *printer) endLine(end token.Pos) int {
	return p.fset.Position(end).Line
}

// takeBefore removes and returns the comments that start before pos.
func (p *printer) takeBefore(pos token.Pos) []*ast.Comment {
	count := 0
	for count < len(p.comments) && p.comments[count].Pos() < pos {
		count++
	}

	taken := p.comments[:count]
	p.comments = p.comments[count:]

	return taken
}

// takeTrailing removes and returns the comment that follows a node ending at
// end on the same line.
func (p *printer) takeTrailing(end token.Pos, limit token.Pos) *ast.Comment {
	if len(p.comments) == 0 {
		return nil
	}

	next := p.comments[0]
	if next.Pos() < end || (limit.IsValid() && next.Pos() >= limit) ||
		p.line(next.Pos()) != p.endLine(end) {
		return nil
	}

	p.comments = p.comments[1:]

	return next
}

// checkConsumed fails when a comment is stranded inside a node that was just
// printed, because the layout has no place to put it.
func (p *printer) checkConsumed(end token.Pos) {
	if len(p.comments) > 0 && p.comments[0].Pos() < end {
		panic(p.unsupportedf(p.comments[0].Pos(), "comment inside an expression"))
	}
}

func commentDoc(comment *ast.Comment) doc.Doc {
	text := strings.ReplaceAll(comment.Text, "\r", "")
	if strings.HasPrefix(text, "//") {
		text = strings.TrimRight(text, " \t")
	}

	return doc.Verbatim(text)
}

type entry struct {
	pos, end token.Pos
	render   func() doc.Doc
	// tabs is the number of column separators before a trailing comment.
	tabs int
	// outdent places the entry one indentation level to the left.
	outdent bool
}

type sequenceOptions struct {
	// blankBefore decides whether entry index (>0) is preceded by a blank
	// line, given whether the source had one. The second result requests a
	// blank line only when the previous entry prints on several lines.
	blankBefore func(index int, previous doc.Doc, hadBlank bool) (blank, ifMulti bool)
}

type builder struct {
	parts []doc.Doc
}

func (b *builder) add(item doc.Doc, blank, ifMulti, outdent bool) {
	if len(b.parts) > 0 {
		var separator doc.Doc

		switch {
		case blank:
			separator = doc.Concat(doc.HardLine(), doc.HardLine())
		case ifMulti:
			separator = doc.Concat(doc.BlankIfMulti(), doc.SectionLine())
		default:
			separator = doc.SectionLine()
		}

		if outdent {
			separator = doc.Dedent(separator)
		}

		b.parts = append(b.parts, separator)
	}

	b.parts = append(b.parts, doc.Mark(), item)
}

// sequence lays out entries one per line with their comments. The caller
// provides the indentation and the surrounding braces. closing is the
// position of the token that ends the sequence. It reports whether anything
// was emitted.
func (p *printer) sequence(
	entries []entry,
	closing token.Pos,
	options sequenceOptions,
) (doc.Doc, bool) {
	var (
		out      builder
		previous doc.Doc
		prevLine int
	)

	for index, item := range entries {
		leading := p.takeBefore(item.pos)

		firstLine := p.line(item.pos)
		if len(leading) > 0 {
			firstLine = p.line(leading[0].Pos())
		}

		blank := len(out.parts) > 0 && firstLine-prevLine > 1

		ifMulti := false
		if options.blankBefore != nil && index > 0 {
			blank, ifMulti = options.blankBefore(index, previous, blank)
		}

		for position, comment := range leading {
			if position > 0 {
				blank = p.line(comment.Pos())-prevLine > 1
			}

			out.add(commentDoc(comment), blank, ifMulti, false)

			ifMulti = false
			prevLine = p.endLine(comment.End())
		}

		if len(leading) > 0 {
			blank = p.line(item.pos)-prevLine > 1
		}

		body := item.render()
		p.checkConsumed(item.end)

		full := body
		prevLine = p.endLine(item.end)

		trailing := p.takeTrailing(item.end, token.NoPos)
		if trailing != nil {
			full = doc.Concat(body, trailingDoc(trailing, item.tabs))
			prevLine = p.endLine(trailing.End())
		}

		out.add(full, blank, ifMulti, item.outdent)

		previous = body
	}

	for _, comment := range p.takeBefore(closing) {
		blank := len(out.parts) > 0 && p.line(comment.Pos())-prevLine > 1
		out.add(commentDoc(comment), blank, false, false)

		prevLine = p.endLine(comment.End())
	}

	return doc.Concat(out.parts...), len(out.parts) > 0
}

func trailingDoc(comment *ast.Comment, tabs int) doc.Doc {
	return doc.LineSuffix(
		doc.Text(strings.Repeat(doc.CellSeparator, max(tabs, 1))),
		commentDoc(comment),
	)
}

// isMultiline reports whether the document spans several lines.
func (p *printer) isMultiline(root doc.Doc) bool {
	return doc.HasForcedBreak(root) || strings.Contains(doc.Print(root, p.options()), "\n")
}
