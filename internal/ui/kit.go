package ui

import (
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	t "r2go/internal/theme"
)

// This file holds r2go's small component kit: text, icons, buttons, keycaps,
// pills, banners, panel headers, rules and the progress line. Everything is
// square and reads its colours from the theme tokens.

func tc(name fyne.ThemeColorName) color.Color { return t.Color(name) }

// ---------------------------------------------------------------- text

// newText is a single unwrapped line in a token colour.
func newText(value string, size float32, colorName fyne.ThemeColorName, style fyne.TextStyle) *canvas.Text {
	text := canvas.NewText(value, tc(colorName))
	text.TextSize = size
	text.TextStyle = style
	return text
}

var (
	styleNormal = fyne.TextStyle{}
	styleStrong = fyne.TextStyle{Bold: true}
	styleMono   = fyne.TextStyle{Monospace: true}
)

// textView is one line of text that, when elide is set, shortens itself with
// an ellipsis to fit the width it is given instead of demanding more. It is
// deliberately not hoverable, so it never steals hover from the row it sits
// in; wrap it in withTip for a tooltip.
type textView struct {
	widget.BaseWidget
	full      string
	elide     bool
	size      float32
	style     fyne.TextStyle
	color     color.Color
	alignment fyne.TextAlign
	text      *canvas.Text
}

func newTextView(value string, size float32, colorName fyne.ThemeColorName, style fyne.TextStyle) *textView {
	view := &textView{full: value, size: size, style: style, color: tc(colorName)}
	view.text = canvas.NewText(value, view.color)
	view.text.TextSize = size
	view.text.TextStyle = style
	view.ExtendBaseWidget(view)
	return view
}

// newElided is a textView that shortens with an ellipsis to fit.
func newElided(value string, size float32, colorName fyne.ThemeColorName, style fyne.TextStyle) *textView {
	view := newTextView(value, size, colorName, style)
	view.elide = true
	return view
}

func (v *textView) SetText(value string) {
	if v.full == value {
		return
	}
	v.full = value
	v.Refresh()
}

func (v *textView) Text() string { return v.full }

func (v *textView) SetColor(c color.Color) {
	v.color = c
	v.text.Color = c
	v.text.Refresh()
}

func (v *textView) SetColorName(name fyne.ThemeColorName) { v.SetColor(tc(name)) }

func (v *textView) SetStyle(style fyne.TextStyle) {
	if v.style == style {
		return
	}
	v.style = style
	v.Refresh()
}

func (v *textView) SetAlignment(alignment fyne.TextAlign) {
	v.alignment = alignment
	v.Refresh()
}

// Elided reports whether the view had to shorten its text to fit.
func (v *textView) Elided() bool { return v.elide && v.text.Text != v.full }

func (v *textView) CreateRenderer() fyne.WidgetRenderer { return &textViewRenderer{view: v} }

type textViewRenderer struct{ view *textView }

func (r *textViewRenderer) Destroy()                     {}
func (r *textViewRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.view.text} }

func (r *textViewRenderer) MinSize() fyne.Size {
	v := r.view
	measure := v.full
	if v.elide {
		measure = "…"
	}
	size := fyne.MeasureText(measure, v.size, v.style)
	if measure == "" {
		size = fyne.MeasureText("M", v.size, v.style)
		size.Width = 0
	}
	return size
}

func (r *textViewRenderer) Layout(size fyne.Size) {
	v := r.view
	v.text.Text = fitText(v.full, size.Width, v.size, v.style, v.elide)
	v.text.TextSize = v.size
	v.text.TextStyle = v.style
	v.text.Color = v.color
	v.text.Alignment = v.alignment
	height := fyne.MeasureText("M", v.size, v.style).Height
	v.text.Move(fyne.NewPos(0, (size.Height-height)/2-opticalNudge(v.size)))
	v.text.Resize(fyne.NewSize(size.Width, height))
}

func (r *textViewRenderer) Refresh() {
	r.Layout(r.view.Size())
	r.view.text.Refresh()
}

// tipBox shows a tooltip over its content. tip is called on hover, so it can
// return the current text, or "" for no tooltip.
type tipBox struct {
	widget.BaseWidget
	content fyne.CanvasObject
	tip     func() string
}

func withTip(content fyne.CanvasObject, tip func() string) *tipBox {
	b := &tipBox{content: content, tip: tip}
	b.ExtendBaseWidget(b)
	return b
}

// elidedTip is the usual tip for an elided view: its full text when shortened.
func elidedTip(view *textView) func() string {
	return func() string {
		if view.Elided() {
			return view.Text()
		}
		return ""
	}
}

