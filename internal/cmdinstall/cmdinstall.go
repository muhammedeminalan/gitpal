// Package cmdinstall makes the terminal commands (gitpal and its short name gp)
// available as soon as the desktop app is installed, with no separate step.
//
//   - macOS: the .app carries gitpal; ~/.local/bin/gitpal and gp link to it, so
//     updating the app updates the commands too.
//   - Linux: the same, with the gitpal next to gitpal-gui.
//   - Windows: the app's folder (gitpal.exe is next to gitpal-gui.exe) is added to
//     the user's PATH, and gp.cmd forwards to gitpal.exe.
//
// When the package holds no gitpal (an older package, a single-file download),
// Fetch downloads the same version's command-line tool instead.
//
// It runs on every start and changes nothing the second time. A gitpal the user
// installed some other way (install.sh, a package manager) is left alone.
package cmdinstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathEditor reads and writes the user's PATH on Windows (the registry).
type PathEditor interface {
	Get() (string, error)
	Set(value string) error
}

// Options are what Ensure works with; the system is passed in so every
// platform's rules can be tested anywhere.
type Options struct {
	GOOS string
	Home string
	// Self is the desktop app's own program (os.Executable).
	Self string
	// Fetch downloads gitpal into dest and returns its path; nil when there is
	// nothing to download from (a development build).
	Fetch func(dest string) (string, error)
	// UserPath is the user's PATH on Windows.
	UserPath PathEditor
	// ProgramsDir is where a downloaded gitpal.exe goes on Windows
	// (%LOCALAPPDATA%\Programs\gitpal).
	ProgramsDir string
}

// Result says what Ensure did.
type Result struct {
	// Changed is true when something was installed or repaired this time.
	Changed bool
	// Dir is the folder the commands are in; empty when there was nothing to do.
	Dir string
	// Profile is the shell file that got the PATH line, if one did.
	Profile string
	// Downloaded is true when gitpal had to be downloaded.
	Downloaded bool
}

// profileLine puts ~/.local/bin on PATH for new terminal windows.
const profileLine = "\n# gitpal: terminal komutları (gitpal, gp) — masaüstü uygulaması ekledi\n" +
	"export PATH=\"$HOME/.local/bin:$PATH\"\n"

// Ensure makes gitpal and gp available in new terminal windows.
func Ensure(o Options) (Result, error) {
	if o.GOOS == "windows" {
		return ensureWindows(o)
	}
	return ensureUnix(o)
}

// bundled is the gitpal that came with the app, or "". On macOS only a real
// .app counts: a development build run from bin/ must not take over the
// user's commands.
func bundled(o Options, name string) string {
	if o.Self == "" {
		return ""
	}
	if o.GOOS == "darwin" && !strings.Contains(filepath.ToSlash(o.Self), ".app/Contents/MacOS/") {
		return ""
	}
	p := filepath.Join(filepath.Dir(o.Self), name)
	if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
		return p
	}
	return ""
}

func ensureUnix(o Options) (Result, error) {
	bin := filepath.Join(o.Home, ".local", "bin")
	cli := bundled(o, "gitpal")
	inApp := o.GOOS != "darwin" || strings.Contains(filepath.ToSlash(o.Self), ".app/Contents/MacOS/")
	var res Result

	main := filepath.Join(bin, "gitpal")
	switch {
	case cli != "":
		changed, err := link(main, cli)
		if err != nil {
			return res, err
		}
		if !changed && !ours(main, cli) {
			// Kullanıcının kendi kurduğu gitpal: hiçbirine dokunma.
			return res, nil
		}
		gpChanged, err := link(filepath.Join(bin, "gp"), cli)
		if err != nil {
			return res, err
		}
		res.Changed = changed || gpChanged
	case !inApp || o.Fetch == nil:
		return res, nil
	default:
		if _, err := os.Lstat(main); err == nil {
			return res, nil // daha önce indirilmiş ya da kullanıcı kurmuş
		}
		if _, err := o.Fetch(bin); err != nil {
			return res, fmt.Errorf("cmdinstall: %w", err)
		}
		res.Changed, res.Downloaded = true, true
		if _, err := link(filepath.Join(bin, "gp"), "gitpal"); err != nil {
			return res, err
		}
	}
	res.Dir = bin

	profile, err := ensureProfile(o.GOOS, o.Home)
	if err != nil {
		return res, err
	}
	if profile != "" {
		res.Changed, res.Profile = true, profile
	}
	return res, nil
}

// ours: the link at target points at the bundled program.
func ours(target, want string) bool {
	got, err := os.Readlink(target)
	return err == nil && got == want
}

