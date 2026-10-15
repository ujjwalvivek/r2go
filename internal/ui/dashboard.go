package ui

import (
	"context"
	"fmt"
	"image/color"
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
	dashboardStale   = 10 * time.Minute // older numbers are counted again on open
	dashboardTopRows = 8
	dashboardWorkers = 2
)

// Dashboard is an account's home page: how much is stored where, what kind of
// files they are, what changed recently and what is biggest. It counts every
// object with recursive listings, two buckets at a time, and keeps the result
// for a while so switching back is instant.
type Dashboard struct {
	mw        *MainWindow
	Container fyne.CanvasObject
	page      *fyne.Container
	scroll    *container.Scroll
	account   string

	stats map[string]*accountStats
}

type accountStats struct {
	account  string
	buckets  map[string]*bucketStats
	order    []string
	started  time.Time
	finished time.Time
	running  int
	cancel   context.CancelFunc
}

type bucketStats struct {
	name    string
	objects int
	size    int64
	newest  time.Time
	kinds   map[fileKind]*kindStat
	recent  []objectRef
	largest []objectRef
	done    bool
	err     error
}

type kindStat struct {
	count int
	size  int64
}

type objectRef struct {
	bucket   string
	key      string
	size     int64
	modified time.Time
}

func NewDashboard(mw *MainWindow) *Dashboard {
	d := &Dashboard{mw: mw, page: vbox(t.SpaceGap), stats: map[string]*accountStats{}}
	d.scroll = container.NewVScroll(inset(t.SpacePageX, t.SpacePageY, t.SpacePageX, t.SpacePageY*2, d.page))
	d.Container = panelBox(t.ColorBase, d.scroll)
	d.Container.Hide()
	return d
}

// Show opens the dashboard for an account, counting again when its numbers
// are missing or stale.
func (d *Dashboard) Show(account string) {
	d.account = account
	d.Container.Show()
	stats := d.stats[account]
	if stats == nil || (stats.running == 0 && time.Since(stats.finished) > dashboardStale) {
		d.Refresh()
		return
	}
	d.render()
}

func (d *Dashboard) Hide() { d.Container.Hide() }

