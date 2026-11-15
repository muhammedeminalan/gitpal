//go:build !windows

package terminal

import (
	"fmt"
	"os/exec"
)

func start(p Plan) error {
	// Kullanıcının kendi terminali; argümanlar ayrı ayrı verilir, hiçbir değer
	// bir kabuk komut satırına eklenmez (betik dosyası ve sh -c içindeki değerler
	// shQuote ile korunur).
	cmd := exec.Command(p.Name, p.Args...) //nolint:gosec,forbidigo,noctx // sistemin terminali; pencere bu programdan bağımsız yaşar
	cmd.Dir = p.Dir
	if p.Env != nil {
		cmd.Env = p.Env
	}
	if p.Script != "" {
		// macOS `open` hemen döner; hata olursa (ör. .command'ı açacak uygulama yok) görülsün.
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("terminal: %w: %s", err, out)
		}
		return nil
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("terminal: %w", err)
	}
	return cmd.Process.Release()
}
