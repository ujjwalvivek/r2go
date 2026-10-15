package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	t "r2go/internal/theme"
	"r2go/internal/transfer"
)

const (
	transferTick      = 350 * time.Millisecond
	transferLinger    = 6 * time.Second // finished groups stay this long
	transferRowsShown = 5
)

// TransferPanel is the drawer above the status bar. It appears only while
// transfers run or need attention. The header carries the totals and folds the
// rows away; each row is one upload or download batch with a thin bar.
type TransferPanel struct {
	mw        *MainWindow
	manager   *transfer.Manager
	Container fyne.CanvasObject
	rows      *fyne.Container
	summary   *textView
	speed     *textView
	overall   *meter
	toggle    *button
	clear     *button
	collapsed bool
	notified  map[string]bool
	rowCache  map[string]*transferRow
	shown     int
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
	firstErr  error
}

func NewTransferPanel(mw *MainWindow, manager *transfer.Manager) *TransferPanel {
	p := &TransferPanel{
		mw:       mw,
		manager:  manager,
		rows:     vbox(0),
		summary:  newTextView("", t.TextSmall, t.ColorMuted, styleMono),
		speed:    newTextView("", t.TextSmall, t.ColorMuted, styleMono),
		overall:  newMeter(2, t.ColorAccent),
		notified: map[string]bool{},
		rowCache: map[string]*transferRow{},
	}
	p.toggle = newIconButton(t.IconChevronD, "Hide transfers", p.toggleCollapsed)
	p.clear = newButton(buttonQuiet, "Clear finished", "", manager.ClearFinished)
	title := newText("Transfers", t.TextSmall, t.ColorSubtle, styleStrong)
	left := container.New(&rowLayout{gap: 10}, title, p.summary)
	right := container.New(&rowLayout{gap: 8}, p.speed, fixed(80, 2, p.overall), p.clear, p.toggle)
	header := newHoverArea(minHeight(t.HeightNavItem, inset(t.SpaceCardX, 0, 4, 0, container.NewBorder(nil, nil, left, right))), p.toggleCollapsed)
	header.hoverFill = t.ColorPanel
	body := vbox(0, hRule(t.ColorBorder), header, p.rows)
	p.Container = panelBox(t.ColorPanel, body)
	p.Container.Hide()

	go func() {
		ticker := time.NewTicker(transferTick)
		defer ticker.Stop()
		for range ticker.C {
			manager.PruneSuccessful(transferLinger)
			fyne.Do(p.refresh)
		}
	}()
	return p
}

func (p *TransferPanel) toggleCollapsed() {
	p.collapsed = !p.collapsed
	if p.collapsed {
		p.toggle.SetIcon(t.IconChevronU)
		p.toggle.SetTip("Show transfers")
	} else {
		p.toggle.SetIcon(t.IconChevronD)
		p.toggle.SetTip("Hide transfers")
	}
	p.refresh()
}