// Refresh counts the account's buckets again.
func (d *Dashboard) Refresh() {
	account, ok := d.mw.cfg.FindAccount(d.account)
	if !ok {
		return
	}
	if old := d.stats[account.Name]; old != nil && old.cancel != nil {
		old.cancel()
	}
	client, err := d.mw.clientFor(account.Name)
	stats := &accountStats{account: account.Name, buckets: map[string]*bucketStats{}, started: time.Now()}
	buckets := append([]string(nil), account.Buckets...)
	sort.Strings(buckets)
	for _, name := range buckets {
		stats.buckets[name] = &bucketStats{name: name, kinds: map[fileKind]*kindStat{}}
		stats.order = append(stats.order, name)
	}
	d.stats[account.Name] = stats
	if err != nil {
		for _, b := range stats.buckets {
			b.done, b.err = true, err
		}
		stats.finished = time.Now()
		d.render()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	stats.cancel = cancel
	stats.running = len(buckets)
	if stats.running > 0 {
		d.mw.progress.Begin()
	}
	sem := make(chan struct{}, dashboardWorkers)
	for _, name := range buckets {
		go func() {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			result := countBucket(ctx, client, name)
			fyne.Do(func() {
				if d.stats[account.Name] != stats {
					return
				}
				stats.buckets[name] = result
				stats.running--
				if stats.running == 0 {
					stats.finished = time.Now()
					d.mw.progress.End()
				}
				if d.account == account.Name && d.Container.Visible() {
					d.render()
				}
			})
		}()
	}
	d.render()
}

// countBucket lists a whole bucket and summarises it.
func countBucket(ctx context.Context, client *r2.Client, bucket string) *bucketStats {
	b := &bucketStats{name: bucket, kinds: map[fileKind]*kindStat{}, done: true}
	objects, err := client.ListAll(ctx, bucket, "")
	if err != nil {
		b.err = err
		return b
	}
	for _, object := range objects {
		if strings.HasSuffix(object.Key, "/") && object.Size == 0 {
			continue // folder marker
		}
		b.objects++
		b.size += object.Size
		if object.Modified.After(b.newest) {
			b.newest = object.Modified
		}
		kind := kindOf(object.Key)
		k := b.kinds[kind]
		if k == nil {
			k = &kindStat{}
			b.kinds[kind] = k
		}
		k.count++
		k.size += object.Size
		ref := objectRef{bucket: bucket, key: object.Key, size: object.Size, modified: object.Modified}
		b.recent = keepTop(b.recent, ref, func(a, c objectRef) bool { return a.modified.After(c.modified) })
		b.largest = keepTop(b.largest, ref, func(a, c objectRef) bool { return a.size > c.size })
	}
	return b
}

// keepTop inserts ref into a list kept sorted by better, capped at the
// dashboard's row count.
func keepTop(list []objectRef, ref objectRef, better func(a, b objectRef) bool) []objectRef {
	if len(list) == dashboardTopRows && !better(ref, list[len(list)-1]) {
		return list
	}
	i := sort.Search(len(list), func(i int) bool { return better(ref, list[i]) })
	list = append(list, objectRef{})
	copy(list[i+1:], list[i:])
	list[i] = ref
	if len(list) > dashboardTopRows {
		list = list[:dashboardTopRows]
	}
	return list
}

// ---------------------------------------------------------------- drawing

func (d *Dashboard) render() {
	stats := d.stats[d.account]
	if stats == nil {
		d.page.Objects = nil
		d.page.Refresh()
		return
	}
	now := time.Now()

	var objects int
	var size int64
	var newest time.Time
	var biggest *bucketStats
	kinds := map[fileKind]*kindStat{}
	var recent, largest []objectRef
	failed := 0
	for _, name := range stats.order {
		b := stats.buckets[name]
		if b.err != nil {
			failed++
		}
		if !b.done {
			continue
		}
		objects += b.objects
		size += b.size
		if b.newest.After(newest) {
			newest = b.newest
		}
		if biggest == nil || b.size > biggest.size {
			biggest = b
		}
		for kind, k := range b.kinds {
			total := kinds[kind]
			if total == nil {
				total = &kindStat{}
				kinds[kind] = total
			}
			total.count += k.count
			total.size += k.size
		}
		for _, ref := range b.recent {
			recent = keepTop(recent, ref, func(a, c objectRef) bool { return a.modified.After(c.modified) })
		}
		for _, ref := range b.largest {
			largest = keepTop(largest, ref, func(a, c objectRef) bool { return a.size > c.size })
		}
	}

	// Header: the account, what this page is, and when it was counted.
	meta := "Overview"
	switch {
	case stats.running > 0:
		meta = fmt.Sprintf("Overview  ·  counting %s of %s", formatCount(len(stats.order)-stats.running), countNoun(len(stats.order), "bucket"))
	case !stats.finished.IsZero():
		meta = "Overview  ·  counted " + formatAge(stats.finished, now)
	}
	refresh := newButton(buttonGhost, "Count again", t.IconRefresh, d.Refresh).withTip("Count again  ·  F5\nList every object in every bucket again")
	refresh.SetDisabled(stats.running > 0)
	title := vbox(2,
		newTextView(stats.account, t.TextDisplay, t.ColorText, styleStrong),
		newTextView(meta, t.TextSmall, t.ColorMuted, styleNormal),
	)
	header := container.NewBorder(nil, nil, nil, container.NewCenter(refresh), title)

	pending := func(value string) string {
		if stats.running > 0 {
			return value + "…"
		}
		return value
	}
	largestBucket := "—"
	if biggest != nil {
		largestBucket = biggest.name
	}
	strip := statStrip(
		[2]string{"Buckets", formatCount(len(stats.order))},
		[2]string{"Objects", pending(formatCount(objects))},
		[2]string{"Stored", pending(formatSize(size))},
		[2]string{"Largest bucket", largestBucket},
		[2]string{"Last change", orDash(formatAge(newest, now))},
	)

	page := []fyne.CanvasObject{header, strip}
	if failed > 0 {
		page = append(page, d.failureNote(stats, failed))
	}
	page = append(page,
		container.NewGridWithColumns(2,
			card("Buckets", countNoun(len(stats.order), "bucket"), d.bucketRows(stats, size, now)),
			card("By type", formatSize(size), kindBreakdown(kinds, size)),
		),
		container.NewGridWithColumns(2,
			card("Recently changed", "newest first", d.objectRows(recent, now, false)),
			card("Largest files", "biggest first", d.objectRows(largest, now, true)),
		),
	)
	d.page.Objects = page
	d.page.Refresh()
}

func (d *Dashboard) failureNote(stats *accountStats, failed int) fyne.CanvasObject {
	var first error
	for _, name := range stats.order {
		if err := stats.buckets[name].err; err != nil {
			first = err
			break
		}
	}
	e := explainError(first)
	b := newBanner()
	b.ShowWithTip(toneWarning, fmt.Sprintf("%s could not be counted", countNoun(failed, "bucket")), e.Title+" · "+e.Detail, e.Raw)
	return b.Container
}

// card is a bordered PANEL block with a small title on the left, meta on the
// right, and its rows underneath.
func card(title, meta string, body fyne.CanvasObject) fyne.CanvasObject {
	head := container.NewBorder(nil, nil,
		newTextView(title, t.TextSmall, t.ColorSubtle, styleStrong),
		newTextView(meta, t.TextSmall, t.ColorMuted, styleNormal))
	content := container.NewBorder(inset(t.SpaceCardX, t.SpaceCardY, t.SpaceCardX, t.SpaceCardY, head), nil, nil, nil,
		inset(t.SpaceCardX, 0, t.SpaceCardX, t.SpaceCardY, body))
	frame := canvas.NewRectangle(tc(t.ColorPanel))
	frame.StrokeColor = tc(t.ColorBorder)
	frame.StrokeWidth = 1
	return container.NewStack(frame, content)
}

// columns lays out a row's cells at fixed widths; a width of 0 takes what is
// left. Every cell gets the full row height.
type columns struct{ widths []float32 }

func (c *columns) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	flex := size.Width
	flexCount := 0
	for _, w := range c.widths {
		if w == 0 {
			flexCount++
		}
		flex -= w
	}
	if flexCount > 0 {
		flex = fyne.Max(0, flex/float32(flexCount))
	}
	x := float32(0)
	for i, object := range objects {
		w := c.widths[i]
		if w == 0 {
			w = flex
		}
		object.Move(fyne.NewPos(x, 0))
		object.Resize(fyne.NewSize(w, size.Height))
		x += w
	}
}

