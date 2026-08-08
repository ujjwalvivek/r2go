package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/model"
	"r2go/internal/r2"
	r2theme "r2go/internal/theme"
)

type Browser struct {
	mw          *MainWindow
	Container   fyne.CanvasObject
	table       *FileTable
	loadMore    *widget.Button
	loadMoreBar fyne.CanvasObject

	list        fyne.CanvasObject
	loading     fyne.CanvasObject
	placeholder fyne.CanvasObject
	empty       fyne.CanvasObject

	session     model.Session
	all         []model.Entry
	visible     []model.Entry
	nextToken   string
	hasMore     bool
	busy        bool
	generation  uint64
	loadCancel  context.CancelFunc
	statsCtx    context.Context
	statsCancel context.CancelFunc
	statsSem    chan struct{}
	statsQueued map[string]struct{}

	filter string
	sort   model.SortState

	selected  map[string]struct{}
	focusKey  string
	anchorKey string
	hoverKey  string

	lastClickKey string
	lastClickAt  time.Time
}

func NewBrowser(mw *MainWindow) *Browser {
	browser := &Browser{
		mw:          mw,
		selected:    map[string]struct{}{},
		sort:        model.SortState{Field: model.SortName},
		statsSem:    make(chan struct{}, 3),
		statsQueued: map[string]struct{}{},
	}
	browser.resetFolderStats()
	browser.table = NewFileTable(
		func() int { return len(browser.visible) },
		func() fyne.CanvasObject { return newFileRow() },
		browser.updateRow,
		browser.sortColumn,
	)
	browser.table.SetSort(0, false)

	browser.loadMore = widget.NewButton("Load more", browser.LoadMore)
	browser.loadMore.Importance = widget.LowImportance
	browser.loadMore.Hide()
	browser.loadMoreBar = panel(r2theme.ColorSurface, inset(8, 3, 8, 3, container.NewCenter(browser.loadMore)))
	browser.loadMoreBar.Hide()
	browser.list = container.NewBorder(nil, browser.loadMoreBar, nil, nil, browser.table)

	spinner := widget.NewProgressBarInfinite()
	spinner.Start()
	loadingText := widget.NewLabelWithStyle("Loading objects", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	loadingText.SizeName = fyneTheme.SizeNameText
	loadingHint := mutedLabel("Cloudflare R2")
	loadingHint.Alignment = fyne.TextAlignCenter
	loadingBody := container.NewVBox(loadingText, fixedSize(180, 4, spinner), loadingHint)
	browser.loading = panel(r2theme.ColorSurface, container.NewCenter(loadingBody))

	placeholderIcon := widget.NewIcon(fyneTheme.Icon(fyneTheme.IconNameStorage))
	placeholderIcon.Resize(fyne.NewSize(42, 42))
	placeholderTitle := widget.NewLabelWithStyle("Choose a bucket", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	placeholderHint := mutedLabel("Select a bucket from the sidebar to start browsing.")
	placeholderHint.Alignment = fyne.TextAlignCenter
	browser.placeholder = panel(r2theme.ColorSurface, container.NewCenter(container.NewVBox(placeholderIcon, placeholderTitle, placeholderHint)))

	emptyIcon := widget.NewIcon(fyneTheme.Icon(fyneTheme.IconNameFolder))
	emptyIcon.Resize(fyne.NewSize(38, 38))
	emptyTitle := widget.NewLabelWithStyle("This folder is empty", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	emptyHint := mutedLabel("Drop files here or use Upload.")
	emptyHint.Alignment = fyne.TextAlignCenter
	browser.empty = panel(r2theme.ColorSurface, container.NewCenter(container.NewVBox(emptyIcon, emptyTitle, emptyHint)))

	browser.Container = container.NewStack(browser.list, browser.loading, browser.empty, browser.placeholder)
	browser.showPlaceholder()
	return browser
}

func (b *Browser) updateRow(rowID int, object fyne.CanvasObject) {
	row := object.(*fileRow)
	row.row = rowID
	row.SetWidths(b.table.Widths())
	row.onClick = b.onCellClick
	row.onMenu = b.onCellMenu
	row.onHover = b.onCellHover

	if rowID < 0 || rowID >= len(b.visible) {
		row.name.SetText("")
		row.size.SetText("")
		row.modified.SetText("")
		row.icon.SetResource(nil)
		row.SetState(false, false)
		return
	}
	entry := b.visible[rowID]
	row.icon.SetResource(entryIcon(entry))
	row.name.SetText(entry.Name)
	row.name.TextStyle = fyne.TextStyle{}
	if entry.IsDir {
		row.name.TextStyle = fyne.TextStyle{Bold: true}
	}
	row.size.Importance = widget.MediumImportance
	row.modified.Importance = widget.MediumImportance

	if entry.IsDir && !entry.StatsKnown {
		if entry.StatsError {
			row.size.SetText("—")
			row.modified.SetText("—")
		} else {
			row.size.SetText("…")
			row.modified.SetText("…")
		}
		row.size.Importance = widget.LowImportance
		row.modified.Importance = widget.LowImportance
	} else {
		row.size.SetText(formatSize(entry.Size))
		if entry.Modified.IsZero() {
			row.modified.SetText("—")
			row.modified.Importance = widget.LowImportance
		} else {
			row.modified.SetText(entry.Modified.Local().Format("02 Jan 2006  15:04"))
		}
	}
	_, selected := b.selected[entry.Key]
	row.SetState(selected, b.hoverKey == entry.Key)
}

func (b *Browser) cancelLoad() {
	if b.loadCancel != nil {
		b.loadCancel()
		b.loadCancel = nil
	}
	b.busy = false
}

func (b *Browser) SetSession(session model.Session) {
	b.cancelLoad()
	b.resetFolderStats()
	b.generation++
	b.session = session
	b.all = nil
	b.visible = nil
	b.nextToken = ""
	b.hasMore = false
	b.filter = ""
	b.sort = model.SortState{Field: model.SortName}
	b.table.SetSort(0, false)
	b.ClearSelection()
	if session.Bucket == "" {
		b.showPlaceholder()
		return
	}
	b.reload()
}

func (b *Browser) Navigate(prefix string) {
	if b.session.Bucket == "" {
		return
	}
	b.cancelLoad()
	b.resetFolderStats()
	b.generation++
	b.session.Prefix = prefix
	b.all = nil
	b.visible = nil
	b.nextToken = ""
	b.hasMore = false
	b.ClearSelection()
	b.reload()
}

func (b *Browser) Reload() {
	if b.session.Bucket == "" {
		return
	}
	b.cancelLoad()
	b.resetFolderStats()
	b.generation++
	b.all = nil
	b.visible = nil
	b.nextToken = ""
	b.hasMore = false
	b.ClearSelection()
	b.reload()
}

func (b *Browser) reload() { b.loadPage("") }

func (b *Browser) LoadMore() {
	if b.busy || !b.hasMore || b.nextToken == "" {
		return
	}
	b.loadPage(b.nextToken)
}

func (b *Browser) loadPage(token string) {
	if b.busy {
		return
	}
	b.busy = true
	initial := token == ""
	if initial {
		b.showLoading()
	}
	b.loadMore.Disable()

	session := b.session
	generation := b.generation
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	b.loadCancel = cancel
	b.mw.status.SetMessage("Loading " + session.Bucket + "/" + session.Prefix + "…")
	go func() {
		defer cancel()
		client, err := b.mw.clientFor(session.Account)
		var page r2.ListPage
		if err == nil {
			page, err = client.ListPage(ctx, session.Bucket, session.Prefix, token)
		}
		fyne.Do(func() {
			if generation != b.generation || session != b.session {
				return
			}
			b.loadCancel = nil
			b.busy = false
			b.loadMore.Enable()
			if err != nil {
				b.showList()
				b.mw.showError("Could not load objects", err)
				return
			}
			entries := pageEntries(page, session.Prefix)
			b.all = append(b.all, entries...)
			b.nextToken = page.NextToken
			b.hasMore = page.Truncated
			b.applyView(false)
			b.queueFolderStats(entries, session, generation)
			b.mw.status.SetMessage(fmt.Sprintf("Loaded %d item%s", len(b.all), plural(len(b.all))))
		})
	}()
}

func pageEntries(page r2.ListPage, prefix string) []model.Entry {
	entries := make([]model.Entry, 0, len(page.Prefixes)+len(page.Objects))
	for _, value := range page.Prefixes {
		name := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "/")
		entries = append(entries, model.Entry{Key: value, Name: name, IsDir: true})
	}
	for _, object := range page.Objects {
		name := strings.TrimPrefix(object.Key, prefix)
		if strings.Contains(name, "/") {
			continue
		}
		entries = append(entries, model.Entry{
			Key: object.Key, Name: name, Size: object.Size,
			Modified: object.Modified, ETag: object.ETag, StatsKnown: true,
		})
	}
	return entries
}

func (b *Browser) resetFolderStats() {
	if b.statsCancel != nil {
		b.statsCancel()
	}
	b.statsCtx, b.statsCancel = context.WithCancel(context.Background())
	b.statsQueued = map[string]struct{}{}
}

func (b *Browser) queueFolderStats(entries []model.Entry, session model.Session, generation uint64) {
	if b.statsCtx == nil {
		b.resetFolderStats()
	}
	ctx := b.statsCtx
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}
		if _, exists := b.statsQueued[entry.Key]; exists {
			continue
		}
		b.statsQueued[entry.Key] = struct{}{}
		entry := entry
		go func() {
			select {
			case b.statsSem <- struct{}{}:
				defer func() { <-b.statsSem }()
			case <-ctx.Done():
				return
			}

			statCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			client, err := b.mw.clientFor(session.Account)
			var size int64
			var modified time.Time
			if err == nil {
				var objects []r2.Object
				objects, err = client.ListAll(statCtx, session.Bucket, entry.Key)
				if err == nil {
					for _, object := range objects {
						size += object.Size
						if object.Modified.After(modified) {
							modified = object.Modified
						}
					}
				}
			}
			fyne.Do(func() {
				if generation != b.generation || session != b.session {
					return
				}
				b.applyFolderStats(entry.Key, size, modified, err)
			})
		}()
	}
}

