package ui

import (
	"fmt"
	"image/color"
	"net/url"
	"path"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"r2go/internal/model"
	t "r2go/internal/theme"
)

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

func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()+0.5))
	case d < time.Hour:
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// formatAge is a short relative time, like echohub's age column.
func formatAge(when time.Time, now time.Time) string {
	if when.IsZero() {
		return ""
	}
	d := now.Sub(when)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case when.Year() == now.Year():
		return when.Local().Format("02 Jan")
	default:
		return when.Local().Format("02 Jan 2006")
	}
}

// ageColor fades with age: TEXT under a day, SUBTLE under 30 days, MUTED after.
func ageColor(when time.Time, now time.Time) fyne.ThemeColorName {
	d := now.Sub(when)
	switch {
	case when.IsZero():
		return t.ColorMuted
	case d < 24*time.Hour:
		return t.ColorText
	case d < 30*24*time.Hour:
		return t.ColorSubtle
	default:
		return t.ColorMuted
	}
}

func formatTimestamp(when time.Time) string {
	if when.IsZero() {
		return "—"
	}
	return when.Local().Format("02 Jan 2006, 15:04:05")
}

// fileKind groups file names by what they hold, for icons, type pills and
// previews.
type fileKind int

const (
	kindOther fileKind = iota
	kindImage
	kindVideo
	kindAudio
	kindText
	kindCode
	kindArchive
	kindDocument
	kindFont
)

// kindNames and kindColors label and colour the kinds in the dashboard's type
// breakdown.
var kindNames = map[fileKind]string{
	kindImage: "Images", kindVideo: "Video", kindAudio: "Audio", kindText: "Text",
	kindCode: "Code and data", kindArchive: "Archives", kindDocument: "Documents",
	kindFont: "Fonts", kindOther: "Other",
}

var kindColors = map[fileKind]color.Color{
	kindImage:    t.RGB(0xF2B453),
	kindVideo:    t.RGB(0xF778BA),
	kindAudio:    t.RGB(0xB392F0),
	kindText:     t.RGB(0xA0A6AE),
	kindCode:     t.RGB(0x4CC38A),
	kindArchive:  t.RGB(0x6CB6FF),
	kindDocument: t.RGB(0xF0565D),
	kindFont:     t.RGB(0xD2A8FF),
	kindOther:    t.RGB(0x4A4F57),
}

// langColors tint a file's icon and type badge by format, in the colours
// people know them by, brightened where needed to read on near-black.
var langColors = map[string]color.Color{
	"js": t.RGB(0xF1E05A), "mjs": t.RGB(0xF1E05A), "jsx": t.RGB(0xF1E05A),
	"ts": t.RGB(0x4B9BEB), "tsx": t.RGB(0x4B9BEB),
	"py":   t.RGB(0x5A9FD4),
	"go":   t.RGB(0x00ADD8),
	"rs":   t.RGB(0xDEA584),
	"json": t.RGB(0xCBCB41),
	"yml":  t.RGB(0xE06C75), "yaml": t.RGB(0xE06C75), "toml": t.RGB(0xC2894F),
	"html": t.RGB(0xE96B4C), "htm": t.RGB(0xE96B4C), "xml": t.RGB(0xE96B4C),
	"css": t.RGB(0x9B7BD4),
	"md":  t.RGB(0x8FA6BF),
	"sh":  t.RGB(0x89E051), "ps1": t.RGB(0x5391FE),
	"sql": t.RGB(0xE38C00),
	"csv": t.RGB(0x3FB950), "tsv": t.RGB(0x3FB950), "xls": t.RGB(0x3FB950), "xlsx": t.RGB(0x3FB950),
	"pdf": t.RGB(0xF0565D),
	"doc": t.RGB(0x5A9FD4), "docx": t.RGB(0x5A9FD4),
	"ppt": t.RGB(0xE96B4C), "pptx": t.RGB(0xE96B4C),
	"woff": t.RGB(0xD2A8FF), "woff2": t.RGB(0xD2A8FF), "ttf": t.RGB(0xD2A8FF), "otf": t.RGB(0xD2A8FF), "eot": t.RGB(0xD2A8FF),
	"zip": t.RGB(0x6CB6FF), "gz": t.RGB(0x6CB6FF), "tar": t.RGB(0x6CB6FF), "7z": t.RGB(0x6CB6FF), "rar": t.RGB(0x6CB6FF),
	"wasm": t.RGB(0x8A7CF0),
	"svg":  t.RGB(0xFFB13B), "ico": t.RGB(0xF2B453),
}

