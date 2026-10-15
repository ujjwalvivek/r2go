//go:build !windows

package ui

import "fyne.io/fyne/v2"

// squareWindowCorners only has an effect on Windows 11, where DWM rounds
// window corners. Other platforms already draw square windows.
func squareWindowCorners(fyne.Window) {}
