package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func (mw *MainWindow) registerShortcuts() {
	canvas := mw.win.Canvas()

	add := func(key fyne.KeyName, mod fyne.KeyModifier, fn func()) {
		canvas.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mod}, func(_ fyne.Shortcut) {
			fn()
		})
	}

	// guards: don't hijack keys while a dialog/popup is open or an entry has focus
	guard := func() bool {
		if len(canvas.Overlays().List()) > 0 {
			return false
		}
		if canvas.Focused() != nil {
			return false
		}
		return true
	}

	add(fyne.KeyF, fyne.KeyModifierControl, func() {
		if mw.toolbar.search.Disabled() {
			return
		}
		canvas.Focus(mw.toolbar.search)
	})

	add(fyne.KeyR, fyne.KeyModifierControl, func() {
		mw.onRefresh()
	})

	add(fyne.KeyF2, 0, func() {
		if !guard() {
			return
		}
		if e, ok := mw.fileView.SelectedEntry(); ok {
			mw.renameEntry(e)
		}
	})

	add(fyne.KeyDelete, 0, func() {
		if !guard() {
			return
		}
		if es := mw.fileView.SelectedEntries(); len(es) > 0 {
			mw.deleteEntries(es)
		}
	})

	add(fyne.KeyA, fyne.KeyModifierControl, func() {
		if !guard() {
			return
		}
		mw.fileView.SelectAll()
	})

	add(fyne.KeyEscape, 0, func() {
		mw.fileView.SetMulti(false)
		mw.fileView.ClearSelection()
		canvas.Unfocus()
	})

	add(fyne.KeyEnter, 0, func() {
		if !guard() {
			return
		}
		if e, ok := mw.fileView.SelectedEntry(); ok {
			if e.IsDir {
				mw.onNavigatePrefix(e.Key)
			} else {
				mw.onOpenEntry(e)
			}
		}
	})
}
