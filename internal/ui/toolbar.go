package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	t "r2go/internal/theme"
)

// Toolbar is the 36 px bar above the file list. On the left: back, forward,
// up and the breadcrumb (which turns into a path field with Ctrl+L). On the
// right: the filter, view toggles, then New folder, Upload, Refresh and
// Settings. It never changes shape; selection actions live in the selection
// strip at the bottom of the list.
type Toolbar struct {
	mw        *MainWindow
	Container fyne.CanvasObject

	back, forward, up *button
	breadcrumb        *Breadcrumb
	path              *field
	crumbArea         *fyne.Container
	editing           bool

	search                    *field
	browseTools               *fyne.Container
	right                     *fyne.Container
	listView, gridView        *button
	details                   *button
	newFolder, upload, reload *button
}

func NewToolbar(mw *MainWindow) *Toolbar {
	tb := &Toolbar{mw: mw}
	tb.back = newIconButton(t.IconArrowLeft, "Back  ·  Alt+←", mw.goBack)
	tb.forward = newIconButton(t.IconArrowRight, "Forward  ·  Alt+→", mw.goForward)
	tb.up = newIconButton(t.IconArrowUp, "Up one folder  ·  Backspace", mw.goParent)

	tb.breadcrumb = NewBreadcrumb(mw.win, mw.navigate, tb.EditPath)
	tb.path = newField("folder/subfolder/")
	tb.path.OnSubmitted = func(value string) { tb.submitPath(value) }
	tb.path.onEscape = tb.stopEditing
	tb.path.onBlur = tb.stopEditing
	tb.path.Hide()
	tb.crumbArea = container.NewStack(tb.breadcrumb, container.New(&fixedHeightLayout{height: 28}, tb.path))

	tb.search = newField("Filter this folder")
	tb.search.ActionItem = fieldIcon(t.IconSearch)
	tb.search.OnChanged = func(value string) {
		if mw.browser != nil {
			mw.browser.SetFilter(value)
		}
	}
	tb.search.onEscape = func() {
		tb.search.SetText("")
		mw.win.Canvas().Unfocus()
	}
	tb.search.onDown = func() {
		mw.win.Canvas().Unfocus()
		mw.browser.MoveFocus(1, false)
	}

	tb.listView = newIconButton(t.IconList, "List view  ·  Ctrl+1", func() { mw.setView(viewList) })
	tb.gridView = newIconButton(t.IconGrid, "Grid view  ·  Ctrl+2\nThumbnails for images", func() { mw.setView(viewGrid) })
	tb.details = newIconButton(t.IconDetails, "Details panel  ·  I\nShow what's selected: size, type, preview and links", mw.toggleDetails)

	tb.newFolder = newIconButton(t.IconFolderPlus, "New folder  ·  Ctrl+Shift+N", mw.newFolder)
	tb.reload = newIconButton(t.IconRefresh, "Refresh  ·  F5\nList this folder again", mw.refresh)
	settings := newIconButton(t.IconSettings, "Settings  ·  Ctrl+,", mw.showSettings)
	tb.upload = newButton(buttonPrimary, "Upload", t.IconUpload, nil)
	tb.upload.tip = "Upload  ·  Ctrl+U\nFiles or a whole folder, into this folder. You can also drop them on the window."
	tb.upload.OnTapped = func() {
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(tb.upload)
		mw.showUploadMenu(pos.Add(fyne.NewPos(0, tb.upload.Size().Height+2)))
	}

	nav := container.New(&rowLayout{gap: 0}, tb.back, tb.forward, tb.up)
	// Browsing tools hide on an account's overview, where they do nothing.
	tb.browseTools = container.New(&rowLayout{gap: 2},
		fixed(220, 28, tb.search),
		spacer(6),
		tb.listView, tb.gridView, tb.details,
		divider(),
		tb.newFolder,
	)
	right := container.New(&rowLayout{gap: 2},
		tb.browseTools,
		tb.reload,
		spacer(4),
		tb.upload,
		spacer(4),
		settings,
	)
	tb.right = right
	row := container.NewBorder(nil, nil, nav, right, inset(8, 0, 12, 0, tb.crumbArea))
	bar := container.NewBorder(nil, hRule(t.ColorBorder), nil, nil, inset(5, 4, 6, 4, row))
	tb.Container = minHeight(t.HeightBar, panelBox(t.ColorBase, bar))
	tb.SetEnabled(false)
	return tb
}

