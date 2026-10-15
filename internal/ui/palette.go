package ui

import (
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/model"
	t "r2go/internal/theme"
)

const paletteRows = 12

// paletteItem is one result in the quick-jump list.
type paletteItem struct {
	icon   string
	title  string
	detail string
	kind   string // "Bucket", "Folder", "Recent" or "Action"
	key    string // keycap hint for actions
	run    func()
	score  int
}

// showPalette opens Ctrl+K: one field that jumps to any bucket, a folder in
// the current listing, a recent location, or runs an action.
func (mw *MainWindow) showPalette() {
	if len(mw.win.Canvas().Overlays().List()) > 0 {
		return
	}
	items := mw.paletteItems()
	query := newField("Jump to a bucket or folder, or run an action…")
	query.ActionItem = fieldIcon(t.IconSearch)
	list := vbox(0)
	empty := newText("Nothing matches", t.TextSmall, t.ColorMuted, styleNormal)

	var pop *widget.PopUp
	var shown []paletteItem
	selected := 0
	closePalette := func() {
		if pop != nil {
			pop.Hide()
		}
	}
	runItem := func(index int) {
		if index < 0 || index >= len(shown) {
			return
		}
		item := shown[index]
		closePalette()
		item.run()
	}

	var render func()
	render = func() {
		shown = rankPalette(items, query.Text)
		if len(shown) > paletteRows {
			shown = shown[:paletteRows]
		}
		selected = max(0, min(selected, len(shown)-1))
		rows := make([]fyne.CanvasObject, 0, len(shown)+1)
		if len(shown) == 0 {
			rows = append(rows, inset(t.SpaceCardX+4, 10, t.SpaceCardX, 10, empty))
		}
		for i, item := range shown {
			index := i
			rows = append(rows, newPaletteRow(item, i == selected, func() { runItem(index) }, func() {
				if selected != index {
					selected = index
					render()
				}
			}))
		}
		list.Objects = rows
		list.Refresh()
		if pop != nil {
			pop.Resize(fyne.NewSize(620, pop.Content.MinSize().Height))
		}
	}
	query.OnChanged = func(string) { selected = 0; render() }
	query.OnSubmitted = func(string) { runItem(selected) }
	query.onEscape = closePalette
	query.onUp = func() {
		if selected > 0 {
			selected--
			render()
		}
	}
	query.onDown = func() {
		if selected < len(shown)-1 {
			selected++
			render()
		}
	}

	hints := container.New(&rowLayout{gap: 12}, keyHint("↑↓", "choose"), keyHint("Enter", "go"), keyHint("Esc", "close"))
	footer := inset(t.SpaceCardX+4, 6, t.SpaceCardX, 6, container.NewBorder(nil, nil, nil, hints))
	content := vbox(0,
		inset(t.SpaceCardX, t.SpaceCardX, t.SpaceCardX, t.SpaceCardX, query),
		hRule(t.ColorBorder),
		inset(0, 4, 0, 4, list),
		hRule(t.ColorBorder),
		footer,
	)
	edge := canvas.NewRectangle(color.Transparent)
	edge.StrokeColor = tc(t.ColorBorderStrong)
	edge.StrokeWidth = 1
	frame := container.NewStack(newRect(t.ColorPanel), content, edge)
	render()
	tips.Hide(nil)
	pop = widget.NewModalPopUp(frame, mw.win.Canvas())
	pop.Show()
	pop.Resize(fyne.NewSize(620, frame.MinSize().Height))
	mw.win.Canvas().Focus(query)
}

func (mw *MainWindow) paletteItems() []paletteItem {
	var items []paletteItem
	session := mw.currentSession()

	// Recent locations, newest first, without repeats.
	seen := map[model.Session]bool{session: true}
	for i := len(mw.back) - 1; i >= 0 && len(seen) < 7; i-- {
		place := mw.back[i]
		if seen[place] {
			continue
		}
		seen[place] = true
		items = append(items, paletteItem{
			icon: t.IconFolderOpen, title: placeName(place), detail: place.Bucket + "/" + place.Prefix,
			kind: "Recent", run: func() { mw.jumpTo(place) },
		})
	}

	// Folders in the current listing.
	if session.Bucket != "" {
		for _, entry := range mw.browser.all {
			if !entry.IsDir {
				continue
			}
			key := entry.Key
			items = append(items, paletteItem{
				icon: t.IconFolder, title: entry.Name, detail: session.Bucket + "/" + key,
				kind: "Folder", run: func() { mw.navigate(key) },
			})
		}
	}

	// Every bucket of every account.
	accounts := mw.cfg.Accounts()
	for _, account := range accounts {
		for _, bucket := range account.Buckets {
			name := account.Name
			detail := ""
			if len(accounts) > 1 {
				detail = name
			}
			items = append(items, paletteItem{
				icon: t.IconBucket, title: bucket, detail: detail,
				kind: "Bucket", run: func() { mw.selectBucket(name, bucket) },
			})
		}
	}

	open := session.Bucket != ""
	action := func(icon, title, key string, enabled bool, run func()) {
		if enabled {
			items = append(items, paletteItem{icon: icon, title: title, key: key, kind: "Action", run: run})
		}
	}
	action(t.IconUpload, "Upload files…", "Ctrl+U", open, mw.pickUploadFile)
	action(t.IconUpload, "Upload a folder…", "Ctrl+Shift+U", open, mw.pickUploadFolder)
	action(t.IconFolderPlus, "New folder", "Ctrl+Shift+N", open, mw.newFolder)
	action(t.IconRefresh, "Refresh", "F5", true, mw.refresh)
	action(t.IconList, "List view", "Ctrl+1", open, func() { mw.setView(viewList) })
	action(t.IconGrid, "Grid view", "Ctrl+2", open, func() { mw.setView(viewGrid) })
	action(t.IconDetails, "Show or hide details", "Ctrl+I", true, mw.toggleDetails)
	action(t.IconGlobe, "Set up public URL…", "", open, mw.showPublicDomainDialog)
	action(t.IconPlus, "Add account…", "", true, func() { mw.showAccountDialog(nil) })
	action(t.IconSettings, "Settings", "Ctrl+,", true, mw.showSettings)
	action(t.IconHelp, "Keyboard shortcuts", "?", true, mw.showHelp)
	return items
}

