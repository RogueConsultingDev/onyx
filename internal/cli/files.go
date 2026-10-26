package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var generatedHeader = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// collect expands the paths into the list of Go files to format. Directories
// are walked, skipping hidden and vendor directories and generated files.
func collect(paths []string) ([]string, error) {
	var files []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", path, err)
		}

		if !info.IsDir() {
			files = append(files, path)

			continue
		}

		found, err := walk(path)
		if err != nil {
			return nil, err
		}

		files = append(files, found...)
	}

	return files, nil
}

func walk(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := entry.Name()

		if entry.IsDir() {
			if path != root && (name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}

			return nil
		}

		if filepath.Ext(name) == ".go" && !isGeneratedFile(path) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking %s: %w", root, err)
	}

	return files, nil
}

// isGeneratedFile reports whether the file starts with the standard
// generated code marker, which has to precede the package clause.
func isGeneratedFile(path string) bool {
	src, err := os.ReadFile(path) //nolint:gosec // paths come from the command line
	if err != nil {
		return false
	}

	header, _, found := strings.Cut(string(src), "\npackage ")
	if !found {
		return false
	}

	return generatedHeader.MatchString(header)
}