var kindsByExt = map[string]fileKind{}

func init() {
	register := func(kind fileKind, exts ...string) {
		for _, ext := range exts {
			kindsByExt[ext] = kind
		}
	}
	register(kindImage, "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif", "tif", "tiff", "heic")
	register(kindVideo, "mp4", "webm", "mov", "avi", "mkv", "m4v")
	register(kindAudio, "mp3", "wav", "flac", "ogg", "m4a", "aac", "opus")
	register(kindText, "txt", "md", "log", "csv", "tsv", "rtf")
	register(kindCode, "json", "html", "htm", "go", "py", "js", "mjs", "ts", "tsx", "jsx", "yml", "yaml", "xml",
		"sh", "ps1", "toml", "sql", "css", "env", "rs", "c", "h", "cpp", "java", "rb", "php", "lua", "wasm")
	register(kindArchive, "zip", "tar", "gz", "bz2", "xz", "7z", "rar", "tgz", "zst", "exe", "msi", "dmg", "apk", "deb", "rpm", "iso")
	register(kindDocument, "doc", "docx", "xls", "xlsx", "ppt", "pptx", "pdf", "odt", "ods", "odp", "epub")
	register(kindFont, "woff", "woff2", "ttf", "otf", "eot")
}

func fileExt(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	if len(ext) > 6 {
		return ""
	}
	return ext
}

func kindOf(name string) fileKind { return kindsByExt[fileExt(name)] }

func entryIconName(entry model.Entry) string {
	if entry.IsDir {
		return t.IconFolder
	}
	switch kindOf(entry.Name) {
	case kindImage:
		return t.IconFileImage
	case kindVideo:
		return t.IconFileVideo
	case kindAudio:
		return t.IconFileAudio
	case kindText:
		return t.IconFileText
	case kindDocument:
		if fileExt(entry.Name) == "pdf" {
			return t.IconFilePDF
		}
		return t.IconFileText
	case kindFont:
		return t.IconFileFont
	case kindCode:
		return t.IconFileCode
	case kindArchive:
		return t.IconFileArchive
	default:
		return t.IconFile
	}
}

// entryIcon is the entry's line icon in its colour: accent for folders, the
// format's colour when it has one, MUTED otherwise.
func entryIcon(entry model.Entry) fyne.Resource {
	return t.IconRGB(entryIconName(entry), entryColor(entry))
}

func entryColor(entry model.Entry) color.Color {
	if entry.IsDir {
		return t.Color(t.ColorAccent)
	}
	if c, ok := langColors[fileExt(entry.Name)]; ok {
		return c
	}
	return t.Color(t.ColorMuted)
}

// typeLabel is the lowercase extension shown in the type pill; folders have none.
func typeLabel(entry model.Entry) string {
	if entry.IsDir {
		return ""
	}
	return fileExt(entry.Name)
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

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func countNoun(count int, noun string) string {
	return fmt.Sprintf("%s %s%s", formatCount(count), noun, plural(count))
}

// joinDot joins non-empty parts with the middle-dot separator used across
// the app's meta lines.
func joinDot(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "  ·  ")
}

// formatBytes is an exact byte count with thousands separators.
func formatBytes(n int64) string {
	digits := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(digits)+len(digits)/3)
	for i := range len(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, digits[i])
	}
	return string(out)
}
