//go:build gui

// Command gitpal-gui is the desktop interface of gitpal: the same core and
// safety layer as the command line, shown in a native window.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/muhammedeminalan/gitpal/frontend"
	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/gui"
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
