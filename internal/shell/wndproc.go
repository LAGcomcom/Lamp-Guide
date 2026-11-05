package shell

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"zenui/internal/native"
)

// wndProc is the window procedure.
//
// Go pointers never cross into native memory: the Window is resolved from the
// registry map by HWND, which is how per-window state survives the callback.
func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	w := lookup(windows.Handle(hwnd))
	if w == nil {
		// Messages that arrive before CreateWindowExW returns (WM_NCCREATE)
		// have no registry entry yet.
		r, _, _ := native.User32DefWindowProcW.Call(hwnd, msg, wparam, lparam)
		return r
	}

	switch msg {
	case native.WM_NCCALCSIZE:
		// wParam TRUE means "recompute the client area"; returning 0 keeps the
		// client area equal to the whole window rect. That removes the caption
		// while WS_THICKFRAME still supplies DWM's shadow, corners and snap.
		if wparam == 0 {
			break
		}
		w.OnFrameChanged()
		return 0

	case native.WM_NCHITTEST:
		return w.hitTest(lparam)

	case native.WM_GETMINMAXINFO:
		// DefWindowProc has to run first: the maximize size and position fields
		// are filled by it. Returning without that call leaves ptMaxSize /
		// ptMaxPosition at whatever the caller handed over, and a maximized
		// borderless window then lands partly outside the work area — measured
		// here at 2580×1460 on a 2560×1440 screen, offset +1230,+840, so the
		// page's right and bottom edge were off the monitor.
		r, _, _ := native.User32DefWindowProcW.Call(hwnd, msg, wparam, lparam)
		w.fitMaximized(lparam)
		return r

	case native.WM_SIZE:
		width := int32(lparam & 0xFFFF)
		height := int32(uint16(lparam >> 16))
		if width > 0 && height > 0 && w.OnResize != nil {
			w.OnResize(width, height)
		}

	case native.WM_DPICHANGED:
		w.refreshScale()
		var rect windows.Rect
		native.ReadStruct(lparam, unsafe.Pointer(&rect), unsafe.Sizeof(rect))
		native.User32SetWindowPos.Call(
			hwnd, 0,
			uintptr(rect.Left), uintptr(rect.Top),
			uintptr(rect.Right-rect.Left), uintptr(rect.Bottom-rect.Top),
			native.SWP_NOZORDER|native.SWP_NOACTIVATE,
		)
		if w.OnResize != nil {
			width, height := w.ClientRect()
			w.OnResize(width, height)
		}

	case native.WM_SETTINGCHANGE:
		w.OnSystemThemeChanged()

	case native.WM_CLOSE:
		if w.OnClose != nil && !w.OnClose() {
			return 0
		}

	case native.WM_DESTROY:
		// DestroyWindow does not post a quit message on its own; without this the
		// message pump blocks forever after the window is gone.
		native.User32PostQuitMessage.Call(0)

	case native.WM_NCDESTROY:
		windowsMu.Lock()
		delete(wndRegistry, windows.Handle(hwnd))
		windowsMu.Unlock()
	}

	r, _, _ := native.User32DefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// fitMaximized rewrites the maximize geometry for a window whose client area
