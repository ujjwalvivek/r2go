package ui

import (
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	t "r2go/internal/theme"
)

// ---------------------------------------------------------------- tooltips

// tips is the app-wide tooltip manager. Fyne has no tooltips, so widgets call
// Show, Move and Hide from their hover handlers and the manager draws one
// tooltip on the topmost layer (the window, or the open modal).
var tips = &tooltipManager{}

const (
	tooltipDelay = 450 * time.Millisecond
	tooltipFade  = 110 * time.Millisecond
)

type tooltipManager struct {
	mu     sync.Mutex
	layers []*tipLayer
	owner  fyne.CanvasObject
	text   string
	pos    fyne.Position
	serial uint64
	timer  *time.Timer
}

func (m *tooltipManager) push(layer *tipLayer) { m.layers = append(m.layers, layer) }

func (m *tooltipManager) pop(layer *tipLayer) {
	m.Hide(nil)
	for i := len(m.layers) - 1; i >= 0; i-- {
		if m.layers[i] == layer {
			m.layers = append(m.layers[:i], m.layers[i+1:]...)
			return
		}
	}
}

// Show starts the hover delay for owner's tooltip at the pointer position.
func (m *tooltipManager) Show(owner fyne.CanvasObject, text string, pos fyne.Position) {
	m.mu.Lock()
	m.hideLocked()
	if strings.TrimSpace(text) == "" {
		m.mu.Unlock()
		return
	}
	m.owner, m.text, m.pos = owner, text, pos
	serial := m.serial
	m.timer = time.AfterFunc(tooltipDelay, func() {
		fyne.Do(func() {
			m.mu.Lock()
			if m.serial != serial || m.owner != owner || len(m.layers) == 0 {
				m.mu.Unlock()
				return
			}
			layer, text, pos := m.layers[len(m.layers)-1], m.text, m.pos
			m.mu.Unlock()
			layer.show(text, pos)
		})
	})
	m.mu.Unlock()
}

// Move follows the pointer until the tooltip appears.
func (m *tooltipManager) Move(owner fyne.CanvasObject, pos fyne.Position) {
	m.mu.Lock()
	if m.owner == owner {
		m.pos = pos
	}
	m.mu.Unlock()
}

// Hide removes owner's tooltip; a nil owner hides whatever is showing.
func (m *tooltipManager) Hide(owner fyne.CanvasObject) {
	m.mu.Lock()
	if owner == nil || owner == m.owner {
		m.hideLocked()
	}
	m.mu.Unlock()
}

func (m *tooltipManager) hideLocked() {
	m.serial++
	m.owner = nil
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	for _, layer := range m.layers {
		layer.hide()
	}
}

// tipLayer is a full-size, non-interactive layer that draws the tooltip box.
// It holds only canvas objects, so pointer events pass through to the widgets
// underneath.
type tipLayer struct {
	Container *fyne.Container
	box       *fyne.Container
	bg        *canvas.Rectangle
	title     *canvas.Text
	key       *canvas.Text
	keyBg     *canvas.Rectangle
	lines     []*canvas.Text
	anim      *fyne.Animation
}

func newTipLayer() *tipLayer {
	l := &tipLayer{
		bg:    newRect(t.ColorSurface),
		title: newText("", t.TextBody, t.ColorText, styleStrong),
		key:   newText("", t.TextCaption, t.ColorSubtle, styleMono),
		keyBg: newRect(t.ColorPanel),
	}
	l.bg.StrokeColor = tc(t.ColorBorderStrong)
	l.bg.StrokeWidth = 1
	l.keyBg.StrokeColor = tc(t.ColorBorderStrong)
	l.keyBg.StrokeWidth = 1
	l.box = container.NewWithoutLayout(l.bg, l.title, l.keyBg, l.key)
	l.box.Hide()
	l.Container = container.NewWithoutLayout(l.box)
	return l
}

func (l *tipLayer) hide() {
	if l.anim != nil {
		l.anim.Stop()
	}
	l.box.Hide()
}

