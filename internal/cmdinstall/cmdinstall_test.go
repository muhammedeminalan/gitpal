package cmdinstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// macApp, geçici bir klasörde gerçek bir gitpal.app düzeni kurar.
func macApp(t *testing.T) (self, cli string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Applications", "gitpal.app", "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	self, cli = filepath.Join(dir, "gitpal-gui"), filepath.Join(dir, "gitpal")
	for _, p := range []string{self, cli} {
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755)) //nolint:gosec // test programı
	}
	return self, cli
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	require.NoError(t, err)
	return string(b)
}

func TestMacLinksTheBundledCommandsAndFixesPath(t *testing.T) {
	self, cli := macApp(t)
	home := t.TempDir()
	o := Options{GOOS: "darwin", Home: home, Self: self}

	res, err := Ensure(o)
	require.NoError(t, err)
	assert.True(t, res.Changed)
	bin := filepath.Join(home, ".local", "bin")
	assert.Equal(t, bin, res.Dir)
	for _, name := range []string{"gitpal", "gp"} {
		got, err := os.Readlink(filepath.Join(bin, name))
		require.NoError(t, err, name)
		assert.Equal(t, cli, got, "uygulamanın içindekine bağlanır; uygulama güncellenince komut da güncellenir")
	}
	assert.Equal(t, filepath.Join(home, ".zprofile"), res.Profile)
	prof := read(t, filepath.Join(home, ".zprofile"))
	assert.Contains(t, prof, `export PATH="$HOME/.local/bin:$PATH"`)

	// İkinci açılış: hiçbir şey değişmez, satır iki kez eklenmez.
	res, err = Ensure(o)
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.Equal(t, prof, read(t, filepath.Join(home, ".zprofile")))
}

func TestBashUsersGetThePathToo(t *testing.T) {
	self, _ := macApp(t)
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("alias ll='ls -l'\n"), 0o600))
	_, err := Ensure(Options{GOOS: "darwin", Home: home, Self: self})
	require.NoError(t, err)
	assert.Contains(t, read(t, filepath.Join(home, ".bash_profile")), ".local/bin")
	assert.Contains(t, read(t, filepath.Join(home, ".bash_profile")), "alias ll", "mevcut içerik korunur")
}

func TestProfileThatAlreadyHasLocalBinIsLeftAlone(t *testing.T) {
	self, _ := macApp(t)
	home := t.TempDir()
	orig := "export PATH=$HOME/.local/bin:$PATH\n"
	require.NoError(t, os.WriteFile(filepath.Join(home, ".zshrc"), []byte(orig), 0o600))
	res, err := Ensure(Options{GOOS: "darwin", Home: home, Self: self})
	require.NoError(t, err)
	assert.Empty(t, res.Profile)
	assert.Empty(t, read(t, filepath.Join(home, ".zprofile")))
	assert.Equal(t, orig, read(t, filepath.Join(home, ".zshrc")))
}

func TestUsersOwnInstallIsNotTouched(t *testing.T) {
	self, _ := macApp(t)
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	own := filepath.Join(bin, "gitpal")
	require.NoError(t, os.WriteFile(own, []byte("kendi kurduğu"), 0o755)) //nolint:gosec // test
	res, err := Ensure(Options{GOOS: "darwin", Home: home, Self: self})
	require.NoError(t, err)
	assert.Equal(t, "kendi kurduğu", read(t, own))
	_, err = os.Lstat(filepath.Join(bin, "gp"))
	assert.True(t, os.IsNotExist(err), "kullanıcı kendisi kurduysa gp'ye de dokunulmaz")
	assert.False(t, res.Changed)
}

func TestMovedAppIsRelinked(t *testing.T) {
	self, cli := macApp(t)
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.Symlink("/Volumes/eski/gitpal.app/Contents/MacOS/gitpal", filepath.Join(bin, "gitpal")))
	res, err := Ensure(Options{GOOS: "darwin", Home: home, Self: self})
	require.NoError(t, err)
	assert.True(t, res.Changed)
	got, _ := os.Readlink(filepath.Join(bin, "gitpal"))
	assert.Equal(t, cli, got)
}

