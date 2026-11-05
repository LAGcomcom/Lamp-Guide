package native

import (
	"syscall"
	"unsafe"
)

// Memory access helpers for structs that live in native memory.
//
// A window procedure receives an LPARAM that points at caller-owned memory
// (MINMAXINFO, RECT, …). Reinterpreting that integer as a Go pointer hides a
// lifetime bug from the compiler and trips vet's unsafe.Pointer check, so the
// contents are copied with RtlMoveMemory instead.

var procRtlMoveMemory = ntdll.NewProc("RtlMoveMemory")

var ntdll = syscall.NewLazyDLL("ntdll.dll")

// ReadStruct copies size bytes from src (a native address) into dst.
func ReadStruct(src uintptr, dst unsafe.Pointer, size uintptr) {
	if src == 0 || dst == nil || size == 0 {
		return
	}
	procRtlMoveMemory.Call(uintptr(dst), src, size)
}

// WriteStruct copies size bytes from src into dst (a native address) plus offset.
func WriteStruct(dst uintptr, src unsafe.Pointer, size, offset uintptr) {
	if dst == 0 || src == nil || size == 0 {
		return
	}
	procRtlMoveMemory.Call(dst+offset, uintptr(src), size)
}