func (l *tipLayer) show(text string, pointer fyne.Position) {
	const padX, padY, gap float32 = 9, 6, 2
	parts := strings.Split(text, "\n")
	title, key := parts[0], ""
	if i := strings.LastIndex(title, "  ·  "); i >= 0 {
		title, key = title[:i], title[i+len("  ·  "):]
	}
	l.title.Text = title

	var details []string
	for _, line := range parts[1:] {
		details = append(details, wrapLine(line, t.TooltipMaxWidth-2*padX, t.TextSmall)...)
	}
	for len(l.lines) < len(details) {
		line := newText("", t.TextSmall, t.ColorSubtle, styleNormal)
		l.lines = append(l.lines, line)
		l.box.Add(line)
	}

	titleSize := fyne.MeasureText(title, t.TextBody, styleStrong)
	width := titleSize.Width
	keySize := fyne.Size{}
	if key != "" {
		keySize = fyne.MeasureText(key, t.TextCaption, styleMono)
		width += 8 + keySize.Width + 8
	}
	y := padY
	l.title.Move(fyne.NewPos(padX, y))
	l.title.Resize(titleSize)
	if key != "" {
		l.key.Text = key
		kx := padX + titleSize.Width + 8
		l.keyBg.Move(fyne.NewPos(kx, y+1))
		l.keyBg.Resize(fyne.NewSize(keySize.Width+8, titleSize.Height-2))
		l.key.Move(fyne.NewPos(kx+4, y+(titleSize.Height-keySize.Height)/2))
		l.key.Resize(keySize)
		l.key.Show()
		l.keyBg.Show()
	} else {
		l.key.Hide()
		l.keyBg.Hide()
	}
	y += titleSize.Height
	for i, line := range l.lines {
		if i >= len(details) {
			line.Hide()
			continue
		}
		line.Text = details[i]
		size := fyne.MeasureText(details[i], t.TextSmall, styleNormal)
		width = fyne.Max(width, size.Width)
		line.Move(fyne.NewPos(padX, y+gap))
		line.Resize(size)
		line.Show()
		y += size.Height + gap
	}
	boxSize := fyne.NewSize(width+2*padX, y+padY)
	l.bg.Resize(boxSize)

	origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(l.Container)
	area := l.Container.Size()
	x := pointer.X - origin.X + 12
	top := pointer.Y - origin.Y + 20
	if x+boxSize.Width > area.Width-4 {
		x = fyne.Max(4, area.Width-4-boxSize.Width)
	}
	if top+boxSize.Height > area.Height-4 {
		top = fyne.Max(4, pointer.Y-origin.Y-boxSize.Height-8)
	}
	l.box.Move(fyne.NewPos(x, top))
	l.box.Resize(boxSize)
	l.fade(0)
	l.box.Show()
	l.box.Refresh()
	if l.anim != nil {
		l.anim.Stop()
	}
	l.anim = fyne.NewAnimation(tooltipFade, l.fade)
	l.anim.Start()
}

func (l *tipLayer) fade(value float32) {
	alpha := uint8(255 * value)
	l.bg.FillColor = t.WithAlpha(tc(t.ColorSurface), alpha)
	l.bg.StrokeColor = t.WithAlpha(tc(t.ColorBorderStrong), alpha)
	l.title.Color = t.WithAlpha(tc(t.ColorText), alpha)
	l.key.Color = t.WithAlpha(tc(t.ColorSubtle), alpha)
	l.keyBg.FillColor = t.WithAlpha(tc(t.ColorPanel), alpha)
	l.keyBg.StrokeColor = t.WithAlpha(tc(t.ColorBorderStrong), alpha)
	for _, line := range l.lines {
		line.Color = t.WithAlpha(tc(t.ColorSubtle), alpha)
	}
	l.box.Refresh()
}

// wrapLine breaks one line of text into lines no wider than width.
func wrapLine(line string, width, size float32) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if fyne.MeasureText(candidate, size, styleNormal).Width > width {
			out = append(out, current)
			current = word
			continue
		}
		current = candidate
	}
	return append(out, current)
}

// ---------------------------------------------------------------- fields

// field is a text entry that also reports Escape and the arrow keys, so
// dialogs and the quick-jump list can react to them while it has focus.
type field struct {
	widget.Entry
	onEscape func()
	onUp     func()
	onDown   func()
	onBlur   func()
}

func (f *field) FocusLost() {
	f.Entry.FocusLost()
	if f.onBlur != nil {
		f.onBlur()
	}
}

func newField(placeholder string) *field {
	f := &field{}
	f.ExtendBaseWidget(f)
	f.SetPlaceHolder(placeholder)
	return f
}