func (b *tipBox) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(b.content) }
func (b *tipBox) MouseIn(event *desktop.MouseEvent) {
	if b.tip != nil {
		tips.Show(b, b.tip(), event.AbsolutePosition)
	}
}
func (b *tipBox) MouseMoved(event *desktop.MouseEvent) { tips.Move(b, event.AbsolutePosition) }
func (b *tipBox) MouseOut()                            { tips.Hide(b) }

// opticalNudge lifts text slightly when centring it vertically. A line box
// includes the font's tall ascender and its descender, so capitals and
// lowercase letters sit below the box's centre; this puts them on it.
func opticalNudge(size float32) float32 { return size * 0.07 }

// fieldIcon is a line icon for a field's trailing slot, centred in the
// field's full height (Fyne gives the slot the field's height).
func fieldIcon(name string) fyne.CanvasObject {
	return container.New(&fixedLayout{size: fyne.NewSize(24, t.SizeIcon)}, container.NewCenter(newIcon(name, t.ColorMuted)))
}

// fitText shortens value with a trailing ellipsis until it fits width.
func fitText(value string, width, size float32, style fyne.TextStyle, elide bool) string {
	if !elide || value == "" || fyne.MeasureText(value, size, style).Width <= width {
		return value
	}
	runes := []rune(value)
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		if fyne.MeasureText(string(runes[:mid])+"…", size, style).Width <= width {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low == 0 {
		return "…"
	}
	return strings.TrimRight(string(runes[:low]), " ") + "…"
}

// wrapped is a word-wrapping label in a token colour. Fyne labels only take
// the foreground colour, so the colour comes from a theme override.
func wrapped(value string, sizeName fyne.ThemeSizeName, colorName fyne.ThemeColorName, style fyne.TextStyle) (*widget.Label, fyne.CanvasObject) {
	label := widget.NewLabelWithStyle(value, fyne.TextAlignLeading, style)
	label.Wrapping = fyne.TextWrapWord
	label.SizeName = sizeName
	return label, container.NewThemeOverride(label, foregroundTheme{colorName})
}

// foregroundTheme recolours the foreground of a subtree.
type foregroundTheme struct{ name fyne.ThemeColorName }

func (f foregroundTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == fyneTheme.ColorNameForeground {
		return tc(f.name)
	}
	return fyne.CurrentApp().Settings().Theme().Color(name, variant)
}
func (f foregroundTheme) Font(style fyne.TextStyle) fyne.Resource {
	return fyne.CurrentApp().Settings().Theme().Font(style)
}
func (f foregroundTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return fyne.CurrentApp().Settings().Theme().Icon(name)
}
func (f foregroundTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == fyneTheme.SizeNameInnerPadding {
		return 0
	}
	return fyne.CurrentApp().Settings().Theme().Size(name)
}

// ---------------------------------------------------------------- icons and shapes

// newIcon is a line icon at the standard 14 px.
func newIcon(name string, colorName fyne.ThemeColorName) *canvas.Image {
	return newIconSized(name, colorName, t.SizeIcon)
}

func newIconSized(name string, colorName fyne.ThemeColorName, size float32) *canvas.Image {
	image := canvas.NewImageFromResource(t.Icon(name, colorName))
	image.FillMode = canvas.ImageFillContain
	image.SetMinSize(fyne.NewSize(size, size))
	return image
}

func setIcon(image *canvas.Image, name string, colorName fyne.ThemeColorName) {
	res := t.Icon(name, colorName)
	if image.Resource == res {
		return
	}
	image.Resource = res
	image.Refresh()
}

// setResource swaps an image's resource, refreshing only on change.
func setResource(image *canvas.Image, res fyne.Resource) {
	if image.Resource == res {
		return
	}
	image.Resource = res
	image.Refresh()
}

func newRect(colorName fyne.ThemeColorName) *canvas.Rectangle {
	return canvas.NewRectangle(tc(colorName))
}

// hRule is a 1 px horizontal line; vRule a 1 px vertical one.
func hRule(colorName fyne.ThemeColorName) fyne.CanvasObject {
	rule := newRect(colorName)
	rule.SetMinSize(fyne.NewSize(1, 1))
	return rule
}

func vRule(colorName fyne.ThemeColorName) fyne.CanvasObject {
	rule := newRect(colorName)
	rule.SetMinSize(fyne.NewSize(1, 1))
	return rule
}

// square is a small solid mark, like the status bar's state square.
func square(size float32, colorName fyne.ThemeColorName) *canvas.Rectangle {
	rect := newRect(colorName)
	rect.SetMinSize(fyne.NewSize(size, size))
	return rect
}

// ---------------------------------------------------------------- buttons

type buttonKind int

const (
	buttonGhost   buttonKind = iota // border only, secondary actions
	buttonPrimary                   // accent fill, the main action
	buttonDanger                    // border only, destructive actions
	buttonQuiet                     // no border, toolbar text actions
	buttonIcon                      // 26 px square, icon only
)

// button is r2go's only button: square, compact, with a hover fill and a
// tooltip. The tooltip follows the echohub convention: a first line of
// "Title  ·  Key" and optional detail lines.
type button struct {
	widget.BaseWidget
	kind     buttonKind
	label    string
	icon     string
	tip      string
	OnTapped func()

	disabled bool
	hovered  bool
	active   bool
	ghost    bool       // takes its space but draws nothing and ignores taps
	compact  bool       // 22 px instead of 26, for dense rows
	onHover  func(bool) // lets a parent row know the pointer is on the button

	bg   *canvas.Rectangle
	edge *canvas.Rectangle
	img  *canvas.Image
	txt  *canvas.Text
}

func newButton(kind buttonKind, label, icon string, tapped func()) *button {
	b := &button{kind: kind, label: label, icon: icon, OnTapped: tapped}
	b.bg = canvas.NewRectangle(color.Transparent)
	b.edge = canvas.NewRectangle(color.Transparent)
	b.edge.StrokeWidth = 1
	b.img = canvas.NewImageFromResource(nil)
	b.img.FillMode = canvas.ImageFillContain
	b.txt = canvas.NewText(label, tc(t.ColorText))
	b.txt.TextSize = t.TextBody
	b.ExtendBaseWidget(b)
	b.apply()
	return b
}

func newIconButton(icon, tip string, tapped func()) *button {
	b := newButton(buttonIcon, "", icon, tapped)
	b.tip = tip
	return b
}

func (b *button) withTip(tip string) *button { b.tip = tip; return b }

func (b *button) SetLabel(label string) {
	b.label = label
	b.txt.Text = label
	b.Refresh()
}

func (b *button) SetIcon(icon string) { b.icon = icon; b.apply() }

func (b *button) SetTip(tip string) { b.tip = tip }

func (b *button) Enable()  { b.SetDisabled(false) }
func (b *button) Disable() { b.SetDisabled(true) }
func (b *button) SetDisabled(disabled bool) {
	if b.disabled == disabled {
		return
	}
	b.disabled = disabled
	if disabled {
		b.hovered = false
	}
	b.apply()
}
func (b *button) Disabled() bool { return b.disabled }

// SetActive marks a toggle button as on: accent icon on a selected fill.
func (b *button) SetActive(active bool) {
	if b.active == active {
		return
	}
	b.active = active
	b.apply()
}

func (b *button) apply() {
	fill, edge, fg, icon := color.Color(color.Transparent), color.Color(color.Transparent), tc(t.ColorText), t.ColorSubtle
	switch b.kind {
	case buttonPrimary:
		fill, fg, icon = tc(t.ColorAccent), tc(t.ColorOnAccent), t.ColorOnAccent
		if b.hovered {
			fill = t.RGB(0xFF9A45)
		}
	case buttonGhost:
		edge = tc(t.ColorBorderStrong)
		if b.hovered {
			fill = tc(t.ColorSurface)
		}
	case buttonDanger:
		edge, fg, icon = tc(t.ColorBorderStrong), tc(t.ColorError), t.ColorError
		if b.hovered {
			fill, edge = t.WithAlpha(tc(t.ColorError), 0x22), tc(t.ColorError)
		}
	case buttonQuiet:
		fg = tc(t.ColorSubtle)
		if b.hovered {
			fill, fg, icon = tc(t.ColorSurface), tc(t.ColorText), t.ColorText
		}
	case buttonIcon:
		if b.hovered {
			fill, icon = tc(t.ColorSurface), t.ColorText
		}
		if b.active {
			fill, icon = tc(t.ColorSelected), t.ColorAccent
		}
	}
	if b.disabled {
		fill, fg, icon = color.Transparent, tc(t.ColorMuted), t.ColorBorderStrong
		if b.kind == buttonPrimary {
			fill = tc(t.ColorSurface)
			icon = t.ColorMuted
		}
		if b.kind == buttonGhost || b.kind == buttonDanger {
			edge = tc(t.ColorBorder)
		}
	}
	if b.ghost {
		fill, edge, fg = color.Transparent, color.Transparent, color.Transparent
	}
	b.bg.FillColor = fill
	b.edge.StrokeColor = edge
	b.txt.Color = fg
	// A ghost hides its icon outright: clearing an image's resource does not
	// remove what was already drawn.
	if b.icon != "" && !b.ghost {
		b.img.Resource = t.Icon(b.icon, icon)
		b.img.Show()
	} else {
		b.img.Hide()
	}
	b.bg.Refresh()
	b.edge.Refresh()
	b.txt.Refresh()
	b.img.Refresh()
}

// SetGhost hides the button without giving up its space, so rows that
// reveal actions on hover do not shift.
func (b *button) SetGhost(ghost bool) {
	if b.ghost == ghost {
		return
	}
	b.ghost = ghost
	b.apply()
}

func (b *button) Tapped(*fyne.PointEvent) {
	if b.disabled || b.ghost || b.OnTapped == nil {
		return
	}
	tips.Hide(b)
	b.OnTapped()
}

func (b *button) MouseIn(event *desktop.MouseEvent) {
	if b.onHover != nil {
		b.onHover(true)
	}
	if !b.disabled {
		b.hovered = true
		b.apply()
	}
	if b.tip != "" && !b.ghost {
		tips.Show(b, b.tip, event.AbsolutePosition)
	}
}
func (b *button) MouseMoved(event *desktop.MouseEvent) { tips.Move(b, event.AbsolutePosition) }
func (b *button) MouseOut() {
	if b.onHover != nil {
		b.onHover(false)
	}
	b.hovered = false
	b.apply()
	tips.Hide(b)
}

func (b *button) MinSize() fyne.Size {
	if b.kind == buttonIcon {
		if b.compact {
			return fyne.NewSize(22, 22)
		}
		return fyne.NewSize(t.SizeIconButton, t.SizeIconButton)
	}
	width := float32(20)
	if b.icon != "" {
		width += t.SizeIcon
		if b.label != "" {
			width += 6
		}
	}
	if b.label != "" {
		width += fyne.MeasureText(b.label, t.TextBody, fyne.TextStyle{}).Width
	}
	return fyne.NewSize(width, t.HeightControl)
}

func (b *button) CreateRenderer() fyne.WidgetRenderer {
	return &buttonRenderer{b: b}
}

type buttonRenderer struct{ b *button }

func (r *buttonRenderer) Destroy() {}
func (r *buttonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.b.bg, r.b.edge, r.b.img, r.b.txt}
}
func (r *buttonRenderer) MinSize() fyne.Size { return r.b.MinSize() }
func (r *buttonRenderer) Refresh() {
	r.b.txt.Text = r.b.label
	r.b.apply()
	r.Layout(r.b.Size())
}

