package ui

import (
	"context"
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/config"
	t "r2go/internal/theme"
)

// bucketFilterThreshold is how many buckets it takes before the sidebar
// offers a filter field.
const bucketFilterThreshold = 8

// Sidebar lists accounts as caption headers with their buckets underneath.
type Sidebar struct {
	mw        *MainWindow
	Container fyne.CanvasObject
	content   *fyne.Container
	filterRow fyne.CanvasObject
	filter    *field
	loading   map[string]bool
	errors    map[string]error
	collapsed map[string]bool
}

func NewSidebar(mw *MainWindow) *Sidebar {
	s := &Sidebar{
		mw:        mw,
		content:   vbox(0),
		loading:   map[string]bool{},
		errors:    map[string]error{},
		collapsed: map[string]bool{},
	}

	// The title bar already names the app, so the sidebar starts with its
	// own content: the accounts.
	add := newIconButton(t.IconPlus, "Add account\nConnect another Cloudflare R2 account", func() { mw.showAccountDialog(nil) })
	title := newTextView("Accounts", t.TextSmall, t.ColorMuted, styleNormal)
	header := minHeight(t.HeightBar-4, inset(12, 0, 5, 0, container.NewBorder(nil, nil, title, container.NewCenter(add))))

	s.filter = newField("Filter buckets")
	s.filter.ActionItem = fieldIcon(t.IconSearch)
	s.filter.OnChanged = func(string) { s.render() }
	s.filter.onEscape = func() { s.filter.SetText(""); mw.win.Canvas().Unfocus() }
	s.filterRow = inset(8, 0, 8, 6, s.filter)
	s.filterRow.Hide()

	scroll := container.NewVScroll(inset(0, 0, 0, 8, s.content))
	body := container.NewBorder(vbox(0, header, s.filterRow), nil, nil, nil, scroll)
	s.Container = floatingCard(body)
	s.Reload()
	return s
}

// floatingCard is a PANEL block with a 1 px BORDER edge, for panels that
// float on the window background instead of touching its edges.
func floatingCard(content fyne.CanvasObject) *fyne.Container {
	edge := canvas.NewRectangle(color.Transparent)
	edge.StrokeColor = tc(t.ColorBorder)
	edge.StrokeWidth = 1
	return container.NewStack(newRect(t.ColorPanel), content, edge)
}

// Reload re-reads accounts and refreshes every account's bucket list.
func (s *Sidebar) Reload() {
	accounts := s.mw.cfg.Accounts()
	known := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		known[account.Name] = struct{}{}
		s.loading[account.Name] = true
		delete(s.errors, account.Name)
	}
	for name := range s.loading {
		if _, ok := known[name]; !ok {
			delete(s.loading, name)
			delete(s.errors, name)
			delete(s.collapsed, name)
		}
	}
	s.render()
	s.mw.accountsChanged()
	for _, account := range accounts {
		go s.refreshBuckets(account)
	}
}

func (s *Sidebar) refreshBuckets(account config.Account) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := s.mw.clientFor(account.Name)
	if err == nil {
		var buckets []string
		buckets, err = client.ListBuckets(ctx)
		if err == nil {
			sort.Strings(buckets)
			err = s.mw.cfg.CacheBuckets(account.Name, buckets)
		}
	}
	fyne.Do(func() {
		s.loading[account.Name] = false
		if err != nil {
			s.errors[account.Name] = err
		} else {
			delete(s.errors, account.Name)
		}
		s.render()
	})
}

func (s *Sidebar) retry(name string) {
	account, ok := s.mw.cfg.FindAccount(name)
	if !ok || s.loading[name] {
		return
	}
	s.loading[name] = true
	delete(s.errors, name)
	s.render()
	go s.refreshBuckets(account)
}

