//go:build windows

package git

import (
	"os/exec"
	"sync"
	"syscall"
)

// createNoWindow is the Windows CREATE_NO_WINDOW process creation flag: the
// child gets a console that has no window, and its own children (ssh,
// credential helpers) inherit that console instead of opening one each.
const createNoWindow = 0x08000000

// hasConsoleWindow reports whether this process has a console window: true in
// a terminal, false in gitpal-gui, which is linked as a windows-subsystem
// program (-H windowsgui) so that starting it does not open a console.
// It is a variable so tests can run git the way the desktop app does.
var hasConsoleWindow = sync.OnceValue(func() bool {
	window, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	return window != 0
})

// configureProcess keeps git from opening console windows. A program without
// a console that starts a console program makes Windows create, and show, a
// new console window for it — for the desktop app that would be one flashing
// window per git command.
func configureProcess(c *exec.Cmd) {
	c.SysProcAttr = processAttr(hasConsoleWindow())
}

// processAttr returns how to start git. With a console (the command line) git
// is started the ordinary way and shares the terminal, which is where ssh and
// git prompt. Without one it is given a windowless console.
func processAttr(console bool) *syscall.SysProcAttr {
	if console {
		return nil
	}
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