func (r *buttonRenderer) Layout(size fyne.Size) {
	b := r.b
	b.bg.Resize(size)
	b.edge.Resize(size)
	textSize := fyne.MeasureText(b.label, t.TextBody, fyne.TextStyle{})
	content := float32(0)
	if b.icon != "" {
		content += t.SizeIcon
	}
	if b.label != "" {
		if b.icon != "" {
			content += 6
		}
		content += textSize.Width
	}
	x := (size.Width - content) / 2
	if b.icon != "" {
		b.img.Move(fyne.NewPos(x, (size.Height-t.SizeIcon)/2))
		b.img.Resize(fyne.NewSize(t.SizeIcon, t.SizeIcon))
		x += t.SizeIcon + 6
	}
	b.txt.Move(fyne.NewPos(x, (size.Height-textSize.Height)/2))
	b.txt.Resize(textSize)
}

// ---------------------------------------------------------------- checkbox

// checkbox is a square 14 px box with a label: a BORDER_STRONG edge when off,
// an accent fill with a check when on, and a SURFACE fill on hover. Fyne's own
// check draws a round focus halo that does not fit the square design.
type checkbox struct {
	widget.BaseWidget
	Checked   bool
	OnChanged func(bool)
	label     string
	box       *canvas.Rectangle
	mark      *canvas.Image
	text      *textView
	hovered   bool
}

