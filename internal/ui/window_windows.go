package ui

import (
	"image/color"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"github.com/ncruces/zenity"

	t "r2go/internal/theme"
)

// DWM window attributes from dwmapi.h. Windows 10 ignores the ones it does
// not know, so each call is best effort.
const (
	dwmaUseImmersiveDarkMode   = 20
	dwmaWindowCornerPreference = 33
	dwmaBorderColor            = 34
	dwmaCaptionColor           = 35
	dwmaTextColor              = 36
	dwmwcpDoNotRound           = 1
)

var procDwmSetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

// styleNativeWindow keeps the native title bar, so dragging, snapping and the
// system menu all behave normally, but paints it to match the app: a dark
// PANEL caption with SUBTLE text, a 1 px BORDER_STRONG edge and square
// corners. It needs the native HWND, which Fyne only has once the window
// exists, so call it after startup.
func styleNativeWindow(win fyne.Window) {
	native, ok := win.(driver.NativeWindow)
	if !ok {
		return
	}
	native.RunNative(func(context any) {
		windows, ok := context.(driver.WindowsWindowContext)
		if !ok || windows.HWND == 0 {
			return
		}
		setDWM(windows.HWND, dwmaUseImmersiveDarkMode, 1)
		setDWM(windows.HWND, dwmaWindowCornerPreference, dwmwcpDoNotRound)
		setDWM(windows.HWND, dwmaCaptionColor, colorRef(t.Color(t.ColorPanel)))
		setDWM(windows.HWND, dwmaTextColor, colorRef(t.Color(t.ColorSubtle)))
		setDWM(windows.HWND, dwmaBorderColor, colorRef(t.Color(t.ColorBorderStrong)))
	})
}

func setDWM(hwnd uintptr, attribute uintptr, value uint32) {
	procDwmSetWindowAttribute.Call(hwnd, attribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}

// colorRef packs a colour as a Win32 COLORREF (0x00BBGGRR).
func colorRef(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()
	return uint32(r>>8) | uint32(g>>8)<<8 | uint32(b>>8)<<16
}

// nativeParent makes a native picker a child of the app window, so it opens
// over it and blocks it like any dialog.
func nativeParent(win fyne.Window) []zenity.Option {
	native, ok := win.(driver.NativeWindow)
	if !ok {
		return nil
	}
	var hwnd uintptr
	native.RunNative(func(context any) {
		if windows, ok := context.(driver.WindowsWindowContext); ok {
			hwnd = windows.HWND
		}
	})
	if hwnd == 0 {
		return nil
	}
	return []zenity.Option{zenity.Attach(hwnd)}
}
