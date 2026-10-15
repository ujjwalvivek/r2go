package ui

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/model"
	t "r2go/internal/theme"
)

// Tile geometry. Each tile keeps a 4 px margin inside its cell, so tiles
// never touch and the hover and selection boxes have air between them.
const (
	tileWidth   float32 = 156
	tileHeight  float32 = 178
	tileMargin  float32 = 4
	tileInset   float32 = 12
	tilePreview float32 = 100
	tileIcon    float32 = 30
)

// GridView shows the folder as tiles: a preview for images and a large line
// icon for everything else, the name, and a size line.
type GridView struct {
	grid *widget.GridWrap
}

func NewGridView(length func() int, create func() fyne.CanvasObject, update func(int, fyne.CanvasObject)) *GridView {
	g := &GridView{}
	g.grid = widget.NewGridWrap(length, create, func(id widget.GridWrapItemID, object fyne.CanvasObject) { update(int(id), object) })
	g.grid.OnSelected = func(id widget.GridWrapItemID) { g.grid.Unselect(id) }
	return g
}

func (g *GridView) Columns() int { return max(1, g.grid.ColumnCount()) }
func (g *GridView) RefreshList() { g.grid.Refresh() }
func (g *GridView) RefreshItem(row int) {
	if row >= 0 {
		g.grid.RefreshItem(widget.GridWrapItemID(row))
	}
}
func (g *GridView) ScrollTo(row int) {
	if row >= 0 {
		g.grid.ScrollTo(widget.GridWrapItemID(row))
	}
}
func (g *GridView) ScrollToTop() { g.grid.ScrollToTop() }

// gridTile is one object in the grid.
type gridTile struct {
	widget.BaseWidget
	bg      *canvas.Rectangle
	edge    *canvas.Rectangle
	frame   *canvas.Rectangle
	preview *canvas.Image
	icon    *canvas.Image
	badge   *canvas.Text
	name    *textView
	meta    *textView

	row      int
	entry    model.Entry
	key      string
	selected bool
	hovered  bool

	onClick func(row int, modifier fyne.KeyModifier)
	onMenu  func(row int, position fyne.Position)
	onHover func(row int, hovering bool)
}

func newGridTile() *gridTile {
	g := &gridTile{
		bg:      canvas.NewRectangle(color.Transparent),
		edge:    canvas.NewRectangle(color.Transparent),
		frame:   newRect(t.ColorBase),
		preview: canvas.NewImageFromImage(nil),
		icon:    newIconSized(t.IconFile, t.ColorMuted, tileIcon),
		badge:   newText("", t.TextCaption, t.ColorMuted, styleMono),
		name:    newElided("", t.TextBody, t.ColorText, styleNormal),
		meta:    newElided("", t.TextSmall, t.ColorMuted, styleNormal),
	}
	g.badge.Alignment = fyne.TextAlignCenter
	g.edge.StrokeWidth = 1
	g.frame.StrokeColor = tc(t.ColorBorder)
	g.frame.StrokeWidth = 1
	g.preview.FillMode = canvas.ImageFillContain
	g.preview.ScaleMode = canvas.ImageScaleSmooth
	g.name.SetAlignment(fyne.TextAlignCenter)
	g.meta.SetAlignment(fyne.TextAlignCenter)
	g.ExtendBaseWidget(g)
	return g
}

func (g *gridTile) Set(entry model.Entry, key string, preview image.Image) {
	g.entry = entry
	g.key = key
	setResource(g.icon, entryIcon(entry))
	// Files without a preview show their format under the icon, in the
	// format's colour.
	g.badge.Text = typeLabel(entry)
	g.badge.Color = entryColor(entry)
	g.badge.Refresh()
	g.name.SetText(entry.Name)
	if entry.IsDir {
		g.name.SetStyle(styleStrong)
		switch {
		case entry.StatsKnown:
			g.meta.SetText(joinDot(countNoun(entry.Count, "object"), formatSize(entry.Size)))
		case entry.StatsError:
			g.meta.SetText("folder")
		default:
			g.meta.SetText("counting…")
		}
	} else {
		g.name.SetStyle(styleNormal)
		g.meta.SetText(joinDot(typeLabel(entry), formatSize(entry.Size)))
	}
	g.SetPreview(preview)
}

