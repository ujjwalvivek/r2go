package ui

import (
	"context"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"r2go/internal/model"
	"r2go/internal/r2"
	t "r2go/internal/theme"
)

const (
	detailsInner   = t.WidthDetails - 2*t.SpaceCardX - 1
	previewHeight  = float32(180)
	headDebounce   = 150 * time.Millisecond
	factLabelWidth = float32(84)
)

// Details is the right-hand panel describing the selection: one object with
// its preview, metadata and links; one folder with its totals; several items
// with a summary; or, with nothing selected, the folder you are in.
type Details struct {
	mw        *MainWindow
	Container fyne.CanvasObject
	body      *fyne.Container
	scroll    *fyne.Container

	key        string
	generation uint64
	heads      map[string]r2.ObjectInfo
	headErrs   map[string]error
	preview    *canvas.Image
	previewMsg *textView
}

func NewDetails(mw *MainWindow) *Details {
	d := &Details{mw: mw, body: vbox(t.SpaceGap), heads: map[string]r2.ObjectInfo{}, headErrs: map[string]error{}}
	scroll := container.NewVScroll(inset(t.SpaceCardX, t.SpacePageY, t.SpaceCardX, t.SpacePageY, d.body))
	d.Container = container.NewBorder(nil, nil, vRule(t.ColorBorder), nil, panelBox(t.ColorPanel, scroll))
	return d
}

func headKey(session model.Session, entry model.Entry) string {
	return session.Account + "\x00" + session.Bucket + "\x00" + entry.Key + "\x00" + entry.ETag
}

// Update redraws the panel for the current selection.
func (d *Details) Update() {
	if !d.Container.Visible() {
		return
	}
	session := d.mw.currentSession()
	entries := d.mw.browser.SelectedEntries()
	d.generation++
	d.preview = nil
	d.previewMsg = nil
	d.key = ""
	var objects []fyne.CanvasObject
	switch {
	case session.Bucket == "":
		objects = d.nothing()
	case len(entries) == 0:
		objects = d.folderSummary(session)
	case len(entries) == 1 && entries[0].IsDir:
		objects = d.folder(session, entries[0])
	case len(entries) == 1:
		objects = d.object(session, entries[0])
	default:
		objects = d.many(entries)
	}
	d.body.Objects = objects
	d.body.Refresh()
}

// previewReady is called when a thumbnail arrives; it fills the preview if
// that object is still the one shown.
func (d *Details) previewReady(entry model.Entry) {
	if d.preview == nil || d.key != entry.Key {
		return
	}
	if img := d.mw.thumbs.Get(nil, d.mw.currentSession(), entry, nil); img != nil {
		d.setPreview(img)
	}
}

func (d *Details) setPreview(img image.Image) {
	if d.preview == nil {
		return
	}
	d.preview.Image = img
	d.preview.Show()
	if d.previewMsg != nil {
		d.previewMsg.SetText("")
	}
	d.preview.Refresh()
}

func (d *Details) nothing() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		fixed(1, 40, layoutSpacer()),
		noticeView(t.IconDetails, "Nothing selected", "Choose a bucket to see details here."),
	}
}

func (d *Details) folderSummary(session model.Session) []fyne.CanvasObject {
	b := d.mw.browser
	name := session.Bucket
	if session.Prefix != "" {
		parts := strings.Split(strings.TrimSuffix(session.Prefix, "/"), "/")
		name = parts[len(parts)-1]
	}
	files, folders := 0, 0
	var size int64
	for _, entry := range b.visible {
		if entry.IsDir {
			folders++
		} else {
			files++
		}
		size += entry.Size
	}
	icon := t.IconFolderOpen
	if session.Prefix == "" {
		icon = t.IconBucket
	}
	objects := []fyne.CanvasObject{
		titleBlock(icon, name, session.Bucket+"/"+session.Prefix),
		statStrip([2]string{"Files", formatCount(files)}, [2]string{"Folders", formatCount(folders)}, [2]string{"Size", formatSize(size)}),
	}
	domain := d.mw.cfg.PublicDomain(session.Account, session.Bucket)
	facts := []fyne.CanvasObject{
		factRow("Account", session.Account, false),
		factRow("Bucket", session.Bucket, false),
	}
	if domain != "" {
		facts = append(facts, factRowWithAction("Public URL", domain, newIconButton(t.IconPencil, "Change public domain", d.mw.showPublicDomainDialog)))
	} else {
		facts = append(facts, factRowWithAction("Public URL", "Not set", newIconButton(t.IconPencil, "Set a public domain\nSo r2go can copy shareable links", d.mw.showPublicDomainDialog)))
	}
	objects = append(objects, section("Bucket", facts...))
	_, hint := wrapped("Select an item to see its details. Ctrl+A selects everything.", t.SizeSmall, t.ColorMuted, styleNormal)
	objects = append(objects, hint)
	return objects
}

