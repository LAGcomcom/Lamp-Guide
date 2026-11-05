package native

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// DwmSetIntAttribute writes a DWORD-sized window attribute.
func DwmSetIntAttribute(hwnd windows.Handle, attribute uint32, value int32) {
	DwmapiDwmSetWindowAttribute.Call(
		uintptr(hwnd),
		uintptr(attribute),
		uintptr(unsafe.Pointer(&value)),
		unsafe.Sizeof(value),
	)
}

// DwmSetUint32Attribute writes a DWORD-sized window attribute where the value
// is a colour or bit-field rather than a signed enum.
func DwmSetUint32Attribute(hwnd windows.Handle, attribute uint32, value uint32) {
	v := value
	DwmapiDwmSetWindowAttribute.Call(
		uintptr(hwnd),
		uintptr(attribute),
		uintptr(unsafe.Pointer(&v)),
		unsafe.Sizeof(v),
	)
}

// SetDarkFrame enables or disables the immersive dark caption.
func SetDarkFrame(hwnd windows.Handle, dark bool) {
	var v int32
	if dark {
		v = 1
	}
	DwmSetIntAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, v)
}

// SetMicaBackdrop requests the DWMSBT_MAINWINDOW (Mica) system backdrop.
//
// On Windows 10 and on Windows 11 before 22H2 this silently fails; callers
// should treat a failure as "no backdrop" and paint an opaque page instead.
func SetMicaBackdrop(hwnd windows.Handle) {
	DwmSetIntAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, DWMSBT_MAINWINDOW)
}

// SetRoundedFrame asks DWM for the standard rounded corners.
func SetRoundedFrame(hwnd windows.Handle) {
	DwmSetIntAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_ROUND)
}

// ClearFrameBorder removes the 1px line DWM draws around a window. Against a
// dark page the default border reads as a light seam, so the shell clears it.
func ClearFrameBorder(hwnd windows.Handle) {
	DwmSetUint32Attribute(hwnd, DWMWA_BORDER_COLOR, DWMWA_COLOR_NONE)
}

// ExtendFrameIntoClientArea makes the whole client area part of the frame, so
// the backdrop shows through instead of the window painting opaque.
func ExtendFrameIntoClientArea(hwnd windows.Handle) {
	m := Margins{LeftWidth: -1, RightWidth: -1, TopHeight: -1, BottomHeight: -1}
	DwmapiDwmExtendFrameIntoClientArea.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&m)),
	)
}
