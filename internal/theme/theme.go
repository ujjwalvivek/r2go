package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	fyneTheme "fyne.io/fyne/v2/theme"
)

const (
	ColorSurface       fyne.ThemeColorName = "r2go.surface"
	ColorSurfaceRaised fyne.ThemeColorName = "r2go.surfaceRaised"
	ColorSidebar       fyne.ThemeColorName = "r2go.sidebar"
	ColorToolbar       fyne.ThemeColorName = "r2go.toolbar"
	ColorTableHeader   fyne.ThemeColorName = "r2go.tableHeader"
	ColorRowHover      fyne.ThemeColorName = "r2go.rowHover"
	ColorRowSelected   fyne.ThemeColorName = "r2go.rowSelected"
	ColorMuted         fyne.ThemeColorName = "r2go.muted"
	ColorAccentSoft    fyne.ThemeColorName = "r2go.accentSoft"
	ColorSelectionBar  fyne.ThemeColorName = "r2go.selectionBar"
	ColorTransferTrack fyne.ThemeColorName = "r2go.transferTrack"
	ColorBorder        fyne.ThemeColorName = "r2go.border"
)

type Theme struct {
	base    fyne.Theme
	variant fyne.ThemeVariant
}

func New(variant fyne.ThemeVariant) *Theme {
	return &Theme{base: fyneTheme.DefaultTheme(), variant: variant}
}

func (t *Theme) Variant() fyne.ThemeVariant           { return t.variant }
func (t *Theme) SetVariant(variant fyne.ThemeVariant) { t.variant = variant }

func (t *Theme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if value, ok := palette(t.variant)[name]; ok {
		return value
	}
	return t.base.Color(name, t.variant)
}

func (t *Theme) Font(style fyne.TextStyle) fyne.Resource    { return t.base.Font(style) }
func (t *Theme) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }

func (t *Theme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case fyneTheme.SizeNamePadding:
		return 2
	case fyneTheme.SizeNameInnerPadding:
		return 3
	case fyneTheme.SizeNameText:
		return 13.5
	case fyneTheme.SizeNameCaptionText:
		return 11.5
	case fyneTheme.SizeNameHeadingText:
		return 20
	case fyneTheme.SizeNameSubHeadingText:
		return 15
	case fyneTheme.SizeNameInlineIcon:
		return 18
	case fyneTheme.SizeNameInputRadius:
		return 4
	case fyneTheme.SizeNameSelectionRadius:
		return 2
	case fyneTheme.SizeNameButtonRadius:
		return 4
	case fyneTheme.SizeNameCardRadius:
		return 5
	case fyneTheme.SizeNameDialogRadius:
		return 6
	case fyneTheme.SizeNamePopupRadius, fyneTheme.SizeNameMenuRadius:
		return 4
	case fyneTheme.SizeNameInputBorder:
		return 1
	case fyneTheme.SizeNameSeparatorThickness:
		return 1
	case fyneTheme.SizeNameSplitThickness:
		return 1
	case fyneTheme.SizeNameScrollBar:
		return 8
	case fyneTheme.SizeNameScrollBarSmall:
		return 4
	case fyneTheme.SizeNameScrollBarRadius:
		return 2
	case fyneTheme.SizeNameInnerWindowRadius:
		return 5
	case fyneTheme.SizeNameModalBlurRadius:
		return 2
	}
	return t.base.Size(name)
}

