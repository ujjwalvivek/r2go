package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	t "r2go/internal/theme"
)

// Breadcrumb shows the bucket and folder path. Earlier segments are SUBTLE,
// the current folder is TEXT. When the path is too long, the middle folds into
// an ellipsis that opens a menu of the hidden folders. Clicking the empty
// space after the path switches to an editable path field.
type Breadcrumb struct {
	widget.BaseWidget
	onNavigate func(prefix string)
	onEdit     func()

	bucketIcon *canvas.Image
	crumbs     []*crumb
	targets    []string
	seps       []*canvas.Image
	more       *crumb
	hidden     []int
	win        fyne.Window
}

func NewBreadcrumb(win fyne.Window, onNavigate func(string), onEdit func()) *Breadcrumb {
	b := &Breadcrumb{win: win, onNavigate: onNavigate, onEdit: onEdit, bucketIcon: newIcon(t.IconBucket, t.ColorMuted)}
	b.more = newCrumb("…", false, nil)
	b.more.onTapped = b.showHidden
	b.ExtendBaseWidget(b)
	return b
}

// SetOverview shows "account › Overview" for an account's dashboard.
func (b *Breadcrumb) SetOverview(account string) {
	setIcon(b.bucketIcon, t.IconDashboard, t.ColorMuted)
	b.crumbs = []*crumb{newCrumb(account, false, nil), newCrumb("Overview", true, nil)}
	b.seps = []*canvas.Image{newIcon(t.IconChevronR, t.ColorBorderStrong)}
	b.targets = []string{"", ""}
	b.Refresh()
}

// Set shows bucket/prefix; an empty bucket clears the breadcrumb.
func (b *Breadcrumb) Set(bucket, prefix string) {
	setIcon(b.bucketIcon, t.IconBucket, t.ColorMuted)
	b.crumbs, b.targets, b.seps = nil, nil, nil
	if bucket != "" {
		parts := []string{bucket}
		targets := []string{""}
		accumulated := ""
		for _, part := range strings.Split(strings.TrimSuffix(prefix, "/"), "/") {
			if part == "" {
				continue
			}
			accumulated += part + "/"
			parts = append(parts, part)
			targets = append(targets, accumulated)
		}
		for i, part := range parts {
			target := targets[i]
			last := i == len(parts)-1
			b.crumbs = append(b.crumbs, newCrumb(part, last, func() { b.onNavigate(target) }))
			if i > 0 {
				b.seps = append(b.seps, newIcon(t.IconChevronR, t.ColorBorderStrong))
			}
		}
		b.targets = targets
	}
	b.Refresh()
}

func (b *Breadcrumb) showHidden() {
	items := make([]menuItem, 0, len(b.hidden))
	for _, index := range b.hidden {
		target := b.targets[index]
		items = append(items, menuItem{label: b.crumbs[index].label, icon: t.IconFolder, action: func() { b.onNavigate(target) }})
	}
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(b.more)
	showMenu(b.win, pos.Add(fyne.NewPos(0, b.more.Size().Height)), items...)
}

// Tapped on the breadcrumb's empty space starts editing the path.
func (b *Breadcrumb) Tapped(*fyne.PointEvent) {
	if len(b.crumbs) > 0 && b.onEdit != nil && !b.overview() {
		b.onEdit()
	}
}

func (b *Breadcrumb) MouseIn(event *desktop.MouseEvent) {
	if len(b.crumbs) > 0 && !b.overview() {
		tips.Show(b, "Edit path  ·  Ctrl+L\nType a folder path to jump straight to it", event.AbsolutePosition)
	}
}
func (b *Breadcrumb) MouseMoved(event *desktop.MouseEvent) { tips.Move(b, event.AbsolutePosition) }
func (b *Breadcrumb) MouseOut()                            { tips.Hide(b) }
func (b *Breadcrumb) Cursor() desktop.Cursor {
	if b.overview() {
		return desktop.DefaultCursor
	}
	return desktop.TextCursor
}

// overview reports whether the breadcrumb shows an account's dashboard.
func (b *Breadcrumb) overview() bool {
	return len(b.crumbs) == 2 && b.crumbs[1].label == "Overview" && b.targets[1] == ""
}

func (b *Breadcrumb) MinSize() fyne.Size { return fyne.NewSize(80, t.HeightControl) }

func (b *Breadcrumb) CreateRenderer() fyne.WidgetRenderer { return &breadcrumbRenderer{b: b} }

type breadcrumbRenderer struct {
	b       *Breadcrumb
	objects []fyne.CanvasObject
}

