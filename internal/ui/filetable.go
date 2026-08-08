package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	r2theme "r2go/internal/theme"
)

const (
	headerHeight      float32 = 30
	defaultRowHeight  float32 = 36
	sizeColumn        float32 = 112
	modifiedColumn    float32 = 176
	minimumNameColumn float32 = 120
	verticalGutter    float32 = 12
)

// FileTable is intentionally backed by widget.List rather than widget.Table.
// It still presents aligned columns, but each file is one full-width row. That
// removes Table's inter-cell gaps and horizontal scroller while allowing a
// continuous hover/selection surface from the left rail to the right edge.
type FileTable struct {
	widget.BaseWidget
	list    *widget.List
	header  *fyne.Container
	headers [3]*headerCell
	layout  *fileTableLayout
	widths  [3]float32
}

func NewFileTable(length func() int, create func() fyne.CanvasObject, update func(int, fyne.CanvasObject), onSort func(int)) *FileTable {
	view := &FileTable{}
	view.list = widget.NewList(
		length,
		create,
		func(id widget.ListItemID, object fyne.CanvasObject) { update(int(id), object) },
	)
	view.list.HideSeparators = true
	view.list.OnSelected = func(id widget.ListItemID) { view.list.Unselect(id) }

	labels := []string{"Name", "Size", "Modified"}
	alignments := []fyne.TextAlign{fyne.TextAlignLeading, fyne.TextAlignTrailing, fyne.TextAlignLeading}
	for i := range labels {
		column := i
		view.headers[i] = newHeaderCell(labels[i], alignments[i], func() { onSort(column) })
	}
	view.header = container.NewWithoutLayout(view.headers[0], view.headers[1], view.headers[2])
	view.layout = &fileTableLayout{owner: view}
	view.ExtendBaseWidget(view)
	return view
}

func (f *FileTable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.New(f.layout, f.header, f.list))
}

func (f *FileTable) MinSize() fyne.Size { return fyne.NewSize(500, 220) }

func (f *FileTable) SetSort(column int, desc bool) {
	for i, header := range f.headers {
		header.SetSort(i == column, desc)
	}
}

func (f *FileTable) Widths() [3]float32 { return f.widths }

func (f *FileTable) RefreshRows(_ int) { f.list.Refresh() }
func (f *FileTable) RefreshRow(row int) {
	if row >= 0 {
		f.list.RefreshItem(widget.ListItemID(row))
	}
}
func (f *FileTable) RefreshList() { f.list.Refresh() }
func (f *FileTable) ScrollToRow(row int) {
	if row >= 0 {
		f.list.ScrollTo(widget.ListItemID(row))
	}
}

func (f *FileTable) syncColumns(width float32) {
	available := fyne.Max(0, width-verticalGutter)
	name := available - sizeColumn - modifiedColumn
	if name < minimumNameColumn {
		name = minimumNameColumn
	}
	widths := [3]float32{name, sizeColumn, modifiedColumn}
	if widths == f.widths {
		return
	}
	f.widths = widths
	position := float32(0)
	for column, columnWidth := range widths {
		f.headers[column].Move(fyne.NewPos(position, 0))
		f.headers[column].Resize(fyne.NewSize(columnWidth, headerHeight))
		position += columnWidth
	}
	f.list.Refresh()
}

type fileTableLayout struct{ owner *FileTable }

func (l *fileTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	l.owner.syncColumns(size.Width)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, headerHeight))
	objects[1].Move(fyne.NewPos(0, headerHeight))
	objects[1].Resize(fyne.NewSize(size.Width, fyne.Max(0, size.Height-headerHeight)))
}
func (l *fileTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(500, 220) }

type headerCell struct {
	widget.BaseWidget
	name     string
	label    *widget.Label
	bg       *canvas.Rectangle
	onTapped func()
}

func newHeaderCell(name string, alignment fyne.TextAlign, onTapped func()) *headerCell {
	cell := &headerCell{
		name:     name,
		label:    widget.NewLabelWithStyle(name, alignment, fyne.TextStyle{Bold: true}),
		bg:       canvas.NewRectangle(fyneTheme.Color(r2theme.ColorTableHeader)),
		onTapped: onTapped,
	}
	cell.label.SizeName = fyneTheme.SizeNameCaptionText
	cell.ExtendBaseWidget(cell)
	return cell
}
func (h *headerCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(h.bg, inset(8, 0, 8, 0, h.label)))
}
func (h *headerCell) Refresh() {
	h.bg.FillColor = fyneTheme.Color(r2theme.ColorTableHeader)
	h.bg.Refresh()
	h.BaseWidget.Refresh()
}
func (h *headerCell) Tapped(*fyne.PointEvent) {
	if h.onTapped != nil {
		h.onTapped()
	}
}
func (h *headerCell) TappedSecondary(*fyne.PointEvent) {}
func (h *headerCell) SetSort(active, desc bool) {
	text := h.name
	if active {
		if desc {
			text += "  ↓"
		} else {
			text += "  ↑"
		}
	}
	h.label.SetText(text)
}