func (b *Browser) applyFolderStats(key string, size int64, modified time.Time, err error) {
	update := func(entries []model.Entry) bool {
		for i := range entries {
			if entries[i].Key != key {
				continue
			}
			entries[i].StatsKnown = err == nil
			entries[i].StatsError = err != nil
			if err == nil {
				entries[i].Size = size
				entries[i].Modified = modified
			}
			return true
		}
		return false
	}
	update(b.all)
	update(b.visible)
	if b.sort.Field == model.SortSize || b.sort.Field == model.SortModified {
		b.applyView(false)
		return
	}
	b.refreshKey(key)
}

func (b *Browser) SetFilter(filter string) {
	filter = strings.TrimSpace(filter)
	if filter == b.filter {
		return
	}
	b.filter = filter
	b.ClearSelection()
	b.applyView(false)
}

func (b *Browser) sortColumn(column int) {
	field := model.SortName
	switch column {
	case 1:
		field = model.SortSize
	case 2:
		field = model.SortModified
	}
	if b.sort.Field == field {
		b.sort.Desc = !b.sort.Desc
	} else {
		b.sort = model.SortState{Field: field}
	}
	b.table.SetSort(column, b.sort.Desc)
	b.applyView(false)
}

func (b *Browser) applyView(clearSelection bool) {
	if clearSelection {
		b.ClearSelection()
	}
	b.visible = model.FilterAndSort(b.all, b.filter, b.sort)
	b.table.RefreshRows(len(b.visible))
	if len(b.visible) == 0 {
		b.showEmpty()
	} else {
		b.showList()
	}
	if b.hasMore {
		b.loadMore.Show()
		b.loadMoreBar.Show()
	} else {
		b.loadMore.Hide()
		b.loadMoreBar.Hide()
	}
	count := fmt.Sprintf("%d item%s", len(b.visible), plural(len(b.visible)))
	if b.hasMore {
		count += " · more available"
	}
	b.mw.status.SetRight(count)
}

