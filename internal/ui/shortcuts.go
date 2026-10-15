package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// appShortcuts holds the window's modified-key shortcuts by name, so a text
// field that does not use a shortcut (Ctrl+K in the filter, say) can pass it
// on instead of swallowing it.
var appShortcuts = map[string]func(){}

// TypedShortcut lets the field handle editing shortcuts and forwards the
// app's own.
func (f *field) TypedShortcut(shortcut fyne.Shortcut) {
	if custom, ok := shortcut.(*desktop.CustomShortcut); ok {
		if fn, found := appShortcuts[custom.ShortcutName()]; found {
			fn()
			return
		}
	}
	f.Entry.TypedShortcut(shortcut)
}

func (mw *MainWindow) registerShortcuts() {
	c := mw.win.Canvas()
	ctrl := fyne.KeyModifierShortcutDefault
	shift := fyne.KeyModifierShift
	alt := fyne.KeyModifierAlt

	// idle is true when no dialog is open and no field has focus, so plain
	// keys belong to the file list.
	idle := func() bool {
		return len(c.Overlays().List()) == 0 && c.Focused() == nil
	}
	noDialog := func() bool { return len(c.Overlays().List()) == 0 }
	add := func(key fyne.KeyName, modifier fyne.KeyModifier, fn func()) {
		shortcut := &desktop.CustomShortcut{KeyName: key, Modifier: modifier}
		run := func() {
			if noDialog() {
				fn()
			}
		}
		appShortcuts[shortcut.ShortcutName()] = run
		c.AddShortcut(shortcut, func(fyne.Shortcut) { run() })
	}

	// Ctrl+A/C/X/V reach the canvas as Fyne's standard shortcuts, not custom
	// ones, and only when no field has focus (a field uses them for text).
	c.AddShortcut(&fyne.ShortcutSelectAll{}, func(fyne.Shortcut) {
		if idle() {
			mw.browser.SelectAll()
		}
	})
	c.AddShortcut(&fyne.ShortcutCopy{}, func(fyne.Shortcut) {
		if idle() {
			mw.copySelection(false)
		}
	})
	c.AddShortcut(&fyne.ShortcutCut{}, func(fyne.Shortcut) {
		if idle() {
			mw.copySelection(true)
		}
	})
	c.AddShortcut(&fyne.ShortcutPaste{}, func(fyne.Shortcut) {
		if idle() {
			mw.pasteClipboard()
		}
	})

	add(fyne.KeyF, ctrl, mw.toolbar.FocusSearch)
	add(fyne.KeyL, ctrl, mw.toolbar.EditPath)
	add(fyne.KeyK, ctrl, mw.showPalette)
	add(fyne.KeyP, ctrl, mw.showPalette)
	add(fyne.KeyR, ctrl, mw.refresh)
	add(fyne.KeyN, ctrl|shift, mw.newFolder)
	add(fyne.KeyU, ctrl, mw.showUploadMenuAtToolbar)
	add(fyne.KeyU, ctrl|shift, mw.pickUploadFolder)
	add(fyne.KeyD, ctrl, mw.downloadSelected)
	add(fyne.KeyC, ctrl|shift, mw.copySelectedPublicURL)
	add(fyne.Key1, ctrl, func() { mw.setView(viewList) })
	add(fyne.Key2, ctrl, func() { mw.setView(viewGrid) })
	add(fyne.KeyI, ctrl, mw.toggleDetails)
	add(fyne.KeyComma, ctrl, mw.showSettings)
	add(fyne.KeyLeft, alt, mw.goBack)
	add(fyne.KeyRight, alt, mw.goForward)
	add(fyne.KeyUp, alt, mw.goParent)

	// Keys without a modifier never become Fyne shortcuts; they arrive here
	// when no widget has focus.
	if dc, ok := c.(desktop.Canvas); ok {
		dc.SetOnKeyDown(func(event *fyne.KeyEvent) {
			if event.Name == desktop.KeyShiftLeft || event.Name == desktop.KeyShiftRight {
				mw.shift = true
			}
		})
		dc.SetOnKeyUp(func(event *fyne.KeyEvent) {
			if event.Name == desktop.KeyShiftLeft || event.Name == desktop.KeyShiftRight {
				mw.shift = false
			}
		})
	}
	c.SetOnTypedKey(func(event *fyne.KeyEvent) {
		if !idle() {
			return
		}
		b := mw.browser
		switch event.Name {
		case fyne.KeyUp:
			b.MoveVertical(-1, mw.shift)
		case fyne.KeyDown:
			b.MoveVertical(1, mw.shift)
		case fyne.KeyLeft:
			b.MoveHorizontal(-1, mw.shift)
		case fyne.KeyRight:
			b.MoveHorizontal(1, mw.shift)
		case fyne.KeyHome:
			b.MoveToEdge(false, mw.shift)
		case fyne.KeyEnd:
			b.MoveToEdge(true, mw.shift)
		case fyne.KeyPageUp:
			b.MoveVertical(-12, mw.shift)
		case fyne.KeyPageDown:
			b.MoveVertical(12, mw.shift)
		case fyne.KeyReturn, fyne.KeyEnter:
			b.OpenSelected()
		case fyne.KeyDelete:
			mw.deleteSelected()
		case fyne.KeyF2:
			mw.renameSelected()
		case fyne.KeyF5:
			mw.refresh()
		case fyne.KeyF1:
			mw.showHelp()
		case fyne.KeyBackspace:
			mw.goParent()
		case fyne.KeyEscape:
			switch {
			case len(b.selected) > 0:
				b.ClearSelection()
			case mw.banner.Visible():
				mw.banner.Hide()
				mw.relayout()
			case b.filter != "":
				mw.toolbar.SetSearchText("")
			}
		}
	})

	// Typing a name jumps to it; "?" opens the key list.
	c.SetOnTypedRune(func(r rune) {
		if !idle() {
			return
		}
		if r == '?' {
			mw.showHelp()
			return
		}
		if r == ' ' && mw.typed == "" {
			return
		}
		now := time.Now()
		if now.Sub(mw.typedAt) > 800*time.Millisecond {
			mw.typed = ""
		}
		mw.typedAt = now
		mw.typed += string(r)
		mw.browser.TypeAhead(mw.typed)
	})
}
