// Package shell creates the native window and hosts the WebView2 child window.
//
// The window is borderless: FrameStyles keeps WS_THICKFRAME so DWM still gives
// drop shadow, rounded corners, snap layouts and resize borders, but the caption
// is removed in WM_NCCALCSIZE so the client area covers the full window. The
// page draws the title bar, and WM_NCHITTEST maps the band the page reports as
// draggable to HTCAPTION (see HitRegions).
package shell

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"zenui/internal/native"
)

// HitRegions describes where the page put the draggable title band.
//
// The page pushes this from JS after layout: the band height in DIP, plus the
// insets occupied by the logo / window buttons, which must stay clickable.
type HitRegions struct {
	TitleBarHeight int32 // DIP
	LeftInset      int32 // DIP
	RightInset     int32 // DIP
}

// Window is the borderless top-level window.
type Window struct {
	Hwnd windows.Handle

	scaleMu sync.RWMutex
	scale   float64

	hitMu sync.RWMutex
	hit   HitRegions

	// minTrack is the smallest size the user can drag to, in physical pixels.
	// It is filled from WM_GETMINMAXINFO, so it must be read on the UI thread.
	minTrack native.Point

	// pendingSize is the requested size in DIP, applied once the monitor scale
	// is known (see Show / refit).
	pendingSize [2]int

	// shown makes Show idempotent: the shell shows the frame before it builds
	// the WebView2 environment, and Run shows it again once the page is ready.
	shown bool

	// OnResize fires from the UI thread whenever the client area changes.
	OnResize func(width, height int32)
	// OnClose fires on WM_CLOSE; return false to keep the window alive, true to
	// let the default destruction run.
	OnClose func() bool
	// OnThemeChanged fires when Windows switches apps between light and dark.
	OnThemeChanged func(dark bool)

	className *uint16
}

var (
	classOnce sync.Once
	classErr  error

	windowsMu   sync.Mutex
	wndRegistry = map[windows.Handle]*Window{}
)

const className = "ZenUI.Shell"

// New creates the window hidden. Call Show once the WebView2 child window is
// embedded, so the first frame the user sees is already painted.
func New(title string, widthDIP, heightDIP int) (*Window, error) {
	native.SetProcessDpiAwareness()

	w := &Window{hit: HitRegions{TitleBarHeight: 40, LeftInset: 0, RightInset: 138}}
	w.scale = float64(native.SystemDPI()) / 96.0

	cls, err := windows.UTF16PtrFromString(className)
	if err != nil {
		return nil, err
	}
	w.className = cls

	if err := registerClass(cls); err != nil {
		return nil, err
	}

	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return nil, err
	}

	hinstance, _, _ := native.Kernel32GetModuleHandleW.Call(0)

	width := uintptr(float64(widthDIP) * w.scale)
	height := uintptr(float64(heightDIP) * w.scale)

	hwnd, _, errno := native.User32CreateWindowExW.Call(
		uintptr(native.FrameExStyles),
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(native.FrameStyles),
		uintptr(native.CW_USEDEFAULT),
		uintptr(native.CW_USEDEFAULT),
		width,
		height,
		0, 0, hinstance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW failed: %w", errno)
	}
	w.Hwnd = windows.Handle(hwnd)

	windowsMu.Lock()
	wndRegistry[w.Hwnd] = w
	windowsMu.Unlock()

	// GetDpiForSystem is useless here: under per-monitor-v2 awareness it always
	// reports 96, while the monitor may be scaled. The window's own DPI is the
	// only reliable source, and it is available right after creation — so the
	// window is resized to the requested DIP size here instead of at
	// CreateWindowExW time.
	// GetDpiForWindow answers 0 until the window has been shown at least once,
	// so the size is applied again from Show (see refit), where the real
	// per-monitor scale is known.
	w.applyBackdrop()
	w.pendingSize = [2]int{widthDIP, heightDIP}
	return w, nil
}

func registerClass(cls *uint16) error {
	classOnce.Do(func() {
		hinstance, _, _ := native.Kernel32GetModuleHandleW.Call(0)

		wc := native.WndClassExW{
			CbSize:        uint32(unsafe.Sizeof(native.WndClassExW{})),
			HInstance:     windows.Handle(hinstance),
			LpszClassName: cls,
			LpfnWndProc:   windows.NewCallback(wndProc),
		}
		icow, _, _ := native.User32GetSystemMetrics.Call(native.SM_CXICON)
		icoh, _, _ := native.User32GetSystemMetrics.Call(native.SM_CYICON)
		wc.HIcon = loadAppIcon(hinstance, icow, icoh)
		smallw, _, _ := native.User32GetSystemMetrics.Call(native.SM_CXSMICON)
		smallh, _, _ := native.User32GetSystemMetrics.Call(native.SM_CYSMICON)
		wc.HIconSm = loadAppIcon(hinstance, smallw, smallh)

		r, _, err := native.User32RegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		if r == 0 {
			classErr = fmt.Errorf("RegisterClassExW failed: %w", err)
		}
	})
	return classErr
}