func (b *Browser) onCellClick(row int, modifier fyne.KeyModifier) {
	if b.busy || row < 0 || row >= len(b.visible) {
		return
	}
	entry := b.visible[row]
	control := modifier&fyne.KeyModifierControl != 0 || modifier&fyne.KeyModifierSuper != 0
	shift := modifier&fyne.KeyModifierShift != 0

	if shift && b.anchorKey != "" {
		b.selectRange(b.anchorKey, entry.Key, control)
	} else if control {
		if _, ok := b.selected[entry.Key]; ok {
			delete(b.selected, entry.Key)
		} else {
			b.selected[entry.Key] = struct{}{}
		}
		b.anchorKey = entry.Key
	} else {
		b.selected = map[string]struct{}{entry.Key: {}}
		b.anchorKey = entry.Key
	}
	b.focusKey = entry.Key
	b.selectionChanged()
	b.table.RefreshList()

	now := time.Now()
	double := !control && !shift && b.lastClickKey == entry.Key && now.Sub(b.lastClickAt) <= 450*time.Millisecond
	b.lastClickKey = entry.Key
	b.lastClickAt = now
	if double {
		b.mw.openEntry(entry)
	}
}

func (b *Browser) onCellMenu(row int, position fyne.Position) {
	if row < 0 || row >= len(b.visible) {
		return
	}
	entry := b.visible[row]
	if _, ok := b.selected[entry.Key]; !ok {
		b.selected = map[string]struct{}{entry.Key: {}}
		b.focusKey = entry.Key
		b.anchorKey = entry.Key
		b.selectionChanged()
		b.table.RefreshList()
	}
	b.mw.showContextMenu(b.SelectedEntries(), position)
}

