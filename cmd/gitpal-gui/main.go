//go:build gui

// Command gitpal-gui is the desktop interface of gitpal: the same core and
// safety layer as the command line, shown in a native window.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/muhammedeminalan/gitpal/frontend"
	"github.com/muhammedeminalan/gitpal/internal/cmdinstall"
	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/gui"
	"github.com/muhammedeminalan/gitpal/internal/terminal"
	"github.com/muhammedeminalan/gitpal/internal/update"
	"github.com/muhammedeminalan/gitpal/internal/version"
)

func main() {
	// git starts gitpal-gui as its editor if an interactive rebase is begun
	// from it; answer the same way the command line does.
	if handled, code := git.RunEditorHelper(os.Args[1:]); handled {
		os.Exit(code)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gitpal-gui:", err)
		os.Exit(1)
	}
}

func run() error {
	assets, err := fs.Sub(frontend.Dist, "dist")
	if err != nil {
		return err
	}
	app := gui.New(gui.Deps{
		NewRunner: git.NewRunner,
		ConfigEnv: config.SystemEnv(),
		Secrets:   secrets.Keyring(),
		NewAI:     ai.New,
		PickFolder: func(ctx context.Context) (string, error) {
			return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{CanCreateDirectories: true})
		},
		OpenTerminal: openTerminal,
		CurrentVersion: func() string {
			// Güncelleme şeridini denemek için: GITPAL_UPDATE_CURRENT=1.0.0 gitpal-gui
			if v := os.Getenv("GITPAL_UPDATE_CURRENT"); v != "" {
				return v
			}
			return version.Version
		},
		DownloadDir: func() string {
			home, _ := os.UserHomeDir()
			return update.DownloadsDir(home)
		},
		OpenDownload:   func(path string) error { return update.Open(goruntime.GOOS, path) },
		EnsureCommands: ensureCommands,
	})
	// A path on the command line opens that repository straight away.
	if len(os.Args) > 1 {
		_, _ = app.OpenRepo(os.Args[1])
	}
	return wails.Run(&options.App{
		Title:       "gitpal",
		Width:       1280,
		Height:      800,
		MinWidth:    900,
		MinHeight:   560,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.Startup,
		// macOS ikonu .app paketinden, Windows ikonu .exe'ye gömülü kaynaktan gelir;
		// Linux'ta pencereye verilmesi gerekir. Vite, frontend/public'i dist'e kopyalar.
		Linux: &linux.Options{Icon: linuxIcon()},
		Bind:  []any{app},
	})
}

// linuxIcon, gömülü arayüzdeki logoyu döndürür; bulunamazsa pencere ikonsuz açılır.
func linuxIcon() []byte {
	icon, err := fs.ReadFile(frontend.Dist, "dist/gitpal-256.png")
	if err != nil {
		return nil
	}
	return icon
}

// openTerminal, sistemin terminalini dir'de açıp gitpal'ı başlatır. gitpal önce bu
// programın yanında aranır (macOS'ta .app paketinin içi): Finder'dan açılan
// uygulamanın PATH'i kısadır, Homebrew gibi yerleri görmez.
func openTerminal(dir, hint string) error {
	self, _ := os.Executable()
	gitpal := terminal.FindGitpal(goruntime.GOOS, self, func(p string) bool {
		info, err := os.Stat(p)
		return err == nil && info.Mode().IsRegular()
	}, exec.LookPath)
	return terminal.Open(terminal.Options{
		GOOS: goruntime.GOOS, Dir: dir, Gitpal: gitpal, Hint: hint,
		LookPath: exec.LookPath, Getenv: os.Getenv, Environ: os.Environ(),
	})
}

// ensureCommands, terminal komutlarını (gitpal, gp) kurar; paket gitpal'ı
// taşımıyorsa ve bu bir sürüm derlemesiyse aynı sürümün komut aracını indirir.
func ensureCommands() (cmdinstall.Result, error) {
	self, err := os.Executable()
	if err != nil {
		return cmdinstall.Result{}, err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return cmdinstall.Result{}, err
	}
	var fetch func(string) (string, error)
	if tag, ok := update.ReleaseTag(version.Version); ok {
		fetch = func(dest string) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			c := update.Client{}
			rel, err := c.ByTag(ctx, tag)
			if err != nil {
				return "", err
			}
			return c.FetchCLI(ctx, rel, goruntime.GOOS, goruntime.GOARCH, dest)
		}
	}
	programs := ""
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		programs = filepath.Join(local, "Programs", "gitpal")
	}
	return cmdinstall.Ensure(cmdinstall.Options{
		GOOS: goruntime.GOOS, Home: home, Self: self, Fetch: fetch,
		UserPath: cmdinstall.UserPath(), ProgramsDir: programs,
	})
}
