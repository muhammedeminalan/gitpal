package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitNUL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"one terminated", "a\x00", []string{"a"}},
		{"several terminated", "a\x00b c\x00", []string{"a", "b c"}},
		{"separated, no terminator", "a\x00b", []string{"a", "b"}},
		{"empty field kept in the middle", "a\x00\x00b\x00", []string{"a", "", "b"}},
		{"newlines are data", "line1\nline2\x00", []string{"line1\nline2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, splitNUL([]byte(tt.in)))
		})
	}
}

func TestPathspecStdin(t *testing.T) {
	args, stdin, err := pathspecStdin([]string{"a.txt", "dir/with space.go", "-rf", ":(glob)*", "türkçe.md"})
	require.NoError(t, err)

	assert.Equal(t, []string{"--pathspec-from-file=-", "--pathspec-file-nul"}, args)
	assert.Equal(t, "a.txt\x00dir/with space.go\x00-rf\x00:(glob)*\x00türkçe.md\x00", string(stdin))
}

func TestPathspecStdinRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		code  ArgCode
	}{
		{"no paths", nil, ArgNoPaths},
		{"empty list", []string{}, ArgNoPaths},
		{"empty path", []string{"a", ""}, ArgInvalidPath},
		{"nul byte", []string{"a\x00b"}, ArgInvalidPath},
		{"absolute", []string{"/etc/passwd"}, ArgInvalidPath},
		{"windows absolute", []string{`C:\Windows`}, ArgInvalidPath},
		{"escapes the repository", []string{"../outside"}, ArgInvalidPath},
		{"escapes after cleaning", []string{"a/../../outside"}, ArgInvalidPath},
		{"parent itself", []string{".."}, ArgInvalidPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := pathspecStdin(tt.paths)
			var argErr *ArgError
			require.ErrorAs(t, err, &argErr)
			assert.Equal(t, tt.code, argErr.Code)
		})
	}
}

func TestCheckPathsAcceptsOrdinaryRelativePaths(t *testing.T) {
	assert.NoError(t, checkPaths([]string{".", "a", "a/b/../c", "dir/", "-dash-first", "with space"}))
}

func TestCheckName(t *testing.T) {
	ok := []string{"main", "feat/login", "origin/main", "HEAD~2", "v1.0", "abc123", "@{u}"}
	for _, value := range ok {
		assert.NoError(t, checkName(value, ArgInvalidRev), value)
	}

	bad := []string{"", "-x", "--force", "a\x00b", "a\nb", "a b", "a\tb"}
	for _, value := range bad {
		err := checkName(value, ArgInvalidBranch)
		var argErr *ArgError
		require.ErrorAs(t, err, &argErr, "%q", value)
		assert.Equal(t, ArgInvalidBranch, argErr.Code)
		assert.Equal(t, value, argErr.Value)
	}
}

func TestNetworkCmdUsesTheNetworkTimeout(t *testing.T) {
	cmd := networkCmd("/repo", "fetch", "--prune")

	assert.Equal(t, NetworkTimeout, cmd.Timeout)
	assert.Equal(t, "/repo", cmd.Dir)
	assert.Equal(t, []string{"fetch", "--prune"}, cmd.Args)
}
