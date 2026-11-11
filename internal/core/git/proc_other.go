//go:build !windows

package git

import "os/exec"

// configureProcess adjusts how git is started on platforms that need it. Only
// Windows does (see proc_windows.go); elsewhere a child process never opens a
// window of its own.
func configureProcess(*exec.Cmd) {}