func TestDevelopmentBuildOutsideAnAppDoesNothing(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "gitpal-gui")
	require.NoError(t, os.WriteFile(self, nil, 0o755))                                 //nolint:gosec // test
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gitpal"), []byte("x"), 0o755)) //nolint:gosec // test
	home := t.TempDir()
	res, err := Ensure(Options{GOOS: "darwin", Home: home, Self: self})
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.Empty(t, res.Dir)
}

func TestMissingCommandsAreDownloaded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gitpal.app", "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	self := filepath.Join(dir, "gitpal-gui")
	require.NoError(t, os.WriteFile(self, nil, 0o755)) //nolint:gosec // test
	home := t.TempDir()
	var fetchedInto string
	res, err := Ensure(Options{GOOS: "darwin", Home: home, Self: self, Fetch: func(dest string) (string, error) {
		fetchedInto = dest
		p := filepath.Join(dest, "gitpal")
		require.NoError(t, os.MkdirAll(dest, 0o755))
		return p, os.WriteFile(p, []byte("indirildi"), 0o755) //nolint:gosec // test
	}})
	require.NoError(t, err)
	bin := filepath.Join(home, ".local", "bin")
	assert.Equal(t, bin, fetchedInto)
	assert.True(t, res.Changed)
	assert.True(t, res.Downloaded)
	got, err := os.Readlink(filepath.Join(bin, "gp"))
	require.NoError(t, err)
	assert.Equal(t, "gitpal", got, "gp yanındaki gitpal'a bağlanır")

	// Bir dahaki açılışta yeniden indirilmez.
	res, err = Ensure(Options{GOOS: "darwin", Home: home, Self: self, Fetch: func(string) (string, error) {
		t.Fatal("ikinci kez indirildi")
		return "", nil
	}})
	require.NoError(t, err)
	assert.False(t, res.Changed)
}

func TestLinuxUsesDotProfile(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "gitpal-gui")
	cli := filepath.Join(dir, "gitpal")
	require.NoError(t, os.WriteFile(self, nil, 0o755)) //nolint:gosec // test
	require.NoError(t, os.WriteFile(cli, nil, 0o755))  //nolint:gosec // test
	home := t.TempDir()
	res, err := Ensure(Options{GOOS: "linux", Home: home, Self: self})
	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t, filepath.Join(home, ".profile"), res.Profile)
	got, _ := os.Readlink(filepath.Join(home, ".local", "bin", "gitpal"))
	assert.Equal(t, cli, got)
}

type fakePath struct{ value string }

func (f *fakePath) Get() (string, error) { return f.value, nil }
func (f *fakePath) Set(v string) error   { f.value = v; return nil }

func TestWindowsAddsTheAppFolderToTheUserPath(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "gitpal-gui.exe")
	require.NoError(t, os.WriteFile(self, nil, 0o755))                             //nolint:gosec // test
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gitpal.exe"), nil, 0o755)) //nolint:gosec // test
	path := &fakePath{value: `C:\Windows;C:\Tools`}
	res, err := Ensure(Options{GOOS: "windows", Home: t.TempDir(), Self: self, UserPath: path})
	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t, dir, res.Dir)
	assert.Equal(t, `C:\Windows;C:\Tools;`+dir, path.value)
	gp := read(t, filepath.Join(dir, "gp.cmd"))
	assert.True(t, strings.Contains(gp, `"%~dp0gitpal.exe" %*`), gp)

	res, err = Ensure(Options{GOOS: "windows", Home: t.TempDir(), Self: self, UserPath: path})
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.Equal(t, `C:\Windows;C:\Tools;`+dir, path.value, "iki kez eklenmez")

	// Büyük/küçük harf ve sondaki \ farkı aynı klasör sayılır.
	path.value = strings.ToUpper(dir) + `\`
	res, err = Ensure(Options{GOOS: "windows", Home: t.TempDir(), Self: self, UserPath: path})
	require.NoError(t, err)
	assert.False(t, res.Changed)
}
