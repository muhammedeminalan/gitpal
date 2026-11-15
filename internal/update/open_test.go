package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenCommandPerPlatform(t *testing.T) {
	name, args := openCommand("darwin", "/Users/a/Downloads/gitpal_1.2.0_macos_arm64.dmg")
	assert.Equal(t, "open", name, "dmg açılır: diski bağlar, Finder'da gösterir")
	assert.Equal(t, []string{"/Users/a/Downloads/gitpal_1.2.0_macos_arm64.dmg"}, args)

	name, args = openCommand("windows", `C:\Users\a\Downloads\x.zip`)
	assert.Equal(t, "explorer.exe", name)
	assert.Equal(t, []string{`/select,C:\Users\a\Downloads\x.zip`}, args)

	name, args = openCommand("linux", "/home/a/Downloads/x.tar.gz")
	assert.Equal(t, "xdg-open", name)
	assert.Equal(t, []string{"/home/a/Downloads"}, args, "Linux'ta klasör açılır")
}

func TestDownloadsDirFallsBackToTemp(t *testing.T) {
	home := t.TempDir()
	assert.NotEqual(t, home+"/Downloads", DownloadsDir(home), "Downloads yoksa geçici klasör")
}
