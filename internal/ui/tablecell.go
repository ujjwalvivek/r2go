package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type tableCell struct {
	widget.BaseWidget
	icon   *widget.Icon
	label  *widget.Label
	row    int
	onMenu func(row int, pos fyne.Position)
}

func newTableCell() *tableCell {
	c := &tableCell{
		icon:  widget.NewIcon(nil),
		label: widget.NewLabel(""),
	}
	c.label.Truncation = fyne.TextTruncateEllipsis
	c.ExtendBaseWidget(c)
	return c
}

func (c *tableCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, c.icon, nil, c.label))
}

func (c *tableCell) TappedSecondary(e *fyne.PointEvent) {
	if c.onMenu != nil {
		c.onMenu(c.row, e.AbsolutePosition)
	}
}