func RGB(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

// The dark variant deliberately leans toward OLED / AMOLED blacks. Layers are
// separated by very small luminance steps instead of large floating cards.
var dark = map[fyne.ThemeColorName]color.Color{
	fyneTheme.ColorNameBackground:                RGB(0x000000),
	fyneTheme.ColorNameForeground:                RGB(0xE7E9EC),
	fyneTheme.ColorNamePrimary:                   RGB(0xFF7A1A),
	fyneTheme.ColorNameButton:                    RGB(0x11151A),
	fyneTheme.ColorNameDisabledButton:            RGB(0x07090C),
	fyneTheme.ColorNameInputBackground:           RGB(0x0B0E12),
	fyneTheme.ColorNameInputBorder:               RGB(0x272D35),
	fyneTheme.ColorNameHover:                     RGB(0x151A20),
	fyneTheme.ColorNamePressed:                   RGB(0x1C232B),
	fyneTheme.ColorNameFocus:                     RGB(0xFF7A1A),
	fyneTheme.ColorNameSelection:                 RGB(0x2A1A10),
	fyneTheme.ColorNamePlaceHolder:               RGB(0x747C87),
	fyneTheme.ColorNameDisabled:                  RGB(0x626A74),
	fyneTheme.ColorNameSeparator:                 RGB(0x1C2025),
	fyneTheme.ColorNameShadow:                    color.NRGBA{A: 0xB0},
	fyneTheme.ColorNameOverlayBackground:         RGB(0x07090C),
	fyneTheme.ColorNameMenuBackground:            RGB(0x080A0D),
	fyneTheme.ColorNameHeaderBackground:          RGB(0x07090C),
	fyneTheme.ColorNameScrollBar:                 RGB(0x444C57),
	fyneTheme.ColorNameScrollBarBackground:       RGB(0x07090C),
	fyneTheme.ColorNameInnerWindowBorder:         RGB(0x2A3038),
	fyneTheme.ColorNameInnerWindowBorderInactive: RGB(0x1C2025),
	fyneTheme.ColorNameError:                     RGB(0xFF5D69),
	fyneTheme.ColorNameWarning:                   RGB(0xFFB454),
	fyneTheme.ColorNameSuccess:                   RGB(0x56D38B),
	fyneTheme.ColorNameHyperlink:                 RGB(0x7AAEFF),
	ColorSurface:                                 RGB(0x020304),
	ColorSurfaceRaised:                           RGB(0x07090B),
	ColorSidebar:                                 RGB(0x030405),
	ColorToolbar:                                 RGB(0x07090C),
	ColorTableHeader:                             RGB(0x07090C),
	ColorRowHover:                                RGB(0x0B0E11),
	ColorRowSelected:                             RGB(0x17100B),
	ColorMuted:                                   RGB(0x818894),
	ColorAccentSoft:                              RGB(0x261409),
	ColorSelectionBar:                            RGB(0x080A0D),
	ColorTransferTrack:                           RGB(0x151A20),
	ColorBorder:                                  RGB(0x1A1F25),
}

var light = map[fyne.ThemeColorName]color.Color{
	fyneTheme.ColorNameBackground:                RGB(0xF3F5F7),
	fyneTheme.ColorNameForeground:                RGB(0x20242A),
	fyneTheme.ColorNamePrimary:                   RGB(0xE8660A),
	fyneTheme.ColorNameButton:                    RGB(0xFFFFFF),
	fyneTheme.ColorNameDisabledButton:            RGB(0xECEFF2),
	fyneTheme.ColorNameInputBackground:           RGB(0xFFFFFF),
	fyneTheme.ColorNameInputBorder:               RGB(0xD4D9E0),
	fyneTheme.ColorNameHover:                     RGB(0xE9EDF2),
	fyneTheme.ColorNamePressed:                   RGB(0xDEE4EA),
	fyneTheme.ColorNameFocus:                     RGB(0xE8660A),
	fyneTheme.ColorNameSelection:                 RGB(0xFFE7D5),
	fyneTheme.ColorNamePlaceHolder:               RGB(0x777F89),
	fyneTheme.ColorNameDisabled:                  RGB(0x999FA8),
	fyneTheme.ColorNameSeparator:                 RGB(0xDDE1E6),
	fyneTheme.ColorNameShadow:                    color.NRGBA{A: 0x24},
	fyneTheme.ColorNameOverlayBackground:         RGB(0xFFFFFF),
	fyneTheme.ColorNameMenuBackground:            RGB(0xFFFFFF),
	fyneTheme.ColorNameHeaderBackground:          RGB(0xF8F9FA),
	fyneTheme.ColorNameScrollBar:                 RGB(0x9CA4AF),
	fyneTheme.ColorNameScrollBarBackground:       RGB(0xF1F3F5),
	fyneTheme.ColorNameInnerWindowBorder:         RGB(0xCDD2D8),
	fyneTheme.ColorNameInnerWindowBorderInactive: RGB(0xE2E5E9),
	fyneTheme.ColorNameError:                     RGB(0xD83A4B),
	fyneTheme.ColorNameWarning:                   RGB(0xB66A06),
	fyneTheme.ColorNameSuccess:                   RGB(0x258A50),
	fyneTheme.ColorNameHyperlink:                 RGB(0x2367D1),
	ColorSurface:                                 RGB(0xFFFFFF),
	ColorSurfaceRaised:                           RGB(0xF8F9FB),
	ColorSidebar:                                 RGB(0xF7F8FA),
	ColorToolbar:                                 RGB(0xFAFBFC),
	ColorTableHeader:                             RGB(0xF4F6F8),
	ColorRowHover:                                RGB(0xEFF2F5),
	ColorRowSelected:                             RGB(0xFFF0E5),
	ColorMuted:                                   RGB(0x6F7680),
	ColorAccentSoft:                              RGB(0xFFE4D0),
	ColorSelectionBar:                            RGB(0xF7F8FA),
	ColorTransferTrack:                           RGB(0xE5E8ED),
	ColorBorder:                                  RGB(0xDDE1E6),
}

func palette(variant fyne.ThemeVariant) map[fyne.ThemeColorName]color.Color {
	if variant == fyneTheme.VariantLight {
		return light
	}
	return dark
}
