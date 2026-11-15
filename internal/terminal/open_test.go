package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gerçek bir süreç başlatılır: "terminal" yerine argümanlarını ve klasörünü bir
// dosyaya yazan küçük bir betik.
func TestOpenStartsTheTerminalInTheRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("betik tabanlı sahte terminal Windows'ta çalışmaz; Windows planı plan() testlerinde")
	}
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	require.NoError(t, os.Mkdir(repo, 0o755))
	record := filepath.Join(tmp, "record")
	fake := filepath.Join(tmp, "fake-terminal")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\n{ pwd; for a in \"$@\"; do echo \"$a\"; done; } > "+record+".tmp && mv "+record+".tmp "+record+"\n"), 0o700)) //nolint:gosec // test betiği

	err := Open(Options{
		GOOS: "linux", Dir: repo, Gitpal: "/opt/gitpal/gitpal", Hint: "ipucu",
		LookPath: func(name string) (string, error) {
			if name == "xterm" {
				return fake, nil
			}
			return "", os.ErrNotExist
		},
		Getenv:  func(string) string { return "" },
		Environ: []string{"PATH=/usr/bin:/bin"},
	})
	require.NoError(t, err)

	var got []byte
	require.Eventually(t, func() bool {
		got, err = os.ReadFile(record)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond)
	lines := strings.Split(string(got), "\n")
	resolved, _ := filepath.EvalSymlinks(repo)
	assert.Contains(t, []string{repo, resolved}, lines[0], "terminal repo klasöründe başlar")
	assert.Equal(t, []string{"-e", "sh", "-c"}, lines[1:4])
	assert.Contains(t, string(got), "'/opt/gitpal/gitpal'")
}

func TestScriptFileIsPrivateAndExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dosya izinleri Windows'ta yok")
	}
	p, err := writeScript("#!/bin/sh\n", ".command")
	require.NoError(t, err)
	defer os.Remove(p)
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	assert.True(t, strings.HasSuffix(p, ".command"))
}
