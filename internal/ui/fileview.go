package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/s3client"
)

type Session struct {
	AccountName string
	Bucket      string
	Prefix      string
}

type FileView struct {
	mw        *MainWindow
	Container *fyne.Container

	header  *fileHeader
	table   *widget.Table
	loadBtn *widget.Button

	placeholder *fyne.Container
	loading     *fyne.Container
	empty       *fyne.Container
	listView    *fyne.Container

	all       []Entry
	visible   []Entry
	nextToken string
	hasMore   bool
	busy      bool

	session  Session
	sort     sortState
	filter   string
	multi    bool
	selected map[int]bool

	lastRow   int
	lastClick time.Time
	selRow    int
}

func NewFileView(mw *MainWindow) *FileView {
	f := &FileView{mw: mw, selected: map[int]bool{}}

	f.header = newFileHeader(func(field int) {
		if f.sort.field == field {
			f.sort.desc = !f.sort.desc
		} else {
			f.sort = sortState{field: field}
		}
		f.header.Set(f.sort)
		f.applyView()
	})

	f.table = widget.NewTable(
		func() (int, int) { return len(f.visible), 3 },
		func() fyne.CanvasObject { return newTableCell() },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			f.updateCell(id, obj)
		},
	)
	f.table.SetColumnWidth(0, 420)
	f.table.SetColumnWidth(1, 110)
	f.table.SetColumnWidth(2, 180)
	f.table.OnSelected = func(id widget.TableCellID) {
		f.onRowSelected(id.Row)
	}

	f.loadBtn = widget.NewButton("Load more…", func() {
		f.loadMore()
	})
	f.loadBtn.Hide()

	f.listView = container.NewBorder(nil, f.loadBtn, nil, nil,
		container.NewBorder(f.header.row, nil, nil, nil, f.table))

	title := widget.NewLabelWithStyle("This folder is empty", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	f.empty = container.NewCenter(container.NewVBox(title,
		widget.NewLabelWithStyle("Upload files with the Upload button.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})))

	spinner := widget.NewProgressBarInfinite()
	spinner.Start()
	loadingLabel := widget.NewLabel("Loading…")
	loadingLabel.Alignment = fyne.TextAlignCenter
	f.loading = container.NewCenter(container.NewVBox(spinner, loadingLabel))

	icon := widget.NewIcon(fyneTheme.Icon(fyneTheme.IconNameFolder))
	icon.Resize(fyne.NewSize(96, 96))
	phTitle := widget.NewLabel("No bucket selected")
	phTitle.Alignment = fyne.TextAlignCenter
	phTitle.TextStyle = fyne.TextStyle{Bold: true}
	phHint := widget.NewLabel("Pick an account, then a bucket from the sidebar to start browsing.")
	phHint.Alignment = fyne.TextAlignCenter
	phHint.TextStyle = fyne.TextStyle{Italic: true}
	phHint.Wrapping = fyne.TextWrapWord
	f.placeholder = container.NewCenter(container.NewVBox(icon, phTitle, phHint))

	f.Container = container.NewStack(f.listView, f.loading, f.empty, f.placeholder)
	f.showPlaceholder()
	return f
}

func (f *FileView) updateCell(id widget.TableCellID, obj fyne.CanvasObject) {
	cell := obj.(*tableCell)
	cell.row = id.Row
	cell.onMenu = func(row int, pos fyne.Position) { f.onCellMenu(row, pos) }

	icon := cell.icon
	label := cell.label

	if id.Row < 0 || id.Row >= len(f.visible) {
		label.SetText("")
		label.TextStyle = fyne.TextStyle{}
		return
	}
	e := f.visible[id.Row]
	switch id.Col {
	case 0:
		icon.SetResource(entryIcon(e))
		label.SetText(e.Name)
	case 1:
		if e.IsDir {
			label.SetText("")
		} else {
			label.SetText(formatSize(e.Size))
		}
	case 2:
		if e.IsDir {
			label.SetText("")
		} else if !e.Mod.IsZero() {
			label.SetText(e.Mod.Format("2006-01-02 15:04"))
		} else {
			label.SetText("")
		}
	}
	if f.multi && f.selected[id.Row] {
		label.TextStyle = fyne.TextStyle{Bold: true}
	} else {
		label.TextStyle = fyne.TextStyle{}
	}
}

func (f *FileView) onCellMenu(row int, pos fyne.Position) {
	if f.busy || row < 0 || row >= len(f.visible) {
		return
	}
	if f.multi {
		if !f.selected[row] {
			f.toggleSelect(row)
		}
		f.mw.showBatchMenu(f.selectedEntries(), pos)
		return
	}
	f.mw.showItemMenu(f.visible[row], pos)
}

func (f *FileView) selectedEntries() []Entry {
	out := make([]Entry, 0, len(f.selected))
	for row := range f.selected {
		if row >= 0 && row < len(f.visible) {
			out = append(out, f.visible[row])
		}
	}
	return out
}

func (f *FileView) toggleSelect(row int) {
	if f.selected[row] {
		delete(f.selected, row)
	} else {
		f.selected[row] = true
	}
	f.table.Refresh()
	f.mw.statusBar.SetRight(fmt.Sprintf("%d selected", len(f.selected)))
}

func (f *FileView) SelectedEntry() (Entry, bool) {
	if f.busy {
		return Entry{}, false
	}
	if f.multi {
		es := f.selectedEntries()
		if len(es) > 0 {
			return es[0], true
		}
		return Entry{}, false
	}
	id := widget.TableCellID{Row: f.selRow}
	if id.Row < 0 || id.Row >= len(f.visible) {
		return Entry{}, false
	}
	return f.visible[id.Row], true
}

func (f *FileView) SelectedEntries() []Entry {
	if f.multi {
		return f.selectedEntries()
	}
	if e, ok := f.SelectedEntry(); ok {
		return []Entry{e}
	}
	return nil
}

func (f *FileView) SelectAll() {
	f.multi = true
	f.selected = map[int]bool{}
	for i := range f.visible {
		f.selected[i] = true
	}
	f.table.Refresh()
	f.mw.toolbar.Refresh()
	f.mw.statusBar.SetRight(fmt.Sprintf("%d selected", len(f.selected)))
}

func entryIcon(e Entry) fyne.Resource {
	if e.IsDir {
		return fyneTheme.Icon(fyneTheme.IconNameFolder)
	}
	switch {
	case strings.HasSuffix(e.Name, ".png"), strings.HasSuffix(e.Name, ".jpg"),
		strings.HasSuffix(e.Name, ".jpeg"), strings.HasSuffix(e.Name, ".gif"),
		strings.HasSuffix(e.Name, ".webp"), strings.HasSuffix(e.Name, ".svg"),
		strings.HasSuffix(e.Name, ".bmp"), strings.HasSuffix(e.Name, ".ico"):
		return fyneTheme.Icon(fyneTheme.IconNameFileImage)
	case strings.HasSuffix(e.Name, ".mp4"), strings.HasSuffix(e.Name, ".webm"),
		strings.HasSuffix(e.Name, ".mov"), strings.HasSuffix(e.Name, ".avi"),
		strings.HasSuffix(e.Name, ".mkv"):
		return fyneTheme.Icon(fyneTheme.IconNameFileVideo)
	case strings.HasSuffix(e.Name, ".mp3"), strings.HasSuffix(e.Name, ".wav"),
		strings.HasSuffix(e.Name, ".flac"), strings.HasSuffix(e.Name, ".ogg"):
		return fyneTheme.Icon(fyneTheme.IconNameFileAudio)
	case strings.HasSuffix(e.Name, ".txt"), strings.HasSuffix(e.Name, ".md"),
		strings.HasSuffix(e.Name, ".log"), strings.HasSuffix(e.Name, ".json"),
		strings.HasSuffix(e.Name, ".csv"), strings.HasSuffix(e.Name, ".html"),
		strings.HasSuffix(e.Name, ".go"), strings.HasSuffix(e.Name, ".py"),
		strings.HasSuffix(e.Name, ".js"), strings.HasSuffix(e.Name, ".ts"),
		strings.HasSuffix(e.Name, ".yml"), strings.HasSuffix(e.Name, ".yaml"),
		strings.HasSuffix(e.Name, ".xml"), strings.HasSuffix(e.Name, ".sh"),
		strings.HasSuffix(e.Name, ".ps1"), strings.HasSuffix(e.Name, ".toml"),
		strings.HasSuffix(e.Name, ".sql"), strings.HasSuffix(e.Name, ".css"):
		return fyneTheme.Icon(fyneTheme.IconNameFileText)
	case strings.HasSuffix(e.Name, ".zip"), strings.HasSuffix(e.Name, ".tar"),
		strings.HasSuffix(e.Name, ".gz"), strings.HasSuffix(e.Name, ".bz2"),
		strings.HasSuffix(e.Name, ".xz"), strings.HasSuffix(e.Name, ".7z"),
		strings.HasSuffix(e.Name, ".rar"), strings.HasSuffix(e.Name, ".tgz"):
		return fyneTheme.Icon(fyneTheme.IconNameFileApplication)
	case strings.HasSuffix(e.Name, ".exe"), strings.HasSuffix(e.Name, ".msi"),
		strings.HasSuffix(e.Name, ".dmg"), strings.HasSuffix(e.Name, ".apk"),
		strings.HasSuffix(e.Name, ".deb"), strings.HasSuffix(e.Name, ".rpm"),
		strings.HasSuffix(e.Name, ".iso"):
		return fyneTheme.Icon(fyneTheme.IconNameFileApplication)
	case strings.HasSuffix(e.Name, ".doc"), strings.HasSuffix(e.Name, ".docx"),
		strings.HasSuffix(e.Name, ".xls"), strings.HasSuffix(e.Name, ".xlsx"),
		strings.HasSuffix(e.Name, ".ppt"), strings.HasSuffix(e.Name, ".pptx"):
		return fyneTheme.Icon(fyneTheme.IconNameDocument)
	case strings.HasSuffix(e.Name, ".pdf"):
		return fyneTheme.Icon(fyneTheme.IconNameDocument)
	}
	return fyneTheme.Icon(fyneTheme.IconNameFile)
}

func (f *FileView) onRowSelected(row int) {
	f.selRow = row
	if f.busy || row < 0 || row >= len(f.visible) {
		return
	}
	if f.multi {
		f.toggleSelect(row)
		return
	}
	now := time.Now()
	double := row == f.lastRow && now.Sub(f.lastClick) < 500*time.Millisecond
	f.lastRow, f.lastClick = row, now
	if !double {
		return
	}
	e := f.visible[row]
	if e.IsDir {
		f.mw.onNavigatePrefix(e.Key)
	} else {
		f.mw.onOpenEntry(e)
	}
}

func (f *FileView) SetMulti(multi bool) {
	f.multi = multi
	f.ClearSelection()
	if multi {
		f.mw.notifyStatus("Selection mode: click rows to select, right-click for batch actions")
	}
}

func (f *FileView) ClearSelection() {
	f.selected = map[int]bool{}
	f.table.UnselectAll()
	f.table.Refresh()
	f.mw.statusBar.SetRight("")
}

func (f *FileView) SetSession(s Session) {
	if s == f.session {
		return
	}
	f.session = s
	f.all = nil
	f.nextToken = ""
	f.hasMore = false
	f.filter = ""
	f.sort = sortState{field: SortName}
	f.header.Set(f.sort)
	f.mw.toolbar.SetSearchText("")
	f.mw.toolbar.SetPath(s.Bucket, s.Prefix)
	f.reload()
}

func (f *FileView) NavigateTo(prefix string) {
	f.session.Prefix = prefix
	f.all = nil
	f.nextToken = ""
	f.hasMore = false
	f.mw.toolbar.SetPath(f.session.Bucket, prefix)
	f.reload()
}

func (f *FileView) Reload() {
	if f.session.Bucket == "" {
		return
	}
	f.all = nil
	f.nextToken = ""
	f.hasMore = false
	f.reload()
}

func (f *FileView) SetFilter(text string) {
	f.filter = strings.TrimSpace(text)
	f.applyView()
}

func (f *FileView) Clear() {
	f.session = Session{}
	f.all = nil
	f.visible = nil
	f.nextToken = ""
	f.hasMore = false
	f.showPlaceholder()
	f.table.Refresh()
	f.loadBtn.Hide()
}

func (f *FileView) reload() {
	if f.session.Bucket == "" {
		return
	}
	f.loadPage("")
}

func (f *FileView) loadMore() {
	if f.nextToken == "" {
		return
	}
	f.loadPage(f.nextToken)
}

func (f *FileView) loadPage(token string) {
	f.busy = true
	f.showLoading()
	f.mw.statusBar.SetMessage("Loading " + f.session.Bucket + "/" + f.session.Prefix + "…")

	sess := f.session
	page, err := f.mw.loadPage(&sess, token)
	fyne.Do(func() {
		dbgLog("loadPage completion: err=%v", err)
		f.busy = false
		if err != nil {
			f.showList()
			f.mw.notifyStatus("Load failed: " + err.Error())
			return
		}
		f.all = append(f.all, pageToEntries(page, f.session.Prefix)...)
		f.nextToken = page.NextToken
		f.hasMore = page.Truncated
		f.applyView()
		dbgDumpFile("applyView", f)
		f.mw.notifyStatus(fmtStatus(len(f.all)))
	})
}

func pageToEntries(page s3client.ListPage, prefix string) []Entry {
	entries := make([]Entry, 0, len(page.Prefixes)+len(page.Objects))
	for _, p := range page.Prefixes {
		entries = append(entries, Entry{
			Key:   p.Prefix,
			Name:  strings.TrimSuffix(strings.TrimPrefix(p.Prefix, prefix), "/"),
			IsDir: true,
		})
	}
	for _, o := range page.Objects {
		entries = append(entries, Entry{
			Key:   o.Key,
			Name:  strings.TrimPrefix(o.Key, prefix),
			IsDir: false,
			Size:  o.Size,
			Mod:   o.ModTime,
			ETag:  o.ETag,
		})
	}
	return entries
}

func (f *FileView) applyView() {
	f.visible = make([]Entry, 0, len(f.all))
	for _, e := range f.all {
		if matchesFilter(e, f.filter) {
			f.visible = append(f.visible, e)
		}
	}
	sortEntries(f.visible, f.sort)
	f.showList()
	f.table.Refresh()
	f.loadBtn.Show()
	if !f.hasMore {
		f.loadBtn.Hide()
	}
	f.mw.statusBar.SetRight(fmtStatus(len(f.all)))
}

func (f *FileView) showPlaceholder() {
	f.listView.Hide()
	f.loading.Hide()
	f.empty.Hide()
	f.placeholder.Show()
}

func (f *FileView) showLoading() {
	f.placeholder.Hide()
	f.empty.Hide()
	f.loading.Show()
}

func (f *FileView) showList() {
	f.placeholder.Hide()
	f.loading.Hide()
	if f.session.Bucket == "" {
		f.showPlaceholder()
		return
	}
	if len(f.visible) == 0 {
		f.empty.Show()
	} else {
		f.empty.Hide()
	}
	f.listView.Show()
	f.listView.Refresh()
	dbgDumpFile("showList", f)
}

func fmtStatus(n int) string {
	if n == 0 {
		return ""
	}
	if n == 1 {
		return "1 item"
	}
	return strconv.Itoa(n) + " items"
}

func formatSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)
	switch {
	case n >= tb:
		return fmtF(n, tb, "TB")
	case n >= gb:
		return fmtF(n, gb, "GB")
	case n >= mb:
		return fmtF(n, mb, "MB")
	case n >= kb:
		return fmtF(n, kb, "KB")
	default:
		return strconv.FormatInt(n, 10) + " B"
	}
}

func fmtF(n int64, unit int64, suffix string) string {
	v := float64(n) / float64(unit)
	return fmt.Sprintf("%.1f %s", v, suffix)
}
