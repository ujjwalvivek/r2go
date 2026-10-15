package theme

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
)

// Line icons, drawn on a 24-unit grid with square caps and mitred joins to
// match the square design. Shown at 14 px in rows, nav items and buttons.
// Each body is SVG markup that inherits stroke and fill from the wrapper, so
// one shape can be produced in any token colour.
const (
	IconFolder      = "folder"
	IconFolderOpen  = "folder-open"
	IconFolderPlus  = "folder-plus"
	IconFile        = "file"
	IconFileImage   = "file-image"
	IconFileVideo   = "file-video"
	IconFileAudio   = "file-audio"
	IconFileText    = "file-text"
	IconFileCode    = "file-code"
	IconFileArchive = "file-archive"
	IconBucket      = "bucket"
	IconCloud       = "cloud"
	IconUpload      = "upload"
	IconDownload    = "download"
	IconRefresh     = "refresh"
	IconSettings    = "settings"
	IconSearch      = "search"
	IconClose       = "close"
	IconChevronR    = "chevron-right"
	IconChevronD    = "chevron-down"
	IconChevronU    = "chevron-up"
	IconArrowUp     = "arrow-up"
	IconArrowLeft   = "arrow-left"
	IconArrowRight  = "arrow-right"
	IconCopy        = "copy"
	IconMove        = "move"
	IconTrash       = "trash"
	IconPencil      = "pencil"
	IconLink        = "link"
	IconExternal    = "external"
	IconDetails     = "details"
	IconGrid        = "grid"
	IconList        = "list"
	IconEye         = "eye"
	IconCheck       = "check"
	IconAlert       = "alert"
	IconMore        = "more"
	IconPlus        = "plus"
	IconHelp        = "help"
	IconGlobe       = "globe"
	IconCommand     = "command"
	IconDashboard   = "dashboard"
	IconFileFont    = "file-font"
	IconFilePDF     = "file-pdf"
)

const fileOutline = `<path d="M6 3h8l5 5v13H6z"/><path d="M14 3v5h5"/>`
const folderOutline = `<path d="M3 5h6l2 2h10v12H3z"/>`