func (r *breadcrumbRenderer) Destroy()                     {}
func (r *breadcrumbRenderer) MinSize() fyne.Size           { return r.b.MinSize() }
func (r *breadcrumbRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *breadcrumbRenderer) Refresh() {
	b := r.b
	r.objects = r.objects[:0]
	if len(b.crumbs) > 0 {
		r.objects = append(r.objects, b.bucketIcon)
	}
	for _, sep := range b.seps {
		r.objects = append(r.objects, sep)
	}
	for _, c := range b.crumbs {
		r.objects = append(r.objects, c)
	}
	r.objects = append(r.objects, b.more)
	r.Layout(b.Size())
	canvas.Refresh(b)
}

// Layout always keeps the bucket and the current folder, then adds folders
// from the end while they fit; anything left out folds into "…".
func (r *breadcrumbRenderer) Layout(size fyne.Size) {
	b := r.b
	for _, object := range r.objects {
		object.Hide()
	}
	b.hidden = b.hidden[:0]
	if len(b.crumbs) == 0 {
		return
	}
	const sepW float32 = 14
	iconW := t.SizeIcon + 6
	widths := make([]float32, len(b.crumbs))
	for i, c := range b.crumbs {
		widths[i] = c.MinSize().Width
	}
	visible := make([]bool, len(b.crumbs))
	visible[0] = true
	visible[len(b.crumbs)-1] = true
	used := iconW + widths[0]
	if len(b.crumbs) > 1 {
		used += sepW + widths[len(b.crumbs)-1]
	}
	moreW := b.more.MinSize().Width + sepW
	for i := len(b.crumbs) - 2; i >= 1; i-- {
		needed := sepW + widths[i]
		reserve := float32(0)
		if i > 1 {
			reserve = moreW
		}
		if used+needed+reserve > size.Width {
			break
		}
		visible[i] = true
		used += needed
	}
	for i := 1; i < len(b.crumbs)-1; i++ {
		if !visible[i] {
			b.hidden = append(b.hidden, i)
		}
	}

	y := func(h float32) float32 { return (size.Height - h) / 2 }
	x := float32(0)
	b.bucketIcon.Move(fyne.NewPos(x, y(t.SizeIcon)))
	b.bucketIcon.Resize(fyne.NewSize(t.SizeIcon, t.SizeIcon))
	b.bucketIcon.Show()
	x += iconW
	place := func(object fyne.CanvasObject, width, height float32) {
		object.Move(fyne.NewPos(x, y(height)))
		object.Resize(fyne.NewSize(width, height))
		object.Show()
		x += width
	}
	sepIndex := 0
	moreShown := false
	for i, c := range b.crumbs {
		if i > 0 {
			if !visible[i] {
				if !moreShown {
					place(b.seps[sepIndex], sepW, 12)
					place(b.more, b.more.MinSize().Width, t.HeightControl)
					moreShown = true
				}
				sepIndex++
				continue
			}
			place(b.seps[sepIndex], sepW, 12)
			sepIndex++
		}
		width := widths[i]
		if x+width > size.Width {
			width = fyne.Max(24, size.Width-x)
		}
		place(c, width, t.HeightControl)
	}
}

// crumb is one path segment: a quiet text button.
type crumb struct {
	widget.BaseWidget
	label    string
	current  bool
	hovered  bool
	bg       *canvas.Rectangle
	text     *textView
	onTapped func()
}

func newCrumb(label string, current bool, tapped func()) *crumb {
	c := &crumb{label: label, current: current, onTapped: tapped, bg: canvas.NewRectangle(color.Transparent)}
	style := styleNormal
	colorName := t.ColorSubtle
	if current {
		style, colorName = styleStrong, t.ColorText
	}
	c.text = newElided(label, t.TextBody, colorName, style)
	c.ExtendBaseWidget(c)
	return c
}

func (c *crumb) MinSize() fyne.Size {
	size := fyne.MeasureText(c.label, t.TextBody, c.text.style)
	return fyne.NewSize(size.Width+12, t.HeightControl)
}

func (c *crumb) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(c.bg, inset(6, 0, 6, 0, c.text)))
}

func (c *crumb) Tapped(*fyne.PointEvent) {
	if c.onTapped != nil {
		tips.Hide(c)
		c.onTapped()
	}
}

func (c *crumb) MouseIn(event *desktop.MouseEvent) {
	c.hovered = true
	c.bg.FillColor = tc(t.ColorSurface)
	c.bg.Refresh()
	if c.text.Elided() {
		tips.Show(c, c.label, event.AbsolutePosition)
	}
}
func (c *crumb) MouseMoved(event *desktop.MouseEvent) { tips.Move(c, event.AbsolutePosition) }
func (c *crumb) MouseOut() {
	c.hovered = false
	c.bg.FillColor = color.Transparent
	c.bg.Refresh()
	tips.Hide(c)
}