func (d *Details) folder(session model.Session, entry model.Entry) []fyne.CanvasObject {
	objects := []fyne.CanvasObject{titleBlock(t.IconFolder, entry.Name, session.Bucket+"/"+entry.Key)}
	open := newButton(buttonPrimary, "Open", t.IconFolderOpen, func() { d.mw.openEntry(entry) })
	download := newButton(buttonGhost, "Download", t.IconDownload, func() { d.mw.downloadEntries([]model.Entry{entry}) })
	objects = append(objects, container.New(&rowLayout{gap: 6}, open, download))
	switch {
	case entry.StatsKnown:
		objects = append(objects, statStrip(
			[2]string{"Objects", formatCount(entry.Count)},
			[2]string{"Size", formatSize(entry.Size)},
			[2]string{"Newest", formatAge(entry.Modified, time.Now())},
		))
	case entry.StatsError:
		objects = append(objects, statStrip([2]string{"Objects", "—"}, [2]string{"Size", "—"}, [2]string{"Newest", "—"}))
	default:
		objects = append(objects, statStrip([2]string{"Objects", "…"}, [2]string{"Size", "…"}, [2]string{"Newest", "…"}))
	}
	facts := []fyne.CanvasObject{
		factRowWithAction("Prefix", entry.Key, d.copyButton("Copy prefix", entry.Key)),
	}
	if entry.StatsKnown && !entry.Modified.IsZero() {
		facts = append(facts, factRow("Last change", formatTimestamp(entry.Modified), true))
	}
	objects = append(objects, section("Folder", facts...))
	return objects
}

func (d *Details) object(session model.Session, entry model.Entry) []fyne.CanvasObject {
	d.key = entry.Key
	var objects []fyne.CanvasObject
	if previewable(entry) && !d.mw.cfg.Settings().NoThumbnails {
		objects = append(objects, d.previewBox(session, entry))
	}
	objects = append(objects, titleBlock(entryIconName(entry), entry.Name, session.Bucket+"/"+entry.Key))

	domain := d.mw.cfg.PublicDomain(session.Account, session.Bucket)
	download := newButton(buttonPrimary, "Download", t.IconDownload, func() { d.mw.downloadEntries([]model.Entry{entry}) })
	open := newButton(buttonGhost, "Open", t.IconExternal, func() { d.mw.openEntry(entry) }).withTip("Open  ·  Enter\nDownload to a temporary folder and open with the default app")
	actions := []fyne.CanvasObject{download, open}
	if domain != "" {
		actions = append(actions, newIconButton(t.IconLink, "Copy public URL  ·  Ctrl+Shift+C", d.mw.copySelectedPublicURL))
	}
	objects = append(objects, container.New(&rowLayout{gap: 6}, actions...))

	now := time.Now()
	objects = append(objects, statStrip(
		[2]string{"Size", formatSize(entry.Size)},
		[2]string{"Modified", formatAge(entry.Modified, now)},
		[2]string{"Type", orDash(typeLabel(entry))},
	))

	key := headKey(session, entry)
	info, haveInfo := d.heads[key]
	headErr := d.headErrs[key]
	contentType, cacheControl := "…", ""
	switch {
	case haveInfo:
		contentType = orDash(info.ContentType)
		cacheControl = info.CacheControl
	case headErr != nil:
		contentType = "—"
	default:
		d.fetchHead(session, entry, d.generation)
	}

	facts := []fyne.CanvasObject{
		factRowWithAction("Key", entry.Key, d.copyButton("Copy key", entry.Key)),
		factRow("Content type", contentType, true),
		factRow("ETag", strings.Trim(entry.ETag, `"`), true),
		factRow("Modified", formatTimestamp(entry.Modified), true),
		factRow("Bytes", formatBytes(entry.Size), true),
	}
	if cacheControl != "" {
		facts = append(facts, factRow("Cache", cacheControl, true))
	}
	if domain != "" {
		url := publicURL(domain, entry.Key)
		facts = append(facts, factRowWithAction("Public URL", url, newIconButton(t.IconExternal, "Open in browser", func() { d.mw.openPublicURL(entry) })))
	} else {
		facts = append(facts, factRowWithAction("Public URL", "Not set", newIconButton(t.IconPencil, "Set a public domain\nSo r2go can copy shareable links", d.mw.showPublicDomainDialog)))
	}
	objects = append(objects, section("Object", facts...))

	if haveInfo && len(info.Metadata) > 0 {
		keys := make([]string, 0, len(info.Metadata))
		for k := range info.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		meta := make([]fyne.CanvasObject, 0, len(keys))
		for _, k := range keys {
			meta = append(meta, factRow(k, info.Metadata[k], true))
		}
		objects = append(objects, section("Metadata", meta...))
	}
	if headErr != nil {
		e := explainError(headErr)
		_, note := wrapped(e.Title+". Content type and metadata are not available.", t.SizeSmall, t.ColorMuted, styleNormal)
		objects = append(objects, note)
	}
	return objects
}