func newCheckbox(label string, checked bool, changed func(bool)) *checkbox {
	c := &checkbox{
		Checked:   checked,
		OnChanged: changed,
		label:     label,
		box:       canvas.NewRectangle(color.Transparent),
		mark:      newIconSized(t.IconCheck, t.ColorOnAccent, 12),
		text:      newTextView(label, t.TextBody, t.ColorText, styleNormal),
	}
	c.box.StrokeWidth = 1
	c.ExtendBaseWidget(c)
	c.apply()
	return c
}

func (c *checkbox) SetChecked(checked bool) {
	if c.Checked == checked {
		return
	}
	c.Checked = checked
	c.apply()
	if c.OnChanged != nil {
		c.OnChanged(checked)
	}
}

func (c *checkbox) apply() {
	switch {
	case c.Checked:
		c.box.FillColor, c.box.StrokeColor = tc(t.ColorAccent), tc(t.ColorAccent)
		c.mark.Show()
	case c.hovered:
		c.box.FillColor, c.box.StrokeColor = tc(t.ColorSurface), tc(t.ColorSubtle)
		c.mark.Hide()
	default:
		c.box.FillColor, c.box.StrokeColor = tc(t.ColorBase), tc(t.ColorBorderStrong)
		c.mark.Hide()
	}
	c.box.Refresh()
}