// jumpTo opens any remembered location, switching bucket if needed.
func (mw *MainWindow) jumpTo(place model.Session) {
	if _, ok := mw.cfg.FindAccount(place.Account); !ok {
		return
	}
	mw.pushHistory()
	mw.open(place)
}

func placeName(place model.Session) string {
	if place.Prefix == "" {
		return place.Bucket
	}
	parts := strings.Split(strings.TrimSuffix(place.Prefix, "/"), "/")
	return parts[len(parts)-1]
}

// rankPalette filters items by query. With no query, recent places and
// buckets come first. Otherwise a name that starts with the query beats one
// that contains it, which beats one that only has its letters in order.
func rankPalette(items []paletteItem, query string) []paletteItem {
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]paletteItem, 0, len(items))
	for i, item := range items {
		score := fuzzyScore(strings.ToLower(item.title), query)
		if score < 0 && item.detail != "" {
			if s := fuzzyScore(strings.ToLower(item.detail), query); s >= 0 {
				score = s / 3
			}
		}
		if score < 0 {
			continue
		}
		item.score = score*1000 - i
		out = append(out, item)
	}
	if query == "" {
		return out
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

func fuzzyScore(text, query string) int {
	switch {
	case query == "":
		return 0
	case text == query:
		return 400
	case strings.HasPrefix(text, query):
		return 300
	case strings.Contains(text, query):
		return 200
	}
	// Letters in order, with a bonus for each run of adjacent matches.
	score, run, pos := 100, 0, 0
	for _, r := range query {
		i := strings.IndexRune(text[pos:], r)
		if i < 0 {
			return -1
		}
		if i == 0 && pos > 0 {
			run++
			score += run
		} else {
			run = 0
		}
		pos += i + len(string(r))
	}
	return score
}

// newPaletteRow is one result: icon, title, detail in MUTED, then the kind
// or a keycap on the right. The chosen row gets the selection fill and bar.
func newPaletteRow(item paletteItem, selected bool, run, hover func()) fyne.CanvasObject {
	title := newElided(item.title, t.TextBody, t.ColorText, styleNormal)
	detail := newElided(item.detail, t.TextSmall, t.ColorMuted, styleMono)
	var right fyne.CanvasObject = newText(item.kind, t.TextSmall, t.ColorMuted, styleNormal)
	if item.key != "" {
		right = keycaps(item.key)
	}
	iconColor := t.ColorMuted
	if selected {
		iconColor = t.ColorAccent
	}
	text := container.New(&paletteTextLayout{}, title, detail)
	row := container.NewBorder(nil, nil, inset(0, 0, 10, 0, container.NewCenter(newIcon(item.icon, iconColor))), inset(10, 0, 0, 0, right), text)
	bar := newRect(t.ColorAccent)
	bar.SetMinSize(fyne.NewSize(t.WidthAccentBar, 1))
	if !selected {
		bar.FillColor = color.Transparent
	}
	area := newHoverArea(minHeight(30, container.NewBorder(nil, nil, bar, nil, inset(t.SpaceCardX+2, 0, t.SpaceCardX+2, 0, row))), run)
	area.SetSelected(selected)
	area.onHover = func(hovering bool) {
		if hovering {
			hover()
		}
	}
	return area
}

// paletteTextLayout puts the title at its natural width and gives the detail
// whatever is left.
type paletteTextLayout struct{}

func (paletteTextLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	title, detail := objects[0], objects[1]
	natural := fyne.MeasureText(title.(*textView).Text(), t.TextBody, styleNormal).Width + 2
	width := fyne.Min(natural, size.Width*0.7)
	title.Move(fyne.NewPos(0, 0))
	title.Resize(fyne.NewSize(width, size.Height))
	detail.Move(fyne.NewPos(width+10, 0))
	detail.Resize(fyne.NewSize(fyne.Max(0, size.Width-width-10), size.Height))
}

func (paletteTextLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(60, fyne.MeasureText("M", t.TextBody, styleNormal).Height)
}
