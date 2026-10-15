package app

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/config"
	r2theme "r2go/internal/theme"
	"r2go/internal/ui"
)

const appID = "dev.r2go"

func Run() {
	// Every background goroutine hands UI work to fyne.Do, so opt in to
	// Fyne's threading model instead of its compatibility safety net.
	fyneapp.SetMetadata(fyne.AppMetadata{
		ID:         appID,
		Name:       "R2GO",
		Migrations: map[string]bool{"fyneDo": true},
	})
	application := fyneapp.NewWithID(appID)
	cfg, err := config.New()
	if err != nil {
		fyne.LogError("R2GO configuration", err)
		window := application.NewWindow("Startup error")
		title := widget.NewLabelWithStyle("R2GO could not open its configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		detail := widget.NewLabel(err.Error())
		detail.Wrapping = fyne.TextWrapWord
		window.SetContent(container.NewPadded(container.NewVBox(title, detail)))
		window.Resize(fyne.NewSize(620, 220))
		window.CenterOnScreen()
		window.ShowAndRun()
		return
	}
	application.Settings().SetTheme(r2theme.New())
	icon := iconResource()
	application.SetIcon(icon)

	window := ui.NewMainWindow(application, cfg, icon)
	window.SetPadded(false)
	window.SetIcon(icon)
	window.Resize(fyne.NewSize(1360, 840))
	window.CenterOnScreen()
	window.ShowAndRun()
}
