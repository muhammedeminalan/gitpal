//go:build windows

package terminal

import (
	"fmt"
	"os/exec"
	"syscall"
)

// createNewConsole: terminal yeni bir pencerede açılır. Windows 11'de varsayılan
// terminal Windows Terminal seçiliyse pencere orada açılır.
const createNewConsole = 0x00000010

func start(p Plan) error {
	cmd := exec.Command(p.Name) //nolint:gosec,forbidigo,noctx // sistemin komut istemcisi; pencere bu programdan bağımsız yaşar
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: p.CmdLine, CreationFlags: createNewConsole}
	cmd.Dir = p.Dir
	cmd.Env = p.Env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("terminal: %w", err)
	}
	return cmd.Process.Release()
}