// link makes target a symbolic link to want. An existing link is replaced only
// when it points at another copy of the app (the app was moved) or at nothing;
// a regular file is the user's own install and is kept.
func link(target, want string) (bool, error) {
	info, err := os.Lstat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return false, fmt.Errorf("cmdinstall: %w", err)
	case info.Mode()&os.ModeSymlink == 0:
		return false, nil
	default:
		got, err := os.Readlink(target)
		if err != nil {
			return false, fmt.Errorf("cmdinstall: %w", err)
		}
		if got == want {
			return false, nil
		}
		_, statErr := os.Stat(target)
		stale := strings.Contains(filepath.ToSlash(got), "gitpal.app/Contents/MacOS/") || statErr != nil
		if !stale {
			return false, nil
		}
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("cmdinstall: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // ~/.local/bin, kullanıcının program klasörü
		return false, fmt.Errorf("cmdinstall: %w", err)
	}
	if err := os.Symlink(want, target); err != nil {
		return false, fmt.Errorf("cmdinstall: %w", err)
	}
	return true, nil
}

// ensureProfile adds ~/.local/bin to PATH in the login shell's file unless one
// of the user's shell files already mentions it. It returns the file it
// changed, or "".
func ensureProfile(goos, home string) (string, error) {
	files := []string{".zprofile", ".zshrc", ".bash_profile", ".bashrc", ".profile"}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(home, f)) //nolint:gosec // kullanıcının kendi kabuk dosyası
		if err == nil && strings.Contains(string(b), ".local/bin") {
			return "", nil
		}
	}
	// macOS'ta varsayılan kabuk zsh; Linux'ta oturum açılışında ~/.profile okunur.
	primary, extra := ".zprofile", ".bash_profile"
	if goos != "darwin" {
		primary, extra = ".profile", ".bashrc"
	}
	if err := appendLine(filepath.Join(home, primary)); err != nil {
		return "", err
	}
	// Diğer kabuğun dosyası zaten varsa ona da eklenir; yoksa yaratılmaz.
	if _, err := os.Stat(filepath.Join(home, extra)); err == nil {
		if err := appendLine(filepath.Join(home, extra)); err != nil {
			return "", err
		}
	}
	return filepath.Join(home, primary), nil
}

func appendLine(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // kabuk dosyası; kullanıcının diğer dosyalarıyla aynı izin
	if err != nil {
		return fmt.Errorf("cmdinstall: %w", err)
	}
	_, err = f.WriteString(profileLine)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("cmdinstall: %w", err)
	}
	return nil
}

// gpCmd forwards gp to gitpal.exe in the same folder (a symbolic link needs
// administrator rights on Windows).
const gpCmd = "@echo off\r\nrem gp: gitpal icin kisa ad\r\n\"%~dp0gitpal.exe\" %*\r\n"

func ensureWindows(o Options) (Result, error) {
	var res Result
	cli := bundled(o, "gitpal.exe")
	if cli == "" {
		if o.Fetch == nil || o.ProgramsDir == "" {
			return res, nil
		}
		existing := filepath.Join(o.ProgramsDir, "gitpal.exe")
		if _, err := os.Stat(existing); err == nil {
			cli = existing
		} else {
			p, err := o.Fetch(o.ProgramsDir)
			if err != nil {
				return res, fmt.Errorf("cmdinstall: %w", err)
			}
			cli, res.Changed, res.Downloaded = p, true, true
		}
	}
	dir := filepath.Dir(cli)
	res.Dir = dir

	gp := filepath.Join(dir, "gp.cmd")
	if _, err := os.Stat(gp); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(gp, []byte(gpCmd), 0o644); err != nil { //nolint:gosec // komut dosyası, herkes okuyabilir
			return res, fmt.Errorf("cmdinstall: %w", err)
		}
		res.Changed = true
	}

	if o.UserPath == nil {
		return res, nil
	}
	current, err := o.UserPath.Get()
	if err != nil {
		return res, fmt.Errorf("cmdinstall: %w", err)
	}
	for _, p := range strings.Split(current, ";") {
		if strings.EqualFold(strings.TrimRight(p, `\/`), strings.TrimRight(dir, `\/`)) {
			return res, nil
		}
	}
	next := dir
	if strings.TrimSpace(current) != "" {
		next = strings.TrimRight(current, ";") + ";" + dir
	}
	if err := o.UserPath.Set(next); err != nil {
		return res, fmt.Errorf("cmdinstall: %w", err)
	}
	res.Changed = true
	return res, nil
}