func (c *checkbox) Tapped(*fyne.PointEvent) { c.SetChecked(!c.Checked) }
func (c *checkbox) MouseIn(*desktop.MouseEvent) {
	c.hovered = true
	c.apply()
}
func (c *checkbox) MouseMoved(*desktop.MouseEvent) {}
func (c *checkbox) MouseOut() {
	c.hovered = false
	c.apply()
}

func (c *checkbox) MinSize() fyne.Size {
	text := fyne.MeasureText(c.label, t.TextBody, styleNormal)
	return fyne.NewSize(14+8+text.Width, fyne.Max(t.HeightControl-4, text.Height))
}

func (c *checkbox) CreateRenderer() fyne.WidgetRenderer {
	box := container.NewStack(c.box, container.NewCenter(c.mark))
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil,
		container.NewCenter(fixed(14, 14, box)), nil, inset(8, 0, 0, 0, c.text)))
}

// ---------------------------------------------------------------- keycaps and pills

// keycap is mono caption text on SURFACE with a BORDER_STRONG edge.
func keycap(key string) fyne.CanvasObject {
	fill := newRect(t.ColorSurface)
	fill.StrokeColor = tc(t.ColorBorderStrong)
	fill.StrokeWidth = 1
	label := newTextView(key, t.TextCaption, t.ColorSubtle, styleMono)
	return container.NewStack(fill, inset(5, 1, 5, 1, container.NewCenter(label)))
}

// keyHint is a keycap followed by what the key does.
func keyHint(key, action string) fyne.CanvasObject {
	return container.New(&rowLayout{gap: 5}, keycap(key), newTextView(action, t.TextSmall, t.ColorMuted, styleNormal))
}

// pill is a small mono tag, for file types and counts.
type pill struct {
	widget.BaseWidget
	fill  *canvas.Rectangle
	label *canvas.Text
}

func newPill(value string) *pill {
	p := &pill{fill: newRect(t.ColorSurface), label: newText(value, t.TextCaption, t.ColorSubtle, styleMono)}
	p.fill.StrokeColor = tc(t.ColorBorderStrong)
	p.fill.StrokeWidth = 1
	p.ExtendBaseWidget(p)
	return p
}

func (p *pill) SetText(value string) {
	p.label.Text = value
	p.Refresh()
}

func (p *pill) MinSize() fyne.Size {
	size := fyne.MeasureText(p.label.Text, t.TextCaption, styleMono)
	return fyne.NewSize(size.Width+10, size.Height+2)
}

func (p *pill) CreateRenderer() fyne.WidgetRenderer {
	return &pillRenderer{p: p}
}

type pillRenderer struct{ p *pill }

func (r *pillRenderer) Destroy()                     {}
func (r *pillRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.p.fill, r.p.label} }
func (r *pillRenderer) MinSize() fyne.Size           { return r.p.MinSize() }
func (r *pillRenderer) Refresh()                     { r.Layout(r.p.Size()); r.p.fill.Refresh(); r.p.label.Refresh() }
func (r *pillRenderer) Layout(size fyne.Size) {
	min := r.p.MinSize()
	if r.p.label.Text == "" {
		r.p.fill.Hide()
		return
	}
	r.p.fill.Show()
	y := (size.Height - min.Height) / 2
	r.p.fill.Move(fyne.NewPos(0, y))
	r.p.fill.Resize(min)
	r.p.label.Move(fyne.NewPos(5, y+1))
	r.p.label.Resize(fyne.NewSize(min.Width-10, min.Height-2))
}

