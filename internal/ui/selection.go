package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"r2go/internal/model"
	t "r2go/internal/theme"
)

// SelectionBar is the strip that appears at the bottom of the file list while
// something is selected. It sits at the bottom so rows never move under the
// pointer when it appears.
type SelectionBar struct {
	mw        *MainWindow
	Container fyne.CanvasObject
	count     *textView
	meta      *textView
	copyURL   *button
	rename    *button
	row       *fyne.Container
}

func NewSelectionBar(mw *MainWindow) *SelectionBar {
	s := &SelectionBar{
		mw:    mw,
		count: newTextView("", t.TextBody, t.ColorText, styleStrong),
		meta:  newTextView("", t.TextSmall, t.ColorMuted, styleMono),
	}
	download := newButton(buttonQuiet, "Download", t.IconDownload, mw.downloadSelected).withTip("Download  ·  Ctrl+D\nSaved to your download folder, keeping folder structure")
	s.copyURL = newButton(buttonQuiet, "Copy URL", t.IconLink, mw.copySelectedPublicURL).withTip("Copy public URL  ·  Ctrl+Shift+C")
	copyTo := newButton(buttonQuiet, "Copy to…", t.IconCopy, func() { mw.showCopyMoveDialog(mw.browser.SelectedEntries(), false) })
	moveTo := newButton(buttonQuiet, "Move to…", t.IconMove, func() { mw.showCopyMoveDialog(mw.browser.SelectedEntries(), true) })
	s.rename = newButton(buttonQuiet, "Rename", t.IconPencil, mw.renameSelected).withTip("Rename  ·  F2")
	remove := newButton(buttonDanger, "Delete", t.IconTrash, mw.deleteSelected).withTip("Delete  ·  Del\nAsks before anything is removed")
	clear := newIconButton(t.IconClose, "Clear selection  ·  Esc", func() { mw.browser.ClearSelection() })

	bar := newRect(t.ColorAccent)
	bar.SetMinSize(fyne.NewSize(t.WidthAccentBar, 1))
	label := container.New(&rowLayout{gap: 10}, s.count, s.meta)
	actions := container.New(&rowLayout{gap: 2}, download, s.copyURL, copyTo, moveTo, s.rename, spacer(6), remove, spacer(4), clear)
	s.row = container.NewBorder(nil, nil, label, actions)
	body := container.NewBorder(hRule(t.ColorBorder), nil, bar, nil, inset(t.SpaceCardX, 4, 6, 4, s.row))
	s.Container = panelBox(t.ColorPanel, body)
	s.Container.Hide()
	return s
}

// Update shows the strip for the current selection, or hides it.
func (s *SelectionBar) Update(entries []model.Entry, publicDomain bool) {
	if len(entries) == 0 {
		s.Container.Hide()
		return
	}
	files, folders := 0, 0
	var size int64
	pending := false
	for _, entry := range entries {
		if entry.IsDir {
			folders++
			if !entry.StatsKnown {
				pending = true
			}
		} else {
			files++
		}
		size += entry.Size
	}
	s.count.SetText(countNoun(len(entries), "item") + " selected")
	sizeText := formatSize(size)
	if pending {
		sizeText += "+"
	}
	switch {
	case folders == 0:
		s.meta.SetText(sizeText)
	case files == 0:
		s.meta.SetText(joinDot(countNoun(folders, "folder"), sizeText))
	default:
		s.meta.SetText(joinDot(countNoun(files, "file"), countNoun(folders, "folder"), sizeText))
	}
	single := len(entries) == 1
	if single && !entries[0].IsDir && publicDomain {
		s.copyURL.Show()
	} else {
		s.copyURL.Hide()
	}
	if single {
		s.rename.Show()
	} else {
		s.rename.Hide()
	}
	s.row.Refresh()
	s.Container.Show()
}
