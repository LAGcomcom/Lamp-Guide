// Package native hosts the raw Win32 / DWM bindings the shell needs.
//
// Only the subset actually used by the window frame is declared here; the point
// is to keep every magic constant in one place so shell code never spells a hex
// literal inline.
package native

import (
	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------- window msgs

const (
	WM_NCLBUTTONDOWN = 0x00A1
	WM_NCDESTROY     = 0x0082
	WM_NCCALCSIZE    = 0x0083
	WM_NCHITTEST     = 0x0084
	WM_GETMINMAXINFO = 0x0024
	WM_SIZE          = 0x0005
	WM_CLOSE         = 0x0010
	WM_DESTROY       = 0x0002
	WM_SETTINGCHANGE = 0x001A
	WM_DPICHANGED    = 0x02E0
	WM_APP           = 0x8000
)

// ----------------------------------------------------------------- hit testing

const (
	// HTTRANSPARENT is -1 as a hit-test reply: "keep looking underneath".
	// The WebView2 child returns it for the title band so the parent window gets
	// the message and performs the caption drag / resize itself.
	HTTRANSPARENT = -1
	HTCLIENT      = 1
	HTCAPTION     = 2
	HTLEFT        = 10
	HTRIGHT       = 11
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	HTBOTTOM      = 15
	HTBOTTOMLEFT  = 16
	HTBOTTOMRIGHT = 17
)

// -------------------------------------------------------------------- styles

const (
	WS_POPUP                  = 0x80000000
	WS_SYSMENU                = 0x00080000
	WS_THICKFRAME             = 0x00040000
	WS_MINIMIZEBOX            = 0x00020000
	WS_MAXIMIZEBOX            = 0x00010000
	WS_CLIPCHILDREN           = 0x02000000
	WS_EX_APPWINDOW           = 0x00040000
	WS_EX_NOREDIRECTIONBITMAP = 0x00200000

	// FrameStyles: thick frame (keeps DWM drop shadow, rounded corners, snap
	// layouts and resize borders) with no caption — the client area is the
	// whole window and HTML draws the title bar.
	FrameStyles = WS_POPUP | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_SYSMENU | WS_CLIPCHILDREN

	// FrameExStyles keeps the window in the taskbar and alt-tab.
	//
	// WS_EX_NOREDIRECTIONBITMAP is deliberately absent: that flag removes the
	// window's redirection surface, and a WebView2 child can only composite
	// into it. Without it the page never reaches the screen (the window paints
	// its background colour and stays blank). Mica behind the page therefore
	// comes from the page's own background, not from DWM.
	FrameExStyles = WS_EX_APPWINDOW
)

const (
	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010

	SW_HIDE = 0
	SW_SHOW = 5

	CW_USEDEFAULT = 0x80000000
)

const (
	MONITOR_DEFAULTTONEAREST = 2

	SM_CXICON         = 11
	SM_CYICON         = 12
	SM_CXSMICON       = 49
	SM_CYSMICON       = 50
	SM_CXSIZEFRAME    = 32
	SM_CXPADDEDBORDER = 92
)

const (
	IDI_APPLICATION = 32512
	IMAGE_ICON      = 1
	LR_DEFAULTSIZE  = 0x0040
	LR_SHARED       = 0x8000
)

// ------------------------------------------------------------------------ DWM

const (
	DWMWA_USE_IMMERSIVE_DARK_MODE  = 20
	DWMWA_WINDOW_CORNER_PREFERENCE = 33
	DWMWA_BORDER_COLOR             = 34
	DWMWA_SYSTEMBACKDROP_TYPE      = 38

	DWMWCP_ROUND = 2

	DWMSBT_MAINWINDOW = 2 // Mica

	DWMWA_COLOR_NONE = 0xFFFFFFFE
)

// ------------------------------------------------------------------- structs

// Point mirrors POINT.
type Point struct {
	X int32
	Y int32
}

// MinMaxInfo mirrors MINMAXINFO; x/sys/windows does not declare it.
type MinMaxInfo struct {
	Reserved     Point
	MaxSize      Point
	MaxPosition  Point
	MinTrackSize Point
	MaxTrackSize Point
}

// Msg mirrors MSG.
type Msg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
	Private uint32
}

// MonitorInfo mirrors MONITORINFO.
type MonitorInfo struct {
	CbSize    uint32
	RcMonitor windows.Rect
	RcWork    windows.Rect
	DwFlags   uint32
}

// WndClassExW mirrors WNDCLASSEXW.
type WndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

// Margins mirrors MARGINS from uxtheme.
type Margins struct {
	LeftWidth    int32
	RightWidth   int32
	TopHeight    int32
	BottomHeight int32
}

// ------------------------------------------------------------------ procedures
//
// Exported proc variables rather than wrappers: call sites stay readable.