type fileRow struct {
	widget.BaseWidget
	background *canvas.Rectangle
	accent     *canvas.Rectangle
	icon       *widget.Icon
	name       *widget.Label
	size       *widget.Label
	modified   *widget.Label
	row        int
	widths     [3]float32
	selected   bool
	hovered    bool
	onClick    func(row int, modifier fyne.KeyModifier)
	onMenu     func(row int, position fyne.Position)
	onHover    func(row int, hovering bool)
}

func newFileRow() *fileRow {
	r := &fileRow{
		background: canvas.NewRectangle(color.Transparent),
		accent:     canvas.NewRectangle(color.Transparent),
		icon:       widget.NewIcon(nil),
		name:       widget.NewLabel(""),
		size:       widget.NewLabel(""),
		modified:   widget.NewLabel(""),
	}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.size.Alignment = fyne.TextAlignTrailing
	r.modified.Alignment = fyne.TextAlignLeading
	r.ExtendBaseWidget(r)
	return r
}

func (r *fileRow) MinSize() fyne.Size { return fyne.NewSize(420, defaultRowHeight) }
func (r *fileRow) CreateRenderer() fyne.WidgetRenderer {
	content := container.New(&fileRowLayout{owner: r}, r.background, r.accent, r.icon, r.name, r.size, r.modified)
	return widget.NewSimpleRenderer(content)
}
func (r *fileRow) SetWidths(widths [3]float32) { r.widths = widths }
func (r *fileRow) SetState(selected, hovered bool) {
	r.selected, r.hovered = selected, hovered
	switch {
	case selected:
		r.background.FillColor = rowSelectedColor()
		r.accent.FillColor = accentColor()
	case hovered:
		r.background.FillColor = rowHoverColor()
		r.accent.FillColor = color.Transparent
	default:
		r.background.FillColor = color.Transparent
		r.accent.FillColor = color.Transparent
	}
	r.background.Refresh()
	r.accent.Refresh()
}
func (r *fileRow) Refresh() { r.SetState(r.selected, r.hovered); r.BaseWidget.Refresh() }

func (r *fileRow) MouseDown(event *desktop.MouseEvent) {
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
func (r *fileRow) MouseIn(*desktop.MouseEvent) {
	if r.onHover != nil {
		r.onHover(r.row, true)
	}
}
func (r *fileRow) MouseMoved(*desktop.MouseEvent) {}
func (r *fileRow) MouseOut() {
	if r.onHover != nil {
		r.onHover(r.row, false)
	}
}

type fileRowLayout struct{ owner *fileRow }

func (l *fileRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 6 {
		return
	}
	nameW, sizeW, modW := l.owner.widths[0], l.owner.widths[1], l.owner.widths[2]
	if nameW <= 0 {
		nameW = fyne.Max(minimumNameColumn, size.Width-sizeColumn-modifiedColumn)
	}
	if sizeW <= 0 {
		sizeW = sizeColumn
	}
	if modW <= 0 {
		modW = modifiedColumn
	}
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
	objects[1].Move(fyne.NewPos(0, 0))
	objects[1].Resize(fyne.NewSize(2, size.Height))
	iconSize := float32(18)
	iconX := float32(10)
	objects[2].Move(fyne.NewPos(iconX, (size.Height-iconSize)/2))
	objects[2].Resize(fyne.NewSize(iconSize, iconSize))
	objects[3].Move(fyne.NewPos(36, 0))
	objects[3].Resize(fyne.NewSize(fyne.Max(0, nameW-44), size.Height))
	objects[4].Move(fyne.NewPos(nameW+6, 0))
	objects[4].Resize(fyne.NewSize(fyne.Max(0, sizeW-12), size.Height))
	objects[5].Move(fyne.NewPos(nameW+sizeW+8, 0))
	objects[5].Resize(fyne.NewSize(fyne.Max(0, modW-16), size.Height))
}
func (l *fileRowLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(420, defaultRowHeight)
}
