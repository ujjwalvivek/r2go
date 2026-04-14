package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Toolbar struct {
	mw        *MainWindow
	Container *fyne.Container

	breadcrumb *Breadcrumb
	search     *widget.Entry
	upload     *widget.Button
	refresh    *widget.Button
	multi      *widget.Button
	themeBtn   *widget.Button
	settings   *widget.Button
}

func NewToolbar(mw *MainWindow) *Toolbar {
	t := &Toolbar{mw: mw}

	t.breadcrumb = NewBreadcrumb(func(prefix string) {
		mw.onNavigatePrefix(prefix)
	})

	t.search = widget.NewEntry()
	t.search.SetPlaceHolder("Search in current folder…")
	t.search.OnChanged = func(text string) { mw.fileView.SetFilter(text) }
	t.search.Disable()
	searchBox := container.NewGridWrap(fyne.NewSize(280, 38), t.search)

	t.upload = widget.NewButtonWithIcon("Upload", fyneTheme.Icon(fyneTheme.IconNameUpload), func() {
		mw.onUploadClick()
	})
	t.upload.Disable()
	t.refresh = widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameViewRefresh), func() {
		mw.onRefresh()
	})
	t.refresh.Disable()
	t.themeBtn = widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameColorChromatic), mw.toggleTheme)
	t.multi = widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameList), func() {
		mw.fileView.SetMulti(!mw.fileView.multi)
		t.Refresh()
	})
	t.multi.Disable()
	t.settings = widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameSettings), func() {
		mw.showSettings()
	})

	t.Container = container.NewBorder(nil, nil, nil, nil,
		container.NewHBox(
			t.breadcrumb.Container,
			layout.NewSpacer(),
			t.upload,
			t.refresh,
			t.multi,
			searchBox,
			t.themeBtn,
			t.settings,
		))
	return t
}

func (t *Toolbar) SetPath(bucket, prefix string) {
	t.breadcrumb.Set(bucket, prefix)
}

func (t *Toolbar) SetSearchText(text string) {
	t.search.SetText(text)
}

func (t *Toolbar) SetActionsEnabled(enabled bool) {
	if enabled {
		t.search.Enable()
		t.upload.Enable()
		t.refresh.Enable()
		t.multi.Enable()
	} else {
		t.search.Disable()
		t.upload.Disable()
		t.refresh.Disable()
		t.multi.Disable()
	}
}

func (t *Toolbar) Refresh() {
	if t.multi == nil {
		return
	}
	if t.mw.fileView.multi {
		t.multi.Importance = widget.HighImportance
	} else {
		t.multi.Importance = widget.LowImportance
	}
	t.multi.Refresh()
}
