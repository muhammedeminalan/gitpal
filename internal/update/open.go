package update

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// DownloadsDir is where updates are saved: the user's Downloads folder when it
// exists, the temporary folder otherwise.
func DownloadsDir(home string) string {
	if home != "" {
		d := filepath.Join(home, "Downloads")
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	return os.TempDir()
}

// openCommand: macOS opens the file itself (a .dmg mounts and shows its
// window), Windows shows it selected in Explorer, Linux opens its folder.
func openCommand(goos, file string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{file}
	case "windows":
		return "explorer.exe", []string{"/select," + file}
	default:
		dir := path.Dir(strings.ReplaceAll(file, `\`, "/"))
		return "xdg-open", []string{dir}
	}
}

// Open shows a downloaded update to the user.
func Open(goos, file string) error {
	name, args := openCommand(goos, file)
	cmd := exec.Command(name, args...) //nolint:gosec,forbidigo,noctx // sistemin dosya açıcısı; yol ayrı bir argüman, kabuk yok
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return cmd.Process.Release()
}
