package ui

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"github.com/ncruces/zenity"
)

// pickLocal asks for local files (several at once) or one folder with the
// platform's own picker, and hands the chosen paths to done on the UI
// goroutine. Where no native picker is available (zenity missing on Linux,
// say) it falls back to Fyne's dialog.
func (mw *MainWindow) pickLocal(folder bool, title, start string, done func(paths []string)) {
	options := []zenity.Option{zenity.Title(title), zenity.Modal()}
	if start != "" {
		options = append(options, zenity.Filename(start))
	}
	options = append(options, nativeParent(mw.win)...)
	go func() {
		var paths []string
		var err error
		if folder {
			var path string
			path, err = zenity.SelectFile(append(options, zenity.Directory())...)
			paths = []string{path}
		} else {
			paths, err = zenity.SelectFileMultiple(options...)
		}
		fyne.Do(func() {
			switch {
			case err == nil:
				done(paths)
			case errors.Is(err, zenity.ErrCanceled):
			default:
				mw.pickLocalFallback(folder, title, done)
			}
		})
	}()
}

// pickLocalFallback is Fyne's own picker. It must be shown before it is
// resized: a FileDialog builds its window in Show.
func (mw *MainWindow) pickLocalFallback(folder bool, title string, done func(paths []string)) {
	var picker *dialog.FileDialog
	if folder {
		picker = dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				done([]string{uri.Path()})
			}
		}, mw.win)
	} else {
		picker = dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			_ = reader.Close()
			done([]string{path})
		}, mw.win)
	}
	picker.SetView(dialog.ListView)
	picker.SetTitleText(title)
	picker.Show()
	picker.Resize(fyne.NewSize(760, 520))
}
