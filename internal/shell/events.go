package shell

import (
	"zenui/internal/native"
)

// OnFrameChanged re-applies the DWM frame attributes. Windows drops them
// whenever the non-client area is recomputed, so WM_NCCALCSIZE has to put them
// back or the Mica backdrop disappears after the first resize.
func (w *Window) OnFrameChanged() {
	w.applyBackdrop()
}

// OnSystemThemeChanged forwards the OS light/dark switch to the embedder, which
// relays it to the page so the HTML can flip themes without a restart.
func (w *Window) OnSystemThemeChanged() {
	dark := systemIsDark()
	native.SetDarkFrame(w.Hwnd, dark)
	if w.OnThemeChanged != nil {
		w.OnThemeChanged(dark)
	}
}
