package ui

import (
	"context"
	"fmt"
	"image"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"r2go/internal/model"
	"r2go/internal/r2"
	t "r2go/internal/theme"
)

const (
	viewList = "list"
	viewGrid = "grid"
)

// Browser lists one folder of one bucket, as a table or a grid, and owns the
// selection.
type Browser struct {
	mw        *MainWindow
	Container fyne.CanvasObject

	table    *FileTable
	grid     *GridView
	views    *fyne.Container
	notice   *fyne.Container
	banner   *banner
	moreBar  fyne.CanvasObject
	moreText *textView
	more     *button
	view     string

	session     model.Session
	all         []model.Entry
	visible     []model.Entry
	maxSize     int64
	nextToken   string
	hasMore     bool
	busy        bool
	loaded      bool
	loadErr     error
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

	previewsDue  bool
	lastClickKey string
	lastClickAt  time.Time
	now          time.Time
}

func NewBrowser(mw *MainWindow) *Browser {
	b := &Browser{
		mw:          mw,
		selected:    map[string]struct{}{},
		sort:        model.SortState{Field: model.SortName},
		statsSem:    make(chan struct{}, 3),
		statsQueued: map[string]struct{}{},
		view:        viewList,
		now:         time.Now(),
	}
	b.resetFolderStats()
	b.table = NewFileTable(
		func() int { return len(b.visible) },
		func() fyne.CanvasObject { return newFileRow() },
		b.updateRow,
		b.sortColumn,
	)
	b.table.SetSort(model.SortName, false)
	b.grid = NewGridView(
		func() int { return len(b.visible) },
		func() fyne.CanvasObject { return newGridTile() },
		b.updateTile,
	)

	b.moreText = newTextView("", t.TextSmall, t.ColorMuted, styleNormal)
	b.more = newButton(buttonGhost, "Load more", "", b.LoadMore)
	b.moreBar = panelBox(t.ColorBase, container.NewBorder(hRule(t.ColorBorder), nil, nil, nil,
		inset(t.SpaceCardX, 4, t.SpaceCardX, 4, container.NewBorder(nil, nil, b.moreText, b.more))))
	b.moreBar.Hide()

	b.banner = newBanner()
	b.notice = container.NewStack()
	b.views = container.NewStack(b.table, b.grid.grid)
	b.grid.grid.Hide()
	body := container.NewBorder(b.banner.Container, b.moreBar, nil, nil, container.NewStack(b.views, b.notice))
	b.Container = panelBox(t.ColorBase, body)
	b.render()
	return b
}

// ---------------------------------------------------------------- rows and tiles

func (b *Browser) updateRow(rowID int, object fyne.CanvasObject) {
	row := object.(*fileRow)
	row.row = rowID
	row.onClick = b.onItemClick
	row.onMenu = b.onItemMenu
	row.onHover = b.onItemHover
	if rowID < 0 || rowID >= len(b.visible) {
		row.Set(model.Entry{}, b.table.Widths(), 0, b.now)
		row.SetState(false, false)
		return
	}
	entry := b.visible[rowID]
	row.Set(entry, b.table.Widths(), b.maxSize, b.now)
	_, selected := b.selected[entry.Key]
	row.SetState(selected, b.hoverKey == entry.Key)
}

func (b *Browser) updateTile(id int, object fyne.CanvasObject) {
	tile := object.(*gridTile)
	tile.row = id
	tile.onClick = b.onItemClick
	tile.onMenu = b.onItemMenu
	tile.onHover = b.onItemHover
	if id < 0 || id >= len(b.visible) {
		tile.Set(model.Entry{}, "", nil)
		tile.SetState(false, false)
		return
	}
	entry := b.visible[id]
	tile.Set(entry, entry.Key, b.preview(entry))
	_, selected := b.selected[entry.Key]
	tile.SetState(selected, b.hoverKey == entry.Key)
}

