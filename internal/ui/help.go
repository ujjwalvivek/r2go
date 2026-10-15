package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	t "r2go/internal/theme"
)

type keyRow struct {
	keys   string // keycaps separated by " / "; a "+" inside one keycap stays
	action string
}

var keyGroups = []struct {
	title string
	rows  []keyRow
}{
	{"Move around", []keyRow{
		{"Ctrl+K", "Jump to a bucket, folder or action"},
		{"Ctrl+L", "Type a folder path"},
		{"Enter", "Open the selected folder or file"},
		{"Backspace / Alt+↑", "Up one folder"},
		{"Alt+← / Alt+→", "Back and forward"},
		{"↑ ↓ / Home / End", "Move the selection"},
		{"a–z", "Jump to a name as you type"},
	}},
	{"Select", []keyRow{
		{"Shift+↑ / Shift+↓", "Extend the selection"},
		{"Ctrl+click / Shift+click", "Add one, or a range"},
		{"Ctrl+A", "Select everything listed"},
		{"Esc", "Clear the selection"},
	}},
	{"Act", []keyRow{
		{"Ctrl+U", "Upload files or a folder"},
		{"Ctrl+D", "Download the selection"},
		{"Ctrl+Shift+N", "New folder"},
		{"F2", "Rename"},
		{"Del", "Delete, after asking"},
		{"Ctrl+C / Ctrl+X / Ctrl+V", "Copy, cut and paste within a bucket"},
		{"Ctrl+Shift+C", "Copy the public URL"},
	}},
	{"View", []keyRow{
		{"Ctrl+F", "Filter this folder"},
		{"Ctrl+1 / Ctrl+2", "List or grid"},
		{"Ctrl+I", "Details panel"},
		{"F5 / Ctrl+R", "Refresh"},
		{"Ctrl+,", "Settings"},
		{"?", "This list"},
	}},
}

func (mw *MainWindow) showHelp() {
	column := func(groups ...int) fyne.CanvasObject {
		objects := []fyne.CanvasObject{}
		for _, index := range groups {
			group := keyGroups[index]
			rows := []fyne.CanvasObject{}
			for _, row := range group.rows {
				rows = append(rows, minHeight(24, container.NewBorder(nil, nil, nil, keycaps(row.keys),
					newElided(row.action, t.TextBody, t.ColorSubtle, styleNormal))))
			}
			objects = append(objects, section(group.title, rows...))
		}
		return vbox(12, objects...)
	}
	body := container.NewGridWithColumns(2, inset(0, 0, 10, 0, column(0, 1)), inset(10, 0, 0, 0, column(2, 3)))
	showModal(mw.win, modalSpec{title: "Keyboard", width: 760, body: body})
}

// keycaps draws "Ctrl+K / Alt+↑" as keycaps with muted separators.
func keycaps(keys string) fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	for i, alternative := range strings.Split(keys, " / ") {
		if i > 0 {
			objects = append(objects, newText("/", t.TextSmall, t.ColorMuted, styleNormal))
		}
		objects = append(objects, keycap(alternative))
	}
	return container.New(&rowLayout{gap: 4}, objects...)
}
