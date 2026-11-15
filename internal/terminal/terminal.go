// Package terminal masaüstü uygulamasındaki "Terminal" düğmesinin arkasındaki iş:
// sistemin varsayılan terminalini açık reponun klasöründe açar, orada gitpal'ın
// ana ekranını başlatır; gitpal'dan çıkınca pencere kullanıcının kabuğu olarak kalır.
//
// Hangi programın hangi argümanlarla çalışacağı plan() içinde, işletim sisteminden
// bağımsız hesaplanır; böylece üç platformun da kuralları her yerde test edilir.
// Süreci başlatma işi open_*.go dosyalarındadır.
package terminal

import (
	"errors"
	"path"
	"strings"
)

var (
	// ErrNoGitpal gitpal komut satırı aracı ne uygulamanın yanında ne PATH'te.
	ErrNoGitpal = errors.New("terminal: gitpal not found")
	// ErrNoTerminal Linux'ta bilinen bir terminal programı bulunamadı.
	ErrNoTerminal = errors.New("terminal: no terminal emulator found")
)

// Options terminali açmak için gerekenler. İşletim sistemi bilgisi dışarıdan
// verilir ki testler her platformun kuralını deneyebilsin.
type Options struct {
	GOOS string
	// Dir, terminalin açılacağı klasör (reponun kökü).
	Dir string
	// Gitpal, çalıştırılacak gitpal programının tam yolu (FindGitpal).
	Gitpal string
	// Hint, gitpal'dan çıkınca gösterilecek tek satırlık ipucu (kullanıcının dilinde).
	Hint     string
	LookPath func(string) (string, error)
	Getenv   func(string) string
	// Environ, terminale aktarılacak ortam değişkenleri (os.Environ()).
	Environ []string
}

// Plan çalıştırılacak program ve nasıl çalıştırılacağı.
type Plan struct {
	Name string
	Args []string
	Dir  string
	Env  []string
	// Script ve ScriptExt doluysa (macOS) betik geçici bir dosyaya yazılır ve
	// dosyanın yolu Args'ın sonuna eklenir.
	Script    string
	ScriptExt string
	// CmdLine ve NewConsole yalnızca Windows içindir: komut satırı olduğu gibi
	// verilir, program yeni bir konsol penceresinde açılır.
	CmdLine    string
	NewConsole bool
}

// linuxTerminals, sırayla denenen terminaller ve bir komutu çalıştırmak için
// istedikleri argümanlar. x-terminal-emulator, Debian/Ubuntu'da kullanıcının
// seçtiği varsayılan terminaldir; o yüzden ilk sırada.
var linuxTerminals = []struct {
	name string
	args []string
}{
	{"x-terminal-emulator", []string{"-e"}},
	{"gnome-terminal", []string{"--"}},
	{"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}},
	{"kitty", nil},
	{"alacritty", []string{"-e"}},
	{"wezterm", []string{"start", "--"}},
	{"xterm", []string{"-e"}},
}

// FindGitpal çalıştırılacak gitpal'ı bulur: önce masaüstü uygulamasının yanında
// (macOS'ta .app paketinin içinde), yoksa PATH'te. Bulamazsa boş döner.
func FindGitpal(goos, self string, exists func(string) bool, lookPath func(string) (string, error)) string {
	name := "gitpal"
	if goos == "windows" {
		name += ".exe"
	}
	if self != "" {
		if p := join(goos, dir(goos, self), name); exists(p) {
			return p
		}
	}
	if p, err := lookPath(name); err == nil {
		return p
	}
	return ""
}

func plan(o Options) (Plan, error) {
	if o.Gitpal == "" {
		return Plan{}, ErrNoGitpal
	}
	switch o.GOOS {
	case "darwin":
		return Plan{
			Name:      "open",
			Dir:       o.Dir,
			Script:    "#!/bin/sh\n# gitpal: masaüstündeki Terminal düğmesiyle açıldı. Dosya kendini siler.\nrm -f \"$0\"\n" + unixScript(o, "/bin/zsh"),
			ScriptExt: ".command",
		}, nil
	case "windows":
		comspec := o.Getenv("ComSpec")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		return Plan{
			Name: comspec,
			Dir:  o.Dir,
			Env:  withPath(o.Environ, dir("windows", o.Gitpal), ";"),
			// gitpal PATH'te olduğu için adıyla çağrılır; tırnak sorunu çıkmaz.
			CmdLine:    "cmd.exe /k gitpal & echo " + cmdEscape(o.Hint),
			NewConsole: true,
		}, nil
	default:
		name, args, err := linuxTerminal(o)
		if err != nil {
			return Plan{}, err
		}
		return Plan{
			Name: name,
			Args: append(args, "sh", "-c", unixScript(o, "/bin/sh")),
			Dir:  o.Dir,
			Env:  withPath(o.Environ, dir(o.GOOS, o.Gitpal), ":"),
		}, nil
	}
}

func linuxTerminal(o Options) (string, []string, error) {
	if want := strings.TrimSpace(o.Getenv("TERMINAL")); want != "" {
		if p, err := o.LookPath(want); err == nil {
			for _, t := range linuxTerminals {
				if path.Base(want) == t.name {
					return p, append([]string(nil), t.args...), nil
				}
			}
			return p, []string{"-e"}, nil
		}
	}
	for _, t := range linuxTerminals {
		if p, err := o.LookPath(t.name); err == nil {
			return p, append([]string(nil), t.args...), nil
		}
	}
	return "", nil, ErrNoTerminal
}

// unixScript: klasöre geç, gitpal'ı başlat, çıkınca ipucunu yaz ve kullanıcının
// kabuğuna devret (pencere açık kalsın). Her değer tek tırnakla korunur.
func unixScript(o Options, fallbackShell string) string {
	return "cd " + shQuote(o.Dir) + " || exit 1\n" +
		"PATH=" + shQuote(dir(o.GOOS, o.Gitpal)) + ":\"$PATH\"; export PATH\n" +
		shQuote(o.Gitpal) + "\n" +
		"printf '%s\\n' " + shQuote(o.Hint) + "\n" +
		"exec \"${SHELL:-" + fallbackShell + "}\" -l\n"
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// cmdEscape, cmd.exe'nin özel karakterlerini ^ ile kaçırır.
func cmdEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("^&|<>()%!\"", r) {
			b.WriteRune('^')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// withPath, ortamın PATH'inin başına klasörü ekler. Windows'ta değişkenin adı
// "Path" de olabilir; aynı adın iki kez geçmemesi için mevcut kayıt değiştirilir.
func withPath(env []string, folder, sep string) []string {
	out := make([]string, 0, len(env)+1)
	done := false
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && !done && strings.EqualFold(k, "PATH") {
			out = append(out, k+"="+folder+sep+v)
			done = true
			continue
		}
		out = append(out, kv)
	}
	if !done {
		out = append(out, "PATH="+folder)
	}
	return out
}

// dir ve join, hedef işletim sisteminin yol ayırıcısıyla çalışır (testler başka
// bir sistemde koşabilir).
func dir(goos, p string) string {
	if goos == "windows" {
		if i := strings.LastIndexAny(p, `\/`); i >= 0 {
			return p[:i]
		}
		return "."
	}
	return path.Dir(p)
}

func join(goos, d, name string) string {
	if goos == "windows" {
		return d + `\` + name
	}
	return path.Join(d, name)
}