func loadAppIcon(instance, width, height uintptr) windows.Handle {
	name, _ := windows.UTF16PtrFromString("APP")
	icon, _, _ := native.User32LoadImageW.Call(instance, uintptr(unsafe.Pointer(name)),
		native.IMAGE_ICON, width, height, native.LR_SHARED)
	if icon == 0 {
		// Development builds without generated resources retain a system icon.
		icon, _, _ = native.User32LoadImageW.Call(0, native.IDI_APPLICATION,
			native.IMAGE_ICON, width, height, native.LR_SHARED)
	}
	return windows.Handle(icon)
}

// applySizeDIP sets the window size from DIP using the current monitor scale,
// so a 980x690 layout request matches what the page lays out at 100% zoom.
func (w *Window) applySizeDIP(widthDIP, heightDIP int) {
	scale := w.Scale()
	width := int32(float64(widthDIP) * scale)
	height := int32(float64(heightDIP) * scale)
	native.User32SetWindowPos.Call(
		uintptr(w.Hwnd), 0, 0, 0,
		uintptr(width), uintptr(height),
		native.SWP_NOMOVE|native.SWP_NOZORDER|native.SWP_NOACTIVATE,
	)
}

// centerOnMonitor places the window in the middle of its monitor's work area.
func (w *Window) centerOnMonitor() {
	monitor, _, _ := native.User32MonitorFromWindow.Call(
		uintptr(w.Hwnd), native.MONITOR_DEFAULTTONEAREST)
	if monitor == 0 {
		return
	}
	var info native.MonitorInfo
	info.CbSize = uint32(unsafe.Sizeof(info))
	r, _, _ := native.User32GetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return
	}
	var wr windows.Rect
	native.User32GetWindowRect.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&wr)))
	width := wr.Right - wr.Left
	height := wr.Bottom - wr.Top

	x := info.RcWork.Left + ((info.RcWork.Right-info.RcWork.Left)-width)/2
	y := info.RcWork.Top + ((info.RcWork.Bottom-info.RcWork.Top)-height)/2

	native.User32SetWindowPos.Call(
		uintptr(w.Hwnd), 0,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		native.SWP_NOZORDER|native.SWP_NOACTIVATE,
	)
}

// Show makes the window visible, then fits it to the requested DIP size.
//
// The refit has to happen after the window is on screen: Win32 only reports the
// real per-monitor DPI once the window has a monitor, and sizing the window
// before that produces a window 1.5x too small on scaled displays — the page
// lays out at N DIP while only N/1.5 DIP is visible, so the right edge of the
// layout gets clipped.
func (w *Window) Show() {
	if w.shown {
		return
	}
	w.shown = true

	native.User32ShowWindow.Call(uintptr(w.Hwnd), native.SW_SHOW)
	native.User32UpdateWindow.Call(uintptr(w.Hwnd))
	native.User32SetForegroundWindow.Call(uintptr(w.Hwnd))

	if w.pendingSize[0] > 0 {
		w.refit()
	}
}

// refit re-applies the DIP size with the now-known monitor scale.
func (w *Window) refit() {
	w.refreshScale()

	w.applySizeDIP(w.pendingSize[0], w.pendingSize[1])
	w.centerOnMonitor()
	if w.OnResize != nil {
		width, height := w.ClientRect()
		w.OnResize(width, height)
	}
}

func (w *Window) Hide() {
	native.User32ShowWindow.Call(uintptr(w.Hwnd), native.SW_HIDE)
}

func (w *Window) Destroy() {
	native.User32DestroyWindow.Call(uintptr(w.Hwnd))
}

// Scale is the DPI factor of the monitor the window is currently on.
func (w *Window) Scale() float64 {
	w.scaleMu.RLock()
	defer w.scaleMu.RUnlock()
	return w.scale
}

func (w *Window) refreshScale() {
	dpi, _, _ := native.User32GetDpiForWindow.Call(uintptr(w.Hwnd))
	if dpi == 0 {
		return
	}
	w.scaleMu.Lock()
	w.scale = float64(dpi) / 96.0
	w.scaleMu.Unlock()
}

