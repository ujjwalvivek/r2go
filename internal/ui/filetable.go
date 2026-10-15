package ui

import (
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/model"
	t "r2go/internal/theme"
)

// Column widths. Name takes whatever is left.
const (
	typeColumn      float32 = 64
	sizeColumn      float32 = 124
	modifiedColumn  float32 = 104
	minNameColumn   float32 = 140
	cellPadX        float32 = 10
	sizeBarWidth    float32 = 36
	scrollbarGutter float32 = 8
)

const (
	colName = iota
	colType
	colSize
	colModified
)

// FileTable is a header over a virtualised list of full-width rows. Rows are
// painted by the table itself: 26 px, 1 px BORDER lines, a SURFACE hover, a
// SELECTED fill with a 2 px accent bar, and no zebra stripes.
type FileTable struct {
	widget.BaseWidget
	list    *widget.List
	header  *fyne.Container
	headers [4]*headerCell
	rule    fyne.CanvasObject
	widths  [4]float32
}

func NewFileTable(length func() int, create func() fyne.CanvasObject, update func(int, fyne.CanvasObject), onSort func(int)) *FileTable {
	view := &FileTable{}
	view.list = widget.NewList(length, create, func(id widget.ListItemID, object fyne.CanvasObject) { update(int(id), object) })
	view.list.HideSeparators = true
	view.list.OnSelected = func(id widget.ListItemID) { view.list.Unselect(id) }

	view.headers = [4]*headerCell{
		newHeaderCell("Name", fyne.TextAlignLeading, "Name\nFolders first, then files A–Z", func() { onSort(colName) }),
		newHeaderCell("Type", fyne.TextAlignLeading, "Type\nThe file extension", nil),
		newHeaderCell("Size", fyne.TextAlignTrailing, "Size\nFolder sizes are counted in the background", func() { onSort(colSize) }),
		newHeaderCell("Modified", fyne.TextAlignLeading, "Modified\nWhen the object was last written; a folder shows its newest object", func() { onSort(colModified) }),
	}
	view.header = container.NewWithoutLayout(view.headers[0], view.headers[1], view.headers[2], view.headers[3])
	view.rule = hRule(t.ColorBorderStrong)
	view.ExtendBaseWidget(view)
	return view
}

func (f *FileTable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.New(&fileTableLayout{owner: f}, f.header, f.rule, f.list))
}

func (f *FileTable) MinSize() fyne.Size { return fyne.NewSize(420, 120) }

func (f *FileTable) SetSort(field model.SortField, desc bool) {
	column := colName
	switch field {
	case model.SortSize:
		column = colSize
	case model.SortModified:
		column = colModified
	}
	for i, header := range f.headers {
		header.SetSort(i == column, desc)
	}
}

func (f *FileTable) Widths() [4]float32 { return f.widths }
func (f *FileTable) RefreshList()       { f.list.Refresh() }
func (f *FileTable) RefreshRow(row int) {
	if row >= 0 {
		f.list.RefreshItem(widget.ListItemID(row))
	}
}
func (f *FileTable) ScrollToRow(row int) {
	if row >= 0 {
		f.list.ScrollTo(widget.ListItemID(row))
	}
}
func (f *FileTable) ScrollToTop() { f.list.ScrollToTop() }

func (f *FileTable) syncColumns(width float32) {
	available := fyne.Max(0, width-scrollbarGutter)
	name := fyne.Max(minNameColumn, available-typeColumn-sizeColumn-modifiedColumn)
	widths := [4]float32{name, typeColumn, sizeColumn, modifiedColumn}
	if widths == f.widths {
		return
	}
	f.widths = widths
	x := float32(0)
	for i, w := range widths {
		f.headers[i].Move(fyne.NewPos(x, 0))
		f.headers[i].Resize(fyne.NewSize(w, t.HeightHeader))
		x += w
	}
	f.list.Refresh()
}

type fileTableLayout struct{ owner *FileTable }