var iconBodies = map[string]string{
	IconFolder:      folderOutline,
	IconFolderOpen:  `<path d="M3 19V5h6l2 2h8v3"/><path d="M3 19l3-9h16l-3 9z"/>`,
	IconFolderPlus:  folderOutline + `<path d="M12 10v6M9 13h6"/>`,
	IconFile:        fileOutline,
	IconFileImage:   fileOutline + `<path d="M8.5 18l2.5-3.5 2 2.5 1.5-1.5 2 2.5"/>`,
	IconFileVideo:   fileOutline + `<path d="M10 12v6l4.5-3z"/>`,
	IconFileAudio:   fileOutline + `<path d="M12.5 17.5V11l3 1"/><circle cx="11" cy="17.5" r="1.5"/>`,
	IconFileText:    fileOutline + `<path d="M9 12h6M9 15h6M9 18h4"/>`,
	IconFileCode:    fileOutline + `<path d="M10.5 12l-2 2.5 2 2.5M13.5 12l2 2.5-2 2.5"/>`,
	IconFileArchive: fileOutline + `<path d="M10 4v1.5M10 7v1.5M10 10v1.5"/><path d="M9 14h2v4H9z"/>`,
	IconBucket:      `<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/>`,
	IconCloud:       `<path d="M7 19a4 4 0 0 1-.6-7.95A6 6 0 0 1 17.7 9.5 4.75 4.75 0 0 1 17 19z"/>`,
	IconUpload:      `<path d="M12 16V4M7 9l5-5 5 5"/><path d="M4 15v5h16v-5"/>`,
	IconDownload:    `<path d="M12 4v12M7 11l5 5 5-5"/><path d="M4 15v5h16v-5"/>`,
	IconRefresh:     `<path d="M20 12a8 8 0 1 1-2.35-5.65L20 8.5"/><path d="M20 3.5v5h-5"/>`,
	IconSettings:    `<path d="M4 7h9M17 7h3M4 17h3M11 17h9"/><path d="M13 5h4v4h-4zM7 15h4v4H7z"/>`,
	IconSearch:      `<circle cx="10.5" cy="10.5" r="6"/><path d="M15 15l5 5"/>`,
	IconClose:       `<path d="M6 6l12 12M18 6L6 18"/>`,
	IconChevronR:    `<path d="M9 5l7 7-7 7"/>`,
	IconChevronD:    `<path d="M5 9l7 7 7-7"/>`,
	IconChevronU:    `<path d="M5 15l7-7 7 7"/>`,
	IconArrowUp:     `<path d="M12 20V5M6 11l6-6 6 6"/>`,
	IconArrowLeft:   `<path d="M20 12H5M11 6l-6 6 6 6"/>`,
	IconArrowRight:  `<path d="M4 12h15M13 6l6 6-6 6"/>`,
	IconCopy:        `<path d="M8 8h12v12H8z"/><path d="M16 8V4H4v12h4"/>`,
	IconMove:        `<path d="M3 5h6l2 2h10v12H3z"/><path d="M8 13h8M13 10l3 3-3 3"/>`,
	IconTrash:       `<path d="M4 7h16M9 7V4h6v3M6 7l1 14h10l1-14"/><path d="M10 11v6M14 11v6"/>`,
	IconPencil:      `<path d="M16 4l4 4L9 19H5v-4z"/><path d="M13 7l4 4"/>`,
	IconLink:        `<path d="M10 14l4-4"/><path d="M8 11l-3 3a3.5 3.5 0 0 0 5 5l3-3"/><path d="M16 13l3-3a3.5 3.5 0 0 0-5-5l-3 3"/>`,
	IconExternal:    `<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 14v6H4V6h6"/>`,
	IconDetails:     `<path d="M3 4h18v16H3z"/><path d="M15 4v16"/>`,
	IconGrid:        `<path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z"/>`,
	IconList:        `<path d="M4 6h16M4 12h16M4 18h16"/>`,
	IconEye:         `<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`,
	IconCheck:       `<path d="M5 12.5l4.5 4.5L19 7"/>`,
	IconAlert:       `<path d="M12 3L2 20h20z"/><path d="M12 10v4.5M12 16.5v1.5"/>`,
	IconMore:        `<path d="M5 11h2.5v2.5H5zM10.75 11h2.5v2.5h-2.5zM16.5 11H19v2.5h-2.5z" fill="currentColor"/>`,
	IconPlus:        `<path d="M12 5v14M5 12h14"/>`,
	IconHelp:        `<path d="M3 3h18v18H3z"/><path d="M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .8-1 1.5v.7M12 16.5V18"/>`,
	IconGlobe:       `<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c2.5 2.5 3.5 5.5 3.5 9s-1 6.5-3.5 9c-2.5-2.5-3.5-5.5-3.5-9s1-6.5 3.5-9z"/>`,
	IconDashboard:   `<path d="M3 3h8v10H3zM13 3h8v6h-8zM13 11h8v10h-8zM3 15h8v6H3z"/>`,
	IconFileFont:    fileOutline + `<path d="M9 18l3-7 3 7M10.2 15.5h3.6"/>`,
	IconFilePDF:     fileOutline + `<path d="M9 18v-5h1.8a1.5 1.5 0 0 1 0 3H9"/>`,
	IconCommand:     `<path d="M9 9h6v6H9z"/><path d="M9 9V6a3 3 0 1 0-3 3h3M15 9V6a3 3 0 1 1 3 3h-3M9 15v3a3 3 0 1 1-3-3h3M15 15v3a3 3 0 1 0 3-3h-3"/>`,
}

var (
	iconMu    sync.Mutex
	iconCache = map[string]fyne.Resource{}
)

// Icon returns the named line icon drawn in a token colour.
func Icon(name string, colorName fyne.ThemeColorName) fyne.Resource {
	return IconRGB(name, Color(colorName))
}

// IconRGB returns the named line icon drawn in an explicit colour.
func IconRGB(name string, c color.Color) fyne.Resource {
	hex := hexColor(c)
	key := name + "-" + hex
	iconMu.Lock()
	defer iconMu.Unlock()
	if res, ok := iconCache[key]; ok {
		return res
	}
	body, ok := iconBodies[name]
	if !ok {
		body = iconBodies[IconFile]
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">`+
		`<g fill="none" stroke="%s" stroke-width="1.75" stroke-linecap="square" stroke-linejoin="miter" color="%s">%s</g></svg>`,
		hex, hex, strings.ReplaceAll(body, "currentColor", hex))
	res := fyne.NewStaticResource("r2go-"+key+".svg", []byte(svg))
	iconCache[key] = res
	return res
}

func hexColor(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