// ---------------------------------------------------------------- banners

type bannerTone int

const (
	toneWarning bannerTone = iota
	toneError
)

// banner is a slim PANEL line with a 2 px gold or red left border. It is
// shown only while the problem exists.
type banner struct {
	Container *fyne.Container
	bar       *canvas.Rectangle
	icon      *canvas.Image
	title     *textView
	detail    *textView
	tip       string
	actions   *fyne.Container
}

func newBanner() *banner {
	b := &banner{
		bar:     newRect(t.ColorError),
		icon:    newIcon(t.IconAlert, t.ColorError),
		title:   newTextView("", t.TextBody, t.ColorText, styleStrong),
		detail:  newElided("", t.TextBody, t.ColorSubtle, styleNormal),
		actions: container.New(&rowLayout{gap: 4}),
	}
	b.bar.SetMinSize(fyne.NewSize(t.WidthAccentBar, 1))
	left := container.New(&rowLayout{gap: 8}, b.icon, b.title)
	detail := withTip(b.detail, func() string {
		if b.tip != "" {
			return b.tip
		}
		return elidedTip(b.detail)()
	})
	row := container.NewBorder(nil, nil, left, b.actions, inset(8, 0, 8, 0, detail))
	body := container.NewBorder(nil, hRule(t.ColorBorder), b.bar, nil, inset(t.SpaceCardX, 5, 6, 5, row))
	b.Container = container.NewStack(newRect(t.ColorPanel), body)
	b.Container.Hide()
	return b
}

// Show displays the banner with a title, one line of detail and actions.
func (b *banner) Show(tone bannerTone, title, detail string, actions ...fyne.CanvasObject) {
	b.ShowWithTip(tone, title, detail, "", actions...)
}

// ShowWithTip is Show with a tooltip on the detail, for the raw error.
func (b *banner) ShowWithTip(tone bannerTone, title, detail, tip string, actions ...fyne.CanvasObject) {
	b.tip = tip
	colorName := t.ColorError
	if tone == toneWarning {
		colorName = t.ColorWarning
	}
	b.bar.FillColor = tc(colorName)
	b.bar.Refresh()
	setIcon(b.icon, t.IconAlert, colorName)
	b.title.SetText(title)
	b.detail.SetText(detail)
	b.actions.Objects = actions
	b.actions.Refresh()
	b.Container.Show()
	b.Container.Refresh()
}

func (b *banner) Hide()         { b.Container.Hide() }
func (b *banner) Visible() bool { return b.Container.Visible() }

// ---------------------------------------------------------------- panel header

// panelHeader is a section heading: a caption on the left, a mono count on the
// right and a 1 px rule above.
type panelHeader struct {
	Container fyne.CanvasObject
	count     *canvas.Text
}

func newPanelHeader(title string, right ...fyne.CanvasObject) *panelHeader {
	h := &panelHeader{count: newText("", t.TextSmall, t.ColorMuted, styleMono)}
	caption := newText(title, t.TextSmall, t.ColorSubtle, styleStrong)
	trailing := append([]fyne.CanvasObject{h.count}, right...)
	row := container.NewBorder(nil, nil, caption, container.New(&rowLayout{gap: 6}, trailing...))
	h.Container = container.NewBorder(hRule(t.ColorBorder), nil, nil, nil, inset(t.SpaceCardX, 6, t.SpaceCardX, 4, row))
	return h
}

func (h *panelHeader) SetCount(value string) { h.count.Text = value; h.count.Refresh() }

// ---------------------------------------------------------------- progress line

// progressLine is the 2 px accent line under the toolbar. It sweeps while
// work of unknown length runs and is invisible otherwise.
type progressLine struct {
	widget.BaseWidget
	track *canvas.Rectangle
	bar   *canvas.Rectangle
	busy  int
	phase float32
	anim  *fyne.Animation
}

func newProgressLine() *progressLine {
	p := &progressLine{track: canvas.NewRectangle(color.Transparent), bar: newRect(t.ColorAccent)}
	p.bar.Hide()
	p.anim = fyne.NewAnimation(1100*time.Millisecond, func(value float32) {
		p.phase = value
		p.layoutBar(p.Size())
	})
	p.anim.RepeatCount = fyne.AnimationRepeatForever
	p.anim.Curve = fyne.AnimationEaseInOut
	p.ExtendBaseWidget(p)
	return p
}

// Begin and End count overlapping pieces of work; the line sweeps while any
// are running.
func (p *progressLine) Begin() {
	p.busy++
	if p.busy == 1 {
		p.bar.Show()
		p.anim.Start()
	}
}

