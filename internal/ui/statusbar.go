package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	r2theme "r2go/internal/theme"
)

type StatusBar struct {
	Container fyne.CanvasObject
	message   *widget.Label
	right     *widget.Label
	mu        sync.Mutex
	serial    uint64
	timer     *time.Timer
}

func NewStatusBar() *StatusBar {
	bar := &StatusBar{
		message: mutedLabel("Ready"),
		right:   mutedLabel(""),
	}
	row := container.NewBorder(nil, nil, bar.message, bar.right)
	bar.Container = panel(r2theme.ColorToolbar, container.NewBorder(widget.NewSeparator(), nil, nil, nil, inset(7, 0, 7, 0, row)))
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
