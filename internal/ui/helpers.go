package ui

import (
	"fmt"
	"image/color"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/model"
	r2theme "r2go/internal/theme"
)

type themedSurface struct {
	widget.BaseWidget
	colorName  fyne.ThemeColorName
	radius     float32
	padding    float32
	content    fyne.CanvasObject
	background *canvas.Rectangle
}

func surface(colorName fyne.ThemeColorName, radius, padding float32, content fyne.CanvasObject) fyne.CanvasObject {
	s := &themedSurface{
		colorName:  colorName,
		radius:     radius,
		padding:    padding,
		content:    content,
		background: canvas.NewRectangle(fyneTheme.Color(colorName)),
	}
	s.background.CornerRadius = radius
	s.ExtendBaseWidget(s)
	return s
}

func panel(colorName fyne.ThemeColorName, content fyne.CanvasObject) fyne.CanvasObject {
	return surface(colorName, 0, 0, content)
}

func card(colorName fyne.ThemeColorName, content fyne.CanvasObject) fyne.CanvasObject {
	return surface(colorName, 0, 6, content)
}

func (s *themedSurface) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(s.background, inset(s.padding, s.padding, s.padding, s.padding, s.content)))
}

func (s *themedSurface) MinSize() fyne.Size {
	min := s.content.MinSize()
	return fyne.NewSize(min.Width+s.padding*2, min.Height+s.padding*2)
}

func (s *themedSurface) Refresh() {
	s.background.FillColor = fyneTheme.Color(s.colorName)
	s.background.CornerRadius = s.radius
	s.background.Refresh()
	s.BaseWidget.Refresh()
}

type insetLayout struct {
	left, top, right, bottom float32
}

func inset(left, top, right, bottom float32, object fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&insetLayout{left: left, top: top, right: right, bottom: bottom}, object)
}

func (l *insetLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 1 {
		return
	}
	objects[0].Move(fyne.NewPos(l.left, l.top))
	objects[0].Resize(fyne.NewSize(
		fyne.Max(0, size.Width-l.left-l.right),
		fyne.Max(0, size.Height-l.top-l.bottom),
	))
}

func (l *insetLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 1 {
		return fyne.NewSize(l.left+l.right, l.top+l.bottom)
	}
	size := objects[0].MinSize()
	return fyne.NewSize(size.Width+l.left+l.right, size.Height+l.top+l.bottom)
}

func tightVBox(objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&tightVBoxLayout{}, objects...)
}

type tightVBoxLayout struct{}

func (l *tightVBoxLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		h := object.MinSize().Height
		object.Move(fyne.NewPos(0, y))
		object.Resize(fyne.NewSize(size.Width, h))
		y += h
	}
}

func (l *tightVBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width, height float32
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		size := object.MinSize()
		if size.Width > width {
			width = size.Width
		}
		height += size.Height
	}
	return fyne.NewSize(width, height)
}

func fixedSize(width, height float32, object fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(width, height), object)
}

func fixedWidth(width float32, object fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(width, object.MinSize().Height), object)
}

func wideEntry(width float32, entry *widget.Entry) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(width, 34), entry)
}

func mutedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Importance = widget.LowImportance
	label.SizeName = fyneTheme.SizeNameCaptionText
	return label
}

type miniProgress struct {
	widget.BaseWidget
	value float64
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

func newMiniProgress() *miniProgress {
	p := &miniProgress{
		track: canvas.NewRectangle(fyneTheme.Color(r2theme.ColorTransferTrack)),
		fill:  canvas.NewRectangle(fyneTheme.Color(fyneTheme.ColorNamePrimary)),
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *miniProgress) SetValue(value float64) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	p.value = value
	p.Refresh()
}

func (p *miniProgress) MinSize() fyne.Size { return fyne.NewSize(100, 3) }

func (p *miniProgress) CreateRenderer() fyne.WidgetRenderer {
	content := container.New(&miniProgressLayout{owner: p}, p.track, p.fill)
	return widget.NewSimpleRenderer(content)
}

func (p *miniProgress) Refresh() {
	p.track.FillColor = fyneTheme.Color(r2theme.ColorTransferTrack)
	p.fill.FillColor = fyneTheme.Color(fyneTheme.ColorNamePrimary)
	p.track.Refresh()
	p.fill.Refresh()
	p.BaseWidget.Refresh()
}

type miniProgressLayout struct{ owner *miniProgress }

func (l *miniProgressLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	height := fyne.Min(size.Height, 3)
	y := (size.Height - height) / 2
	objects[0].Move(fyne.NewPos(0, y))
	objects[0].Resize(fyne.NewSize(size.Width, height))
	objects[1].Move(fyne.NewPos(0, y))
	objects[1].Resize(fyne.NewSize(size.Width*float32(l.owner.value), height))
}

func (l *miniProgressLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(100, 3)
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)
	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return ""
	}
	return formatSize(int64(bytesPerSecond)) + "/s"
}

func entryIcon(entry model.Entry) fyne.Resource {
	if entry.IsDir {
		return fyneTheme.Icon(fyneTheme.IconNameFolder)
	}
	name := strings.ToLower(entry.Name)
	switch {
	case hasAnySuffix(name, ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico", ".avif"):
		return fyneTheme.Icon(fyneTheme.IconNameFileImage)
	case hasAnySuffix(name, ".mp4", ".webm", ".mov", ".avi", ".mkv", ".m4v"):
		return fyneTheme.Icon(fyneTheme.IconNameFileVideo)
	case hasAnySuffix(name, ".mp3", ".wav", ".flac", ".ogg", ".m4a"):
		return fyneTheme.Icon(fyneTheme.IconNameFileAudio)
	case hasAnySuffix(name, ".txt", ".md", ".log", ".json", ".csv", ".html", ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".yml", ".yaml", ".xml", ".sh", ".ps1", ".toml", ".sql", ".css", ".env"):
		return fyneTheme.Icon(fyneTheme.IconNameFileText)
	case hasAnySuffix(name, ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz", ".exe", ".msi", ".dmg", ".apk", ".deb", ".rpm", ".iso"):
		return fyneTheme.Icon(fyneTheme.IconNameFileApplication)
	case hasAnySuffix(name, ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".pdf"):
		return fyneTheme.Icon(fyneTheme.IconNameDocument)
	default:
		return fyneTheme.Icon(fyneTheme.IconNameFile)
	}
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func publicURL(domain, key string) string {
	if domain == "" {
		return ""
	}
	segments := strings.Split(strings.TrimPrefix(key, "/"), "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return "https://" + strings.TrimSuffix(domain, "/") + "/" + strings.Join(segments, "/")
}

func withAlpha(value color.Color, alpha uint8) color.Color {
	r, g, b, _ := value.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}

func accentColor() color.Color      { return fyneTheme.Color(fyneTheme.ColorNamePrimary) }
func rowSelectedColor() color.Color { return fyneTheme.Color(r2theme.ColorRowSelected) }
func rowHoverColor() color.Color    { return fyneTheme.Color(r2theme.ColorRowHover) }