func (l *fileTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.owner.syncColumns(size.Width)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, t.HeightHeader))
	objects[1].Move(fyne.NewPos(0, t.HeightHeader))
	objects[1].Resize(fyne.NewSize(size.Width, 1))
	objects[2].Move(fyne.NewPos(0, t.HeightHeader+1))
	objects[2].Resize(fyne.NewSize(size.Width, fyne.Max(0, size.Height-t.HeightHeader-1)))
}

func (l *fileTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(420, 120) }

// headerCell is a column caption in MUTED. The sorted column turns TEXT and
// gets a small accent arrow.
type headerCell struct {
	widget.BaseWidget
	label    *textView
	arrow    *canvas.Image
	align    fyne.TextAlign
	tip      string
	sorted   bool
	desc     bool
	hovered  bool
	onTapped func()
}

func newHeaderCell(name string, align fyne.TextAlign, tip string, onTapped func()) *headerCell {
	h := &headerCell{
		label:    newTextView(name, t.TextSmall, t.ColorMuted, styleNormal),
		arrow:    newIconSized(t.IconChevronU, t.ColorAccent, 10),
		align:    align,
		tip:      tip,
		onTapped: onTapped,
	}
	h.arrow.Hide()
	h.ExtendBaseWidget(h)
	return h
}

func (h *headerCell) apply() {
	switch {
	case h.sorted:
		h.label.SetColorName(t.ColorText)
	case h.hovered && h.onTapped != nil:
		h.label.SetColorName(t.ColorSubtle)
	default:
		h.label.SetColorName(t.ColorMuted)
	}
	if h.sorted {
		icon := t.IconChevronU
		if h.desc {
			icon = t.IconChevronD
		}
		setIcon(h.arrow, icon, t.ColorAccent)
		h.arrow.Show()
	} else {
		h.arrow.Hide()
	}
	h.Refresh()
}

func (h *headerCell) SetSort(active, desc bool) {
	h.sorted, h.desc = active, desc
	h.apply()
}

func (h *headerCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.New(&headerCellLayout{h: h}, h.label, h.arrow))
}

func (h *headerCell) Tapped(*fyne.PointEvent) {
	if h.onTapped != nil {
		tips.Hide(h)
		h.onTapped()
	}
}
func (h *headerCell) MouseIn(event *desktop.MouseEvent) {
	h.hovered = true
	h.apply()
	tips.Show(h, h.tip, event.AbsolutePosition)
}
func (h *headerCell) MouseMoved(event *desktop.MouseEvent) { tips.Move(h, event.AbsolutePosition) }
func (h *headerCell) MouseOut() {
	h.hovered = false
	h.apply()
	tips.Hide(h)
}

type headerCellLayout struct{ h *headerCell }

func (l *headerCellLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	label, arrow := objects[0], objects[1]
	textW := fyne.MeasureText(l.h.label.Text(), t.TextSmall, styleNormal).Width
	const arrowW, gap float32 = 10, 4
	x := cellPadX
	if l.h.align == fyne.TextAlignTrailing {
		x = size.Width - cellPadX - textW
		if l.h.sorted {
			x -= arrowW + gap
		}
	}
	label.Move(fyne.NewPos(x, 0))
	label.Resize(fyne.NewSize(textW+1, size.Height))
	arrow.Move(fyne.NewPos(x+textW+gap, (size.Height-arrowW)/2))
	arrow.Resize(fyne.NewSize(arrowW, arrowW))
}

func (l *headerCellLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(40, t.HeightHeader)
}

// fileRow is one object in the list.
type fileRow struct {
	widget.BaseWidget
	bg       *canvas.Rectangle
	bar      *canvas.Rectangle
	rule     *canvas.Rectangle
	icon     *canvas.Image
	name     *textView
	kind     *pill
	sizeBar  *meter
	size     *textView
	modified *textView

	row      int
	entry    model.Entry
	widths   [4]float32
	selected bool
	hovered  bool
	tipCol   int

	onClick func(row int, modifier fyne.KeyModifier)
	onMenu  func(row int, position fyne.Position)
	onHover func(row int, hovering bool)
}