func (p *TransferPanel) refresh() {
	groups := aggregateTransfers(p.manager.Snapshot())
	state := transferStatusState(groups)
	p.mw.setTransferState(state)

	visible := make([]transferGroup, 0, len(groups))
	var total, done int64
	var speed float64
	active, failed := 0, 0
	for _, group := range groups {
		p.notifyIfFinished(group)
		if group.active > 0 || group.failed > 0 || (!group.lastDone.IsZero() && time.Since(group.lastDone) < transferLinger) {
			visible = append(visible, group)
			total += group.total
			done += group.done
			if group.active > 0 {
				speed += group.speed
			}
			active += group.active
			failed += group.failed
		}
	}
	live := make(map[string]struct{}, len(visible))
	for _, group := range visible {
		live[group.id] = struct{}{}
	}
	for id := range p.rowCache {
		if _, ok := live[id]; !ok {
			delete(p.rowCache, id)
		}
	}
	if len(visible) == 0 {
		if p.Container.Visible() {
			p.Container.Hide()
			p.mw.relayout()
		}
		p.shown = 0
		return
	}

	parts := []string{}
	if active > 0 {
		parts = append(parts, fmt.Sprintf("%s active", formatCount(active)))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%s failed", formatCount(failed)))
	}
	if len(parts) == 0 {
		parts = append(parts, "done")
	}
	p.summary.SetText(joinDot(parts...))
	if failed > 0 {
		p.summary.SetColorName(t.ColorError)
	} else {
		p.summary.SetColorName(t.ColorMuted)
	}
	p.speed.SetText(formatSpeed(speed))
	if total > 0 {
		p.overall.SetValue(float64(done) / float64(total))
	}
	if active == 0 {
		p.overall.SetValue(1)
	}
	if failed > 0 {
		p.overall.SetFill(t.ColorError)
	} else if active == 0 {
		p.overall.SetFill(t.ColorSuccess)
	} else {
		p.overall.SetFill(t.ColorAccent)
	}
	p.clear.SetDisabled(len(visible) == active && failed == 0)

	objects := []fyne.CanvasObject{}
	if !p.collapsed {
		for i := len(visible) - 1; i >= 0 && len(objects) < transferRowsShown; i-- {
			objects = append(objects, p.row(visible[i]).Container)
		}
		if hidden := len(visible) - transferRowsShown; hidden > 0 {
			objects = append(objects, inset(t.SpaceCardX, 3, t.SpaceCardX, 5,
				newText(fmt.Sprintf("and %s more", countNoun(hidden, "batch")), t.TextSmall, t.ColorMuted, styleNormal)))
		}
	}
	p.rows.Objects = objects
	p.rows.Refresh()

	wasVisible := p.Container.Visible()
	p.Container.Show()
	if !wasVisible || p.shown != len(objects) {
		p.shown = len(objects)
		p.mw.relayout()
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
			if group.firstErr == nil {
				group.firstErr = item.Err
			}
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

// transferRow is one batch in the drawer. Rows are kept between ticks and
// updated in place, so hover states and tooltips survive the refresh.
type transferRow struct {
	Container fyne.CanvasObject
	icon      *canvas.Image
	label     *textView
	status    *textView
	progress  *meter
	actions   *fyne.Container
	top       *fyne.Container
	phase     string
}

func (p *TransferPanel) row(group transferGroup) *transferRow {
	if row, ok := p.rowCache[group.id]; ok {
		row.update(p, group)
		return row
	}
	row := &transferRow{
		icon:     newIcon(t.IconUpload, t.ColorAccent),
		label:    newElided(group.label, t.TextBody, t.ColorText, styleNormal),
		status:   newTextView("", t.TextSmall, t.ColorMuted, styleMono),
		progress: newMeter(2, t.ColorAccent),
		actions:  container.New(&rowLayout{gap: 2}),
	}
	row.top = container.NewBorder(nil, nil, nil, container.New(&rowLayout{gap: 10}, row.status, row.actions), row.label)
	line := container.NewBorder(nil, nil, inset(0, 0, 8, 0, container.NewCenter(row.icon)), nil, vbox(3, minHeight(t.HeightControl, row.top), row.progress))
	row.Container = inset(t.SpaceCardX, 0, 6, 6, line)
	p.rowCache[group.id] = row
	row.update(p, group)
	return row
}

func (r *transferRow) update(p *TransferPanel, group transferGroup) {
	icon := t.IconUpload
	if group.kind == transfer.Download {
		icon = t.IconDownload
	}
	phase, colorName := "active", t.ColorAccent
	switch {
	case group.active > 0:
	case group.failed > 0:
		phase, colorName = "failed", t.ColorError
	case group.cancelled == len(group.items):
		phase, colorName = "cancelled", t.ColorMuted
	default:
		phase, colorName = "done", t.ColorSuccess
	}
	setIcon(r.icon, icon, colorName)
	r.label.SetText(group.label)
	r.status.SetText(transferGroupStatus(group))
	if colorName == t.ColorMuted {
		r.progress.SetFill(t.ColorBorderStrong)
	} else {
		r.progress.SetFill(colorName)
	}
	switch {
	case group.total > 0:
		r.progress.SetValue(float64(group.done) / float64(group.total))
	case group.active == 0:
		r.progress.SetValue(1)
	}
	if phase == r.phase && phase != "failed" {
		r.top.Refresh()
		return
	}
	r.phase = phase
	var actions []fyne.CanvasObject
	switch phase {
	case "active":
		actions = append(actions, newIconButton(t.IconClose, "Cancel\nStop the files that have not finished", func() {
			for _, item := range group.items {
				p.manager.Cancel(item.ID)
			}
		}))
	case "failed":
		e := explainError(group.firstErr)
		why := newIconButton(t.IconAlert, fmt.Sprintf("%s\n%s\n%s", e.Title, e.Detail, e.Raw), nil)
		ids := []string{}
		for _, item := range group.items {
			if item.State == transfer.Failed {
				ids = append(ids, item.ID)
			}
		}
		retry := newButton(buttonQuiet, "Retry", t.IconRefresh, func() {
			for _, id := range ids {
				p.manager.Retry(id)
			}
		}).withTip(fmt.Sprintf("Retry  ·  %s\nQueue the failed files again", countNoun(group.failed, "file")))
		actions = append(actions, why, retry)
	case "done":
		actions = append(actions, container.NewCenter(newIcon(t.IconCheck, t.ColorSuccess)))
	}
	r.actions.Objects = actions
	r.actions.Refresh()
	r.top.Refresh()
}

// transferStatusState reports the status bar square. A failure outranks running
// work because it needs attention; failed groups stay listed until cleared.
func transferStatusState(groups []transferGroup) StatusState {
	state := StatusReady
	for _, group := range groups {
		if group.failed > 0 {
			return StatusFailed
		}
		if group.active > 0 {
			state = StatusWorking
		}
	}
	return state
}

func transferGroupStatus(group transferGroup) string {
	count := len(group.items)
	parts := []string{}
	if count > 1 {
		parts = append(parts, fmt.Sprintf("%s/%s", formatCount(group.finished), formatCount(count)))
	}
	if group.total > 0 {
		if group.active > 0 {
			parts = append(parts, fmt.Sprintf("%s of %s", formatSize(group.done), formatSize(group.total)))
		} else {
			parts = append(parts, formatSize(group.total))
		}
	}
	if group.speed > 0 && group.active > 0 {
		parts = append(parts, formatSpeed(group.speed))
		if remaining := group.total - group.done; remaining > 0 {
			parts = append(parts, formatDuration(time.Duration(float64(remaining)/group.speed*float64(time.Second)))+" left")
		}
	}
	if group.failed > 0 {
		parts = append(parts, fmt.Sprintf("%s failed", formatCount(group.failed)))
	}
	if group.cancelled > 0 {
		parts = append(parts, fmt.Sprintf("%s cancelled", formatCount(group.cancelled)))
	}
	if group.active == 0 && group.failed == 0 && group.cancelled == 0 {
		parts = append(parts, "done")
	}
	return joinDot(parts...)
}

func (p *TransferPanel) notifyIfFinished(group transferGroup) {
	if !p.manager.GroupClosed(group.id) || group.active > 0 || len(group.items) == 0 || group.finished != len(group.items) || p.notified[group.id] {
		return
	}
	p.notified[group.id] = true
	if group.failed > 0 {
		p.mw.notify("Transfer needs attention", fmt.Sprintf("%s: %s failed", group.label, countNoun(group.failed, "file")))
		return
	}
	if group.cancelled == len(group.items) {
		return
	}
	p.mw.notify("Transfer complete", group.label)
	if group.kind == transfer.Upload {
		p.mw.uploadsFinished(group.items)
	}
}