func (d *Details) many(entries []model.Entry) []fyne.CanvasObject {
	files, folders := 0, 0
	var size int64
	pending := false
	for _, entry := range entries {
		if entry.IsDir {
			folders++
			pending = pending || !entry.StatsKnown
		} else {
			files++
		}
		size += entry.Size
	}
	sizeText := formatSize(size)
	if pending {
		sizeText += "+"
	}
	objects := []fyne.CanvasObject{
		titleBlock(t.IconCopy, countNoun(len(entries), "item")+" selected", ""),
		container.New(&rowLayout{gap: 6},
			newButton(buttonPrimary, "Download", t.IconDownload, d.mw.downloadSelected),
			newButton(buttonDanger, "Delete", t.IconTrash, d.mw.deleteSelected),
		),
		statStrip([2]string{"Files", formatCount(files)}, [2]string{"Folders", formatCount(folders)}, [2]string{"Size", sizeText}),
	}
	const shown = 10
	names := make([]fyne.CanvasObject, 0, shown+1)
	for i, entry := range entries {
		if i == shown {
			names = append(names, inset(0, 2, 0, 0, newText(fmt.Sprintf("and %s more", formatCount(len(entries)-shown)), t.TextSmall, t.ColorMuted, styleNormal)))
			break
		}
		icon := canvas.NewImageFromResource(entryIcon(entry))
		icon.SetMinSize(fyne.NewSize(t.SizeIcon, t.SizeIcon))
		names = append(names, minHeight(22, container.NewBorder(nil, nil, container.NewCenter(icon), nil,
			inset(8, 0, 0, 0, newElided(entry.Name, t.TextBody, t.ColorSubtle, styleNormal)))))
	}
	objects = append(objects, section("Selection", names...))
	return objects
}

// previewBox is a BASE frame holding the image preview once it arrives.
func (d *Details) previewBox(session model.Session, entry model.Entry) fyne.CanvasObject {
	frame := newRect(t.ColorBase)
	frame.StrokeColor = tc(t.ColorBorder)
	frame.StrokeWidth = 1
	d.preview = canvas.NewImageFromImage(nil)
	d.preview.FillMode = canvas.ImageFillContain
	d.preview.ScaleMode = canvas.ImageScaleSmooth
	d.preview.Hide()
	d.previewMsg = newTextView("Loading preview…", t.TextSmall, t.ColorMuted, styleNormal)
	d.previewMsg.SetAlignment(fyne.TextAlignCenter)
	if img := d.mw.browser.preview(entry); img != nil {
		d.preview.Image = img
		d.preview.Show()
		d.previewMsg.SetText("")
	}
	return fixed(detailsInner, previewHeight, container.NewStack(frame, inset(1, 1, 1, 1, d.preview), d.previewMsg))
}