func newFileRow() *fileRow {
	r := &fileRow{
		bg:       canvas.NewRectangle(color.Transparent),
		bar:      canvas.NewRectangle(color.Transparent),
		rule:     newRect(t.ColorBorder),
		icon:     newIcon(t.IconFile, t.ColorMuted),
		name:     newElided("", t.TextBody, t.ColorText, styleNormal),
		kind:     newPill(""),
		sizeBar:  newMeter(3, t.ColorBorderStrong),
		size:     newTextView("", t.TextSmall, t.ColorSubtle, styleMono),
		modified: newTextView("", t.TextSmall, t.ColorSubtle, styleNormal),
		tipCol:   -1,
	}
	r.size.SetAlignment(fyne.TextAlignTrailing)
	r.ExtendBaseWidget(r)
	return r
}

// Set fills the row for an entry. maxSize scales the inline size bar.
func (r *fileRow) Set(entry model.Entry, widths [4]float32, maxSize int64, now time.Time) {
	r.entry = entry
	r.widths = widths
	setResource(r.icon, entryIcon(entry))
	r.name.SetText(entry.Name)
	if entry.IsDir {
		r.name.SetStyle(styleStrong)
	} else {
		r.name.SetStyle(styleNormal)
	}
	r.kind.SetText(typeLabel(entry))

	switch {
	case entry.IsDir && entry.StatsError:
		r.size.SetText("—")
		r.size.SetColorName(t.ColorMuted)
		r.sizeBar.Hide()
	case entry.IsDir && !entry.StatsKnown:
		r.size.SetText("…")
		r.size.SetColorName(t.ColorMuted)
		r.sizeBar.Hide()
	default:
		r.size.SetText(formatSize(entry.Size))
		r.size.SetColorName(t.ColorSubtle)
		r.sizeBar.SetValue(barScale(entry.Size, maxSize))
		r.sizeBar.Show()
	}

	if entry.Modified.IsZero() {
		r.modified.SetText("")
	} else {
		r.modified.SetText(formatAge(entry.Modified, now))
		r.modified.SetColorName(ageColor(entry.Modified, now))
	}
	r.Refresh()
}

// barScale is square-root scaled, so small files still show a mark.
func barScale(size, max int64) float64 {
	if max <= 0 || size <= 0 {
		return 0
	}
	return math.Sqrt(float64(size)) / math.Sqrt(float64(max))
}

func (r *fileRow) SetState(selected, hovered bool) {
	r.selected, r.hovered = selected, hovered
	switch {
	case selected:
		r.bg.FillColor, r.bar.FillColor = tc(t.ColorSelected), tc(t.ColorAccent)
	case hovered:
		r.bg.FillColor, r.bar.FillColor = tc(t.ColorSurface), color.Transparent
	default:
		r.bg.FillColor, r.bar.FillColor = color.Transparent, color.Transparent
	}
	r.bg.Refresh()
	r.bar.Refresh()
}

func (r *fileRow) MinSize() fyne.Size { return fyne.NewSize(420, t.HeightRow) }

func (r *fileRow) CreateRenderer() fyne.WidgetRenderer {
	objects := []fyne.CanvasObject{r.bg, r.bar, r.rule, r.icon, r.name, r.kind, r.sizeBar, r.size, r.modified}
	return widget.NewSimpleRenderer(container.New(&fileRowLayout{r: r}, objects...))
}

func (r *fileRow) MouseDown(event *desktop.MouseEvent) {
	tips.Hide(r)
	switch event.Button {
	case desktop.MouseButtonPrimary:
		if r.onClick != nil {
			r.onClick(r.row, event.Modifier)
		}
	case desktop.MouseButtonSecondary:
		if r.onMenu != nil {
			r.onMenu(r.row, event.AbsolutePosition)
		}
	}
}
func (r *fileRow) MouseUp(*desktop.MouseEvent) {}

// Tapped and TappedSecondary are handled in MouseDown. Claiming them here
// keeps the list from taking keyboard focus, so the app's own keys keep
// working after a click.
func (r *fileRow) Tapped(*fyne.PointEvent)          {}
func (r *fileRow) TappedSecondary(*fyne.PointEvent) {}

