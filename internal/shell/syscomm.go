package shell

import "zenui/internal/native"

const (
	WM_SYSCOMMAND = 0x0112
	SC_MINIMIZE   = 0xF020
	SC_MAXIMIZE   = 0xF030
	SC_RESTORE    = 0xF120
)

// ShowMinimized mirrors the caption's minimize button.
//
// SC_MINIMIZE is posted rather than sent so the command arrives on the UI
// thread even when it originates from a bridge goroutine.
func (w *Window) ShowMinimized() {
	native.User32PostMessageW.Call(uintptr(w.Hwnd), WM_SYSCOMMAND, SC_MINIMIZE, 0)
}

// ToggleMaximize flips between zoomed and restored, matching the caption button.
func (w *Window) ToggleMaximize() {
	if w.IsMaximized() {
		native.User32PostMessageW.Call(uintptr(w.Hwnd), WM_SYSCOMMAND, SC_RESTORE, 0)
		return
	}
	native.User32PostMessageW.Call(uintptr(w.Hwnd), WM_SYSCOMMAND, SC_MAXIMIZE, 0)
}
