package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (mw *MainWindow) showSettings() {
	themeGroup := widget.NewRadioGroup([]string{"Dark", "Light"}, func(_ string) {})
	if mw.theme.Variant() == fyneTheme.VariantLight {
		themeGroup.SetSelected("Light")
	} else {
		themeGroup.SetSelected("Dark")
	}

	dlDir := widget.NewEntry()
	dlDir.SetText(mw.cfg.Settings().DownloadDir)
	dlDir.Disable()
	pickDir := widget.NewButtonWithIcon("Browse…", fyneTheme.Icon(fyneTheme.IconNameFolderOpen), func() {
		dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err == nil && lu != nil {
				dlDir.SetText(lu.Path())
			}
		}, mw.win).Show()
	})

	concurrency := widget.NewEntry()
	concurrency.SetText(strconv.Itoa(mw.cfg.Settings().Concurrency))
	concurrency.SetPlaceHolder("Parallel transfers (1-16)")

	mountsTitle := widget.NewLabelWithStyle("Mounted drives", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	mountsBox := container.NewVBox()
	var refreshMounts func()
	refreshMounts = func() {
		mountsBox.Objects = nil
		any := false
		for _, mo := range mw.mounts.Mounts() {
			any = true
			row := container.NewHBox(
				widget.NewLabelWithStyle(mo.Bucket, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(mo.Path),
				container.NewPadded(container.NewHBox()),
			)
			un := widget.NewButtonWithIcon("Unmount", fyneTheme.Icon(fyneTheme.IconNameCancel), func() {
				if err := mw.mounts.Unmount(mo.Account, mo.Bucket); err != nil {
					ShowError(mw.win, "Unmount", err)
					return
				}
				refreshMounts()
			})
			un.Importance = widget.DangerImportance
			row.Add(un)
			mountsBox.Add(row)
		}
		if !any {
			mountsBox.Add(widget.NewLabelWithStyle("No drives mounted", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		}
		mountsBox.Refresh()
	}
	refreshMounts()

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Theme", themeGroup),
			widget.NewFormItem("Download folder", container.NewBorder(nil, nil, nil, pickDir, wideEntry(320, dlDir))),
			widget.NewFormItem("Concurrency", wideEntry(140, concurrency)),
		),
		mountsTitle,
		mountsBox,
	)

	d := dialog.NewCustomConfirm("Settings", "Save", "Cancel", container.NewVScroll(content), func(ok bool) {
		if !ok {
			return
		}
		cur := mw.theme.Variant()
		want := fyneTheme.VariantDark
		if themeGroup.Selected == "Light" {
			want = fyneTheme.VariantLight
		}
		if cur != want {
			mw.theme.SetVariant(want)
			mw.app.Settings().SetTheme(mw.theme)
			mw.cfg.Settings().Theme = "dark"
			if want == fyneTheme.VariantLight {
				mw.cfg.Settings().Theme = "light"
			}
		}
		mw.cfg.Settings().DownloadDir = dlDir.Text
		if n, err := strconv.Atoi(concurrency.Text); err == nil && n > 0 && n <= 16 {
			mw.cfg.Settings().Concurrency = n
			mw.transfers.SetConcurrency(n)
		}
		_ = mw.cfg.Save()
		mw.notifyStatus("Settings saved")
	}, mw.win)
	d.Resize(fyne.NewSize(620, 480))
	d.Show()
}
