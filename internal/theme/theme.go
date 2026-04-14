package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type Theme struct {
	base    fyne.Theme
	variant fyne.ThemeVariant
}

func New(variant fyne.ThemeVariant) *Theme {
	return &Theme{base: theme.DefaultTheme(), variant: variant}
}

func (t *Theme) Variant() fyne.ThemeVariant { return t.variant }

func (t *Theme) SetVariant(v fyne.ThemeVariant) { t.variant = v }

func (t *Theme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if c, ok := palette(t.variant)[name]; ok {
		return c
	}
	return t.base.Color(name, t.variant)
}

func (t *Theme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *Theme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *Theme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

func nrgb(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 0xff} }

func palette(v fyne.ThemeVariant) map[fyne.ThemeColorName]color.Color {
	if v == theme.VariantDark {
		return darkPalette
	}
	return lightPalette
}

var darkPalette = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:       nrgb(0x1b, 0x1d, 0x21),
	theme.ColorNameForeground:       nrgb(0xe6, 0xe7, 0xe9),
	theme.ColorNamePrimary:          nrgb(0xf6, 0x82, 0x1f),
	theme.ColorNameHover:            nrgb(0x2a, 0x2d, 0x33),
	theme.ColorNameFocus:            nrgb(0xf6, 0x82, 0x1f),
	theme.ColorNameSelection:        nrgb(0x3f, 0x3a, 0x33),
	theme.ColorNameInputBackground:  nrgb(0x23, 0x26, 0x2b),
	theme.ColorNameDisabled:         nrgb(0x6b, 0x6f, 0x76),
	theme.ColorNamePlaceHolder:      nrgb(0x8a, 0x8f, 0x98),
	theme.ColorNamePressed:          nrgb(0x3a, 0x3d, 0x45),
	theme.ColorNameScrollBar:        nrgb(0x5a, 0x5f, 0x66),
	theme.ColorNameShadow:           color.NRGBA{R: 0, G: 0, B: 0, A: 0x80},
	theme.ColorNameOverlayBackground: nrgb(0x20, 0x22, 0x27),
	theme.ColorNameMenuBackground:   nrgb(0x23, 0x26, 0x2b),
	theme.ColorNameError:            nrgb(0xe5, 0x48, 0x4d),
	theme.ColorNameSuccess:          nrgb(0x4c, 0xaf, 0x50),
	theme.ColorNameWarning:          nrgb(0xf5, 0xa6, 0x23),
	theme.ColorNameHeaderBackground: nrgb(0x23, 0x26, 0x2b),
	theme.ColorNameSeparator:        nrgb(0x2e, 0x31, 0x36),
	theme.ColorNameButton:           nrgb(0x2c, 0x2f, 0x35),
	theme.ColorNameHyperlink:        nrgb(0x7f, 0xb4, 0xff),
}

var lightPalette = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:        color.White,
	theme.ColorNameForeground:        nrgb(0x1a, 0x1c, 0x1f),
	theme.ColorNamePrimary:           nrgb(0xe0, 0x6a, 0x00),
	theme.ColorNameHover:             nrgb(0xf0, 0xf1, 0xf3),
	theme.ColorNameFocus:             nrgb(0xe0, 0x6a, 0x00),
	theme.ColorNameSelection:         nrgb(0xff, 0xe7, 0xd1),
	theme.ColorNameInputBackground:   nrgb(0xf5, 0xf6, 0xf8),
	theme.ColorNameDisabled:          nrgb(0xb0, 0xb3, 0xb8),
	theme.ColorNamePlaceHolder:       nrgb(0x9a, 0x9f, 0xa6),
	theme.ColorNamePressed:           nrgb(0xe9, 0xea, 0xed),
	theme.ColorNameScrollBar:         nrgb(0xc4, 0xc8, 0xce),
	theme.ColorNameShadow:            color.NRGBA{R: 0, G: 0, B: 0, A: 0x40},
	theme.ColorNameOverlayBackground: nrgb(0xfa, 0xfa, 0xfb),
	theme.ColorNameMenuBackground:    color.White,
	theme.ColorNameError:             nrgb(0xd9, 0x30, 0x25),
	theme.ColorNameSuccess:           nrgb(0x1e, 0x8e, 0x3e),
	theme.ColorNameWarning:           nrgb(0xf9, 0xab, 0x00),
	theme.ColorNameHeaderBackground:  nrgb(0xf5, 0xf6, 0xf8),
	theme.ColorNameSeparator:         nrgb(0xe4, 0xe6, 0xea),
	theme.ColorNameButton:            nrgb(0xf5, 0xf6, 0xf8),
	theme.ColorNameHyperlink:         nrgb(0x1a, 0x5f, 0xb4),
}