// newSecretField is a masked field with an eye that shows the value only
// while it is held down.
func newSecretField(placeholder string) *field {
	f := newField(placeholder)
	f.Password = true
	f.ActionItem = newHoldReveal(f)
	return f
}

func (f *field) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyEscape:
		if f.onEscape != nil {
			f.onEscape()
			return
		}
	case fyne.KeyUp:
		if f.onUp != nil {
			f.onUp()
			return
		}
	case fyne.KeyDown:
		if f.onDown != nil {
			f.onDown()
			return
		}
	}
	f.Entry.TypedKey(key)
}

// holdReveal is the eye in a secret field: it unmasks the field while the
// mouse button is held on it and masks it again on release or leave.
type holdReveal struct {
	widget.BaseWidget
	entry *field
	icon  *canvas.Image
}

func newHoldReveal(entry *field) *holdReveal {
	h := &holdReveal{entry: entry, icon: newIcon(t.IconEye, t.ColorMuted)}
	h.ExtendBaseWidget(h)
	return h
}

func (h *holdReveal) MinSize() fyne.Size { return fyne.NewSize(24, 20) }

func (h *holdReveal) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewCenter(h.icon))
}

func (h *holdReveal) reveal(show bool) {
	if h.entry.Password != show {
		return
	}
	h.entry.Password = !show
	if show {
		setIcon(h.icon, t.IconEye, t.ColorAccent)
	} else {
		setIcon(h.icon, t.IconEye, t.ColorMuted)
	}
	h.entry.Refresh()
}

func (h *holdReveal) MouseDown(*desktop.MouseEvent) { h.reveal(true); tips.Hide(h) }
func (h *holdReveal) MouseUp(*desktop.MouseEvent)   { h.reveal(false) }
func (h *holdReveal) MouseIn(event *desktop.MouseEvent) {
	tips.Show(h, "Hold to show", event.AbsolutePosition)
}
func (h *holdReveal) MouseMoved(event *desktop.MouseEvent) { tips.Move(h, event.AbsolutePosition) }
func (h *holdReveal) MouseOut() {
	h.reveal(false)
	tips.Hide(h)
}

// ---------------------------------------------------------------- modals

// modal is r2go's dialog frame: a square PANEL box with a BORDER_STRONG edge,
// a title row, a body and a footer with a Ghost cancel and a Primary confirm.
// Escape cancels and Enter confirms, from a field or from the frame itself.
type modal struct {
	pop       *widget.PopUp
	frame     *fyne.Container
	layer     *tipLayer
	width     float32
	confirm   *button
	cancel    *button
	catcher   *keyCatcher
	onConfirm func() bool
	onCancel  func()
	closed    bool
}

type modalSpec struct {
	title       string
	width       float32
	body        fyne.CanvasObject
	footerLeft  []fyne.CanvasObject
	confirmText string // empty for a dialog with only a close button
	confirmKind buttonKind
	cancelText  string
	onConfirm   func() bool // return false to keep the dialog open
	onCancel    func()
	focus       fyne.Focusable
}

func showModal(win fyne.Window, spec modalSpec) *modal {
	m := &modal{width: spec.width, onConfirm: spec.onConfirm, onCancel: spec.onCancel, layer: newTipLayer()}
	if m.width == 0 {
		m.width = 440
	}
	m.catcher = newKeyCatcher(m.Confirm, m.Cancel)

	title := newText(spec.title, t.TextMedium, t.ColorText, styleStrong)
	closeButton := newIconButton(t.IconClose, "Close  ·  Esc", m.Cancel)
	header := inset(t.SpacePageX, 5, 5, 5, container.NewBorder(nil, nil, title, closeButton))

	cancelText := spec.cancelText
	if cancelText == "" {
		cancelText = "Cancel"
		if spec.confirmText == "" {
			cancelText = "Close"
		}
	}
	m.cancel = newButton(buttonGhost, cancelText, "", m.Cancel)
	right := []fyne.CanvasObject{m.cancel}
	if spec.confirmText != "" {
		kind := spec.confirmKind
		if kind == buttonGhost {
			kind = buttonPrimary
		}
		m.confirm = newButton(kind, spec.confirmText, "", m.Confirm)
		right = append(right, m.confirm)
	}
	footer := container.NewBorder(nil, nil,
		container.New(&rowLayout{gap: 6}, spec.footerLeft...),
		container.New(&rowLayout{gap: 6}, right...))

	body := inset(t.SpacePageX, t.SpacePageY, t.SpacePageX, t.SpacePageY, spec.body)
	content := container.NewBorder(
		vbox(0, header, hRule(t.ColorBorder)),
		vbox(0, hRule(t.ColorBorder), inset(t.SpacePageX, 8, t.SpacePageX, 8, footer)),
		nil, nil, body)

	bg := newRect(t.ColorPanel)
	edge := canvas.NewRectangle(color.Transparent)
	edge.StrokeColor = tc(t.ColorBorderStrong)
	edge.StrokeWidth = 1
	m.frame = container.NewStack(bg, m.catcher, content, edge, m.layer.Container)

	m.pop = widget.NewModalPopUp(m.frame, win.Canvas())
	tips.Hide(nil)
	tips.push(m.layer)
	m.pop.Show()
	m.relayout()
	// Wrapped labels only know their height once they have a width, so fit
	// the dialog again after the first layout.
	time.AfterFunc(40*time.Millisecond, func() { fyne.Do(m.relayout) })
	if spec.focus != nil {
		win.Canvas().Focus(spec.focus)
	} else {
		win.Canvas().Focus(m.catcher)
	}
	return m
}