// ClientRect returns the client area in physical pixels.
func (w *Window) ClientRect() (width, height int32) {
	var rc windows.Rect
	native.User32GetClientRect.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&rc)))
	return rc.Right - rc.Left, rc.Bottom - rc.Top
}

// SetHitRegions updates the draggable band and the caption-button insets. The
// values are DIP and converted on use, so a DPI change does not invalidate them.
func (w *Window) SetHitRegions(h HitRegions) {
	w.hitMu.Lock()
	w.hit = h
	w.hitMu.Unlock()
}

func (w *Window) hitRegions() HitRegions {
	w.hitMu.RLock()
	defer w.hitMu.RUnlock()
	return w.hit
}

// StartDrag enters the operating system's caption move loop.
//
// The page cannot reach this through WM_NCHITTEST: the WebView2 host child
// covers the whole client area and answers HTCLIENT for every point, so the
// parent window never sees the press. Mouse buttons do reach the page, so the
// page asks for the loop and Windows runs the same modal move it would have run
// from a real caption — Aero Snap and double-click-maximize included.
//
// Two details are load-bearing:
//
//   - It must run on the UI thread. Mouse capture is per-thread, so
//     ReleaseCapture from a bridge goroutine releases that goroutine's capture,
//     never the one the child window holds. SetCapture here takes the capture
//     back, which is the state the move loop expects to start from.
//   - lParam carries the cursor position, not zero. The loop reads it to work
//     out the drag origin; passing 0 makes the window snap to screen origin
//     before it starts following the mouse.
func (w *Window) StartDrag() {
	w.beginSystemMove(native.HTCAPTION)
}

// StartResize enters the OS resize loop for one edge or corner.
//
// Same reason as StartDrag: DWM paints the resize border around the window, but
// the hit that would start the loop belongs to the WebView child, so dragging
// the visible edge did nothing at all.
func (w *Window) StartResize(edge string) {
	if code := resizeCode(edge); code != 0 {
		w.beginSystemMove(code)
	}
}

// beginSystemMove hands the drag over to Windows for the given hit code.
//
// SendMessageW rather than PostMessageW: WM_NCLBUTTONDOWN starts a modal loop
// that does not return until the user releases the button, and posting it would
// let the bridge reply "ok" to a drag that is still running.
func (w *Window) beginSystemMove(hit uintptr) {
	hwnd := uintptr(w.Hwnd)
	native.User32SetCapture.Call(hwnd)
	native.User32ReleaseCapture.Call()

	var pt native.Point
	native.User32GetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	lparam := uintptr(uint32(pt.X)&0xFFFF | (uint32(pt.Y)&0xFFFF)<<16)

	native.User32SendMessageW.Call(hwnd, native.WM_NCLBUTTONDOWN, hit, lparam)
}

// resizeCode maps the page's edge name to the matching HT* constant.
//
// An unknown name returns 0 rather than a default: starting an arbitrary
// resize loop on a typo would look like "the window jumped for no reason".
func resizeCode(edge string) uintptr {
	switch edge {
	case "left":
		return native.HTLEFT
	case "right":
		return native.HTRIGHT
	case "top":
		return native.HTTOP
	case "bottom":
		return native.HTBOTTOM
	case "top-left":
		return native.HTTOPLEFT
	case "top-right":
		return native.HTTOPRIGHT
	case "bottom-left":
		return native.HTBOTTOMLEFT
	case "bottom-right":
		return native.HTBOTTOMRIGHT
	}
	return 0
}

// SetMinTrackDIP sets the smallest window size the user can drag to.
func (w *Window) SetMinTrackDIP(widthDIP, heightDIP int32) {
	scale := w.Scale()
	w.minTrack = native.Point{
		X: int32(float64(widthDIP) * scale),
		Y: int32(float64(heightDIP) * scale),
	}
}

// IsMaximized reports the zoomed state (the page draws the restore glyph).
func (w *Window) IsMaximized() bool {
	r, _, _ := native.User32IsZoomed.Call(uintptr(w.Hwnd))
	return r != 0
}

func (w *Window) applyBackdrop() {
	hwnd := w.Hwnd
	native.ExtendFrameIntoClientArea(hwnd)
	native.SetMicaBackdrop(hwnd)
	native.SetRoundedFrame(hwnd)
	native.ClearFrameBorder(hwnd)
	native.SetDarkFrame(hwnd, systemIsDark())
}