func (c *columns) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, object := range objects {
		min := object.MinSize()
		h = fyne.Max(h, min.Height)
		w += c.widths[i]
	}
	return fyne.NewSize(w+60, h)
}

func rightText(value string, size float32, colorName fyne.ThemeColorName, style fyne.TextStyle) *textView {
	v := newTextView(value, size, colorName, style)
	v.SetAlignment(fyne.TextAlignTrailing)
	return v
}

func (d *Dashboard) bucketRows(stats *accountStats, total int64, now time.Time) fyne.CanvasObject {
	var largest int64
	for _, b := range stats.buckets {
		largest = max(largest, b.size)
	}
	rows := []fyne.CanvasObject{d.tableHeader("Bucket", "Objects", "Size", "Changed")}
	for _, name := range stats.order {
		b := stats.buckets[name]
		bucket := name
		label := newElided(name, t.TextBody, t.ColorText, styleStrong)
		icon := newIcon(t.IconBucket, t.ColorAccent)
		objects, sizeText, changed := "…", "", ""
		var bar fyne.CanvasObject = layoutSpacer()
		switch {
		case b.err != nil:
			objects, changed = "—", "failed"
		case b.done:
			objects = formatCount(b.objects)
			sizeText = formatSize(b.size)
			changed = orDash(formatAge(b.newest, now))
			m := newMeter(3, t.ColorAccent)
			m.SetValue(barScale(b.size, largest))
			bar = container.NewCenter(fixed(36, 3, m))
		}
		size := container.NewBorder(nil, nil, bar, nil, rightText(sizeText, t.TextSmall, t.ColorSubtle, styleMono))
		cells := container.New(&columns{widths: []float32{0, 70, 120, 80}},
			container.NewBorder(nil, nil, inset(0, 0, 8, 0, container.NewCenter(icon)), nil, label),
			rightText(objects, t.TextSmall, t.ColorSubtle, styleMono),
			inset(12, 0, 0, 0, size),
			inset(14, 0, 0, 0, newTextView(changed, t.TextSmall, ageColor(b.newest, now), styleNormal)),
		)
		rows = append(rows, d.row(cells, func() { d.mw.selectBucket(stats.account, bucket) }))
	}
	if len(stats.order) == 0 {
		rows = append(rows, inset(0, 10, 0, 10, newTextView("This account has no buckets yet.", t.TextSmall, t.ColorMuted, styleNormal)))
	}
	return vbox(0, rows...)
}