// SetPreview shows a decoded preview, or the icon when there is none.
func (g *gridTile) SetPreview(img image.Image) {
	if img != nil && g.preview.Image == img && g.preview.Visible() {
		return // already showing it; skip re-uploading the texture
	}
	g.preview.Image = img
	if img != nil {
		g.preview.Show()
		g.icon.Hide()
		g.badge.Hide()
	} else {
		g.preview.Hide()
		g.icon.Show()
		g.badge.Show()
	}
	g.preview.Refresh()
}

func (g *gridTile) SetState(selected, hovered bool) {
	g.selected, g.hovered = selected, hovered
	switch {
	case selected:
		g.bg.FillColor, g.edge.StrokeColor = tc(t.ColorSelected), tc(t.ColorAccent)
	case hovered:
		g.bg.FillColor, g.edge.StrokeColor = tc(t.ColorSurface), color.Transparent
	default:
		g.bg.FillColor, g.edge.StrokeColor = color.Transparent, color.Transparent
	}
	g.bg.Refresh()
	g.edge.Refresh()
}

func (g *gridTile) MinSize() fyne.Size { return fyne.NewSize(tileWidth, tileHeight) }

func (g *gridTile) CreateRenderer() fyne.WidgetRenderer {
	objects := []fyne.CanvasObject{g.bg, g.frame, g.preview, g.icon, g.badge, g.name, g.meta, g.edge}
	return widget.NewSimpleRenderer(container.New(&gridTileLayout{g: g}, objects...))
}

func (g *gridTile) MouseDown(event *desktop.MouseEvent) {
	tips.Hide(g)
	switch event.Button {
	case desktop.MouseButtonPrimary:
		if g.onClick != nil {
			g.onClick(g.row, event.Modifier)
		}
	case desktop.MouseButtonSecondary:
		if g.onMenu != nil {
			g.onMenu(g.row, event.AbsolutePosition)
		}
	}
}
func (g *gridTile) MouseUp(*desktop.MouseEvent) {}

// Tapped and TappedSecondary are handled in MouseDown. Claiming them here
// keeps the list from taking keyboard focus, so the app's own keys keep
// working after a click.
func (g *gridTile) Tapped(*fyne.PointEvent)          {}
func (g *gridTile) TappedSecondary(*fyne.PointEvent) {}
func (g *gridTile) MouseIn(event *desktop.MouseEvent) {
	if g.onHover != nil {
		g.onHover(g.row, true)
	}
	if g.name.Elided() {
		tips.Show(g, g.entry.Name, event.AbsolutePosition)
	}
}
func (g *gridTile) MouseMoved(event *desktop.MouseEvent) { tips.Move(g, event.AbsolutePosition) }
func (g *gridTile) MouseOut() {
	if g.onHover != nil {
		g.onHover(g.row, false)
	}
	tips.Hide(g)
}

type gridTileLayout struct{ g *gridTile }

func (l *gridTileLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	g := l.g
	box := fyne.NewSize(size.Width-2*tileMargin, size.Height-2*tileMargin)
	g.bg.Move(fyne.NewPos(tileMargin, tileMargin))
	g.bg.Resize(box)
	g.edge.Move(fyne.NewPos(tileMargin, tileMargin))
	g.edge.Resize(box)

	frameW := size.Width - 2*tileInset
	g.frame.Move(fyne.NewPos(tileInset, tileInset))
	g.frame.Resize(fyne.NewSize(frameW, tilePreview))
	g.preview.Move(fyne.NewPos(tileInset+1, tileInset+1))
	g.preview.Resize(fyne.NewSize(frameW-2, tilePreview-2))

	iconY := tileInset + (tilePreview-tileIcon)/2
	if g.badge.Text != "" {
		iconY -= 8
	}
	g.icon.Move(fyne.NewPos((size.Width-tileIcon)/2, iconY))
	g.icon.Resize(fyne.NewSize(tileIcon, tileIcon))
	g.badge.Move(fyne.NewPos(tileInset, iconY+tileIcon+4))
	g.badge.Resize(fyne.NewSize(frameW, 14))

	y := tileInset + tilePreview + 8
	g.name.Move(fyne.NewPos(tileInset, y))
	g.name.Resize(fyne.NewSize(frameW, 18))
	g.meta.Move(fyne.NewPos(tileInset, y+20))
	g.meta.Resize(fyne.NewSize(frameW, 16))
}

func (l *gridTileLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(tileWidth, tileHeight)
}
