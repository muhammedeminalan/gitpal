package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	cfg := Default()
	for key, want := range map[string]string{
		"language":                  "tr",
		"commit.suggestions":        "3",
		"safety.warn_on_secrets":    "true",
		"safety.protected_branches": "main,master,develop",
		"ai.model":                  "",
	} {
		got, err := Get(cfg, key)
		require.NoError(t, err, key)
		assert.Equal(t, want, got, key)
	}
	for _, bad := range []string{"nope", "commit", "commit.nope", ""} {
		_, err := Get(cfg, bad)
		assert.ErrorIs(t, err, ErrUnknownKey, bad)
	}
	assert.Contains(t, Keys(), "safety.large_file_mb")
	assert.NotContains(t, Keys(), "safety")
}

func TestSetGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitpal", "config.yaml")

	t.Run("creates the file", func(t *testing.T) {
		require.NoError(t, SetGlobal(path, "language", "en"))
		require.NoError(t, SetGlobal(path, "safety.protected_branches", "main, release/*"))
		require.NoError(t, SetGlobal(path, "safety.large_file_mb", "10"))
		require.NoError(t, SetGlobal(path, "safety.warn_on_secrets", "false"))

		loaded, err := Load(LoadOptions{GlobalPath: path})
		require.NoError(t, err)
		assert.Equal(t, "en", string(loaded.Config.Language))
		assert.Equal(t, []string{"main", "release/*"}, loaded.Config.Safety.ProtectedBranches)
		assert.Equal(t, 10, loaded.Config.Safety.LargeFileMB)
		assert.False(t, loaded.Config.Safety.WarnOnSecrets)
		assert.Empty(t, loaded.Warnings)
	})

	t.Run("keeps comments and other settings", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("# benim ayarlarım\nlanguage: tr # arayüz\ncommit:\n  style: free\n"), 0o600))

		require.NoError(t, SetGlobal(path, "commit.suggestions", "2"))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "# benim ayarlarım")
		assert.Contains(t, string(data), "# arayüz")
		loaded, err := Load(LoadOptions{GlobalPath: path})
		require.NoError(t, err)
		assert.Equal(t, StyleFree, loaded.Config.Commit.Style)
		assert.Equal(t, 2, loaded.Config.Commit.Suggestions)
	})

	t.Run("an invalid value leaves the file alone", func(t *testing.T) {
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		for key, value := range map[string]string{
			"language": "de", "commit.suggestions": "9", "safety.warn_on_secrets": "maybe", "commit.max_subject_length": "x",
		} {
			err := SetGlobal(path, key, value)
			require.Error(t, err, key)
		}
		var invalid *ValidationError
		require.ErrorAs(t, SetGlobal(path, "language", "de"), &invalid)
		assert.Equal(t, "language", invalid.Key)

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after))
		_, err = os.Stat(path + ".new")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("unknown key", func(t *testing.T) {
		assert.True(t, errors.Is(SetGlobal(path, "ai", "x"), ErrUnknownKey))
	})

	t.Run("a file that is not a mapping", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "c.yaml")
		require.NoError(t, os.WriteFile(bad, []byte("- a\n- b\n"), 0o600))
		var fileErr *FileError
		require.ErrorAs(t, SetGlobal(bad, "language", "en"), &fileErr)
	})
}