func (p *progressLine) End() {
	if p.busy == 0 {
		return
	}
	p.busy--
	if p.busy == 0 {
		p.anim.Stop()
		p.bar.Hide()
	}
}

func (p *progressLine) MinSize() fyne.Size { return fyne.NewSize(10, t.ProgressLineH) }

func (p *progressLine) layoutBar(size fyne.Size) {
	width := size.Width * 0.28
	x := -width + (size.Width+width)*p.phase
	p.bar.Move(fyne.NewPos(x, 0))
	p.bar.Resize(fyne.NewSize(width, size.Height))
	p.bar.Refresh()
}

func (p *progressLine) CreateRenderer() fyne.WidgetRenderer {
	return &progressLineRenderer{p: p}
}

type progressLineRenderer struct{ p *progressLine }

func (r *progressLineRenderer) Destroy() { r.p.anim.Stop() }
func (r *progressLineRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.track, r.p.bar}
}
func (r *progressLineRenderer) MinSize() fyne.Size { return r.p.MinSize() }
func (r *progressLineRenderer) Refresh()           { r.Layout(r.p.Size()) }
func (r *progressLineRenderer) Layout(size fyne.Size) {
	r.p.track.Resize(size)
	r.p.layoutBar(size)
}

// meter is a thin determinate bar on a BORDER track, for transfers and the
// size column.
type meter struct {
	widget.BaseWidget
	value     float64
	thickness float32
	track     *canvas.Rectangle
	fill      *canvas.Rectangle
}

func newMeter(thickness float32, fillColor fyne.ThemeColorName) *meter {
	m := &meter{thickness: thickness, track: newRect(t.ColorBorder), fill: newRect(fillColor)}
	m.ExtendBaseWidget(m)
	return m
}

func (m *meter) SetValue(value float64) {
	value = max(0, min(1, value))
	if m.value == value {
		return
	}
	m.value = value
	m.Refresh()
}

func (m *meter) SetFill(name fyne.ThemeColorName) {
	m.fill.FillColor = tc(name)
	m.fill.Refresh()
}

func (m *meter) MinSize() fyne.Size { return fyne.NewSize(20, m.thickness) }
func (m *meter) CreateRenderer() fyne.WidgetRenderer {
	return &meterRenderer{m: m}
}

type meterRenderer struct{ m *meter }

func (r *meterRenderer) Destroy() {}
func (r *meterRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.m.track, r.m.fill}
}
func (r *meterRenderer) MinSize() fyne.Size { return r.m.MinSize() }
func (r *meterRenderer) Refresh() {
	r.Layout(r.m.Size())
	r.m.track.Refresh()
	r.m.fill.Refresh()
}
func (r *meterRenderer) Layout(size fyne.Size) {
	h := fyne.Min(size.Height, r.m.thickness)
	y := (size.Height - h) / 2
	r.m.track.Move(fyne.NewPos(0, y))
	r.m.track.Resize(fyne.NewSize(size.Width, h))
	r.m.fill.Move(fyne.NewPos(0, y))
	r.m.fill.Resize(fyne.NewSize(size.Width*float32(r.m.value), h))
}

// ---------------------------------------------------------------- tappable areas

// hoverArea wraps content with a hover fill, a tap handler and an optional
// secondary-tap handler, for list rows that are not buttons.
type hoverArea struct {
	widget.BaseWidget
	content   fyne.CanvasObject
	bg        *canvas.Rectangle
	hovered   bool
	selected  bool
	onTap     func()
	onMenu    func(fyne.Position)
	onHover   func(bool)
	tip       string
	hoverFill fyne.ThemeColorName
}

func newHoverArea(content fyne.CanvasObject, tapped func()) *hoverArea {
	h := &hoverArea{content: content, bg: canvas.NewRectangle(color.Transparent), onTap: tapped, hoverFill: t.ColorSurface}
	h.ExtendBaseWidget(h)
	return h
}

func (h *hoverArea) apply() {
	switch {
	case h.selected:
		h.bg.FillColor = tc(t.ColorSelected)
	case h.hovered:
		h.bg.FillColor = tc(h.hoverFill)
	default:
		h.bg.FillColor = color.Transparent
	}
	h.bg.Refresh()
}

func (h *hoverArea) SetSelected(selected bool) { h.selected = selected; h.apply() }