// preview returns the entry's thumbnail if it is ready, and otherwise asks
// for it and refreshes the tile once it arrives.
func (b *Browser) preview(entry model.Entry) image.Image {
	if b.mw.cfg.Settings().NoThumbnails || !previewable(entry) {
		return nil
	}
	client, err := b.mw.clientFor(b.session.Account)
	if err != nil {
		return nil
	}
	session := b.session
	return b.mw.thumbs.Get(client, session, entry, func(image.Image) {
		if b.session == session {
			b.previewArrived(entry)
		}
	})
}

// previewArrived batches thumbnails that finish close together into one
// refresh, so a folder full of images does not redraw once per file.
func (b *Browser) previewArrived(entry model.Entry) {
	b.mw.details.previewReady(entry)
	if b.previewsDue {
		return
	}
	b.previewsDue = true
	generation := b.generation
	time.AfterFunc(120*time.Millisecond, func() {
		fyne.Do(func() {
			b.previewsDue = false
			if b.generation == generation && b.view == viewGrid {
				b.grid.RefreshList()
			}
		})
	})
}

// ---------------------------------------------------------------- loading

func (b *Browser) cancelLoad() {
	if b.loadCancel != nil {
		b.loadCancel()
		b.loadCancel = nil
	}
	if b.busy {
		b.busy = false
		b.mw.progress.End()
	}
}

func (b *Browser) reset() {
	b.cancelLoad()
	b.resetFolderStats()
	b.mw.thumbs.Reset()
	b.generation++
	b.all = nil
	b.visible = nil
	b.maxSize = 0
	b.nextToken = ""
	b.hasMore = false
	b.loaded = false
	b.loadErr = nil
	b.banner.Hide()
	b.ClearSelection()
}

func (b *Browser) SetSession(session model.Session) {
	b.reset()
	b.session = session
	b.filter = ""
	b.sort = model.SortState{Field: model.SortName}
	b.table.SetSort(model.SortName, false)
	if session.Bucket == "" {
		b.render()
		return
	}
	b.loadPage("")
}

func (b *Browser) Navigate(prefix string) {
	if b.session.Bucket == "" {
		return
	}
	b.reset()
	b.session.Prefix = prefix
	b.filter = ""
	b.loadPage("")
}

func (b *Browser) Reload() {
	if b.session.Bucket == "" {
		return
	}
	// Keep the selection by key across a reload, so refreshing after an
	// operation does not lose the user's place.
	keep := b.selected
	focus := b.focusKey
	b.reset()
	b.selected = keep
	b.focusKey = focus
	b.loadPage("")
}

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
	b.mw.progress.Begin()
	b.more.Disable()
	b.render()

	session := b.session
	generation := b.generation
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	b.loadCancel = cancel
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
			b.mw.progress.End()
			b.more.Enable()
			if err != nil {
				b.loadErr = err
				e := explainError(err)
				b.banner.Show(toneError, e.Title, e.Detail,
					newButton(buttonGhost, "Try again", t.IconRefresh, b.Reload),
				)
				b.mw.status.SetFailure(e)
				b.render()
				return
			}
			b.loadErr = nil
			b.banner.Hide()
			b.loaded = true
			entries := pageEntries(page, session.Prefix)
			b.all = append(b.all, entries...)
			b.nextToken = page.NextToken
			b.hasMore = page.Truncated
			b.applyView()
			if token == "" {
				b.scrollToFocus()
			}
			b.queueFolderStats(entries, session, generation)
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

// queueFolderStats counts each folder's objects, size and newest change in
// the background, three folders at a time.
func (b *Browser) queueFolderStats(entries []model.Entry, session model.Session, generation uint64) {
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
			var count int
			var modified time.Time
			if err == nil {
				var objects []r2.Object
				objects, err = client.ListAll(statCtx, session.Bucket, entry.Key)
				for _, object := range objects {
					if strings.HasSuffix(object.Key, "/") && object.Size == 0 {
						continue
					}
					count++
					size += object.Size
					if object.Modified.After(modified) {
						modified = object.Modified
					}
				}
			}
			fyne.Do(func() {
				if generation != b.generation || session != b.session {
					return
				}
				b.applyFolderStats(entry.Key, size, count, modified, err)
			})
		}()
	}
}