func (s *Sidebar) render() {
	accounts := s.mw.cfg.Accounts()
	session := s.mw.currentSession()
	query := strings.ToLower(strings.TrimSpace(s.filter.Text))

	total := 0
	for _, account := range accounts {
		total += len(account.Buckets)
	}
	if total > bucketFilterThreshold {
		s.filterRow.Show()
	} else {
		s.filterRow.Hide()
		query = ""
	}

	objects := make([]fyne.CanvasObject, 0, total+len(accounts)*2+1)
	if len(accounts) == 0 {
		hint, hintBox := wrapped("Add a Cloudflare R2 account to start browsing.", t.SizeSmall, t.ColorMuted, styleNormal)
		hint.Alignment = fyne.TextAlignLeading
		add := newButton(buttonGhost, "Add account", t.IconPlus, func() { s.mw.showAccountDialog(nil) })
		objects = append(objects, inset(12, 12, 12, 0, vbox(8,
			newText("No accounts", t.TextBody, t.ColorSubtle, styleStrong), hintBox, container.NewHBox(add))))
	}

	for i, account := range accounts {
		name := account.Name
		buckets := append([]string(nil), account.Buckets...)
		sort.Strings(buckets)
		if query != "" {
			filtered := buckets[:0]
			for _, bucket := range buckets {
				if strings.Contains(strings.ToLower(bucket), query) {
					filtered = append(filtered, bucket)
				}
			}
			buckets = filtered
			if len(buckets) == 0 {
				continue
			}
		}
		if i > 0 {
			objects = append(objects, fixed(1, 8, layoutSpacer()))
		}
		collapsed := s.collapsed[name] && query == ""
		objects = append(objects, newAccountRow(s, account, len(account.Buckets), collapsed))
		if err := s.errors[name]; err != nil && !s.loading[name] {
			objects = append(objects, s.errorRow(name, err, len(account.Buckets) > 0))
		}
		if collapsed {
			continue
		}
		for _, bucket := range buckets {
			bucket := bucket
			selected := session.Account == name && session.Bucket == bucket
			objects = append(objects, newNavItem(t.IconBucket, bucket, selected, func() {
				s.mw.selectBucket(name, bucket)
			}))
		}
		if len(buckets) == 0 && !s.loading[name] && s.errors[name] == nil {
			objects = append(objects, inset(30, 4, 8, 4, newText("No buckets", t.TextSmall, t.ColorMuted, styleNormal)))
		}
	}
	if query != "" && len(objects) == 0 {
		objects = append(objects, inset(12, 12, 12, 0, newText("Nothing matches", t.TextSmall, t.ColorMuted, styleNormal)))
	}
	s.content.Objects = objects
	s.content.Refresh()
}

// accountRow is the caption row for an account: a chevron that folds its
// buckets away, the name and the bucket count. Clicking the row opens the
// account's overview, and the row is marked like a selected item while the
// overview is on screen. Refresh and Edit appear on hover. They keep their space while hidden, and the row stays revealed while
// the pointer is on either button, so nothing shifts or flickers.
type accountRow struct {
	widget.BaseWidget
	sidebar   *Sidebar
	account   config.Account
	collapsed bool
	active    bool
	content   fyne.CanvasObject
	bg        *canvas.Rectangle
	bar       *canvas.Rectangle
	label     *textView
	actions   []*button
	hovered   bool
	onButton  bool
	serial    int
}

