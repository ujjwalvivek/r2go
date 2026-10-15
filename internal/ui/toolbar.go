package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	r2theme "r2go/internal/theme"
)

type Toolbar struct {
	mw             *MainWindow
	Container      fyne.CanvasObject
	breadcrumb     *Breadcrumb
	search         *widget.Entry
	selectionLabel *widget.Label
	publicURL      *widget.Button
	upload         *widget.Button
	newFolder      *widget.Button
	normalActions  *fyne.Container
	selectActions  *fyne.Container
}

func NewToolbar(mw *MainWindow) *Toolbar {
	toolbar := &Toolbar{mw: mw}
	toolbar.breadcrumb = NewBreadcrumb(func(prefix string) { mw.navigate(prefix) })

	toolbar.search = widget.NewEntry()
	toolbar.search.SetPlaceHolder("Filter files")
	toolbar.search.SetIcon(fyneTheme.Icon(fyneTheme.IconNameSearch))
	toolbar.search.OnChanged = func(value string) {
		if mw.browser != nil {
			mw.browser.SetFilter(value)
		}
	}

	toolbar.newFolder = widget.NewButtonWithIcon("New folder", fyneTheme.Icon(fyneTheme.IconNameContentAdd), mw.newFolder)
	toolbar.newFolder.Importance = widget.LowImportance
	toolbar.upload = widget.NewButtonWithIcon("Upload", fyneTheme.Icon(fyneTheme.IconNameUpload), mw.showUploadMenu)
	toolbar.upload.Importance = widget.HighImportance
	toolbar.normalActions = container.NewHBox(toolbar.newFolder, toolbar.upload)

	toolbar.selectionLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	toolbar.selectionLabel.SizeName = fyneTheme.SizeNameCaptionText
	download := widget.NewButtonWithIcon("Download", fyneTheme.Icon(fyneTheme.IconNameDownload), mw.downloadSelected)
	download.Importance = widget.LowImportance
	copyButton := widget.NewButton("Copy to…", func() { mw.showCopyMoveDialog(mw.browser.SelectedEntries(), false) })
	copyButton.Importance = widget.LowImportance
	moveButton := widget.NewButton("Move to…", func() { mw.showCopyMoveDialog(mw.browser.SelectedEntries(), true) })
	moveButton.Importance = widget.LowImportance
	deleteButton := widget.NewButton("Delete", mw.deleteSelected)
	deleteButton.Importance = widget.DangerImportance
	toolbar.publicURL = widget.NewButton("Copy URL", mw.copySelectedPublicURL)
	toolbar.publicURL.Importance = widget.LowImportance
	toolbar.publicURL.Hide()
	clear := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameContentClear), func() { mw.browser.ClearSelection() })
	clear.Importance = widget.LowImportance
	toolbar.selectActions = container.NewHBox(
		toolbar.selectionLabel,
		download,
		copyButton,
		moveButton,
		toolbar.publicURL,
		deleteButton,
		fixedSize(28, 28, clear),
	)
	toolbar.selectActions.Hide()

	refresh := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameViewRefresh), mw.refresh)
	refresh.Importance = widget.LowImportance
	settings := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameSettings), mw.showSettings)
	settings.Importance = widget.LowImportance

	modeActions := container.NewStack(toolbar.normalActions, toolbar.selectActions)
	right := container.NewHBox(
		modeActions,
		fixedSize(30, 30, refresh),
		fixedSize(250, 32, toolbar.search),
		fixedSize(30, 30, settings),
	)
	// Breadcrumb owns the flexible center. Keeping selection actions in this
	// same row makes toolbar height invariant, so the table never slides under
	// it when selection state changes.
	topRow := container.NewBorder(nil, nil, nil, right, toolbar.breadcrumb.Container)
	toolbar.Container = panel(r2theme.ColorToolbar, inset(r2theme.SpaceGap, r2theme.SpaceRowY, r2theme.SpaceGap, r2theme.SpaceRowY, topRow))
	toolbar.SetEnabled(false)
	return toolbar
}

func (t *Toolbar) SetPath(bucket, prefix string) {
	if bucket == "" {
		t.breadcrumb.Clear()
		return
	}
	t.breadcrumb.Set(bucket, prefix)
}

func (t *Toolbar) SetSearchText(value string) {
	if t.search.Text != value {
		t.search.SetText(value)
	}
}

func (t *Toolbar) SetSelection(count int) {
	if count <= 0 {
		t.selectActions.Hide()
		t.normalActions.Show()
		if t.Container != nil {
			t.Container.Refresh()
		}
		return
	}
	t.selectionLabel.SetText(fmt.Sprintf("%d selected", count))
	t.normalActions.Hide()
	t.selectActions.Show()
	if t.Container != nil {
		t.Container.Refresh()
	}
}

func (t *Toolbar) SetPublicAvailable(available bool) {
	if available {
		t.publicURL.Show()
	} else {
		t.publicURL.Hide()
	}
}

func (t *Toolbar) SetEnabled(enabled bool) {
	if enabled {
		t.search.Enable()
		t.upload.Enable()
		t.newFolder.Enable()
		return
	}
	t.search.Disable()
	t.upload.Disable()
	t.newFolder.Disable()
	t.publicURL.Hide()
}

func (t *Toolbar) FocusSearch() {
	if t.search.Disabled() {
		return
	}
	t.mw.win.Canvas().Focus(t.search)
}
