// Package shell — capture.go
//
// SelfCapture renders the window into a bitmap with PrintWindow and writes it
// as a 24-bit BMP. It exists because validating a WebView2-hosted desktop app
// from an automated harness is unreliable: screen grabs depend on the window
// being foreground, and any overlay (lock screen, notification, other window)
// silently replaces the pixels under test.
package shell

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"unsafe"

	"zenui/internal/native"
)

const (
	PW_RENDERFULLCONTENT = 0x00000002
	BI_RGB               = 0
	DIB_RGB_COLORS       = 0
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [3]uint32
}

// Capture writes the window contents to path as a BMP and returns the size.
func (w *Window) Capture(path string) (int32, int32, error) {
	width, height := w.ClientRect()
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("capture: empty client area")
	}

	hdcScreen, _, _ := native.User32GetDC.Call(uintptr(w.Hwnd))
	if hdcScreen == 0 {
		return 0, 0, fmt.Errorf("capture: GetDC failed")
	}
	defer native.User32ReleaseDC.Call(uintptr(w.Hwnd), hdcScreen)

	hdcMem, _, _ := native.Gdi32CreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return 0, 0, fmt.Errorf("capture: CreateCompatibleDC failed")
	}
	defer native.Gdi32DeleteDC.Call(hdcMem)

	hbm, _, _ := native.Gdi32CreateCompatibleBitmap.Call(hdcScreen, uintptr(width), uintptr(height))
	if hbm == 0 {
		return 0, 0, fmt.Errorf("capture: CreateCompatibleBitmap failed")
	}
	defer native.Gdi32DeleteObject.Call(hbm)

	old, _, _ := native.Gdi32SelectObject.Call(hdcMem, hbm)
	defer native.Gdi32SelectObject.Call(hdcMem, old)

	// PW_RENDERFULLCONTENT is what makes DirectComposition content (the WebView2
	// child) appear; without it the result is the window background only.
	r, _, _ := native.User32PrintWindow.Call(uintptr(w.Hwnd), hdcMem, PW_RENDERFULLCONTENT)
	if r == 0 {
		return 0, 0, fmt.Errorf("capture: PrintWindow failed")
	}

	var bi bitmapInfo
	bi.Header.Size = uint32(unsafe.Sizeof(bitmapInfoHeader{}))
	bi.Header.Width = width
	bi.Header.Height = -height // top-down rows
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = BI_RGB

	stride := width * 4
	pixels := make([]byte, int(stride*height))
	r, _, _ = native.Gdi32GetDIBits.Call(
		hdcMem,
		hbm,
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
	)
	_ = r

	// The extension selects the container: a .png path gets a real PNG, anything
	// else a BMP. Writing a BMP into a .png file would leave a file every decoder
	// rejects, which is exactly what a check harness must not do.
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".png") {
		if err := writePNG(path, pixels, int(width), int(height)); err != nil {
			return 0, 0, err
		}
		return width, height, nil
	}
	if err := writeBMP(path, pixels, int(width), int(height)); err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

// writePNG encodes the captured BGRA buffer as a PNG.
//
// GetDIBits hands back premultiplied B,G,R,A rows with A=255 for an opaque
// window, so the alpha channel is dropped and the colour order swapped on the
// way into image.RGBA.
func writePNG(path string, bgra []byte, width, height int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	stride := width * 4
	for y := 0; y < height; y++ {
		row := bgra[y*stride : (y+1)*stride]
		for x := 0; x < width; x++ {
			b := row[x*4]
			g := row[x*4+1]
			r := row[x*4+2]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeBMP(path string, bgra []byte, width, height int) error {
	stride := width * 4
	rowBytes := width * 3
	pad := (4 - (rowBytes % 4)) % 4
	imageSize := (rowBytes + pad) * height
	fileSize := 14 + 40 + imageSize

	header := make([]byte, 14)
	header[0], header[1] = 0x42, 0x4D
	binary.LittleEndian.PutUint32(header[2:], uint32(fileSize))
	binary.LittleEndian.PutUint32(header[10:], 54)

	dib := make([]byte, 40)
	binary.LittleEndian.PutUint32(dib[0:], 40)
	binary.LittleEndian.PutUint32(dib[4:], uint32(width))
	binary.LittleEndian.PutUint32(dib[8:], uint32(height))
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 24)
	binary.LittleEndian.PutUint32(dib[20:], uint32(imageSize))

	out := make([]byte, 0, fileSize)
	out = append(out, header...)
	out = append(out, dib...)
	for y := height - 1; y >= 0; y-- {
		row := bgra[y*stride : (y+1)*stride]
		for x := 0; x < width; x++ {
			// GetDIBits fills DIB rows as B,G,R,A; a 24-bit BMP stores B,G,R
			// in that same order, so the first three bytes are copied as-is.
			// Swapping them would turn every blue accent into orange.
			out = append(out, row[x*4], row[x*4+1], row[x*4+2])
		}
		for i := 0; i < pad; i++ {
			out = append(out, 0)
		}
	}
	return os.WriteFile(path, out, 0o644)
}