func (h *hoverArea) Tapped(*fyne.PointEvent) {
	tips.Hide(h)
	if h.onTap != nil {
		h.onTap()
	}
}
func (h *hoverArea) TappedSecondary(event *fyne.PointEvent) {
	if h.onMenu != nil {
		h.onMenu(event.AbsolutePosition)
	}
}
func (h *hoverArea) MouseIn(event *desktop.MouseEvent) {
	h.hovered = true
	h.apply()
	if h.onHover != nil {
		h.onHover(true)
	}
	if h.tip != "" {
		tips.Show(h, h.tip, event.AbsolutePosition)
	}
}
func (h *hoverArea) MouseMoved(event *desktop.MouseEvent) { tips.Move(h, event.AbsolutePosition) }
func (h *hoverArea) MouseOut() {
	h.hovered = false
	h.apply()
	if h.onHover != nil {
		h.onHover(false)
	}
	tips.Hide(h)
}

func (h *hoverArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(h.bg, h.content))
}

// ---------------------------------------------------------------- layouts

type insetLayout struct {
	left, top, right, bottom float32
}

func inset(left, top, right, bottom float32, object fyne.CanvasObject) *fyne.Container {
	return container.New(&insetLayout{left: left, top: top, right: right, bottom: bottom}, object)
}

func (l *insetLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(l.left, l.top))
		object.Resize(fyne.NewSize(
			fyne.Max(0, size.Width-l.left-l.right),
			fyne.Max(0, size.Height-l.top-l.bottom),
		))
	}
}

func (l *insetLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var size fyne.Size
	for _, object := range objects {
		if object.Visible() {
			size = size.Max(object.MinSize())
		}
	}
	return fyne.NewSize(size.Width+l.left+l.right, size.Height+l.top+l.bottom)
}

// rowLayout places visible objects left to right at their minimum widths,
// with a fixed gap, each centred vertically in the row.
type rowLayout struct{ gap float32 }

func (l *rowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		object.Move(fyne.NewPos(x, (size.Height-min.Height)/2))
		object.Resize(min)
		x += min.Width + l.gap
	}
}

func (l *rowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width, height float32
	count := 0
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		width += min.Width
		height = fyne.Max(height, min.Height)
		count++
	}
	if count > 1 {
		width += l.gap * float32(count-1)
	}
	return fyne.NewSize(width, height)
}

// columnLayout stacks visible objects top to bottom at their minimum heights,
// full width, with a fixed gap.
type columnLayout struct{ gap float32 }

func vbox(gap float32, objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&columnLayout{gap: gap}, objects...)
}

func (l *columnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		h := object.MinSize().Height
		object.Move(fyne.NewPos(0, y))
		object.Resize(fyne.NewSize(size.Width, h))
		y += h + l.gap
	}
}

func (l *columnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width, height float32
	count := 0
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		width = fyne.Max(width, min.Width)
		height += min.Height
		count++
	}
	if count > 1 {
		height += l.gap * float32(count-1)
	}
	return fyne.NewSize(width, height)
}

// fixedLayout gives its content an exact size, whatever the content asks for.
type fixedLayout struct{ size fyne.Size }

func fixed(width, height float32, object fyne.CanvasObject) *fyne.Container {
	return container.New(&fixedLayout{size: fyne.NewSize(width, height)}, object)
}

func (l *fixedLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, (size.Height-l.size.Height)/2))
		object.Resize(fyne.NewSize(fyne.Min(size.Width, l.size.Width), l.size.Height))
	}
}

func (l *fixedLayout) MinSize([]fyne.CanvasObject) fyne.Size { return l.size }

// fillHeight gives its content a minimum height, keeping its width flexible.
func minHeight(height float32, object fyne.CanvasObject) *fyne.Container {
	return container.New(&minHeightLayout{height: height}, object)
}

type minHeightLayout struct{ height float32 }

func (l *minHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

func (l *minHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var size fyne.Size
	for _, object := range objects {
		size = size.Max(object.MinSize())
	}
	return fyne.NewSize(size.Width, fyne.Max(size.Height, l.height))
}

// panelBox is content on a token-coloured background.
func panelBox(colorName fyne.ThemeColorName, content fyne.CanvasObject) *fyne.Container {
	return container.NewStack(newRect(colorName), content)
}

// fixedHeightLayout centres its content vertically at an exact height and
// asks for nothing more, so a taller widget cannot grow the bar it sits in.
type fixedHeightLayout struct{ height float32 }

func (l *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, (size.Height-l.height)/2))
		object.Resize(fyne.NewSize(size.Width, l.height))
	}
}

func (l *fixedHeightLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(40, l.height)
}
