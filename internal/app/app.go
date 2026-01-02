package app

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	fyneTheme "fyne.io/fyne/v2/theme"

	"r2go/internal/config"
	"r2go/internal/theme"
	"r2go/internal/ui"
)

const appID = "dev.r2go"

func Run() {
	f := fyneapp.NewWithID(appID)

	cfg, err := config.New()
	if err != nil {
		fyne.LogError("config", err)
	}

	th := theme.New(themeVariant(cfg.Settings().Theme))
	f.Settings().SetTheme(th)

	win := ui.NewMainWindow(f, cfg, th)
	win.Resize(fyne.NewSize(1280, 800))
	win.CenterOnScreen()
	win.ShowAndRun()
}

func themeVariant(v string) fyne.ThemeVariant {
	if v == "light" {
		return fyneTheme.VariantLight
	}
	return fyneTheme.VariantDark
}
