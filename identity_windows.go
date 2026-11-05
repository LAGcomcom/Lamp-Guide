//go:build windows

package main

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Setup checks this mutex before replacing or uninstalling a running build.
// Keep its handle alive until process exit, including gateway shutdown.
var appMutex windows.Handle

func prepareAppIdentity() error {
	name, err := windows.UTF16PtrFromString(`Local\LampGuide.Running`)
	if err != nil {
		return err
	}
	appMutex, err = windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return fmt.Errorf("create application mutex: %w", err)
	}
	appID, _ := windows.UTF16PtrFromString("LampGuide.Desktop")
	setAppID := windows.NewLazySystemDLL("shell32.dll").NewProc("SetCurrentProcessExplicitAppUserModelID")
	result, _, _ := setAppID.Call(uintptr(unsafe.Pointer(appID)))
	if int32(result) < 0 {
		return fmt.Errorf("set application identity: HRESULT %#x", result)
	}
	return nil
}