func (b *Browser) applyFolderStats(key string, size int64, count int, modified time.Time, err error) {
	update := func(entries []model.Entry) {
		for i := range entries {
			if entries[i].Key != key {
				continue
			}
			entries[i].StatsKnown = err == nil
			entries[i].StatsError = err != nil
			if err == nil {
				entries[i].Size = size
				entries[i].Count = count
				entries[i].Modified = modified
			}
			return
		}
	}
	update(b.all)
	update(b.visible)
	if size > b.maxSize {
		b.maxSize = size
		b.refreshAll()
	}
	if b.sort.Field == model.SortSize || b.sort.Field == model.SortModified {
		b.applyView()
	} else {
		b.refreshKey(key)
	}
	if _, ok := b.selected[key]; ok || len(b.selected) == 0 {
		b.mw.selectionChanged()
	}
	b.mw.updateTotals()
}

// ---------------------------------------------------------------- view

func (b *Browser) SetFilter(filter string) {
	filter = strings.TrimSpace(filter)
	if filter == b.filter {
		return
	}
	b.filter = filter
	b.ClearSelection()
	b.applyView()
	b.scrollToTop()
}

func (b *Browser) sortColumn(column int) {
	field := model.SortName
	switch column {
	case colSize:
		field = model.SortSize
	case colModified:
		field = model.SortModified
	}
	if b.sort.Field == field {
		b.sort.Desc = !b.sort.Desc
	} else {
		// Size and age read best biggest and newest first.
		b.sort = model.SortState{Field: field, Desc: field != model.SortName}
	}
	b.table.SetSort(b.sort.Field, b.sort.Desc)
	b.applyView()
}

// SetView switches between the table and the grid.
func (b *Browser) SetView(view string) {
	if view != viewGrid {
		view = viewList
	}
	b.view = view
	if view == viewGrid {
		b.table.Hide()
		b.grid.grid.Show()
	} else {
		b.grid.grid.Hide()
		b.table.Show()
	}
	b.refreshAll()
	b.views.Refresh()
	b.scrollToFocus()
}

func (b *Browser) applyView() {
	b.now = time.Now()
	b.visible = model.FilterAndSort(b.all, b.filter, b.sort)
	b.maxSize = 0
	for _, entry := range b.visible {
		if entry.Size > b.maxSize {
			b.maxSize = entry.Size
		}
	}
	// Drop selected keys that are no longer listed.
	present := make(map[string]struct{}, len(b.visible))
	for _, entry := range b.visible {
		present[entry.Key] = struct{}{}
	}
	for key := range b.selected {
		if _, ok := present[key]; !ok {
			delete(b.selected, key)
		}
	}
	b.refreshAll()
	b.render()
	b.mw.selectionChanged()
	b.mw.updateTotals()
}

func (b *Browser) refreshAll() {
	if b.view == viewGrid {
		b.grid.RefreshList()
	} else {
		b.table.RefreshList()
	}
}

func (b *Browser) refreshKey(key string) {
	if key == "" {
		return
	}
	for row, entry := range b.visible {
		if entry.Key != key {
			continue
		}
		if b.view == viewGrid {
			b.grid.RefreshItem(row)
		} else {
			b.table.RefreshRow(row)
		}
		return
	}
}

func (b *Browser) scrollTo(row int) {
	if b.view == viewGrid {
		b.grid.ScrollTo(row)
	} else {
		b.table.ScrollToRow(row)
	}
}

