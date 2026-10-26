// Package config loads onyx settings from project, global and CLI sources.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/samber/lo"
)

const (
	// ProjectFileName is the name of the per-project configuration file.
	ProjectFileName = ".onyx.toml"

	DefaultLineLength = 100
	DefaultTabWidth   = 4
)

// Settings are the fully resolved formatter settings.
type Settings struct {
	LineLength         int
	TabWidth           int
	MagicTrailingComma bool
}

// Overrides is a partial set of settings; nil fields are unset.
type Overrides struct {
	LineLength         *int  `toml:"line-length"`
	TabWidth           *int  `toml:"tab-width"`
	MagicTrailingComma *bool `toml:"magic-trailing-comma"`
}

// Defaults returns the built-in settings.
func Defaults() Settings {
	return Settings{
		LineLength:         DefaultLineLength,
		TabWidth:           DefaultTabWidth,
		MagicTrailingComma: true,
	}
}

// Apply returns the settings with every set override applied.
func (s Settings) Apply(overrides Overrides) Settings {
	s.LineLength = lo.FromPtrOr(overrides.LineLength, s.LineLength)
	s.TabWidth = lo.FromPtrOr(overrides.TabWidth, s.TabWidth)
	s.MagicTrailingComma = lo.FromPtrOr(overrides.MagicTrailingComma, s.MagicTrailingComma)

	return s
}

// Validate checks that the settings are usable.
func (s Settings) Validate() error {
	if s.LineLength <= 0 {
		return fmt.Errorf("line-length must be positive, got %d", s.LineLength)
	}

	if s.TabWidth <= 0 {
		return fmt.Errorf("tab-width must be positive, got %d", s.TabWidth)
	}

	return nil
}

// Load reads and strictly decodes a TOML configuration file.
func Load(path string) (Overrides, error) {
	var overrides Overrides

	content, err := os.ReadFile(path) //nolint:gosec // the path is a user-supplied config location
	if err != nil {
		return overrides, fmt.Errorf("error reading config %s: %w", path, err)
	}

	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()

	err = decoder.Decode(&overrides)
	if err != nil {
		return overrides, fmt.Errorf("error parsing config %s: %w", path, err)
	}

	return overrides, nil
}

// FindProjectFile walks up from the directory of filePath looking for a
// project file. The walk stops at the first directory containing go.mod,
// go.work or .git, after checking that directory. It returns "" if none is found.
func FindProjectFile(filePath string) (string, error) {
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("error resolving %s: %w", filePath, err)
	}

	dir := filepath.Dir(absolute)

	for {
		candidate := filepath.Join(dir, ProjectFileName)
		if isFile(candidate) {
			return candidate, nil
		}

		if isRoot(dir) {
			return "", nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}

		dir = parent
	}
}

// GlobalFile returns the path of the global configuration file.
func GlobalFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("error locating user config dir: %w", err)
	}

	return filepath.Join(dir, "onyx", "config.toml"), nil
}

// Resolve computes the settings for filePath: defaults, then the project
// file (or the global file if there is no project file), then cliOverrides.
// An explicit configPath replaces the file discovery.
func Resolve(filePath, configPath string, cliOverrides Overrides) (Settings, error) {
	settings := Defaults()

	path, err := discover(filePath, configPath)
	if err != nil {
		return settings, err
	}

	if path != "" {
		fileOverrides, err := Load(path)
		if err != nil {
			return settings, err
		}

		settings = settings.Apply(fileOverrides)
	}

	settings = settings.Apply(cliOverrides)

	err = settings.Validate()
	if err != nil {
		return settings, err
	}

	return settings, nil
}

func discover(filePath, configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	project, err := FindProjectFile(filePath)
	if err != nil {
		return "", err
	}

	if project != "" {
		return project, nil
	}

	global, err := GlobalFile()
	if err != nil {
		// No user config dir (e.g. HOME unset) simply means no global file.
		return "", nil //nolint:nilerr // a missing user config dir is not an error
	}

	if isFile(global) {
		return global, nil
	}

	return "", nil
}

func isRoot(dir string) bool {
	for _, marker := range []string{"go.mod", "go.work", ".git"} {
		_, err := os.Stat(filepath.Join(dir, marker))
		if err == nil {
			return true
		}
	}

	return false
}

func isFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}
