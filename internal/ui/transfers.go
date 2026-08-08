package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	r2theme "r2go/internal/theme"
	"r2go/internal/transfer"
)

type TransferPanel struct {
	mw        *MainWindow
	manager   *transfer.Manager
	Container fyne.CanvasObject
	root      *fyne.Container
	content   *fyne.Container
	count     *widget.Label
	notified  map[string]bool
	rows      int
}

type transferGroup struct {
	id        string
	label     string
	kind      transfer.Kind
	items     []transfer.Item
	total     int64
	done      int64
	speed     float64
	finished  int
	failed    int
	cancelled int
	active    int
	lastDone  time.Time
}

func NewTransferPanel(mw *MainWindow, manager *transfer.Manager) *TransferPanel {
	panelView := &TransferPanel{mw: mw, manager: manager, content: tightVBox(), notified: map[string]bool{}}
	title := widget.NewLabelWithStyle("Transfers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = fyneTheme.SizeNameCaptionText
	panelView.count = mutedLabel("")
	clear := widget.NewButton("Clear", manager.ClearFinished)
	clear.Importance = widget.LowImportance
	header := container.NewBorder(nil, nil, container.NewHBox(title, panelView.count), fixedSize(62, 28, clear))
	header = container.NewBorder(widget.NewSeparator(), nil, nil, nil, inset(8, 1, 8, 1, header))
	panelView.root = tightVBox(header, panelView.content)
	panelView.Container = panel(r2theme.ColorToolbar, panelView.root)
	panelView.Container.Hide()

	go func() {
		ticker := time.NewTicker(350 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			manager.PruneSuccessful(6 * time.Second)
			fyne.Do(panelView.refresh)
		}
	}()
	return panelView
}

func (p *TransferPanel) refresh() {
	groups := aggregateTransfers(p.manager.Snapshot())
	visible := make([]transferGroup, 0, len(groups))
	for _, group := range groups {
		if group.active > 0 || group.failed > 0 || (!group.lastDone.IsZero() && time.Since(group.lastDone) < 6*time.Second) {
			visible = append(visible, group)
		}
		p.notifyIfFinished(group)
	}
	if len(visible) == 0 {
		wasVisible := p.Container.Visible()
		p.Container.Hide()
		p.content.Objects = nil
		p.content.Refresh()
		p.root.Refresh()
		if wasVisible || p.rows != 0 {
			p.rows = 0
			p.mw.relayoutShell()
		}
		return
	}
	wasVisible := p.Container.Visible()
	p.Container.Show()
	active := p.manager.ActiveCount()
	if active > 0 {
		p.count.SetText(fmt.Sprintf("· %d active", active))
	} else {
		p.count.SetText("")
	}

	objects := make([]fyne.CanvasObject, 0, len(visible))
	for i := len(visible) - 1; i >= 0 && len(objects) < 3; i-- {
		objects = append(objects, p.groupRow(visible[i]))
	}
	p.content.Objects = objects
	p.content.Refresh()
	p.root.Refresh()
	if !wasVisible || p.rows != len(objects) {
		p.rows = len(objects)
		p.mw.relayoutShell()
	}
}

func aggregateTransfers(items []transfer.Item) []transferGroup {
	order := []string{}
	groups := map[string]*transferGroup{}
	for _, item := range items {
		id := item.GroupID
		if id == "" {
			id = item.ID
		}
		group := groups[id]
		if group == nil {
			label := item.GroupLabel
			if label == "" {
				verb := "Uploading"
				if item.Kind == transfer.Download {
					verb = "Downloading"
				}
				label = verb + " " + item.Name()
			}
			group = &transferGroup{id: id, label: label, kind: item.Kind}
			groups[id] = group
			order = append(order, id)
		}
		group.items = append(group.items, item)
		group.total += item.Total
		group.done += item.Completed
		group.speed += item.Speed()
		if item.FinishedAt.After(group.lastDone) {
			group.lastDone = item.FinishedAt
		}
		switch item.State {
		case transfer.Waiting, transfer.Running:
			group.active++
		case transfer.Done:
			group.finished++
		case transfer.Failed:
			group.finished++
			group.failed++
		case transfer.Cancelled:
			group.finished++
			group.cancelled++
		}
	}
	out := make([]transferGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *groups[id])
	}
	return out
}

func (p *TransferPanel) groupRow(group transferGroup) fyne.CanvasObject {
	label := widget.NewLabel(group.label)
	label.Truncation = fyne.TextTruncateEllipsis
	label.TextStyle = fyne.TextStyle{Bold: true}
	status := mutedLabel(transferGroupStatus(group))

	progress := newMiniProgress()
	if group.total > 0 {
		progress.SetValue(float64(group.done) / float64(group.total))
	} else if group.active == 0 && group.failed == 0 {
		progress.SetValue(1)
	}

	var action fyne.CanvasObject
	if group.active > 0 {
		cancel := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameCancel), func() {
			for _, item := range group.items {
				p.manager.Cancel(item.ID)
			}
		})
		cancel.Importance = widget.LowImportance
		action = fixedSize(26, 26, cancel)
	} else if group.failed > 0 {
		failed := widget.NewLabel(fmt.Sprintf("%d failed", group.failed))
		failed.Importance = widget.DangerImportance
		failed.SizeName = fyneTheme.SizeNameCaptionText
		action = failed
	} else {
		done := widget.NewLabel("Done")
		done.Importance = widget.SuccessImportance
		done.SizeName = fyneTheme.SizeNameCaptionText
		action = done
	}

	top := container.NewBorder(nil, nil, label, action)
	body := tightVBox(top, status, progress)
	return container.NewBorder(nil, widget.NewSeparator(), nil, nil, inset(8, 2, 8, 2, body))
}

func transferGroupStatus(group transferGroup) string {
	count := len(group.items)
	completed := group.finished
	parts := []string{fmt.Sprintf("%d/%d", completed, count)}
	if group.total > 0 {
		parts = append(parts, fmt.Sprintf("%s / %s", formatSize(group.done), formatSize(group.total)))
	}
	if group.speed > 0 && group.active > 0 {
		parts = append(parts, formatSpeed(group.speed))
	}
	if group.cancelled > 0 {
		parts = append(parts, fmt.Sprintf("%d cancelled", group.cancelled))
	}
	return joinDot(parts)
}

func joinDot(values []string) string {
	result := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if result != "" {
			result += "  ·  "
		}
		result += value
	}
	return result
}

func (p *TransferPanel) notifyIfFinished(group transferGroup) {
	if !p.manager.GroupClosed(group.id) || group.active > 0 || len(group.items) == 0 || group.finished != len(group.items) || p.notified[group.id] {
		return
	}
	p.notified[group.id] = true
	if group.failed > 0 {
		p.mw.notify("Transfer needs attention", fmt.Sprintf("%s — %d file(s) failed", group.label, group.failed))
		return
	}
	if group.cancelled == len(group.items) {
		return
	}
	p.mw.notify("Transfer complete", group.label)
}
