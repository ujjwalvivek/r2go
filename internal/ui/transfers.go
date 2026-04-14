package ui

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/transfer"
	"r2go/internal/util"
)

type TransfersPanel struct {
	mw        *MainWindow
	Container *fyne.Container

	rows      *fyne.Container
	none      *widget.Label
	wasActive bool
}

func NewTransfersPanel(mw *MainWindow) *TransfersPanel {
	t := &TransfersPanel{mw: mw}
	t.rows = container.NewVBox()

	t.none = widget.NewLabel("No active transfers")
	t.none.Importance = widget.LowImportance
	t.none.Alignment = fyne.TextAlignCenter

	header := widget.NewLabelWithStyle("Transfers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := container.NewStack(t.rows, t.none)
	t.Container = container.NewBorder(header, nil, nil, nil, body)
	t.Container.Hide()
	return t
}

func (t *TransfersPanel) Refresh() {
	items := t.mw.transfers.Items()
	if len(items) == 0 {
		if t.wasActive {
			t.wasActive = false
			t.mw.notifyStatus("All transfers finished")
			t.mw.fileView.Reload()
		}
		t.Container.Hide()
		t.mw.statusBar.SetRight("")
		return
	}
	t.Container.Show()

	active := 0
	for _, it := range items {
		if it.Status == transfer.Queued || it.Status == transfer.Running {
			active++
		}
	}
	t.wasActive = active > 0

	t.rows.Objects = nil
	t.none.Hide()
	for _, it := range items {
		t.rows.Add(newTransferRow(t.mw, it.ID))
	}
	t.rows.Refresh()
	t.mw.statusBar.SetRight(transfersSummary(items))
}

func transfersSummary(items []*transfer.Item) string {
	running, queued := 0, 0
	for _, it := range items {
		switch it.Status {
		case transfer.Running:
			running++
		case transfer.Queued:
			queued++
		}
	}
	switch {
	case running > 0 && queued > 0:
		return strconv.Itoa(running) + " transferring, " + strconv.Itoa(queued) + " queued"
	case running > 0:
		if running == 1 {
			return "1 transfer in progress"
		}
		return strconv.Itoa(running) + " transfers in progress"
	case queued > 0:
		return strconv.Itoa(queued) + " queued"
	}
	return ""
}

func newTransferRow(mw *MainWindow, id int64) fyne.CanvasObject {
	snap := func() *transfer.Item { return mw.transfers.Find(id) }

	name := widget.NewLabel("")
	name.Truncation = fyne.TextTruncateEllipsis
	status := widget.NewLabel("")
	status.TextStyle = fyne.TextStyle{Italic: true}
	detail := widget.NewLabel("")
	detail.Truncation = fyne.TextTruncateEllipsis
	detail.Importance = widget.LowImportance

	bar := widget.NewProgressBar()

	cols := container.NewVBox(
		container.NewHBox(name, container.NewPadded(container.NewHBox()), status),
		bar,
		detail,
	)

	actions := container.NewVBox()
	if it := snap(); it != nil {
		name.SetText(it.Key)
		status.SetText(it.Status.String())
		bar.SetValue(it.Progress())

		switch {
		case (it.Status == transfer.Running || it.Status == transfer.Queued) && it.Kind == transfer.Download:
			pause := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameMediaPause), func() {
				mw.transfers.Pause(id)
			})
			pause.Importance = widget.LowImportance
			actions.Add(pause)
		case it.Status == transfer.Paused:
			resume := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameMediaPlay), func() {
				if client, err := mw.clientFor(it.Account); err == nil {
					mw.transfers.Resume(client, id)
				}
			})
			resume.Importance = widget.LowImportance
			actions.Add(resume)
		}
		if it.Status == transfer.Running || it.Status == transfer.Queued {
			stop := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameCancel), func() {
				mw.transfers.Cancel(id)
			})
			stop.Importance = widget.LowImportance
			actions.Add(stop)
		}
		if it.Status == transfer.Done || it.Status == transfer.Failed || it.Status == transfer.Cancelled {
			if it.AutoOpen && it.Status == transfer.Done {
				if err := util.Open(it.Local); err == nil {
					detail.SetText("Opened " + it.Local)
				}
			}
			dismiss := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameContentClear), func() {
				mw.transfers.Dismiss(id)
			})
			dismiss.Importance = widget.LowImportance
			actions.Add(dismiss)
		}
	}

	go func() {
		for {
			it := mw.transfers.Find(id)
			if it == nil {
				return
			}
			fyne.Do(func() {
				bar.SetValue(it.Progress())
				status.SetText(it.Status.String())
				detail.SetText(it.Detail())
			})
			if it.Status != transfer.Running && it.Status != transfer.Queued && it.Status != transfer.Paused {
				fyne.Do(mw.transfersPanel.Refresh)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	return container.NewBorder(nil, nil, nil, actions,
		container.NewPadded(cols))
}