func (b *Browser) scrollToTop() {
	if b.view == viewGrid {
		b.grid.ScrollToTop()
	} else {
		b.table.ScrollToTop()
	}
}

func (b *Browser) scrollToFocus() {
	for i, entry := range b.visible {
		if entry.Key == b.focusKey {
			b.scrollTo(i)
			return
		}
	}
}

// render shows the right state for the body: the list, or a centred notice
// for an empty folder, a filter with no matches, a first load or a failure.
func (b *Browser) render() {
	if b.hasMore {
		b.moreText.SetText(fmt.Sprintf("Showing the first %s items. R2 lists a folder in pages.", formatCount(len(b.all))))
		b.moreBar.Show()
	} else {
		b.moreBar.Hide()
	}

	var notice fyne.CanvasObject
	switch {
	case b.session.Bucket == "":
		notice = noticeView(t.IconBucket, "Choose a bucket", "Pick a bucket in the sidebar, or press Ctrl+K to jump to one.")
	case !b.loaded && b.busy:
		notice = noticeView("", "Loading…", "")
	case !b.loaded && b.loadErr != nil:
		notice = noticeView(t.IconAlert, "This folder could not be listed", "See the message above.")
	case b.loaded && len(b.all) == 0:
		notice = noticeView(t.IconFolderOpen, "This folder is empty", "Drop files on the window, or use Upload.")
	case b.loaded && len(b.visible) == 0:
		clear := newButton(buttonGhost, "Clear filter", t.IconClose, func() { b.mw.toolbar.SetSearchText("") })
		notice = noticeView(t.IconSearch, "Nothing matches", fmt.Sprintf("No item in this folder has “%s” in its name.", b.filter), clear)
	}
	if notice == nil {
		b.notice.Objects = nil
		b.notice.Hide()
		b.views.Show()
	} else {
		b.notice.Objects = []fyne.CanvasObject{notice}
		b.notice.Show()
		// The table keeps its header for an empty folder, so the page does not
		// jump when items arrive.
		if b.session.Bucket == "" || b.view == viewGrid {
			b.views.Hide()
		} else {
			b.views.Show()
		}
	}
	b.notice.Refresh()
	if container, ok := b.Container.(*fyne.Container); ok {
		container.Refresh()
	}
}

// noticeView is a quiet centred message: an icon, a title, one sentence and
// optional actions.
func noticeView(icon, title, detail string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	if icon != "" {
		objects = append(objects, container.NewCenter(newIconSized(icon, t.ColorBorderStrong, 28)), fixed(1, 4, layoutSpacer()))
	}
	titleText := newText(title, t.TextMedium, t.ColorSubtle, styleStrong)
	titleText.Alignment = fyne.TextAlignCenter
	objects = append(objects, container.NewCenter(titleText))
	if detail != "" {
		text := newText(detail, t.TextSmall, t.ColorMuted, styleNormal)
		objects = append(objects, container.NewCenter(text))
	}
	if len(actions) > 0 {
		objects = append(objects, fixed(1, 6, layoutSpacer()), container.NewCenter(container.New(&rowLayout{gap: 6}, actions...)))
	}
	return container.NewCenter(vbox(4, objects...))
}

// ---------------------------------------------------------------- selection

func (b *Browser) onItemClick(row int, modifier fyne.KeyModifier) {
	if row < 0 || row >= len(b.visible) {
		return
	}
	b.mw.win.Canvas().Unfocus()
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

	now := time.Now()
	double := !control && !shift && b.lastClickKey == entry.Key && now.Sub(b.lastClickAt) <= 450*time.Millisecond
	b.lastClickKey = entry.Key
	b.lastClickAt = now
	if double {
		b.lastClickKey = ""
		b.mw.openEntry(entry)
	}
}

