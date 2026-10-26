// Package safety verifies that formatting did not change the meaning of a
// program and that the formatter reached a fixed point.
package safety

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

var (
	posType   = reflect.TypeFor[token.Pos]()
	groupType = reflect.TypeFor[*ast.CommentGroup]()
)

// skippedFields are derived or position-only fields that carry no meaning.
// Comments are compared separately from the file's comment list.
var skippedFields = map[string]bool{
	"Obj":        true,
	"Scope":      true,
	"Unresolved": true,
	"Imports":    true,
	"Comments":   true,
	"GoVersion":  true,
}

// Equivalent reports whether two files have the same syntax tree, ignoring
// positions, redundant parentheses, empty statements and number literal
// casing, and the same comment text in the same order.
func Equivalent(before, after *ast.File) error {
	left, right := fingerprint(before), fingerprint(after)
	if left != right {
		return fmt.Errorf("syntax tree changed:\n%s", firstDifference(left, right))
	}

	return nil
}

func fingerprint(file *ast.File) string {
	var builder strings.Builder

	dump(&builder, reflect.ValueOf(file))

	for _, group := range file.Comments {
		for _, comment := range group.List {
			builder.WriteString("\ncomment ")
			builder.WriteString(normalizeComment(comment.Text))
		}
	}

	return builder.String()
}

func normalizeComment(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " \t")
	}

	return strings.Join(lines, "\n")
}

func dump(builder *strings.Builder, value reflect.Value) {
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		dumpReference(builder, value)
	case reflect.Slice:
		builder.WriteByte('[')

		for index := range value.Len() {
			item := value.Index(index)
			if _, empty := reflect.TypeAssert[*ast.EmptyStmt](item); empty {
				continue
			}

			dump(builder, item)
			builder.WriteByte(',')
		}

		builder.WriteByte(']')
	case reflect.Struct:
		dumpStruct(builder, value)
	default:
		fmt.Fprintf(builder, "%v", value.Interface())
	}
}

func dumpReference(builder *strings.Builder, value reflect.Value) {
	// An empty result list prints as nothing, which parses back as absent.
	if fields, ok := reflect.TypeAssert[*ast.FieldList](value); ok && fields != nil &&
		len(fields.List) == 0 {
		builder.WriteString("nil")

		return
	}

	if value.IsNil() {
		builder.WriteString("nil")

		return
	}

	switch node := value.Interface().(type) {
	case *ast.ParenExpr:
		dump(builder, reflect.ValueOf(node.X))

		return
	case *ast.BasicLit:
		fmt.Fprintf(builder, "lit(%v %s)", node.Kind, literalText(node))

		return
	case *ast.BinaryExpr:
		if text, ok := foldStrings(node); ok {
			fmt.Fprintf(builder, "lit(%v %s)", token.STRING, strconv.Quote(text))

			return
		}
	}

	dump(builder, value.Elem())
}

// literalText normalizes literals: strings compare by value, so quoting style
// does not matter, and numbers compare case-insensitively.
func literalText(node *ast.BasicLit) string {
	if node.Kind == token.STRING {
		value, err := strconv.Unquote(node.Value)
		if err == nil {
			return strconv.Quote(value)
		}
	}

	return strings.ToLower(node.Value)
}

// foldStrings evaluates a concatenation made only of string literals.
func foldStrings(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case *ast.ParenExpr:
		return foldStrings(node.X)
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}

		value, err := strconv.Unquote(node.Value)

		return value, err == nil
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return "", false
		}

		left, leftOK := foldStrings(node.X)
		right, rightOK := foldStrings(node.Y)

		return left + right, leftOK && rightOK
	}

	return "", false
}

func dumpStruct(builder *strings.Builder, value reflect.Value) {
	kind := value.Type()

	builder.WriteString(kind.Name())
	builder.WriteByte('{')

	for index := range kind.NumField() {
		field := kind.Field(index)
		if skippedFields[field.Name] || field.Type == posType || field.Type == groupType {
			continue
		}

		builder.WriteString(field.Name)
		builder.WriteByte('=')
		dump(builder, value.Field(index))
		builder.WriteByte(' ')
	}

	builder.WriteByte('}')
}

func firstDifference(left, right string) string {
	limit := min(len(left), len(right))

	start := 0
	for start < limit && left[start] == right[start] {
		start++
	}

	return fmt.Sprintf("  before: …%s\n  after:  …%s", excerpt(left, start), excerpt(right, start))
}

func excerpt(text string, at int) string {
	const context = 60

	return text[max(at-context, 0):min(at+context, len(text))]
}