func newAccountRow(s *Sidebar, account config.Account, count int, collapsed bool) *accountRow {
	r := &accountRow{sidebar: s, account: account, collapsed: collapsed}
	name := account.Name
	r.active = s.mw.showingDashboard() && s.mw.currentSession().Account == name
	chevron := t.IconChevronD
	if collapsed {
		chevron = t.IconChevronR
	}
	countText := newTextView(formatCount(count), t.TextSmall, t.ColorMuted, styleMono)
	if s.loading[name] {
		countText.SetText("…")
	}
	refresh := newIconButton(t.IconRefresh, "Refresh buckets\nList this account's buckets again", func() { s.retry(name) })
	edit := newIconButton(t.IconPencil, "Edit account\nName, keys and API token, or remove it", func() {
		copy := account
		s.mw.showAccountDialog(&copy)
	})
	fold := newIconButton(chevron, "Fold buckets", func() {
		s.collapsed[name] = !collapsed
		s.render()
	})
	if collapsed {
		fold.SetTip("Show buckets")
	}
	fold.onHover = func(on bool) {
		r.onButton = on
		r.update()
	}
	r.actions = []*button{refresh, edit}
	for _, b := range []*button{fold, refresh, edit} {
		b.compact = true
	}
	for _, b := range r.actions {
		b.SetGhost(true)
		b.onHover = func(on bool) {
			r.onButton = on
			r.update()
		}
	}

	right := []fyne.CanvasObject{}
	if err := s.errors[name]; err != nil && count > 0 {
		e := explainError(err)
		right = append(right, withTip(newIcon(t.IconAlert, t.ColorWarning), func() string {
			return "Showing the saved bucket list\n" + e.Title + ". " + e.Detail
		}))
	}
	right = append(right, inset(0, 0, 4, 0, countText), refresh, edit)

	r.label = newElided(name, t.TextSmall, t.ColorSubtle, styleStrong)
	row := container.NewBorder(nil, nil,
		inset(0, 0, 2, 0, container.NewCenter(fold)),
		container.New(&rowLayout{gap: 0}, right...),
		r.label)
	r.bg = canvas.NewRectangle(color.Transparent)
	r.bar = canvas.NewRectangle(color.Transparent)
	r.bar.SetMinSize(fyne.NewSize(t.WidthAccentBar, 1))
	r.content = container.NewStack(r.bg,
		container.NewBorder(nil, nil, r.bar, nil, minHeight(t.HeightNavItem, inset(2, 0, 4, 0, row))))
	r.ExtendBaseWidget(r)
	r.update()
	return r
}

func (r *accountRow) update() {
	show := r.hovered || r.onButton
	for _, b := range r.actions {
		b.SetGhost(!show)
	}
	switch {
	case r.active:
		r.bg.FillColor, r.bar.FillColor = tc(t.ColorSelected), tc(t.ColorAccent)
		r.label.SetColorName(t.ColorText)
	case show:
		r.bg.FillColor, r.bar.FillColor = tc(t.ColorSurface), color.Transparent
		r.label.SetColorName(t.ColorText)
	default:
		r.bg.FillColor, r.bar.FillColor = color.Transparent, color.Transparent
		r.label.SetColorName(t.ColorSubtle)
	}
	r.bg.Refresh()
	r.bar.Refresh()
}

func (r *accountRow) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.content) }

func (r *accountRow) Tapped(*fyne.PointEvent) {
	tips.Hide(nil)
	r.sidebar.mw.openDashboard(r.account.Name)
}

func (r *accountRow) TappedSecondary(event *fyne.PointEvent) {
	r.sidebar.showAccountMenu(r.account, event.AbsolutePosition)
}

func (r *accountRow) MouseIn(*desktop.MouseEvent) {
	r.hovered = true
	r.update()
}
func (r *accountRow) MouseMoved(*desktop.MouseEvent) {}

// MouseOut waits a moment before hiding the actions: moving onto a button
// leaves the row first, and the button reports itself just after.
func (r *accountRow) MouseOut() {
	r.hovered = false
	r.serial++
	serial := r.serial
	time.AfterFunc(120*time.Millisecond, func() {
		fyne.Do(func() {
			if r.serial == serial {
				r.update()
			}
		})
	})
}

// errorRow appears under an account whose bucket list failed to load. With
// a cached list it is a quiet note; without one it offers a retry.
func (s *Sidebar) errorRow(name string, err error, cached bool) fyne.CanvasObject {
	e := explainError(err)
	if cached {
		return fixed(1, 0, newRect(t.ColorPanel))
	}
	bar := newRect(t.ColorError)
	bar.SetMinSize(fyne.NewSize(t.WidthAccentBar, 1))
	retry := newButton(buttonQuiet, "Retry", "", func() { s.retry(name) })
	title := withTip(newElided(e.Title, t.TextSmall, t.ColorText, styleNormal), func() string {
		return e.Title + "\n" + e.Detail + "\n" + e.Raw
	})
	row := container.NewBorder(nil, nil, nil, retry, title)
	return inset(10, 2, 6, 4, container.NewBorder(nil, nil, bar, nil, inset(8, 0, 0, 0, row)))
}