// spacer is fixed empty width in a row.
func spacer(width float32) fyne.CanvasObject { return fixed(width, 1, layoutSpacer()) }

// divider is a short vertical rule between toolbar groups.
func divider() fyne.CanvasObject {
	return inset(6, 0, 6, 0, fixed(1, 16, newRect(t.ColorBorderStrong)))
}

func layoutSpacer() fyne.CanvasObject { return container.NewWithoutLayout() }

func (tb *Toolbar) SetPath(bucket, prefix string) {
	tb.stopEditing()
	if !tb.browseTools.Visible() {
		tb.browseTools.Show()
		tb.upload.Show()
		tb.reload.Show()
		tb.reload.SetTip("Refresh  ·  F5\nList this folder again")
		tb.right.Refresh()
	}
	tb.breadcrumb.Set(bucket, prefix)
}

func (tb *Toolbar) SetOverview(account string) {
	tb.stopEditing()
	tb.breadcrumb.SetOverview(account)
	tb.upload.Hide()
	tb.browseTools.Hide()
	tb.reload.Show()
	tb.reload.Enable()
	tb.reload.SetTip("Count again  ·  F5\nList every object in this account's buckets again")
	tb.right.Refresh()
}

// SetIdle clears the toolbar when nothing is open (no accounts yet), keeping
// only Settings.
func (tb *Toolbar) SetIdle() {
	tb.stopEditing()
	tb.breadcrumb.Set("", "")
	tb.upload.Hide()
	tb.browseTools.Hide()
	tb.reload.Hide()
	tb.right.Refresh()
}

func (tb *Toolbar) SetSearchText(value string) {
	if tb.search.Text != value {
		tb.search.SetText(value)
	}
}

func (tb *Toolbar) SetHistory(canBack, canForward, canUp bool) {
	tb.back.SetDisabled(!canBack)
	tb.forward.SetDisabled(!canForward)
	tb.up.SetDisabled(!canUp)
}

func (tb *Toolbar) SetView(view string, details bool) {
	tb.listView.SetActive(view != viewGrid)
	tb.gridView.SetActive(view == viewGrid)
	tb.details.SetActive(details)
}

func (tb *Toolbar) SetEnabled(enabled bool) {
	for _, b := range []*button{tb.upload, tb.newFolder, tb.reload} {
		b.SetDisabled(!enabled)
	}
	if enabled {
		tb.search.Enable()
	} else {
		tb.search.Disable()
		tb.SetHistory(false, false, false)
	}
}

func (tb *Toolbar) FocusSearch() {
	if tb.search.Disabled() {
		return
	}
	tb.mw.win.Canvas().Focus(tb.search)
}

// EditPath swaps the breadcrumb for a path field holding the current folder.
func (tb *Toolbar) EditPath() {
	session := tb.mw.currentSession()
	if session.Bucket == "" || tb.editing {
		return
	}
	tb.editing = true
	tb.breadcrumb.Hide()
	tb.path.SetText(session.Prefix)
	tb.path.Show()
	tb.crumbArea.Refresh()
	tb.mw.win.Canvas().Focus(tb.path)
	tb.path.TypedShortcut(&fyne.ShortcutSelectAll{})
}

func (tb *Toolbar) stopEditing() {
	if !tb.editing {
		return
	}
	tb.editing = false
	tb.path.Hide()
	tb.breadcrumb.Show()
	tb.crumbArea.Refresh()
}

func (tb *Toolbar) submitPath(value string) {
	session := tb.mw.currentSession()
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimPrefix(value, session.Bucket+"/")
	if value == session.Bucket {
		value = ""
	}
	if value != "" && !strings.HasSuffix(value, "/") {
		value += "/"
	}
	tb.stopEditing()
	tb.mw.win.Canvas().Unfocus()
	tb.mw.navigate(value)
}
