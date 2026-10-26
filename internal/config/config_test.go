package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RogueConsultingDev/onyx/internal/config"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestApplyAndValidate(t *testing.T) {
	t.Parallel()

	fake := faker.New()
	width := fake.IntBetween(1, 200)

	settings := config.Defaults().Apply(config.Overrides{
		LineLength:         &width,
		MagicTrailingComma: new(false),
	})

	assert.Equal(t, width, settings.LineLength)
	assert.Equal(t, config.DefaultTabWidth, settings.TabWidth)
	assert.False(t, settings.MagicTrailingComma)
	require.NoError(t, settings.Validate())

	require.ErrorContains(t, config.Settings{LineLength: 0, TabWidth: 4}.Validate(), "line-length")
	require.ErrorContains(t, config.Settings{LineLength: 1, TabWidth: 0}.Validate(), "tab-width")
}

func TestLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	good := filepath.Join(dir, "good.toml")
	write(t, good, "line-length = 80\ntab-width = 2\nmagic-trailing-comma = false\n")

	overrides, err := config.Load(good)
	require.NoError(t, err)
	assert.Equal(t, 80, *overrides.LineLength)
	assert.Equal(t, 2, *overrides.TabWidth)
	assert.False(t, *overrides.MagicTrailingComma)

	unknown := filepath.Join(dir, "unknown.toml")
	write(t, unknown, "bogus = 1\n")

	_, err = config.Load(unknown)
	require.ErrorContains(t, err, "error parsing config")

	_, err = config.Load(filepath.Join(dir, "missing.toml"))
	require.ErrorContains(t, err, "error reading config")
}

func TestFindProjectFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module x\n")
	write(t, filepath.Join(root, ".onyx.toml"), "")
	write(t, filepath.Join(root, "a", "b", "file.go"), "")
	write(t, filepath.Join(root, "nested", "go.mod"), "module y\n")
	write(t, filepath.Join(root, "nested", "pkg", "file.go"), "")
	write(t, filepath.Join(root, "a", ".onyx.toml"), "")

	found, err := config.FindProjectFile(filepath.Join(root, "a", "b", "file.go"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "a", ".onyx.toml"), found)

	// The search stops at the nested go.mod and does not inherit the root file.
	found, err = config.FindProjectFile(filepath.Join(root, "nested", "pkg", "file.go"))
	require.NoError(t, err)
	assert.Empty(t, found)

	// The go.mod directory itself is checked.
	found, err = config.FindProjectFile(filepath.Join(root, "main.go"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, ".onyx.toml"), found)
}

func TestFindProjectFileNoMarker(t *testing.T) {
	t.Parallel()

	// Walks to the filesystem root without finding anything.
	found, err := config.FindProjectFile("/nonexistent-onyx-dir/file.go")
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestResolve(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module x\n")

	file := filepath.Join(root, "main.go")

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	t.Setenv("HOME", filepath.Join(root, "home"))

	global, err := config.GlobalFile()
	require.NoError(t, err)

	t.Run("defaults", func(t *testing.T) {
		settings, err := config.Resolve(file, "", config.Overrides{})
		require.NoError(t, err)
		assert.Equal(t, config.Defaults(), settings)
	})

	t.Run("global", func(t *testing.T) {
		write(t, global, "line-length = 70\n")

		settings, err := config.Resolve(file, "", config.Overrides{})
		require.NoError(t, err)
		assert.Equal(t, 70, settings.LineLength)
	})

	t.Run("project beats global, cli beats project", func(t *testing.T) {
		write(t, filepath.Join(root, ".onyx.toml"), "line-length = 60\ntab-width = 8\n")

		settings, err := config.Resolve(file, "", config.Overrides{TabWidth: new(2)})
		require.NoError(t, err)
		assert.Equal(t, 60, settings.LineLength)
		assert.Equal(t, 2, settings.TabWidth)
	})

	t.Run("explicit config", func(t *testing.T) {
		explicit := filepath.Join(root, "explicit.toml")
		write(t, explicit, "line-length = 50\n")

		settings, err := config.Resolve(file, explicit, config.Overrides{})
		require.NoError(t, err)
		assert.Equal(t, 50, settings.LineLength)
	})

	t.Run("errors", func(t *testing.T) {
		_, err := config.Resolve(file, filepath.Join(root, "nope.toml"), config.Overrides{})
		require.Error(t, err)

		_, err = config.Resolve(file, "", config.Overrides{LineLength: new(-1)})
		require.ErrorContains(t, err, "line-length")
	})

	t.Run("no user config dir", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		_, err := config.Resolve(file, "", config.Overrides{})
		require.NoError(t, err)
	})
}
