//go:build !windows

package ui

import (
	"fyne.io/fyne/v2"
	"github.com/ncruces/zenity"
)

// styleNativeWindow only has an effect on Windows, where DWM lets an app
// colour its title bar and turn off rounded corners.
func styleNativeWindow(fyne.Window) {}

// nativeParent has nothing to attach outside Windows; zenity's modal hint
// keeps the picker on top.
func nativeParent(fyne.Window) []zenity.Option { return nil }
