package gui

import (
	"errors"
	"os"
	"runtime"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/update"
	"github.com/muhammedeminalan/gitpal/internal/version"
)

// UpdateInfo is what the update banner needs.
type UpdateInfo struct {
	// Enabled is false when the user turned the check off (updates.check).
	Enabled bool `json:"enabled"`
	// Available: a newer release exists and this build's version is known.
	Available bool `json:"available"`
	// Known is false for a development build, whose version cannot be compared.
	Known   bool   `json:"known"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Notes   string `json:"notes"`
	Page    string `json:"page"`
	// HasPackage: the release has a package for this system.
	HasPackage bool `json:"hasPackage"`
}

// Downloaded is a saved update and what the user should do next.
type Downloaded struct {
	Path string `json:"path"`
	Hint string `json:"hint"`
}

// CommandsResult is what EnsureCommands did, for a one-time notice.
type CommandsResult struct {
	Changed bool   `json:"changed"`
	Dir     string `json:"dir"`
	Message string `json:"message"`
}

func (a *App) updates() *update.Client {
	if a.deps.Updates != nil {
		return a.deps.Updates
	}
	return &update.Client{}
}

func (a *App) platform() (string, string) {
	goos, goarch := a.deps.GOOS, a.deps.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

func (a *App) currentVersion() string {
	if a.deps.CurrentVersion != nil {
		return a.deps.CurrentVersion()
	}
	return version.Version
}

// CheckUpdate asks GitHub for the newest release. Nothing about the user is
// sent. With updates.check off it does not reach the network at all.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	a.mu.Lock()
	enabled, ctx := a.cfg.Updates.Check, a.ctx
	a.mu.Unlock()
	info := UpdateInfo{Enabled: enabled, Current: a.currentVersion()}
	if !enabled {
		return info, nil
	}
	rel, err := a.updates().Latest(ctx)
	if err != nil {
		return info, errors.New(i18n.T(a.lang(), i18n.MsgUpdateCheckFailed))
	}
	info.Latest, info.Notes, info.Page = rel.Version(), rel.Notes, rel.Page
	info.Available, info.Known = update.Newer(info.Current, rel.Tag)
	goos, goarch := a.platform()
	_, info.HasPackage = rel.App(goos, goarch)
	a.mu.Lock()
	a.latest = &rel
	a.mu.Unlock()
	return info, nil
}

// DownloadUpdate downloads the release CheckUpdate found, verifies it and shows
// it to the user. Installing it (replacing the app) is left to the user: the
// app does not replace itself while running.
func (a *App) DownloadUpdate() (Downloaded, error) {
	a.mu.Lock()
	rel, ctx := a.latest, a.ctx
	a.mu.Unlock()
	if rel == nil {
		if _, err := a.CheckUpdate(); err != nil {
			return Downloaded{}, err
		}
		a.mu.Lock()
		rel = a.latest
		a.mu.Unlock()
		if rel == nil {
			return Downloaded{}, errors.New(i18n.T(a.lang(), i18n.MsgUpdateCheckFailed))
		}
	}
	goos, goarch := a.platform()
	asset, ok := rel.App(goos, goarch)
	if !ok {
		return Downloaded{}, errors.New(i18n.T(a.lang(), i18n.MsgUpdateNoPackage, rel.Page))
	}
	dir := os.TempDir()
	if a.deps.DownloadDir != nil {
		dir = a.deps.DownloadDir()
	}
	path, err := a.updates().DownloadVerified(ctx, *rel, asset, dir)
	switch {
	case errors.Is(err, update.ErrChecksumMismatch), errors.Is(err, update.ErrNoChecksum):
		return Downloaded{}, errors.New(i18n.T(a.lang(), i18n.MsgUpdateNotVerified, rel.Page))
	case err != nil:
		return Downloaded{}, errors.New(i18n.T(a.lang(), i18n.MsgUpdateDownloadFailed, rel.Page))
	}
	if a.deps.OpenDownload != nil {
		_ = a.deps.OpenDownload(path) // açılamasa da dosya yerinde; ipucu yolu gösterir
	}
	key := i18n.MsgUpdateHintLinux
	switch goos {
	case "darwin":
		key = i18n.MsgUpdateHintMac
	case "windows":
		key = i18n.MsgUpdateHintWindows
	}
	return Downloaded{Path: path, Hint: i18n.T(a.lang(), key, path)}, nil
}

// EnsureCommands makes the terminal commands available (see cmdinstall). It
// runs on every start; the message is only set when something changed. A
// failure is not shown as an error: the app works without the commands, and
// the Terminal button finds gitpal by itself.
func (a *App) EnsureCommands() CommandsResult {
	if a.deps.EnsureCommands == nil {
		return CommandsResult{}
	}
	res, err := a.deps.EnsureCommands()
	if err != nil || !res.Changed {
		return CommandsResult{Dir: res.Dir}
	}
	return CommandsResult{Changed: true, Dir: res.Dir, Message: i18n.T(a.lang(), i18n.MsgCommandsInstalled, res.Dir)}
}
