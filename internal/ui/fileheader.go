package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type fileHeader struct {
	row    *fyne.Container
	cells  [3]*headerCell
	onSort func(int)
}

func newFileHeader(onSort func(int)) *fileHeader {
	h := &fileHeader{onSort: onSort}
	names := []string{"Name", "Size", "Modified"}
	for i, n := range names {
		cell := &headerCell{label: widget.NewLabelWithStyle(n, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})}
		idx := i
		cell.onTap = func() { h.onSort(idx) }
		h.cells[i] = cell
	}
	h.row = container.NewHBox(
		h.cells[0],
		container.NewPadded(container.NewHBox(h.cells[1], h.cells[2])),
	)
	return h
}

func (h *fileHeader) Set(s sortState) {
	markers := [3]string{"", "", ""}
	switch {
	case s.field == SortName:
		if s.desc {
			markers[0] = " ▼"
		} else {
			markers[0] = " ▲"
		}
	case s.field == SortSize:
		if s.desc {
			markers[1] = " ▼"
		} else {
			markers[1] = " ▲"
		}
	case s.field == SortMod:
		if s.desc {
			markers[2] = " ▼"
		} else {
			markers[2] = " ▲"
		}
	}
	names := []string{"Name", "Size", "Modified"}
	for i := range h.cells {
		h.cells[i].label.SetText(names[i] + markers[i])
	}
}

type headerCell struct {
	widget.BaseWidget
	label *widget.Label
	onTap func()
}

func (c *headerCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.label)
}

func (c *headerCell) Tapped(_ *fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *headerCell) TappedSecondary(_ *fyne.PointEvent) {}
