package ui

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	r2theme "r2go/internal/theme"
)

// StatusState is the coloured square at the start of the status bar: foam when
// idle, accent while transfers run, error when one has failed.
type StatusState int

const (
	StatusReady StatusState = iota
	StatusWorking
	StatusFailed
)

type StatusBar struct {
	Container fyne.CanvasObject
	state     *canvas.Rectangle
	message   *widget.Label
	right     *widget.Label
	mu        sync.Mutex
	serial    uint64
	timer     *time.Timer
}

func NewStatusBar() *StatusBar {
	bar := &StatusBar{
		state:   canvas.NewRectangle(color.Transparent),
		message: mutedLabel("Ready"),
		right:   mutedLabel(""),
	}
	left := container.NewHBox(fixedSize(6, 6, bar.state), bar.message)
	hints := container.NewHBox(
		bar.right,
		keyHint("Ctrl+F", "filter"),
		keyHint("Ctrl+U", "upload"),
		keyHint("F5", "refresh"),
		keyHint("Del", "delete"),
	)
	row := container.NewBorder(nil, nil, left, hints)
	bar.Container = panel(r2theme.ColorToolbar, container.NewBorder(widget.NewSeparator(), nil, nil, nil, inset(r2theme.SpaceGap, 0, r2theme.SpaceGap, 0, row)))
	bar.SetState(StatusReady)
	return bar
}

func (s *StatusBar) SetMessage(message string) {
	s.message.SetText(message)
	s.mu.Lock()
	s.serial++
	serial := s.serial
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(5*time.Second, func() {
		s.mu.Lock()
		current := s.serial
		s.mu.Unlock()
		if current != serial {
			return
		}
		fyne.Do(func() { s.message.SetText("Ready") })
	})
	s.mu.Unlock()
}

func (s *StatusBar) SetRight(message string) {
	s.right.SetText(message)
}

// SetState recolours the state square. It is cheap and is called on every
// transfer-panel tick, so the colour also follows a theme change without a
// separate hook.
func (s *StatusBar) SetState(state StatusState) {
	switch state {
	case StatusWorking:
		s.state.FillColor = fyneTheme.Color(fyneTheme.ColorNamePrimary)
	case StatusFailed:
		s.state.FillColor = fyneTheme.Color(fyneTheme.ColorNameError)
	default:
		s.state.FillColor = fyneTheme.Color(fyneTheme.ColorNameSuccess)
	}
	s.state.Refresh()
}

// keyHint is a keycap followed by what the key does, for the status bar.
func keyHint(key, action string) fyne.CanvasObject {
	return container.NewHBox(newKeycap(key), mutedLabel(action))
}

// keycapView is a small mono label on SURFACE with a BORDER_STRONG edge.
type keycapView struct {
	widget.BaseWidget
	fill  *canvas.Rectangle
	edge  *canvas.Rectangle
	label *widget.Label
}

func newKeycap(key string) *keycapView {
	k := &keycapView{
		fill:  canvas.NewRectangle(color.Transparent),
		edge:  canvas.NewRectangle(color.Transparent),
		label: widget.NewLabelWithStyle(key, fyne.TextAlignCenter, fyne.TextStyle{Monospace: true}),
	}
	k.edge.StrokeWidth = 1
	k.label.SizeName = fyneTheme.SizeNameCaptionText
	k.ExtendBaseWidget(k)
	return k
}

func (k *keycapView) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewStack(k.fill, k.edge, inset(r2theme.SpaceRowX, 0, r2theme.SpaceRowX, 0, k.label))
	return widget.NewSimpleRenderer(content)
}

func (k *keycapView) Refresh() {
	k.fill.FillColor = fyneTheme.Color(r2theme.ColorSurface)
	k.edge.StrokeColor = fyneTheme.Color(r2theme.ColorBorderStrong)
	k.fill.Refresh()
	k.edge.Refresh()
	k.BaseWidget.Refresh()
}
