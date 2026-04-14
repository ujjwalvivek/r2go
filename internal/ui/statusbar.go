package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type StatusBar struct {
	Container *fyne.Container

	msg     *widget.Label
	right   *widget.Label
}

func NewStatusBar() *StatusBar {
	s := &StatusBar{}
	s.msg = widget.NewLabel("Ready")
	s.msg.Truncation = fyne.TextTruncateEllipsis
	s.right = widget.NewLabel("")
	s.right.Truncation = fyne.TextTruncateEllipsis
	s.Container = container.NewBorder(nil, nil, nil, nil,
		container.NewHBox(s.msg, layout.NewSpacer(), s.right))
	return s
}

func (s *StatusBar) SetMessage(msg string) {
	s.msg.SetText(msg)
}

func (s *StatusBar) SetRight(text string) {
	s.right.SetText(text)
}
