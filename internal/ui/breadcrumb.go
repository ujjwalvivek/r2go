package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Breadcrumb struct {
	Container  *fyne.Container
	onNavigate func(prefix string)
}

func NewBreadcrumb(onNavigate func(prefix string)) *Breadcrumb {
	b := &Breadcrumb{onNavigate: onNavigate}
	b.Container = container.NewHBox()
	return b
}

func (b *Breadcrumb) Set(bucket, prefix string) {
	b.Container.Objects = nil
	if bucket == "" {
		b.Container.Add(widget.NewLabel("No bucket selected"))
		b.Container.Refresh()
		return
	}

	home := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameFolder), func() {
		if b.onNavigate != nil {
			b.onNavigate("")
		}
	})
	home.Importance = widget.LowImportance
	b.Container.Add(home)
	b.Container.Add(sepLabel())

	bucketBtn := widget.NewButtonWithIcon(bucket, fyneTheme.Icon(fyneTheme.IconNameStorage), func() {
		if b.onNavigate != nil {
			b.onNavigate("")
		}
	})
	bucketBtn.Importance = widget.LowImportance
	b.Container.Add(bucketBtn)

	if prefix == "" {
		b.Container.Refresh()
		return
	}

	segments := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
	accum := ""
	for i, seg := range segments {
		accum += seg + "/"
		sep := widget.NewLabel("›")
		sep.Importance = widget.LowImportance
		b.Container.Add(sep)
		if i == len(segments)-1 {
			cur := widget.NewLabelWithStyle(seg, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			b.Container.Add(cur)
		} else {
			btn := widget.NewButton(seg, func() {
				if b.onNavigate != nil {
					b.onNavigate(accum)
				}
			})
			btn.Importance = widget.LowImportance
			b.Container.Add(btn)
		}
	}
	b.Container.Refresh()
}

func sepLabel() *widget.Label {
	l := widget.NewLabel("›")
	l.Importance = widget.LowImportance
	return l
}
