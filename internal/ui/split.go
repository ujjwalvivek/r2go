package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"

	r2theme "r2go/internal/theme"
)

const sidebarWidth float32 = 260

// newHSplit deliberately avoids Fyne's Split widget. Split reserves themed
// padding around its content and divider; this shell layout places the sidebar,
// a one-pixel divider and the workspace directly against the client edges.
func newHSplit(left, right fyne.CanvasObject) *fyne.Container {
	divider := canvas.NewRectangle(fyneTheme.Color(r2theme.ColorBorder))
	return container.New(&edgeSplitLayout{sidebarWidth: sidebarWidth, dividerWidth: 1}, left, divider, right)
}

type edgeSplitLayout struct {
	sidebarWidth float32
	dividerWidth float32
}

func (l *edgeSplitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 3 {
		return
	}
	leftWidth := fyne.Min(l.sidebarWidth, fyne.Max(0, size.Width-420))
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(leftWidth, size.Height))
	objects[1].Move(fyne.NewPos(leftWidth, 0))
	objects[1].Resize(fyne.NewSize(l.dividerWidth, size.Height))
	objects[2].Move(fyne.NewPos(leftWidth+l.dividerWidth, 0))
	objects[2].Resize(fyne.NewSize(fyne.Max(0, size.Width-leftWidth-l.dividerWidth), size.Height))
}

func (l *edgeSplitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.sidebarWidth+l.dividerWidth+620, 480)
}
