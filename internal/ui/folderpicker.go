package ui

import (
	"context"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"r2go/internal/model"
	t "r2go/internal/theme"
)

const folderPickerHeight float32 = 220

// folderPicker browses one bucket's folders to choose a destination. The path
// field always holds the choice and can be typed into; the list below shows
// the folders inside it, a click opens one, and Up goes back. Folders being
// moved are listed but cannot be opened, so nothing moves into itself.
type folderPicker struct {
	mw       *MainWindow
	session  model.Session
	blocked  map[string]bool
	Path     *field
	list     *fyne.Container
	note     *textView
	up       *button
	shown    string
	serial   int
	onChange func()

	Container fyne.CanvasObject
}

func newFolderPicker(mw *MainWindow, session model.Session, start string, blocked []model.Entry) *folderPicker {
	p := &folderPicker{mw: mw, session: session, blocked: map[string]bool{}, list: vbox(0)}
	for _, entry := range blocked {
		if entry.IsDir {
			p.blocked[entry.Key] = true
		}
	}
	p.Path = newField("blank for the bucket root")
	p.Path.SetText(start)
	p.Path.OnChanged = func(value string) {
		// Follow typing once it names a whole folder.
		if prefix := cleanPrefix(value); prefix != p.shown && (prefix == "" || strings.HasSuffix(value, "/")) {
			p.load(prefix)
		}
	}
	p.up = newIconButton(t.IconArrowUp, "Up one folder", func() { p.open(model.ParentPrefix(p.shown)) })
	p.note = newElided("", t.TextSmall, t.ColorMuted, styleMono)

	head := container.NewBorder(nil, nil, p.up, nil, inset(6, 0, 0, 0, p.note))
	frame := newRect(t.ColorBase)
	frame.StrokeColor = tc(t.ColorBorder)
	frame.StrokeWidth = 1
	scroll := container.NewVScroll(p.list)
	box := container.NewStack(frame, inset(1, 1, 1, 1,
		container.NewBorder(vbox(0, inset(4, 2, 6, 2, head), hRule(t.ColorBorder)), nil, nil, nil, scroll)))
	p.Container = vbox(6, p.Path, container.New(&minHeightLayout{height: folderPickerHeight}, box))
	p.load(cleanPrefix(start))
	return p
}

// Value is the chosen destination prefix.
func (p *folderPicker) Value() string { return cleanPrefix(p.Path.Text) }

func cleanPrefix(value string) string {
	value = strings.Trim(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")), "/")
	if value == "" {
		return ""
	}
	return value + "/"
}

// open moves into prefix and puts it in the path field.
func (p *folderPicker) open(prefix string) {
	p.Path.SetText(prefix)
	p.load(prefix)
}

func (p *folderPicker) load(prefix string) {
	p.shown = prefix
	p.serial++
	serial := p.serial
	p.up.SetDisabled(prefix == "")
	p.note.SetText(p.session.Bucket + "/" + prefix + "  ·  loading…")
	p.list.Objects = nil
	p.list.Refresh()
	client, err := p.mw.clientFor(p.session.Account)
	if err != nil {
		p.note.SetText(explainError(err).Title)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var folders []string
		token := ""
		var listErr error
		for pages := 0; pages < 10; pages++ {
			page, err := client.ListPage(ctx, p.session.Bucket, prefix, token)
			if err != nil {
				listErr = err
				break
			}
			folders = append(folders, page.Prefixes...)
			if !page.Truncated {
				break
			}
			token = page.NextToken
		}
		sort.Strings(folders)
		fyne.Do(func() {
			if serial != p.serial {
				return
			}
			p.show(prefix, folders, listErr)
		})
	}()
}

func (p *folderPicker) show(prefix string, folders []string, err error) {
	location := p.session.Bucket + "/" + prefix
	switch {
	case err != nil:
		p.note.SetText(location + "  ·  " + explainError(err).Title)
	case len(folders) == 0:
		p.note.SetText(location + "  ·  no folders inside")
	default:
		p.note.SetText(location + "  ·  " + countNoun(len(folders), "folder"))
	}
	rows := make([]fyne.CanvasObject, 0, len(folders))
	for _, key := range folders {
		key := key
		name := strings.TrimSuffix(strings.TrimPrefix(key, prefix), "/")
		blocked := p.blocked[key]
		iconColor, textColor := t.ColorAccent, t.ColorText
		if blocked {
			iconColor, textColor = t.ColorBorderStrong, t.ColorMuted
		}
		row := container.NewBorder(nil, nil, inset(0, 0, 8, 0, container.NewCenter(newIcon(t.IconFolder, iconColor))),
			container.NewCenter(newIcon(t.IconChevronR, t.ColorBorderStrong)),
			newElided(name, t.TextBody, textColor, styleNormal))
		area := newHoverArea(minHeight(t.HeightRow, inset(10, 0, 8, 0, row)), func() {
			if !blocked {
				p.open(key)
			}
		})
		if blocked {
			area.tip = "Being moved\nA folder cannot go inside itself"
		}
		rows = append(rows, area)
	}
	p.list.Objects = rows
	p.list.Refresh()
}
