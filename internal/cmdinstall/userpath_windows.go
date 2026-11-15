//go:build windows

package cmdinstall

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// UserPath is the current user's PATH in the registry (HKCU\Environment).
// Only the user's own PATH is changed; no administrator rights are needed.
func UserPath() PathEditor { return regPath{} }

type regPath struct{}

func (regPath) Get() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	v, _, err := k.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	return v, err
}

func (regPath) Set(value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// REG_EXPAND_SZ: PATH'teki %USERPROFILE% gibi değişkenler açılmaya devam etsin.
	if err := k.SetExpandStringValue("Path", value); err != nil {
		return err
	}
	broadcastEnvironmentChange()
	return nil
}

// broadcastEnvironmentChange tells Explorer the environment changed, so
// terminals opened from now on see the new PATH without signing out.
func broadcastEnvironmentChange() {
	const (
		hwndBroadcast    = 0xFFFF
		wmSettingChange  = 0x001A
		smtoAbortIfHung  = 0x0002
		timeoutMillisecs = 5000
	)
	env, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	proc := syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW")
	_, _, _ = proc.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)), smtoAbortIfHung, timeoutMillisecs, 0) //nolint:gosec // Win32 API çağrısı
}