// relayout resizes the dialog after its body changed height.
func (m *modal) relayout() {
	if m.closed {
		return
	}
	m.pop.Resize(fyne.NewSize(m.width, m.frame.MinSize().Height))
}

func (m *modal) Confirm() {
	if m.closed || (m.confirm != nil && m.confirm.Disabled()) {
		return
	}
	if m.onConfirm != nil && !m.onConfirm() {
		return
	}
	m.close()
}

func (m *modal) Cancel() {
	if m.closed {
		return
	}
	if m.onCancel != nil {
		m.onCancel()
	}
	m.close()
}

func (m *modal) close() {
	m.closed = true
	tips.pop(m.layer)
	m.pop.Hide()
}

// keyCatcher is an invisible focus target inside a modal, so Enter and Escape
// work when no field has focus.
type keyCatcher struct {
	widget.BaseWidget
	onEnter, onEscape func()
}

func newKeyCatcher(onEnter, onEscape func()) *keyCatcher {
	k := &keyCatcher{onEnter: onEnter, onEscape: onEscape}
	k.ExtendBaseWidget(k)
	return k
}

func (k *keyCatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout())
}
func (k *keyCatcher) MinSize() fyne.Size { return fyne.Size{} }
func (k *keyCatcher) FocusGained()       {}
func (k *keyCatcher) FocusLost()         {}
func (k *keyCatcher) TypedRune(rune)     {}
func (k *keyCatcher) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyEscape:
		k.onEscape()
	case fyne.KeyReturn, fyne.KeyEnter:
		k.onEnter()
	}
}

// labelled stacks a small caption above a control, the form style for every
// dialog.
func labelled(caption string, control fyne.CanvasObject, hint ...string) fyne.CanvasObject {
	objects := []fyne.CanvasObject{newText(caption, t.TextSmall, t.ColorSubtle, styleNormal), control}
	for _, line := range hint {
		_, label := wrapped(line, t.SizeSmall, t.ColorMuted, styleNormal)
		objects = append(objects, label)
	}
	return vbox(4, objects...)
}

// ---------------------------------------------------------------- menus

type menuItem struct {
	label    string
	icon     string
	danger   bool
	disabled bool
	action   func()
}

var menuSeparator = menuItem{}

// showMenu opens a context menu at an absolute position.
func showMenu(win fyne.Window, pos fyne.Position, items ...menuItem) {
	tips.Hide(nil)
	entries := make([]*fyne.MenuItem, 0, len(items))
	for _, item := range items {
		if item.label == "" {
			entries = append(entries, fyne.NewMenuItemSeparator())
			continue
		}
		entry := fyne.NewMenuItem(item.label, item.action)
		if item.icon != "" {
			colorName := t.ColorSubtle
			if item.danger {
				colorName = t.ColorError
			}
			entry.Icon = t.Icon(item.icon, colorName)
		}
		entry.Disabled = item.disabled
		entries = append(entries, entry)
	}
	widget.NewPopUpMenu(fyne.NewMenu("", entries...), win.Canvas()).ShowAtPosition(pos)
}
