package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// ErrNoGUI is what Env.LaunchGUI returns when the desktop app is not
// installed.
var ErrNoGUI = errors.New("gitpal-gui not found")

func (a *app) newGUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gui",
		Short: i18n.T(a.lang, i18n.MsgGUIShort),
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			// Outside a repository the window opens on its welcome screen.
			if err := a.env.LaunchGUI(a.session.repoRoot); err != nil {
				if errors.Is(err, ErrNoGUI) {
					return errors.New(i18n.T(a.lang, i18n.MsgGUINotInstalled))
				}
				return err
			}
			return nil
		},
	}
}

// launchGUI starts gitpal-gui on dir and returns without waiting for the
// window to close. The program installed next to gitpal wins over one on
// PATH, so the two always come from the same release.
func launchGUI(dir string) error {
	name := "gitpal-gui"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := ""
	if self, err := os.Executable(); err == nil {
		if candidate := filepath.Join(filepath.Dir(self), name); isFile(candidate) {
			path = candidate
		}
	}
	if path == "" {
		found, err := exec.LookPath(name)
		if err != nil {
			return ErrNoGUI
		}
		path = found
	}
	var args []string
	if dir != "" {
		args = append(args, dir)
	}
	// Not tied to a context: the window outlives this command.
	cmd := exec.Command(path, args...) //nolint:gosec,forbidigo,noctx // gitpal's own desktop app; the repository path is a single argument, no shell
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