func (s *Sidebar) showAccountMenu(account config.Account, pos fyne.Position) {
	name := account.Name
	showMenu(s.mw.win, pos,
		menuItem{label: "Edit account…", icon: t.IconPencil, action: func() {
			copy := account
			s.mw.showAccountDialog(&copy)
		}},
		menuItem{label: "Refresh buckets", icon: t.IconRefresh, action: func() { s.retry(name) }},
		menuSeparator,
		menuItem{label: "Remove account…", icon: t.IconTrash, danger: true, action: func() { s.mw.confirmRemoveAccount(name, nil) }},
	)
}

// navItem is a 28 px sidebar row: a 14 px line icon and a name. The checked
// item gets a SELECTED fill, a 2 px accent bar, a stronger name and an
// accent icon.
type navItem struct {
	widget.BaseWidget
	bg       *canvas.Rectangle
	bar      *canvas.Rectangle
	icon     *canvas.Image
	iconName string
	label    *textView
	selected bool
	hovered  bool
	onTapped func()
}

func newNavItem(icon, label string, selected bool, tapped func()) *navItem {
	n := &navItem{
		bg:       canvas.NewRectangle(color.Transparent),
		bar:      canvas.NewRectangle(color.Transparent),
		icon:     newIcon(icon, t.ColorMuted),
		iconName: icon,
		label:    newElided(label, t.TextBody, t.ColorSubtle, styleNormal),
		selected: selected,
		onTapped: tapped,
	}
	n.ExtendBaseWidget(n)
	n.apply()
	return n
}

func (n *navItem) apply() {
	switch {
	case n.selected:
		n.bg.FillColor, n.bar.FillColor = tc(t.ColorSelected), tc(t.ColorAccent)
		setIcon(n.icon, n.iconName, t.ColorAccent)
		n.label.SetColorName(t.ColorText)
		n.label.SetStyle(styleStrong)
	case n.hovered:
		n.bg.FillColor, n.bar.FillColor = tc(t.ColorSurface), color.Transparent
		setIcon(n.icon, n.iconName, t.ColorSubtle)
		n.label.SetColorName(t.ColorText)
		n.label.SetStyle(styleNormal)
	default:
		n.bg.FillColor, n.bar.FillColor = color.Transparent, color.Transparent
		setIcon(n.icon, n.iconName, t.ColorMuted)
		n.label.SetColorName(t.ColorSubtle)
		n.label.SetStyle(styleNormal)
	}
	n.bg.Refresh()
	n.bar.Refresh()
}

func (n *navItem) MinSize() fyne.Size { return fyne.NewSize(120, t.HeightNavItem) }

func (n *navItem) CreateRenderer() fyne.WidgetRenderer {
	n.bar.SetMinSize(fyne.NewSize(t.WidthAccentBar, 1))
	body := container.NewBorder(nil, nil, container.NewCenter(n.icon), nil, inset(8, 0, 0, 0, n.label))
	content := container.NewBorder(nil, nil, n.bar, nil, inset(20, 0, 10, 0, body))
	return widget.NewSimpleRenderer(container.NewStack(n.bg, content))
}

func (n *navItem) Tapped(*fyne.PointEvent) {
	if n.onTapped != nil {
		n.onTapped()
	}
}

func (n *navItem) MouseIn(*desktop.MouseEvent) {
	n.hovered = true
	n.apply()
}
func (n *navItem) MouseMoved(*desktop.MouseEvent) {}
func (n *navItem) MouseOut() {
	n.hovered = false
	n.apply()
}