func (d *Dashboard) tableHeader(labels ...string) fyne.CanvasObject {
	cells := []fyne.CanvasObject{newTextView(labels[0], t.TextSmall, t.ColorMuted, styleNormal)}
	for _, label := range labels[1:] {
		cells = append(cells, rightText(label, t.TextSmall, t.ColorMuted, styleNormal))
	}
	cells[3] = inset(14, 0, 0, 0, newTextView(labels[3], t.TextSmall, t.ColorMuted, styleNormal))
	row := container.New(&columns{widths: []float32{0, 70, 120, 80}}, cells...)
	return container.NewBorder(nil, hRule(t.ColorBorderStrong), nil, nil, minHeight(24, row))
}

// row is a 26 px dashboard line with a hover fill and a 1 px rule.
func (d *Dashboard) row(content fyne.CanvasObject, tapped func()) fyne.CanvasObject {
	area := newHoverArea(minHeight(t.HeightRow, inset(4, 0, 4, 0, content)), tapped)
	return container.NewBorder(nil, hRule(t.ColorBorder), nil, nil, area)
}

func (d *Dashboard) objectRows(refs []objectRef, now time.Time, bySize bool) fyne.CanvasObject {
	if len(refs) == 0 {
		return inset(0, 10, 0, 10, newTextView("Nothing counted yet.", t.TextSmall, t.ColorMuted, styleNormal))
	}
	rows := make([]fyne.CanvasObject, 0, len(refs))
	for _, ref := range refs {
		ref := ref
		name := ref.key[strings.LastIndex(ref.key, "/")+1:]
		entry := model.Entry{Key: ref.key, Name: name}
		icon := canvas.NewImageFromResource(entryIcon(entry))
		icon.SetMinSize(fyne.NewSize(t.SizeIcon, t.SizeIcon))
		where := ref.bucket + "/" + strings.TrimSuffix(ref.key, name)
		text := container.New(&paletteTextLayout{},
			newElided(name, t.TextBody, t.ColorText, styleNormal),
			newElided(where, t.TextSmall, t.ColorMuted, styleMono))
		value := rightText(formatAge(ref.modified, now), t.TextSmall, ageColor(ref.modified, now), styleNormal)
		if bySize {
			value = rightText(formatSize(ref.size), t.TextSmall, t.ColorSubtle, styleMono)
		}
		cells := container.NewBorder(nil, nil, inset(0, 0, 8, 0, container.NewCenter(icon)), fixed(80, t.HeightRow, value), text)
		tip := withTip(cells, func() string {
			return name + "\n" + ref.bucket + "/" + ref.key + "\n" + formatSize(ref.size) + "  ·  " + formatTimestamp(ref.modified)
		})
		rows = append(rows, d.row(tip, func() { d.mw.reveal(d.account, ref.bucket, ref.key) }))
	}
	return vbox(0, rows...)
}

