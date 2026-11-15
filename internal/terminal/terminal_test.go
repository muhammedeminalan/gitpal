package terminal

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookIn(found ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, f := range found {
			if f == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func opts(goos string) Options {
	return Options{
		GOOS:     goos,
		Dir:      "/home/ali/projem's repo",
		Gitpal:   "/Applications/gitpal.app/Contents/MacOS/gitpal",
		Hint:     "Komut listesi için: gitpal rehber",
		LookPath: lookIn(),
		Getenv:   func(string) string { return "" },
		Environ:  []string{"HOME=/home/ali", "PATH=/usr/bin"},
	}
}

func TestMacOpensTheDefaultAppForCommandFiles(t *testing.T) {
	p, err := plan(opts("darwin"))
	require.NoError(t, err)
	// `open` dosyayı .command türünün varsayılan uygulamasıyla açar: Terminal,
	// ya da kullanıcı iTerm'i varsayılan yaptıysa iTerm.
	assert.Equal(t, "open", p.Name)
	assert.Equal(t, ".command", p.ScriptExt)
	s := p.Script
	assert.True(t, strings.HasPrefix(s, "#!/bin/sh\n"))
	assert.Contains(t, s, `rm -f "$0"`, "geçici dosya kendini siler")
	// Tek tırnak içindeki tek tırnak kaçırılır: klasör adı komut olarak yorumlanamaz.
	assert.Contains(t, s, `cd '/home/ali/projem'\''s repo' || exit 1`)
	assert.Contains(t, s, `'/Applications/gitpal.app/Contents/MacOS/gitpal'`)
	assert.Contains(t, s, `PATH='/Applications/gitpal.app/Contents/MacOS':"$PATH"`)
	assert.Contains(t, s, `printf '%s\n' 'Komut listesi için: gitpal rehber'`)
	// gitpal'dan çıkınca pencere kapanmasın: kullanıcının kabuğu açılır.
	assert.True(t, strings.HasSuffix(s, "exec \"${SHELL:-/bin/zsh}\" -l\n"))
}

func TestWindowsOpensANewConsoleThatStays(t *testing.T) {
	o := opts("windows")
	o.Dir = `C:\Users\Ali\proje`
	o.Gitpal = `C:\Program Files\gitpal\gitpal.exe`
	o.Environ = []string{`Path=C:\Windows`, `TEMP=C:\t`}
	o.Getenv = func(k string) string {
		if k == "ComSpec" {
			return `C:\Windows\system32\cmd.exe`
		}
		return ""
	}
	o.Hint = "Liste için: gitpal rehber & daha fazlası"
	p, err := plan(o)
	require.NoError(t, err)
	assert.Equal(t, `C:\Windows\system32\cmd.exe`, p.Name)
	assert.True(t, p.NewConsole)
	assert.Equal(t, o.Dir, p.Dir)
	// /k: komut bitince pencere açık kalır. Özel karakterler ^ ile kaçırılır.
	assert.Equal(t, `cmd.exe /k gitpal & echo Liste için: gitpal rehber ^& daha fazlası`, p.CmdLine)
	// PATH'in başına gitpal'ın klasörü eklenir; Windows'ta adın büyük/küçük harfi farklı olabilir.
	assert.Contains(t, p.Env, `Path=C:\Program Files\gitpal;C:\Windows`)
	assert.Contains(t, p.Env, `TEMP=C:\t`)
	assert.Len(t, p.Env, 2)
}

func TestLinuxUsesTheFirstTerminalItFinds(t *testing.T) {
	o := opts("linux")
	o.Gitpal = "/usr/local/bin/gitpal"
	o.LookPath = lookIn("xterm", "gnome-terminal")
	p, err := plan(o)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/gnome-terminal", p.Name, "listede xterm'den önce gelir")
	require.Len(t, p.Args, 4)
	assert.Equal(t, []string{"--", "sh", "-c"}, p.Args[:3])
	assert.Contains(t, p.Args[3], `cd '/home/ali/projem'\''s repo' || exit 1`)
	assert.Equal(t, o.Dir, p.Dir)
	assert.Contains(t, p.Env, "PATH=/usr/local/bin:/usr/bin")

	// Kullanıcının TERMINAL değişkeni her şeyden önce gelir.
	o.Getenv = func(k string) string {
		if k == "TERMINAL" {
			return "kitty"
		}
		return ""
	}
	o.LookPath = lookIn("kitty", "gnome-terminal")
	p, err = plan(o)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/kitty", p.Name)
	assert.Equal(t, []string{"sh", "-c"}, p.Args[:2])

	o.Getenv = func(string) string { return "" }
	o.LookPath = lookIn("xfce4-terminal")
	p, err = plan(o)
	require.NoError(t, err)
	assert.Equal(t, "-x", p.Args[0], "xfce4-terminal -e tek bir dize ister; -x kalan argümanları çalıştırır")

	o.LookPath = lookIn()
	_, err = plan(o)
	assert.ErrorIs(t, err, ErrNoTerminal)
}

func TestNeedsGitpal(t *testing.T) {
	o := opts("darwin")
	o.Gitpal = ""
	_, err := plan(o)
	assert.ErrorIs(t, err, ErrNoGitpal)
}

func TestFindGitpalPrefersTheOneNextToTheApp(t *testing.T) {
	exists := func(p string) bool { return p == "/Applications/gitpal.app/Contents/MacOS/gitpal" }
	got := FindGitpal("darwin", "/Applications/gitpal.app/Contents/MacOS/gitpal-gui", exists, lookIn("gitpal"))
	assert.Equal(t, "/Applications/gitpal.app/Contents/MacOS/gitpal", got)

	// Yanında yoksa PATH'teki.
	got = FindGitpal("linux", "/opt/gitpal-gui", func(string) bool { return false }, lookIn("gitpal"))
	assert.Equal(t, "/usr/bin/gitpal", got)

	// Windows'ta .exe uzantısıyla aranır.
	exists = func(p string) bool { return p == `C:\gp\gitpal.exe` }
	got = FindGitpal("windows", `C:\gp\gitpal-gui.exe`, exists, lookIn())
	assert.Equal(t, `C:\gp\gitpal.exe`, got)

	assert.Empty(t, FindGitpal("linux", "/opt/gitpal-gui", func(string) bool { return false }, lookIn()))
}
