package terminal

import (
	"fmt"
	"os"
)

// Open terminali açar ve beklemeden döner; pencere bu programdan bağımsız yaşar.
func Open(o Options) error {
	p, err := plan(o)
	if err != nil {
		return err
	}
	if p.Script != "" {
		path, err := writeScript(p.Script, p.ScriptExt)
		if err != nil {
			return err
		}
		p.Args = append(p.Args, path)
	}
	return start(p)
}

// writeScript, betiği yalnızca kullanıcının okuyup çalıştırabileceği geçici bir
// dosyaya yazar. Betik çalışınca kendini siler.
func writeScript(script, ext string) (string, error) {
	f, err := os.CreateTemp("", "gitpal-terminal-*"+ext)
	if err != nil {
		return "", fmt.Errorf("terminal: %w", err)
	}
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("terminal: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("terminal: %w", err)
	}
	if err := os.Chmod(f.Name(), 0o700); err != nil { //nolint:gosec // betik çalıştırılabilir olmalı; yalnızca sahibi
		return "", fmt.Errorf("terminal: %w", err)
	}
	return f.Name(), nil
}