func (d *Details) fetchHead(session model.Session, entry model.Entry, generation uint64) {
	key := headKey(session, entry)
	time.AfterFunc(headDebounce, func() {
		fyne.Do(func() {
			if d.generation != generation {
				return
			}
			client, err := d.mw.clientFor(session.Account)
			if err != nil {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				info, err := client.Head(ctx, session.Bucket, entry.Key)
				fyne.Do(func() {
					if err != nil {
						d.headErrs[key] = err
					} else {
						d.heads[key] = info
					}
					if d.generation == generation {
						d.Update()
					}
				})
			}()
		})
	})
}

func (d *Details) copyButton(tip, value string) *button {
	return newIconButton(t.IconCopy, tip, func() {
		d.mw.win.Clipboard().SetContent(value)
		d.mw.status.SetMessage("Copied to the clipboard")
	})
}

func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

// titleBlock is the panel heading: an icon and name, then a path in mono.
func titleBlock(icon, name, path string) fyne.CanvasObject {
	nameLabel, nameBox := wrapped(name, t.SizeMedium, t.ColorText, styleStrong)
	nameLabel.Wrapping = fyne.TextWrapBreak
	head := container.NewBorder(nil, nil, inset(0, 3, 6, 0, container.NewVBox(newIcon(icon, t.ColorSubtle))), nil, nameBox)
	if path == "" {
		return head
	}
	pathLabel, pathBox := wrapped(path, t.SizeSmall, t.ColorMuted, styleMono)
	pathLabel.Wrapping = fyne.TextWrapBreak
	return vbox(2, head, pathBox)
}

// statStrip is a bordered strip of headline numbers separated by 1 px
// dividers, each a caption over a value.
func statStrip(cells ...[2]string) fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, 0, len(cells)*2)
	for i, cell := range cells {
		if i > 0 {
			objects = append(objects, newRect(t.ColorBorder))
		}
		caption := newText(cell[0], t.TextSmall, t.ColorMuted, styleNormal)
		value := newElided(cell[1], t.TextMedium, t.ColorText, styleStrong)
		objects = append(objects, inset(8, 5, 6, 6, vbox(1, caption, value)))
	}
	frame := newRect(t.ColorBase)
	frame.StrokeColor = tc(t.ColorBorder)
	frame.StrokeWidth = 1
	return container.NewStack(frame, container.New(&stripLayout{}, objects...))
}

// stripLayout takes cells interleaved with 1 px dividers and gives every cell
// an equal share of the width.
type stripLayout struct{}

func (stripLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	cells := (len(objects) + 1) / 2
	if cells == 0 {
		return
	}
	w := (size.Width - float32(cells-1)) / float32(cells)
	x := float32(0)
	for i, object := range objects {
		if i%2 == 1 {
			object.Move(fyne.NewPos(x, 1))
			object.Resize(fyne.NewSize(1, size.Height-2))
			x++
			continue
		}
		object.Move(fyne.NewPos(x, 0))
		object.Resize(fyne.NewSize(w, size.Height))
		x += w
	}
}

func (stripLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, object := range objects {
		h = fyne.Max(h, object.MinSize().Height)
	}
	return fyne.NewSize(60*float32((len(objects)+1)/2), h)
}

// section is a panel header followed by rows.
func section(title string, rows ...fyne.CanvasObject) fyne.CanvasObject {
	caption := newText(title, t.TextSmall, t.ColorSubtle, styleStrong)
	head := vbox(5, hRule(t.ColorBorder), caption)
	return vbox(4, append([]fyne.CanvasObject{inset(0, 4, 0, 2, head)}, rows...)...)
}

// factRow is a caption in MUTED on the left and a value on the right, cut
// short with its full text in a tooltip.
func factRow(caption, value string, mono bool) fyne.CanvasObject {
	return factRowWithAction(caption, value, nil, mono)
}

func factRowWithAction(caption, value string, action fyne.CanvasObject, mono ...bool) fyne.CanvasObject {
	style := styleNormal
	if len(mono) == 0 || mono[0] {
		style = styleMono
	}
	label := newTextView(caption, t.TextSmall, t.ColorMuted, styleNormal)
	text := newElided(value, t.TextSmall, t.ColorSubtle, style)
	if value == "Not set" || value == "—" || value == "…" {
		text.SetColorName(t.ColorMuted)
	}
	var right fyne.CanvasObject
	if action != nil {
		right = action
	}
	row := container.NewBorder(nil, nil, fixed(factLabelWidth, 20, label), right, withTip(text, elidedTip(text)))
	return minHeight(t.HeightControl, row)
}
