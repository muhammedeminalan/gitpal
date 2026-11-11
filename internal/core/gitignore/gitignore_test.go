package gitignore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplates(t *testing.T) {
	names := Names()
	assert.Contains(t, names, "go")
	assert.Contains(t, names, "flutter")
	assert.Contains(t, names, "dotnet")
	for _, n := range names {
		content, err := Template(n)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
	}
	_, err := Template("cobol")
	assert.ErrorIs(t, err, ErrUnknownTemplate)
}

func TestApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(path, []byte("# mine\n.DS_Store\n/secret.txt"), 0o600))

	require.NoError(t, Apply(dir, []string{"macos", "go"}))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	assert.True(t, len(text) > 0 && text[:len("# mine\n.DS_Store\n/secret.txt\n")] == "# mine\n.DS_Store\n/secret.txt\n", "existing content first, unchanged")
	assert.Equal(t, 1, countLines(text, ".DS_Store"), "a rule already present is not repeated")
	assert.Contains(t, text, "*.exe")

	before := text
	require.NoError(t, Apply(dir, []string{"go"}))
	data, _ = os.ReadFile(path)
	assert.Contains(t, string(data), before, "applying again only adds comments, never repeats rules")

	assert.ErrorIs(t, Apply(dir, []string{"nope"}), ErrUnknownTemplate)
}

func countLines(text, line string) int {
	n := 0
	for _, l := range splitLines(text) {
		if l == line {
			n++
		}
	}
	return n
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
