// Package doc implements a Wadler/Prettier-style document IR and its printer.
package doc

import (
	"slices"
	"strings"
)

type kind int

const (
	kindText kind = iota
	kindLine
	kindSoftLine
	kindHardLine
	kindConcat
	kindGroup
	kindIndent
	kindIfBreak
)

// Doc is a node of the document IR.
type Doc struct {
	kind     kind
	text     string
	children []Doc
	broken   *Doc
	flat     *Doc
}

// Text is literal text without newlines.
func Text(text string) Doc { return Doc{kind: kindText, text: text} }

// Line is a space when flat and a newline when broken.
func Line() Doc { return Doc{kind: kindLine} }

// SoftLine is nothing when flat and a newline when broken.
func SoftLine() Doc { return Doc{kind: kindSoftLine} }

// HardLine is always a newline and forces every enclosing group to break.
func HardLine() Doc { return Doc{kind: kindHardLine} }

// Concat joins documents.
func Concat(docs ...Doc) Doc { return Doc{kind: kindConcat, children: docs} }

// Group is printed flat if it fits on the line, broken otherwise.
func Group(docs ...Doc) Doc { return Doc{kind: kindGroup, children: docs} }

// Indent indents the newlines inside docs by one level.
func Indent(docs ...Doc) Doc { return Doc{kind: kindIndent, children: docs} }

// IfBreak prints broken when the enclosing group is broken, flat otherwise.
func IfBreak(broken, flat Doc) Doc { return Doc{kind: kindIfBreak, broken: &broken, flat: &flat} }

// Join concatenates docs separated by sep.
func Join(sep Doc, docs []Doc) Doc {
	parts := make([]Doc, 0, len(docs)*2)

	for index, item := range docs {
		if index > 0 {
			parts = append(parts, sep)
		}

		parts = append(parts, item)
	}

	return Concat(parts...)
}

// Options configure the printer.
type Options struct {
	LineLength int
	TabWidth   int
}

type mode int

const (
	modeFlat mode = iota
	modeBreak
)

type command struct {
	indent int
	mode   mode
	doc    Doc
}

// Print renders the document. Indentation uses tabs; a tab counts for
// TabWidth columns. Trailing whitespace is trimmed from every line.
func Print(root Doc, options Options) string {
	var out strings.Builder

	column := 0
	stack := []command{{indent: 0, mode: modeBreak, doc: root}}

	for len(stack) > 0 {
		cmd := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch cmd.doc.kind {
		case kindText:
			out.WriteString(cmd.doc.text)

			column += width(cmd.doc.text)
		case kindConcat:
			stack = pushAll(stack, cmd.indent, cmd.mode, cmd.doc.children)
		case kindIndent:
			stack = pushAll(stack, cmd.indent+1, cmd.mode, cmd.doc.children)
		case kindIfBreak:
			chosen := *cmd.doc.flat
			if cmd.mode == modeBreak {
				chosen = *cmd.doc.broken
			}

			stack = append(stack, command{cmd.indent, cmd.mode, chosen})
		case kindGroup:
			next := modeBreak

			candidate := command{cmd.indent, modeFlat, Concat(cmd.doc.children...)}
			if !hasHardLine(cmd.doc) &&
				fits(candidate, stack, options.LineLength-column) {
				next = modeFlat
			}

			stack = append(stack, command{cmd.indent, next, Concat(cmd.doc.children...)})
		case kindLine, kindSoftLine:
			if cmd.mode == modeFlat {
				if cmd.doc.kind == kindLine {
					out.WriteByte(' ')

					column++
				}

				continue
			}

			column = newline(&out, cmd.indent, options.TabWidth)
		case kindHardLine:
			column = newline(&out, cmd.indent, options.TabWidth)
		}
	}

	return out.String()
}

func newline(out *strings.Builder, indent, tabWidth int) int {
	trimmed := strings.TrimRight(out.String(), " \t")
	out.Reset()
	out.WriteString(trimmed)
	out.WriteByte('\n')
	out.WriteString(strings.Repeat("\t", indent))

	return indent * tabWidth
}

func pushAll(stack []command, indent int, mode mode, docs []Doc) []command {
	for _, item := range slices.Backward(docs) {
		stack = append(stack, command{indent, mode, item})
	}

	return stack
}

func width(text string) int {
	return len([]rune(text))
}

// hasHardLine reports whether a hard line is anywhere inside the document.
func hasHardLine(root Doc) bool {
	switch root.kind {
	case kindHardLine:
		return true
	case kindIfBreak:
		return hasHardLine(*root.flat)
	default:
		return slices.ContainsFunc(root.children, hasHardLine)
	}
}

// fits reports whether the candidate, followed by the rest of the stack up
// to its first possible newline, fits in the remaining width.
func fits(candidate command, rest []command, remaining int) bool {
	work := []command{candidate}
	restIndex := len(rest)

	for remaining >= 0 {
		if len(work) == 0 {
			if restIndex == 0 {
				return true
			}

			restIndex--
			work = append(work, rest[restIndex])

			continue
		}

		cmd := work[len(work)-1]
		work = work[:len(work)-1]

		switch cmd.doc.kind {
		case kindText:
			remaining -= width(cmd.doc.text)
		case kindConcat, kindGroup, kindIndent:
			work = pushAll(work, cmd.indent, cmd.mode, cmd.doc.children)
		case kindIfBreak:
			chosen := *cmd.doc.flat
			if cmd.mode == modeBreak {
				chosen = *cmd.doc.broken
			}

			work = append(work, command{cmd.indent, cmd.mode, chosen})
		case kindLine:
			if cmd.mode == modeBreak {
				return true
			}

			remaining--
		case kindSoftLine:
			if cmd.mode == modeBreak {
				return true
			}
		case kindHardLine:
			return true
		}
	}

	return false
}
