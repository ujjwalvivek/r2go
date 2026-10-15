package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	t "r2go/internal/theme"
)

// StatusState is the coloured square at the start of the status bar: green
// when ready, accent while work runs, red when something failed.
type StatusState int

const (
	StatusReady StatusState = iota
	StatusWorking
	StatusFailed
)

// StatusBar is the 26 px strip at the bottom: state square, message, the
// current location, then folder totals and key hints on the right.
type StatusBar struct {
	Container fyne.CanvasObject
	state     *canvas.Rectangle
	message   *textView
	source    *textView
	totals    *textView
	line      *fyne.Container
	row       *fyne.Container
	mu        sync.Mutex
	serial    uint64
	timer     *time.Timer
	current   StatusState
	failure   string
}

func NewStatusBar() *StatusBar {
	bar := &StatusBar{
		state:   square(6, t.ColorSuccess),
		message: newElided("Ready", t.TextSmall, t.ColorSubtle, styleNormal),
		source:  newElided("", t.TextSmall, t.ColorMuted, styleNormal),
		totals:  newTextView("", t.TextSmall, t.ColorMuted, styleMono),
	}
	hints := container.New(&rowLayout{gap: 12},
		bar.totals,
		keyHint("Ctrl+K", "jump"),
		keyHint("Ctrl+F", "filter"),
		keyHint("?", "keys"),
	)
	message := withTip(bar.message, func() string {
		if bar.failure != "" {
			return bar.failure
		}
		return elidedTip(bar.message)()
	})
	bar.line = container.New(&statusLeftLayout{}, message, vRule(t.ColorBorderStrong), withTip(bar.source, elidedTip(bar.source)))
	left := container.NewBorder(nil, nil, container.NewCenter(bar.state), nil, bar.line)
	bar.row = container.NewBorder(nil, nil, nil, hints, inset(8, 0, 12, 0, left))
	// The top rule takes a pixel; the row gives two back at the bottom so its
	// contents sit on the bar's true centre.
	body := container.NewBorder(hRule(t.ColorBorder), nil, nil, nil, inset(t.SpaceCardX, 0, t.SpaceCardX, 2, bar.row))
	bar.Container = minHeight(t.HeightStatus, panelBox(t.ColorPanel, body))
	return bar
}

// SetMessage shows a transient message that falls back to "Ready" after a
// few seconds.
func (s *StatusBar) SetMessage(message string) { s.show(message, 6*time.Second) }

func (s *StatusBar) show(message string, hold time.Duration) {
	s.message.SetText(message)
	s.message.SetColorName(t.ColorSubtle)
	s.failure = ""
	s.line.Refresh()
	s.mu.Lock()
	s.serial++
	serial := s.serial
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(hold, func() {
		s.mu.Lock()
		current := s.serial
		s.mu.Unlock()
		if current != serial {
			return
		}
		fyne.Do(func() {
			s.message.SetText("Ready")
			s.message.SetColorName(t.ColorSubtle)
			s.failure = ""
			s.line.Refresh()
		})
	})
	s.mu.Unlock()
}

// SetFailure shows a plain-words failure; the raw error goes in the tooltip.
func (s *StatusBar) SetFailure(e explained) {
	s.show(e.Title+" · "+e.Detail, 20*time.Second)
	s.message.SetColorName(t.ColorError)
	s.failure = e.Title + "\n" + e.Raw
}

func (s *StatusBar) SetSource(value string) { s.source.SetText(value); s.line.Refresh() }
func (s *StatusBar) SetTotals(value string) { s.totals.SetText(value); s.row.Refresh() }

// SetState recolours the state square.
func (s *StatusBar) SetState(state StatusState) {
	if s.current == state {
		return
	}
	s.current = state
	switch state {
	case StatusWorking:
		s.state.FillColor = tc(t.ColorAccent)
	case StatusFailed:
		s.state.FillColor = tc(t.ColorError)
	default:
		s.state.FillColor = tc(t.ColorSuccess)
	}
	s.state.Refresh()
}

// statusLeftLayout gives the message its natural width (up to 60% of the
// row), then a divider and the source, which takes what is left.
type statusLeftLayout struct{}

func (statusLeftLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	message, divider, source := objects[0], objects[1], objects[2]
	natural := fyne.MeasureText(message.(*tipBox).content.(*textView).Text(), t.TextSmall, styleNormal).Width + 2
	width := fyne.Min(natural, size.Width*0.6)
	message.Move(fyne.NewPos(0, 0))
	message.Resize(fyne.NewSize(width, size.Height))
	if source.(*tipBox).content.(*textView).Text() == "" {
		divider.Hide()
		return
	}
	divider.Show()
	divider.Move(fyne.NewPos(width+10, (size.Height-12)/2))
	divider.Resize(fyne.NewSize(1, 12))
	source.Move(fyne.NewPos(width+21, 0))
	source.Resize(fyne.NewSize(fyne.Max(0, size.Width-width-21), size.Height))
}

func (statusLeftLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(40, fyne.MeasureText("M", t.TextSmall, styleNormal).Height)
}
