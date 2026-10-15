package theme

import (
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
)

// fontSet holds the system fonts r2go found at startup. Like echohub it
// prefers the platform's own UI and mono families and never bundles fonts;
// any slot left nil falls back to Fyne's bundled Noto.
type fontSet struct {
	regular, bold, italic, mono, monoBold fyne.Resource
}

// Candidate files, best first. Fyne loads fonts by file rather than family
// name, so these are the files behind echohub's family lists: Segoe UI
// (semibold for emphasis, as echohub uses demibold) and Cascadia Mono, then
// Consolas.
var windowsFonts = struct {
	regular, bold, italic, mono, monoBold []string
}{
	regular:  []string{"segoeui.ttf"},
	bold:     []string{"seguisb.ttf", "segoeuib.ttf"},
	italic:   []string{"segoeuii.ttf"},
	mono:     []string{"CascadiaMono.ttf", "consola.ttf"},
	monoBold: []string{"CascadiaMono.ttf", "consolab.ttf"},
}

func loadFonts() fontSet {
	if runtime.GOOS != "windows" {
		return fontSet{}
	}
	dir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
	if os.Getenv("WINDIR") == "" {
		dir = `C:\Windows\Fonts`
	}
	return fontSet{
		regular:  firstFont(dir, windowsFonts.regular),
		bold:     firstFont(dir, windowsFonts.bold),
		italic:   firstFont(dir, windowsFonts.italic),
		mono:     firstFont(dir, windowsFonts.mono),
		monoBold: firstFont(dir, windowsFonts.monoBold),
	}
}

func firstFont(dir string, names []string) fyne.Resource {
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil && len(data) > 0 {
			return fyne.NewStaticResource(name, data)
		}
	}
	return nil
}

func (f fontSet) pick(style fyne.TextStyle) fyne.Resource {
	switch {
	case style.Monospace && style.Bold:
		return f.monoBold
	case style.Monospace:
		return f.mono
	case style.Symbol:
		return nil
	case style.Bold:
		return f.bold
	case style.Italic:
		return f.italic
	default:
		return f.regular
	}
}