func (b *Browser) onCellHover(row int, hovering bool) {
	if row < 0 || row >= len(b.visible) {
		return
	}
	old := b.hoverKey
	if hovering {
		b.hoverKey = b.visible[row].Key
	} else if b.hoverKey == b.visible[row].Key {
		b.hoverKey = ""
	}
	b.refreshKey(old)
	b.refreshKey(b.hoverKey)
}

func (b *Browser) refreshKey(key string) {
	if key == "" {
		return
	}
	for row, entry := range b.visible {
		if entry.Key != key {
			continue
		}
		b.table.RefreshRow(row)
		return
	}
}

func (b *Browser) selectRange(fromKey, toKey string, preserve bool) {
	from, to := -1, -1
	for i, entry := range b.visible {
		if entry.Key == fromKey {
			from = i
		}
		if entry.Key == toKey {
			to = i
		}
	}
	if from < 0 || to < 0 {
		return
	}
	if from > to {
		from, to = to, from
	}
	if !preserve {
		b.selected = map[string]struct{}{}
	}
	for i := from; i <= to; i++ {
		b.selected[b.visible[i].Key] = struct{}{}
	}
}

func (b *Browser) SelectAll() {
	b.selected = make(map[string]struct{}, len(b.visible))
	for _, entry := range b.visible {
		b.selected[entry.Key] = struct{}{}
	}
	if len(b.visible) > 0 {
		b.focusKey = b.visible[0].Key
		b.anchorKey = b.focusKey
	}
	b.selectionChanged()
	b.table.RefreshList()
}

func (b *Browser) ClearSelection() {
	b.selected = map[string]struct{}{}
	b.focusKey = ""
	b.anchorKey = ""
	b.selectionChanged()
	if b.table != nil {
		b.table.RefreshList()
	}
}

func (b *Browser) SelectedEntries() []model.Entry {
	if len(b.selected) == 0 {
		return nil
	}
	out := make([]model.Entry, 0, len(b.selected))
	for _, entry := range b.visible {
		if _, ok := b.selected[entry.Key]; ok {
			out = append(out, entry)
		}
	}
	return out
}

func (b *Browser) SelectedEntry() (model.Entry, bool) {
	entries := b.SelectedEntries()
	if len(entries) != 1 {
		return model.Entry{}, false
	}
	return entries[0], true
}

func (b *Browser) MoveFocus(delta int, extend bool) {
	if len(b.visible) == 0 {
		return
	}
	index := -1
	for i, entry := range b.visible {
		if entry.Key == b.focusKey {
			index = i
			break
		}
	}
	if index < 0 {
		if delta < 0 {
			index = len(b.visible) - 1
		} else {
			index = 0
		}
	} else {
		index += delta
		if index < 0 {
			index = 0
		}
		if index >= len(b.visible) {
			index = len(b.visible) - 1
		}
	}
	key := b.visible[index].Key
	if extend && b.anchorKey != "" {
		b.selectRange(b.anchorKey, key, false)
	} else {
		b.selected = map[string]struct{}{key: {}}
		b.anchorKey = key
	}
	b.focusKey = key
	b.selectionChanged()
	b.table.RefreshList()
	b.table.ScrollToRow(index)
}

func (b *Browser) OpenSelected() {
	if entry, ok := b.SelectedEntry(); ok {
		b.mw.openEntry(entry)
	}
}

func (b *Browser) selectionChanged() {
	if b.mw == nil {
		return
	}
	b.mw.selectionChanged(len(b.selected))
	if len(b.selected) > 0 {
		b.mw.status.SetRight(fmt.Sprintf("%d selected", len(b.selected)))
	} else if b.session.Bucket != "" {
		count := fmt.Sprintf("%d item%s", len(b.visible), plural(len(b.visible)))
		if b.hasMore {
			count += " · more available"
		}
		b.mw.status.SetRight(count)
	}
}

func (b *Browser) Session() model.Session { return b.session }
func (b *Browser) Filter() string         { return b.filter }

func (b *Browser) showPlaceholder() {
	b.list.Hide()
	b.loading.Hide()
	b.empty.Hide()
	b.placeholder.Show()
}

func (b *Browser) showLoading() {
	b.placeholder.Hide()
	b.empty.Hide()
	b.list.Hide()
	b.loading.Show()
}

func (b *Browser) showEmpty() {
	b.placeholder.Hide()
	b.loading.Hide()
	b.list.Hide()
	b.empty.Show()
}

func (b *Browser) showList() {
	b.placeholder.Hide()
	b.loading.Hide()
	b.empty.Hide()
	b.list.Show()
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
