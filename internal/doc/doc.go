// Package doc implements a Wadler/Prettier-style document IR and its printer.
package doc

import (
	"bytes"
	"slices"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
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
	kindVerbatim
	kindLineSuffix
	kindBreakParent
	kindFormFeed
	kindDedent
	kindMark
	kindCellFit
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

// Verbatim is source text that may contain newlines and tabs and must be
// emitted unchanged. Only its first line counts for fitting.
func Verbatim(text string) Doc { return Doc{kind: kindVerbatim, text: text} }

// LineSuffix defers its content to the end of the current line. It does not
// count for fitting.
func LineSuffix(docs ...Doc) Doc { return Doc{kind: kindLineSuffix, children: docs} }

// BreakParent prints nothing but forces every enclosing group to break.
func BreakParent() Doc { return Doc{kind: kindBreakParent} }

// FormFeed is a HardLine that also ends the alignment section.
func FormFeed() Doc { return Doc{kind: kindHardLine, text: "\f"} }

// BlankIfMulti emits an empty line when the item that started at the last
// Mark printed on several lines. It must be followed by a line break.
func BlankIfMulti() Doc { return Doc{kind: kindHardLine, text: blankIfMulti} }

// Mark records the current line so that a later SectionLine can tell whether
// anything printed since then spans several lines.
func Mark() Doc { return Doc{kind: kindMark} }

// SectionedLine is a Line that, when broken, also ends the alignment section
// if a line break was printed since the last Mark.
func SectionedLine() Doc { return Doc{kind: kindLine, text: sectionLine} }

// SectionLine is a HardLine that also ends the alignment section if a line
// break was printed since the last Mark.
func SectionLine() Doc { return Doc{kind: kindHardLine, text: sectionLine} }

const (
	sectionLine  = "section"
	blankIfMulti = "blank-if-multi"
)

// FormFeedLine is a Line that also ends the alignment section when broken.
func FormFeedLine() Doc { return Doc{kind: kindFormFeed} }

// FormFeedSoftLine is a SoftLine that also ends the alignment section when
// broken.
func FormFeedSoftLine() Doc { return Doc{kind: kindSoftLine, text: "\f"} }

// Dedent removes one indentation level from the newlines inside docs.
func Dedent(docs ...Doc) Doc { return Doc{kind: kindDedent, children: docs} }

// CellFit prints docs after an alignment column separator when the enclosing
// group is broken and the docs fit on the rest of the line. Otherwise it
// prints a single space.
func CellFit(docs ...Doc) Doc { return Doc{kind: kindCellFit, children: docs} }

// CellSeparator is the raw alignment column separator.
const CellSeparator = "\v"

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

const escape = tabwriter.Escape

type printer struct {
	out      []byte
	column   int
	suffixes []command
	options  Options
	lines    int
	marked   int
}

// Print renders the document. Indentation uses tabs; a tab counts for
// TabWidth columns. Cells separated by CellSeparator are aligned like gofmt
// does. Trailing whitespace is trimmed from every line.
func Print(root Doc, options Options) string {
	state := &printer{options: options}
	stack := []command{{indent: 0, mode: modeBreak, doc: root}}

	for len(stack) > 0 {
		cmd := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		stack = state.step(cmd, stack)
	}

	state.flushSuffixes()

	return align(state.out)
}

func (p *printer) step(cmd command, stack []command) []command {
	switch cmd.doc.kind {
	case kindConcat:
		return pushAll(stack, cmd.indent, cmd.mode, cmd.doc.children)
	case kindIndent:
		return pushAll(stack, cmd.indent+1, cmd.mode, cmd.doc.children)
	case kindDedent:
		return pushAll(stack, max(cmd.indent-1, 0), cmd.mode, cmd.doc.children)
	case kindLineSuffix:
		p.suffixes = append(p.suffixes, command{cmd.indent, cmd.mode, Concat(cmd.doc.children...)})
	case kindIfBreak:
		return append(stack, command{cmd.indent, cmd.mode, pick(cmd)})
	case kindGroup:
		next := modeBreak

		candidate := command{cmd.indent, modeFlat, Concat(cmd.doc.children...)}
		if !hasForcedBreak(cmd.doc) && fits(candidate, stack, p.options.LineLength-p.column) {
			next = modeFlat
		}

		return append(stack, command{cmd.indent, next, Concat(cmd.doc.children...)})
	case kindCellFit:
		return p.cellFit(cmd, stack)
	default:
		p.leaf(cmd)
	}

	return stack
}

// leaf prints documents that have no children to expand.
func (p *printer) leaf(cmd command) {
	switch cmd.doc.kind {
	case kindText:
		p.out = append(p.out, cmd.doc.text...)
		p.column += width(cmd.doc.text)
	case kindVerbatim:
		p.out = append(p.out, escape)
		p.out = append(p.out, cmd.doc.text...)
		p.out = append(p.out, escape)

		p.lines += strings.Count(cmd.doc.text, "\n")

		if last := strings.LastIndexByte(cmd.doc.text, '\n'); last >= 0 {
			p.column = width(cmd.doc.text[last+1:])
		} else {
			p.column += width(cmd.doc.text)
		}
	case kindLine, kindSoftLine, kindFormFeed:
		if cmd.mode == modeFlat {
			if cmd.doc.kind != kindSoftLine {
				p.out = append(p.out, ' ')
				p.column++
			}

			return
		}

		p.newline(cmd.indent, p.endsSection(cmd.doc))
	case kindHardLine:
		if cmd.doc.text == blankIfMulti {
			if p.lines != p.marked {
				p.newline(cmd.indent, false)
			}

			return
		}

		p.newline(
			cmd.indent,
			cmd.doc.text == "\f" || (cmd.doc.text == sectionLine && p.lines != p.marked),
		)
	case kindMark:
		p.marked = p.lines
	default:
	}
}

// endsSection reports whether breaking the line also ends the alignment
// section.
func (p *printer) endsSection(line Doc) bool {
	return line.kind == kindFormFeed || line.text == "\f" ||
		(line.text == sectionLine && p.lines != p.marked)
}

func (p *printer) cellFit(cmd command, stack []command) []command {
	body := Concat(cmd.doc.children...)
	separator := " "

	if cmd.mode == modeBreak && !spansLines(body) &&
		(!canBreak(body) || fits(command{cmd.indent, modeFlat, body}, stack, p.options.LineLength-p.column-1)) {
		separator = CellSeparator
	}

	p.out = append(p.out, separator...)
	p.column++

	return append(stack, command{cmd.indent, cmd.mode, body})
}

func pick(cmd command) Doc {
	if cmd.mode == modeBreak {
		return *cmd.doc.broken
	}

	return *cmd.doc.flat
}

func (p *printer) flushSuffixes() {
	pending := p.suffixes
	p.suffixes = nil

	stack := []command{}
	for _, suffix := range slices.Backward(pending) {
		stack = append(stack, suffix)
	}

	for len(stack) > 0 {
		cmd := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		stack = p.step(cmd, stack)
	}
}

func (p *printer) newline(indent int, formFeed bool) {
	p.flushSuffixes()

	end := len(p.out)
	for end > 0 && (p.out[end-1] == ' ' || p.out[end-1] == '\t') {
		end--
	}

	p.out = p.out[:end]

	if formFeed {
		p.out = append(p.out, '\f')
	} else {
		p.out = append(p.out, '\n')
	}

	for range indent {
		p.out = append(p.out, '\t')
	}

	p.column = indent * p.options.TabWidth
	p.lines++
}

func align(raw []byte) string {
	var out bytes.Buffer

	writer := tabwriter.NewWriter(
		&out, 0, tabWidth, 1, ' ',
		tabwriter.DiscardEmptyColumns|tabwriter.TabIndent|tabwriter.StripEscape,
	)

	// A bytes.Buffer never fails, so neither does the tabwriter on top of it.
	_, _ = writer.Write(raw)
	_ = writer.Flush()

	return out.String()
}

// tabWidth is the tab width used by the alignment pass, as in gofmt.
const tabWidth = 8

func pushAll(stack []command, indent int, mode mode, docs []Doc) []command {
	for _, item := range slices.Backward(docs) {
		stack = append(stack, command{indent, mode, item})
	}

	return stack
}

func width(text string) int {
	return utf8.RuneCountInString(text)
}

// canBreak reports whether the document holds a line that may break.
func canBreak(root Doc) bool {
	switch root.kind {
	case kindLine, kindSoftLine, kindFormFeed, kindHardLine:
		return true
	case kindIfBreak:
		return canBreak(*root.flat) || canBreak(*root.broken)
	case kindLineSuffix:
		return false
	default:
		return slices.ContainsFunc(root.children, canBreak)
	}
}

// spansLines reports whether printing the document always spans lines,
// including through raw multi-line text.
func spansLines(root Doc) bool {
	switch root.kind {
	case kindVerbatim:
		return strings.Contains(root.text, "\n")
	case kindHardLine, kindBreakParent:
		return true
	case kindIfBreak:
		return spansLines(*root.flat)
	case kindLineSuffix:
		return false
	default:
		return slices.ContainsFunc(root.children, spansLines)
	}
}

// hasForcedBreak reports whether a hard line or break parent is anywhere
// inside the document.
func hasForcedBreak(root Doc) bool {
	switch root.kind {
	case kindHardLine, kindBreakParent:
		return true
	case kindIfBreak:
		return hasForcedBreak(*root.flat)
	case kindLineSuffix:
		return false
	default:
		return slices.ContainsFunc(root.children, hasForcedBreak)
	}
}

// HasForcedBreak reports whether printing the document always spans lines.
func HasForcedBreak(root Doc) bool { return hasForcedBreak(root) }

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
		case kindVerbatim:
			first, _, multiline := strings.Cut(cmd.doc.text, "\n")

			remaining -= width(first)
			if multiline {
				return remaining >= 0
			}
		case kindConcat, kindGroup, kindIndent, kindDedent, kindCellFit, kindLineSuffix:
			if cmd.doc.kind == kindCellFit {
				remaining--
			}

			work = pushAll(work, cmd.indent, cmd.mode, cmd.doc.children)
		case kindIfBreak:
			work = append(work, command{cmd.indent, cmd.mode, pick(cmd)})
		case kindLine, kindFormFeed, kindSoftLine:
			if cmd.mode == modeBreak {
				return true
			}

			if cmd.doc.kind != kindSoftLine {
				remaining--
			}
		case kindHardLine:
			return true
		case kindBreakParent, kindMark:
		}
	}

	return false
}

// LastText returns the rightmost literal text of the document, ignoring line
// suffixes and anything that prints nothing.
func LastText(root Doc) string {
	switch root.kind {
	case kindText, kindVerbatim:
		return root.text
	case kindIfBreak:
		return LastText(*root.flat)
	case kindConcat, kindGroup, kindIndent, kindDedent, kindCellFit:
		for _, child := range slices.Backward(root.children) {
			if text := LastText(child); text != "" {
				return text
			}
		}
	case kindLine,
		kindSoftLine,
		kindHardLine,
		kindLineSuffix,
		kindBreakParent,
		kindFormFeed,
		kindMark:
	}

	return ""
}
