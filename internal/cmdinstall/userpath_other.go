//go:build !windows

package cmdinstall

// UserPath is only used on Windows; elsewhere PATH lives in shell files.
func UserPath() PathEditor { return nil }
