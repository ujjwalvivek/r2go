package app

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed r2go.png
var iconPNG []byte

func iconResource() fyne.Resource {
	return fyne.NewStaticResource("r2go.png", iconPNG)
}