func (b *Browser) onItemMenu(row int, position fyne.Position) {
	if row < 0 || row >= len(b.visible) {
		return
	}
	entry := b.visible[row]
	if _, ok := b.selected[entry.Key]; !ok {
		b.selected = map[string]struct{}{entry.Key: {}}
		b.focusKey = entry.Key
		b.anchorKey = entry.Key
		b.selectionChanged()
	}
	b.mw.showContextMenu(b.SelectedEntries(), position)
}

func (b *Browser) onItemHover(row int, hovering bool) {
	if row < 0 || row >= len(b.visible) {
		return
	}
	old := b.hoverKey
	if hovering {
		b.hoverKey = b.visible[row].Key
	} else if b.hoverKey == b.visible[row].Key {
		b.hoverKey = ""
	}
	if old != b.hoverKey {
		b.refreshKey(old)
		b.refreshKey(b.hoverKey)
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
}

func (b *Browser) ClearSelection() {
	if len(b.selected) == 0 && b.focusKey == "" {
		return
	}
	b.selected = map[string]struct{}{}
	b.focusKey = ""
	b.anchorKey = ""
	b.selectionChanged()
}

// SelectKey selects one entry by key and scrolls to it.
func (b *Browser) SelectKey(key string) {
	for i, entry := range b.visible {
		if entry.Key == key {
			b.selected = map[string]struct{}{key: {}}
			b.focusKey, b.anchorKey = key, key
			b.selectionChanged()
			b.scrollTo(i)
			return
		}
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

// MoveFocus moves the selection by delta items; in the grid, up and down move
// by a row of tiles.
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
		index = max(0, min(len(b.visible)-1, index+delta))
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
	b.scrollTo(index)
}

// MoveVertical moves one row in the table or one row of tiles in the grid.
func (b *Browser) MoveVertical(direction int, extend bool) {
	step := 1
	if b.view == viewGrid {
		step = b.grid.Columns()
	}
	b.MoveFocus(direction*step, extend)
}

// MoveHorizontal moves one tile in the grid; in the table it does nothing.
func (b *Browser) MoveHorizontal(direction int, extend bool) bool {
	if b.view != viewGrid {
		return false
	}
	b.MoveFocus(direction, extend)
	return true
}

// MoveToEdge jumps to the first or last item.
func (b *Browser) MoveToEdge(last bool, extend bool) {
	if len(b.visible) == 0 {
		return
	}
	delta := -len(b.visible)
	if last {
		delta = len(b.visible)
	}
	b.MoveFocus(delta, extend)
}

// TypeAhead selects the next item whose name starts with prefix.
func (b *Browser) TypeAhead(prefix string) {
	prefix = strings.ToLower(prefix)
	start := 0
	for i, entry := range b.visible {
		if entry.Key == b.focusKey {
			start = i
			break
		}
	}
	for offset := 0; offset < len(b.visible); offset++ {
		i := (start + offset) % len(b.visible)
		if len(prefix) == 1 && offset == 0 {
			continue
		}
		if strings.HasPrefix(strings.ToLower(b.visible[i].Name), prefix) {
			b.SelectKey(b.visible[i].Key)
			return
		}
	}
}

func (b *Browser) OpenSelected() {
	if entry, ok := b.SelectedEntry(); ok {
		b.mw.openEntry(entry)
	}
}

func (b *Browser) selectionChanged() {
	b.refreshAll()
	b.mw.selectionChanged()
}

func (b *Browser) Session() model.Session { return b.session }
func (b *Browser) Filter() string         { return b.filter }

// Totals summarises the listed folder for the status bar.
func (b *Browser) Totals() string {
	if b.session.Bucket == "" || !b.loaded {
		return ""
	}
	var size int64
	for _, entry := range b.visible {
		size += entry.Size
	}
	text := countNoun(len(b.visible), "item")
	if b.filter != "" {
		text = fmt.Sprintf("%s of %s", formatCount(len(b.visible)), countNoun(len(b.all), "item"))
	}
	if b.hasMore {
		text += "+"
	}
	return joinDot(text, formatSize(size))
}
