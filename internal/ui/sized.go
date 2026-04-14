package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// sized wraps a widget in a GridWrap cell of a fixed size.
// Fyne Entry MinSize is ~1 character, which collapses inputs in HBox rows
// and shrinks dialogs to a narrow column; this gives them a usable minimum.
func sized(w, h float32, o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(w, h), o)
}

// wideEntry is a convenience for the common single-line input case.
func wideEntry(w float32, e *widget.Entry) fyne.CanvasObject {
	return sized(w, 40, e)
}

// fixedMin forces a minimum width on any canvas object. The Fyne Tree lays
// its rows out at its minimum width during startup and never re-lays them
// when it later grows, so row widgets (and their tap targets) end up ~100px
// wide while the sidebar is 270px. Wrapping the tree in a fixedMin keeps its
// minimum at the sidebar width so rows always span the full row.
func fixedMin(w float32, o fyne.CanvasObject) fyne.CanvasObject {
	return newFixedMin(w, o)
}

type minWidthObject struct {
	widget.BaseWidget
	content fyne.CanvasObject
	w       float32
}

func newFixedMin(w float32, o fyne.CanvasObject) *minWidthObject {
	m := &minWidthObject{content: o, w: w}
	m.ExtendBaseWidget(m)
	return m
}

func (m *minWidthObject) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.content)
}

func (m *minWidthObject) MinSize() fyne.Size {
	s := m.content.MinSize()
	s.Width = fyne.Max(s.Width, m.w)
	return s
}
