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
	onNavigate func(string)
}

func NewBreadcrumb(onNavigate func(string)) *Breadcrumb {
	return &Breadcrumb{Container: container.NewHBox(), onNavigate: onNavigate}
}

func (b *Breadcrumb) Set(bucket, prefix string) {
	objects := []fyne.CanvasObject{}
	bucketTarget := ""
	bucketButton := widget.NewButtonWithIcon(bucket, fyneTheme.Icon(fyneTheme.IconNameStorage), func() {
		b.onNavigate(bucketTarget)
	})
	bucketButton.Importance = widget.LowImportance
	objects = append(objects, bucketButton)

	if prefix != "" {
		accumulated := ""
		parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
		for _, part := range parts {
			if part == "" {
				continue
			}
			objects = append(objects, mutedLabel("›"))
			accumulated += part + "/"
			target := accumulated
			button := widget.NewButton(part, func() { b.onNavigate(target) })
			button.Importance = widget.LowImportance
			objects = append(objects, button)
		}
	}
	b.Container.Objects = objects
	b.Container.Refresh()
}

func (b *Breadcrumb) Clear() {
	b.Container.Objects = nil
	b.Container.Refresh()
}