var (
	User32   = windows.NewLazySystemDLL("user32.dll")
	Kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	Dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	Gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	User32DefWindowProcW         = User32.NewProc("DefWindowProcW")
	User32RegisterClassExW       = User32.NewProc("RegisterClassExW")
	User32CreateWindowExW        = User32.NewProc("CreateWindowExW")
	User32DestroyWindow          = User32.NewProc("DestroyWindow")
	User32ShowWindow             = User32.NewProc("ShowWindow")
	User32UpdateWindow           = User32.NewProc("UpdateWindow")
	User32SetWindowPos           = User32.NewProc("SetWindowPos")
	User32GetClientRect          = User32.NewProc("GetClientRect")
	User32GetSystemMetrics       = User32.NewProc("GetSystemMetrics")
	User32GetSystemMetricsForDpi = User32.NewProc("GetSystemMetricsForDpi")
	User32GetMessageW            = User32.NewProc("GetMessageW")
	User32TranslateMessage       = User32.NewProc("TranslateMessage")
	User32DispatchMessageW       = User32.NewProc("DispatchMessageW")
	User32PostQuitMessage        = User32.NewProc("PostQuitMessage")
	User32PostThreadMessageW     = User32.NewProc("PostThreadMessageW")
	User32PostMessageW           = User32.NewProc("PostMessageW")
	Kernel32GetCurrentThreadID   = Kernel32.NewProc("GetCurrentThreadId")
	User32IsZoomed               = User32.NewProc("IsZoomed")
	User32SetForegroundWindow    = User32.NewProc("SetForegroundWindow")
	User32GetDpiForWindow        = User32.NewProc("GetDpiForWindow")
	User32GetDpiForSystem        = User32.NewProc("GetDpiForSystem")
	User32GetWindowRect          = User32.NewProc("GetWindowRect")
	User32MonitorFromWindow      = User32.NewProc("MonitorFromWindow")
	User32GetMonitorInfoW        = User32.NewProc("GetMonitorInfoW")
	User32GetDC                  = User32.NewProc("GetDC")
	User32ReleaseDC              = User32.NewProc("ReleaseDC")
	User32PrintWindow            = User32.NewProc("PrintWindow")
	User32SendMessageW           = User32.NewProc("SendMessageW")
	User32ScreenToClient         = User32.NewProc("ScreenToClient")
	User32EnumChildWindows       = User32.NewProc("EnumChildWindows")
	User32GetClassNameW          = User32.NewProc("GetClassNameW")

	ComCtl32                  = windows.NewLazySystemDLL("comctl32.dll")
	ComCtl32SetWindowSubclass = ComCtl32.NewProc("SetWindowSubclass")
	ComCtl32DefSubclassProc   = ComCtl32.NewProc("DefSubclassProc")
	User32SetCapture          = User32.NewProc("SetCapture")
	User32ReleaseCapture      = User32.NewProc("ReleaseCapture")
	User32GetCursorPos        = User32.NewProc("GetCursorPos")
	User32GetParent           = User32.NewProc("GetParent")

	Gdi32CreateCompatibleDC             = Gdi32.NewProc("CreateCompatibleDC")
	Gdi32CreateCompatibleBitmap         = Gdi32.NewProc("CreateCompatibleBitmap")
	Gdi32SelectObject                   = Gdi32.NewProc("SelectObject")
	Gdi32DeleteObject                   = Gdi32.NewProc("DeleteObject")
	Gdi32DeleteDC                       = Gdi32.NewProc("DeleteDC")
	Gdi32GetDIBits                      = Gdi32.NewProc("GetDIBits")
	User32SetProcessDpiAwarenessContext = User32.NewProc("SetProcessDpiAwarenessContext")
	User32LoadImageW                    = User32.NewProc("LoadImageW")

	Kernel32GetModuleHandleW = Kernel32.NewProc("GetModuleHandleW")

	DwmapiDwmSetWindowAttribute        = Dwmapi.NewProc("DwmSetWindowAttribute")
	DwmapiDwmExtendFrameIntoClientArea = Dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

// DpiAwarenessContextPerMonitorAwareV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
// expressed as the underlying HANDLE value (-4).
const DpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

// SetProcessDpiAwareness makes the process per-monitor-v2 aware, so every
// coordinate handed to Win32 is a physical pixel on the target monitor.
func SetProcessDpiAwareness() {
	User32SetProcessDpiAwarenessContext.Call(DpiAwarenessContextPerMonitorAwareV2)
}

// SystemDPI is the DPI of the primary monitor.
func SystemDPI() uint32 {
	dpi, _, _ := User32GetDpiForSystem.Call()
	if dpi == 0 {
		return 96
	}
	return uint32(dpi)
}
