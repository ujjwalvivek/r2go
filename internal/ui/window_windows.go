package ui

import (
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

const (
	// DWMWA_WINDOW_CORNER_PREFERENCE and DWMWCP_DONOTROUND from dwmapi.h.
	// Windows 11 rounds top-level windows unless the app opts out; Windows 10
	// ignores the attribute.
	dwmaWindowCornerPreference = 33
	dwmwcpDoNotRound           = 1
)

var procDwmSetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

// squareWindowCorners asks DWM for square window corners. It needs the native
// HWND, which Fyne only has once the window exists, so call it after startup.
func squareWindowCorners(win fyne.Window) {
	native, ok := win.(driver.NativeWindow)
	if !ok {
		return
	}
	native.RunNative(func(context any) {
		windows, ok := context.(driver.WindowsWindowContext)
		if !ok || windows.HWND == 0 {
			return
		}
		preference := uint32(dwmwcpDoNotRound)
		procDwmSetWindowAttribute.Call(
			windows.HWND,
			dwmaWindowCornerPreference,
			uintptr(unsafe.Pointer(&preference)),
			unsafe.Sizeof(preference),
		)
	})
}
