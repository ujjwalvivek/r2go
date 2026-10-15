// Package theme holds r2go's design tokens: colours, type sizes, spacing,
// fonts and line icons. It follows the echohub design system (square,
// compact, dark, calm) with r2go's own near-black surfaces and Cloudflare
// orange accent. Widgets read these tokens; they never use literal colours.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	fyneTheme "fyne.io/fyne/v2/theme"
)

// Colour tokens. Names match the echohub palette table.
const (
	ColorBase         fyne.ThemeColorName = "r2go.base"         // window and page background
	ColorPanel        fyne.ThemeColorName = "r2go.panel"        // sidebar, toolbar, status bar, cards
	ColorSurface      fyne.ThemeColorName = "r2go.surface"      // hover, tooltips, keycaps
	ColorSelected     fyne.ThemeColorName = "r2go.selected"     // selected rows and nav items
	ColorBorder       fyne.ThemeColorName = "r2go.border"       // card and divider lines
	ColorBorderStrong fyne.ThemeColorName = "r2go.borderStrong" // window edge, ghost buttons, header rule
	ColorText         fyne.ThemeColorName = "r2go.text"         // primary text
	ColorSubtle       fyne.ThemeColorName = "r2go.subtle"       // secondary text
	ColorMuted        fyne.ThemeColorName = "r2go.muted"        // tertiary text, captions
	ColorAccent       fyne.ThemeColorName = "r2go.accent"       // primary action, selection bar
	ColorOnAccent     fyne.ThemeColorName = "r2go.onAccent"     // text on an accent fill
	ColorSuccess      fyne.ThemeColorName = "r2go.success"      // ready, done
	ColorWarning      fyne.ThemeColorName = "r2go.warning"      // needs a look
	ColorError        fyne.ThemeColorName = "r2go.error"        // broken, destructive
)

// Type scale, in Fyne units. Use these as a widget's SizeName or with Size().
const (
	SizeCaption fyne.ThemeSizeName = "r2go.caption" // pills, keycaps, column headers, hints
	SizeSmall   fyne.ThemeSizeName = "r2go.small"   // meta, status bar, captions
	SizeBody    fyne.ThemeSizeName = "r2go.body"    // default text
	SizeMedium  fyne.ThemeSizeName = "r2go.medium"  // emphasis, panel titles
	SizeDisplay fyne.ThemeSizeName = "r2go.display" // page titles, headline numbers
)

const (
	TextCaption float32 = 10
	TextSmall   float32 = 11
	TextBody    float32 = 12
	TextMedium  float32 = 13
	TextDisplay float32 = 18
)

// Spacing tokens, named and valued after the echohub spacing table.
const (
	SpaceGap   float32 = 8  // between blocks
	SpacePageX float32 = 16 // page margins
	SpacePageY float32 = 12
	SpaceCardX float32 = 10 // card padding
	SpaceCardY float32 = 8
	SpaceRowX  float32 = 6 // list row padding
	SpaceRowY  float32 = 3
)

// Fixed heights for the shell, so every bar lines up.
const (
	HeightBar       float32 = 36 // toolbar and sidebar header
	HeightStatus    float32 = 26
	HeightRow       float32 = 26 // table rows
	HeightHeader    float32 = 28 // table header
	HeightNavItem   float32 = 28
	HeightControl   float32 = 26 // buttons and inputs
	SizeIcon        float32 = 14 // line icons in rows, nav and buttons
	SizeIconButton  float32 = 26
	WidthSidebar    float32 = 240
	WidthDetails    float32 = 300
	WidthAccentBar  float32 = 2
	ProgressLineH   float32 = 2
	TooltipMaxWidth float32 = 320
)

// Theme is r2go's only theme. The app is dark by design, so there is no
// light variant to switch to.
type Theme struct {
	base  fyne.Theme
	fonts fontSet
}

func New() *Theme {
	return &Theme{base: fyneTheme.DefaultTheme(), fonts: loadFonts()}
}

func (t *Theme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if value, ok := palette[name]; ok {
		return value
	}
	return t.base.Color(name, fyneTheme.VariantDark)
}

func (t *Theme) Font(style fyne.TextStyle) fyne.Resource {
	if font := t.fonts.pick(style); font != nil {
		return font
	}
	return t.base.Font(style)
}

func (t *Theme) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }

func (t *Theme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case SizeCaption:
		return TextCaption
	case SizeSmall, fyneTheme.SizeNameCaptionText:
		return TextSmall
	case SizeBody, fyneTheme.SizeNameText:
		return TextBody
	case SizeMedium, fyneTheme.SizeNameSubHeadingText:
		return TextMedium
	case SizeDisplay, fyneTheme.SizeNameHeadingText:
		return TextDisplay
	case fyneTheme.SizeNamePadding:
		return 2
	case fyneTheme.SizeNameInnerPadding:
		return 5
	case fyneTheme.SizeNameLineSpacing:
		return 3
	case fyneTheme.SizeNameInlineIcon:
		return SizeIcon
	// Square corners everywhere, following the echohub design system.
	case fyneTheme.SizeNameInputRadius, fyneTheme.SizeNameSelectionRadius,
		fyneTheme.SizeNameButtonRadius, fyneTheme.SizeNameCardRadius,
		fyneTheme.SizeNameDialogRadius, fyneTheme.SizeNamePopupRadius,
		fyneTheme.SizeNameMenuRadius, fyneTheme.SizeNameScrollBarRadius,
		fyneTheme.SizeNameInnerWindowRadius:
		return 0
	case fyneTheme.SizeNameInputBorder:
		return 1
	case fyneTheme.SizeNameSeparatorThickness:
		return 1
	case fyneTheme.SizeNameSplitThickness:
		return 1
	case fyneTheme.SizeNameScrollBar:
		return 6
	case fyneTheme.SizeNameScrollBarSmall:
		return 3
	case fyneTheme.SizeNameModalBlurRadius:
		return 0
	}
	return t.base.Size(name)
}

// Color returns a token's colour from the r2go palette.
func Color(name fyne.ThemeColorName) color.Color {
	if value, ok := palette[name]; ok {
		return value
	}
	return fyneTheme.Color(name)
}

func RGB(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

// WithAlpha returns c with its alpha replaced.
func WithAlpha(c color.Color, alpha uint8) color.NRGBA {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}

// Near-black AMOLED layers, separated by small luminance steps, with one
// warm accent. Selection is tinted toward the accent.
var (
	base         = RGB(0x000000)
	panel        = RGB(0x0A0B0D)
	surface      = RGB(0x15171B)
	selected     = RGB(0x231811)
	border       = RGB(0x1A1C21)
	borderStrong = RGB(0x2C3037)
	text         = RGB(0xE8E9EB)
	subtle       = RGB(0xA0A6AE)
	muted        = RGB(0x6B717A)
	accent       = RGB(0xF6821F)
	success      = RGB(0x4CC38A)
	warning      = RGB(0xF2B453)
	errorColor   = RGB(0xF0565D)
)

var palette = map[fyne.ThemeColorName]color.Color{
	ColorBase:         base,
	ColorPanel:        panel,
	ColorSurface:      surface,
	ColorSelected:     selected,
	ColorBorder:       border,
	ColorBorderStrong: borderStrong,
	ColorText:         text,
	ColorSubtle:       subtle,
	ColorMuted:        muted,
	ColorAccent:       accent,
	ColorOnAccent:     base,
	ColorSuccess:      success,
	ColorWarning:      warning,
	ColorError:        errorColor,

	fyneTheme.ColorNameBackground:                base,
	fyneTheme.ColorNameForeground:                text,
	fyneTheme.ColorNamePrimary:                   accent,
	fyneTheme.ColorNameForegroundOnPrimary:       base,
	fyneTheme.ColorNameForegroundOnError:         base,
	fyneTheme.ColorNameForegroundOnSuccess:       base,
	fyneTheme.ColorNameForegroundOnWarning:       base,
	fyneTheme.ColorNameButton:                    surface,
	fyneTheme.ColorNameDisabledButton:            panel,
	fyneTheme.ColorNameInputBackground:           base,
	fyneTheme.ColorNameInputBorder:               borderStrong,
	fyneTheme.ColorNameHover:                     surface,
	fyneTheme.ColorNamePressed:                   selected,
	fyneTheme.ColorNameFocus:                     selected, // menu highlight, focus rings
	fyneTheme.ColorNameSelection:                 RGB(0x5A3312),
	fyneTheme.ColorNamePlaceHolder:               muted,
	fyneTheme.ColorNameDisabled:                  RGB(0x4A4F57),
	fyneTheme.ColorNameSeparator:                 border,
	fyneTheme.ColorNameShadow:                    color.NRGBA{A: 0x90},
	fyneTheme.ColorNameOverlayBackground:         panel,
	fyneTheme.ColorNameMenuBackground:            panel,
	fyneTheme.ColorNameHeaderBackground:          panel,
	fyneTheme.ColorNameScrollBar:                 RGB(0x3A3F47),
	fyneTheme.ColorNameScrollBarBackground:       color.Transparent,
	fyneTheme.ColorNameInnerWindowBorder:         borderStrong,
	fyneTheme.ColorNameInnerWindowBorderInactive: border,
	fyneTheme.ColorNameError:                     errorColor,
	fyneTheme.ColorNameWarning:                   warning,
	fyneTheme.ColorNameSuccess:                   success,
	fyneTheme.ColorNameHyperlink:                 accent,
}