// is the whole window rect.
//
// A captioned window is maximized to the work area *expanded* by the resize
// border, because that border is supposed to hang off-screen. This window has
// no non-client area at all — the page is the client rect — so the same expansion
// pushes the bottom-right of the layout past the monitor edge and leaves a
// scrollbar-sized strip of the page unreachable. Pinning ptMaxSize / ptMaxPosition
// to the work area itself makes "最大化" mean what the page measures.
func (w *Window) fitMaximized(lparam uintptr) {
	if lparam == 0 {
		return
	}
	var mmi native.MinMaxInfo
	native.ReadStruct(lparam, unsafe.Pointer(&mmi), unsafe.Sizeof(mmi))

	monitor, _, _ := native.User32MonitorFromWindow.Call(
		uintptr(w.Hwnd), native.MONITOR_DEFAULTTONEAREST)
	if monitor != 0 {
		var info native.MonitorInfo
		info.CbSize = uint32(unsafe.Sizeof(info))
		if r, _, _ := native.User32GetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); r != 0 {
			work := info.RcWork
			mmi.MaxPosition = native.Point{X: work.Left, Y: work.Top}
			mmi.MaxSize = native.Point{X: work.Right - work.Left, Y: work.Bottom - work.Top}
			// MaxTrackSize is the largest size a drag-to-resize may reach. The
			// default is monitor+border again; the monitor itself is the real
			// ceiling once nothing hangs off-screen.
			if mmi.MaxTrackSize.X <= 0 || mmi.MaxTrackSize.Y <= 0 {
				mmi.MaxTrackSize = native.Point{
					X: info.RcMonitor.Right - info.RcMonitor.Left,
					Y: info.RcMonitor.Bottom - info.RcMonitor.Top,
				}
			}
		}
	}

	if w.minTrack.X > 0 && w.minTrack.Y > 0 {
		mmi.MinTrackSize = w.minTrack
	}

	// The struct belongs to the caller, so it is written through RtlMoveMemory
	// rather than by reinterpreting lparam as a Go pointer.
	native.WriteStruct(lparam, unsafe.Pointer(&mmi), unsafe.Sizeof(mmi), 0)
}

func lookup(hwnd windows.Handle) *Window {
	windowsMu.Lock()
	defer windowsMu.Unlock()
	return wndRegistry[hwnd]
}

// hitTest splits the window into resize borders, the draggable title band and
// the interactive caption buttons.
//
// lParam arrives in **screen** coordinates — that is the WM_NCHITTEST contract,
// and reading it as client coordinates is what silently disabled dragging:
// on a 150% display the band is 57 physical pixels tall, while the window sat at
// y=66 on screen, so every point inside the title bar compared as y≈86 and fell
// through to HTCLIENT. The whole bar looked inert and nothing logged an error.
func (w *Window) hitTest(lparam uintptr) uintptr {
	x := int32(int16(lparam & 0xFFFF))
	y := int32(int16(uint16(lparam >> 16)))

	pt := native.Point{X: x, Y: y}
	native.User32ScreenToClient.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&pt)))

	width, height := w.ClientRect()
	return w.classify(native.Point{X: pt.X, Y: pt.Y}, width, height)
}

// classify maps a client-area point to a hit code.
//
// Split out from hitTest so the geometry can be tested without a live window:
// every branch here is a pixel contract, and a wrong one is invisible in code
// review — the band simply stops dragging, or the buttons stop clicking.
func (w *Window) classify(pt native.Point, width, height int32) uintptr {
	x, y := pt.X, pt.Y

	scale := w.Scale()

	frame, _, _ := native.User32GetSystemMetricsForDpi.Call(native.SM_CXSIZEFRAME, uintptr(96*scale))
	padded, _, _ := native.User32GetSystemMetricsForDpi.Call(native.SM_CXPADDEDBORDER, uintptr(96*scale))
	border := int32(frame) + int32(padded)
	if frame == 0 {
		border = int32(8 * scale)
	}

	// Corners first, so diagonal resize wins over the straight edges.
	switch {
	case x < border && y < border:
		return native.HTTOPLEFT
	case x >= width-border && y < border:
		return native.HTTOPRIGHT
	case x < border && y >= height-border:
		return native.HTBOTTOMLEFT
	case x >= width-border && y >= height-border:
		return native.HTBOTTOMRIGHT
	case x < border:
		return native.HTLEFT
	case x >= width-border:
		return native.HTRIGHT
	case y < border:
		return native.HTTOP
	case y >= height-border:
		return native.HTBOTTOM
	}

	hit := w.hitRegions()
	band := int32(float64(hit.TitleBarHeight) * scale)
	if band > 0 && y < band {
		rightInset := int32(float64(hit.RightInset) * scale)
		leftInset := int32(float64(hit.LeftInset) * scale)
		if (rightInset > 0 && x >= width-rightInset) || (leftInset > 0 && x < leftInset) {
			return native.HTCLIENT // the page handles its own buttons
		}
		return native.HTCAPTION
	}

	return native.HTCLIENT
}