// kindBreakdown is a share bar of stored bytes by kind, then a legend row per
// kind: a colour square, the name, the object count, the size and the share.
func kindBreakdown(kinds map[fileKind]*kindStat, total int64) fyne.CanvasObject {
	if total == 0 {
		return inset(0, 10, 0, 10, newTextView("Nothing counted yet.", t.TextSmall, t.ColorMuted, styleNormal))
	}
	order := make([]fileKind, 0, len(kinds))
	for kind := range kinds {
		order = append(order, kind)
	}
	sort.Slice(order, func(i, j int) bool { return kinds[order[i]].size > kinds[order[j]].size })

	parts := make([]shareBarPart, 0, len(order))
	rows := []fyne.CanvasObject{}
	for _, kind := range order {
		k := kinds[kind]
		share := float64(k.size) / float64(total)
		parts = append(parts, shareBarPart{share: share, color: kindColors[kind]})
		mark := canvas.NewRectangle(kindColors[kind])
		mark.SetMinSize(fyne.NewSize(8, 8))
		cells := container.New(&columns{widths: []float32{0, 70, 80, 50}},
			container.NewBorder(nil, nil, inset(0, 0, 8, 0, container.NewCenter(mark)), nil,
				newElided(kindNames[kind], t.TextBody, t.ColorText, styleNormal)),
			rightText(formatCount(k.count), t.TextSmall, t.ColorSubtle, styleMono),
			rightText(formatSize(k.size), t.TextSmall, t.ColorSubtle, styleMono),
			rightText(fmt.Sprintf("%.0f%%", share*100), t.TextSmall, t.ColorMuted, styleMono),
		)
		rows = append(rows, container.NewBorder(nil, hRule(t.ColorBorder), nil, nil, minHeight(t.HeightRow, inset(4, 0, 4, 0, cells))))
	}
	bar := newShareBar(parts)
	return vbox(0, inset(0, 4, 0, 10, bar), vbox(0, rows...))
}

type shareBarPart struct {
	share float64
	color color.Color
}

// newShareBar is an 8 px bar split into coloured parts with 1 px gaps.
func newShareBar(parts []shareBarPart) fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, 0, len(parts))
	for _, part := range parts {
		objects = append(objects, canvas.NewRectangle(part.color))
	}
	return container.New(&shareBarLayout{parts: parts}, objects...)
}

type shareBarLayout struct{ parts []shareBarPart }

func (l *shareBarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, object := range objects {
		w := float32(l.parts[i].share) * size.Width
		if w < 1 {
			object.Hide()
			continue
		}
		object.Show()
		object.Move(fyne.NewPos(x, 0))
		object.Resize(fyne.NewSize(fyne.Max(1, w-1), size.Height))
		x += w
	}
}

func (l *shareBarLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(40, 8) }

// reveal opens an object's folder and selects it.
func (mw *MainWindow) reveal(account, bucket, key string) {
	prefix := ""
	if i := strings.LastIndex(key, "/"); i >= 0 {
		prefix = key[:i+1]
	}
	mw.pushHistory()
	mw.open(model.Session{Account: account, Bucket: bucket, Prefix: prefix})
	mw.browser.selected = map[string]struct{}{key: {}}
	mw.browser.focusKey, mw.browser.anchorKey = key, key
}