func (r *fileRow) MouseIn(event *desktop.MouseEvent) {
	if r.onHover != nil {
		r.onHover(r.row, true)
	}
	r.updateTip(event)
}
func (r *fileRow) MouseMoved(event *desktop.MouseEvent) { r.updateTip(event) }
func (r *fileRow) MouseOut() {
	if r.onHover != nil {
		r.onHover(r.row, false)
	}
	r.tipCol = -1
	tips.Hide(r)
}

// updateTip shows a cell's tooltip: the full name when it is cut short, the
// exact time for the age, and the object count for a folder's size.
func (r *fileRow) updateTip(event *desktop.MouseEvent) {
	column := colName
	x := event.Position.X
	switch {
	case x >= r.widths[0]+r.widths[1]+r.widths[2]:
		column = colModified
	case x >= r.widths[0]+r.widths[1]:
		column = colSize
	case x >= r.widths[0]:
		column = colType
	}
	if column == r.tipCol {
		tips.Move(r, event.AbsolutePosition)
		return
	}
	r.tipCol = column
	text := ""
	switch column {
	case colName:
		if r.name.Elided() {
			text = r.entry.Name
		}
	case colSize:
		if r.entry.IsDir && r.entry.StatsKnown {
			text = formatSize(r.entry.Size) + "\n" + countNoun(r.entry.Count, "object") + " inside"
		} else if !r.entry.IsDir {
			text = formatSize(r.entry.Size) + "\n" + formatBytes(r.entry.Size) + " bytes"
		}
	case colModified:
		if !r.entry.Modified.IsZero() {
			text = formatTimestamp(r.entry.Modified)
			if r.entry.IsDir {
				text += "\nNewest object in this folder"
			}
		}
	}
	if text == "" {
		tips.Hide(r)
		return
	}
	tips.Show(r, text, event.AbsolutePosition)
}

type fileRowLayout struct{ r *fileRow }

func (l *fileRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	r := l.r
	w := r.widths
	if w[0] <= 0 {
		w = [4]float32{fyne.Max(minNameColumn, size.Width-typeColumn-sizeColumn-modifiedColumn), typeColumn, sizeColumn, modifiedColumn}
	}
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(size)
	r.bar.Move(fyne.NewPos(0, 0))
	r.bar.Resize(fyne.NewSize(t.WidthAccentBar, size.Height))
	r.rule.Move(fyne.NewPos(0, size.Height-1))
	r.rule.Resize(fyne.NewSize(size.Width, 1))

	mid := func(h float32) float32 { return (size.Height - h) / 2 }
	r.icon.Move(fyne.NewPos(cellPadX, mid(t.SizeIcon)))
	r.icon.Resize(fyne.NewSize(t.SizeIcon, t.SizeIcon))
	nameX := cellPadX + t.SizeIcon + 8
	r.name.Move(fyne.NewPos(nameX, 0))
	r.name.Resize(fyne.NewSize(fyne.Max(0, w[0]-nameX-cellPadX), size.Height))

	x := w[0]
	r.kind.Move(fyne.NewPos(x+cellPadX, 0))
	r.kind.Resize(fyne.NewSize(fyne.Max(0, w[1]-2*cellPadX), size.Height))
	x += w[1]

	r.sizeBar.Move(fyne.NewPos(x+cellPadX, mid(3)))
	r.sizeBar.Resize(fyne.NewSize(sizeBarWidth, 3))
	sizeX := x + cellPadX + sizeBarWidth + 6
	r.size.Move(fyne.NewPos(sizeX, 0))
	r.size.Resize(fyne.NewSize(fyne.Max(0, x+w[2]-cellPadX-sizeX), size.Height))
	x += w[2]

	r.modified.Move(fyne.NewPos(x+cellPadX, 0))
	r.modified.Resize(fyne.NewSize(fyne.Max(0, w[3]-2*cellPadX), size.Height))
}

func (l *fileRowLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(420, t.HeightRow) }
