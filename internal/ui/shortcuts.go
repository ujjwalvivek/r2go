package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/model"
)

func (mw *MainWindow) registerShortcuts() {
	canvas := mw.win.Canvas()
	add := func(key fyne.KeyName, modifier fyne.KeyModifier, fn func()) {
		canvas.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: modifier}, func(fyne.Shortcut) { fn() })
	}
	guard := func() bool {
		if len(canvas.Overlays().List()) > 0 {
			return false
		}
		if _, editing := canvas.Focused().(*widget.Entry); editing {
			return false
		}
		return true
	}

	add(fyne.KeyF, fyne.KeyModifierControl, mw.toolbar.FocusSearch)
	add(fyne.KeyR, fyne.KeyModifierControl, mw.refresh)
	add(fyne.KeyF5, 0, mw.refresh)

	add(fyne.KeyA, fyne.KeyModifierControl, func() {
		if guard() {
			mw.browser.SelectAll()
		}
	})
	add(fyne.KeyC, fyne.KeyModifierControl, func() {
		if guard() {
			mw.copySelection(false)
		}
	})
	add(fyne.KeyX, fyne.KeyModifierControl, func() {
		if guard() {
			mw.copySelection(true)
		}
	})
	add(fyne.KeyV, fyne.KeyModifierControl, func() {
		if guard() {
			mw.pasteClipboard()
		}
	})

	add(fyne.KeyF2, 0, func() {
		if guard() {
			mw.renameSelected()
		}
	})
	add(fyne.KeyDelete, 0, func() {
		if guard() {
			mw.deleteSelected()
		}
	})
	add(fyne.KeyEnter, 0, func() {
		if guard() {
			mw.browser.OpenSelected()
		}
	})
	add(fyne.KeyEscape, 0, func() {
		mw.browser.ClearSelection()
		canvas.Unfocus()
	})

	add(fyne.KeyUp, 0, func() {
		if guard() {
			mw.browser.MoveFocus(-1, false)
		}
	})
	add(fyne.KeyDown, 0, func() {
		if guard() {
			mw.browser.MoveFocus(1, false)
		}
	})
	add(fyne.KeyUp, fyne.KeyModifierShift, func() {
		if guard() {
			mw.browser.MoveFocus(-1, true)
		}
	})
	add(fyne.KeyDown, fyne.KeyModifierShift, func() {
		if guard() {
			mw.browser.MoveFocus(1, true)
		}
	})

	goParent := func() {
		if !guard() {
			return
		}
		session := mw.currentSession()
		if session.Bucket != "" && session.Prefix != "" {
			mw.navigate(model.ParentPrefix(session.Prefix))
		}
	}
	add(fyne.KeyBackspace, 0, goParent)
	add(fyne.KeyLeft, fyne.KeyModifierAlt, goParent)

	add(fyne.KeyN, fyne.KeyModifierControl|fyne.KeyModifierShift, func() {
		if guard() {
			mw.newFolder()
		}
	})
	add(fyne.KeyU, fyne.KeyModifierControl, func() {
		if guard() {
			mw.showUploadMenu()
		}
	})
}
