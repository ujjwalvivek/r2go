package ui

import (
	"context"
	"image/color"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/config"
	r2theme "r2go/internal/theme"
)

type Sidebar struct {
	mw        *MainWindow
	Container fyne.CanvasObject
	content   *fyne.Container
	loading   map[string]bool
	errors    map[string]string
}

func NewSidebar(mw *MainWindow) *Sidebar {
	sidebar := &Sidebar{
		mw:      mw,
		content: tightVBox(),
		loading: map[string]bool{},
		errors:  map[string]string{},
	}

	brand := widget.NewLabelWithStyle("R2GO", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	brand.SizeName = fyneTheme.SizeNameSubHeadingText
	subtitle := mutedLabel("R2 Explorer")
	brandBlock := tightVBox(brand, subtitle)
	add := widget.NewButtonWithIcon("Add", fyneTheme.Icon(fyneTheme.IconNameContentAdd), func() { mw.showAccountDialog(nil) })
	add.Importance = widget.LowImportance
	headerRow := container.NewBorder(nil, nil, brandBlock, container.NewCenter(fixedSize(66, 30, add)))
	header := inset(12, 9, 8, 8, headerRow)

	scroll := container.NewVScroll(sidebar.content)
	scroll.SetMinSize(fyne.NewSize(0, 420))
	body := container.NewBorder(header, nil, nil, nil, scroll)
	sidebar.Container = panel(r2theme.ColorSidebar, body)
	sidebar.Reload()
	return sidebar
}

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
		}
	}
	s.render()
	for _, account := range accounts {
		account := account
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
			s.errors[account.Name] = err.Error()
			if len(account.Buckets) == 0 {
				s.mw.status.SetMessage(account.Name + ": could not load buckets")
			}
		} else {
			delete(s.errors, account.Name)
		}
		s.render()
	})
}

func (s *Sidebar) retry(account config.Account) {
	if s.loading[account.Name] {
		return
	}
	s.loading[account.Name] = true
	delete(s.errors, account.Name)
	s.render()
	go s.refreshBuckets(account)
}

func (s *Sidebar) render() {
	accounts := s.mw.cfg.Accounts()
	objects := make([]fyne.CanvasObject, 0, len(accounts)*5+2)
	if len(accounts) == 0 {
		title := widget.NewLabelWithStyle("No accounts", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		hint := mutedLabel("Add an R2 account to get started.")
		hint.Alignment = fyne.TextAlignCenter
		objects = append(objects, inset(12, 22, 12, 0, tightVBox(title, hint)))
	}

	session := s.mw.currentSession()
	for _, account := range accounts {
		account := account
		name := widget.NewLabelWithStyle(account.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		name.Truncation = fyne.TextTruncateEllipsis
		accountIcon := widget.NewIcon(fyneTheme.Icon(fyneTheme.IconNameAccount))
		edit := widget.NewButtonWithIcon("", fyneTheme.Icon(fyneTheme.IconNameSettings), func() {
			copy := account
			s.mw.showAccountDialog(&copy)
		})
		edit.Importance = widget.LowImportance
		accountRow := container.NewBorder(nil, nil, accountIcon, fixedSize(28, 28, edit), name)
		objects = append(objects, inset(10, 6, 7, 3, accountRow))

		buckets := append([]string(nil), account.Buckets...)
		sort.Strings(buckets)
		for _, bucket := range buckets {
			bucket := bucket
			selected := session.Account == account.Name && session.Bucket == bucket
			objects = append(objects, newSidebarBucketRow(bucket, selected, func() {
				s.mw.selectBucket(account.Name, bucket)
				s.render()
			}))
		}

		switch {
		case s.loading[account.Name] && len(buckets) == 0:
			objects = append(objects, inset(30, 4, 8, 5, mutedLabel("Loading buckets…")))
		case s.loading[account.Name]:
			objects = append(objects, inset(30, 3, 8, 4, mutedLabel("Refreshing…")))
		case s.errors[account.Name] != "" && len(buckets) == 0:
			retry := widget.NewButton("Retry", func() { s.retry(account) })
			retry.Importance = widget.LowImportance
			objects = append(objects, inset(28, 4, 8, 5, container.NewHBox(retry)))
		case s.errors[account.Name] != "":
			warning := mutedLabel("Offline · cached buckets")
			warning.Importance = widget.WarningImportance
			objects = append(objects, inset(30, 3, 8, 4, warning))
		case len(buckets) == 0:
			objects = append(objects, inset(30, 3, 8, 4, mutedLabel("No buckets")))
		}
		objects = append(objects, widget.NewSeparator())
	}
	s.content.Objects = objects
	s.content.Refresh()
}

type sidebarBucketRow struct {
	widget.BaseWidget
	label      *widget.Label
	icon       *widget.Icon
	background *canvas.Rectangle
	accent     *canvas.Rectangle
	selected   bool
	hovered    bool
	onTapped   func()
}

func newSidebarBucketRow(name string, selected bool, tapped func()) *sidebarBucketRow {
	row := &sidebarBucketRow{
		label:      widget.NewLabel(name),
		icon:       widget.NewIcon(fyneTheme.Icon(fyneTheme.IconNameStorage)),
		background: canvas.NewRectangle(color.Transparent),
		accent:     canvas.NewRectangle(color.Transparent),
		selected:   selected,
		onTapped:   tapped,
	}
	row.label.Truncation = fyne.TextTruncateEllipsis
	if selected {
		row.label.TextStyle = fyne.TextStyle{Bold: true}
		row.background.FillColor = fyneTheme.Color(r2theme.ColorRowSelected)
		row.accent.FillColor = fyneTheme.Color(fyneTheme.ColorNamePrimary)
	}
	row.background.CornerRadius = 0
	row.ExtendBaseWidget(row)
	return row
}

func (r *sidebarBucketRow) MinSize() fyne.Size { return fyne.NewSize(210, 32) }

func (r *sidebarBucketRow) CreateRenderer() fyne.WidgetRenderer {
	r.accent.SetMinSize(fyne.NewSize(2, 1))
	body := container.NewBorder(nil, nil, r.icon, nil, r.label)
	content := container.NewBorder(nil, nil, r.accent, nil, inset(r2theme.SpaceRowX, 0, r2theme.SpaceRowX, 0, body))
	return widget.NewSimpleRenderer(container.NewStack(r.background, content))
}

func (r *sidebarBucketRow) Refresh() {
	switch {
	case r.selected:
		r.background.FillColor = fyneTheme.Color(r2theme.ColorRowSelected)
		r.accent.FillColor = fyneTheme.Color(fyneTheme.ColorNamePrimary)
	case r.hovered:
		r.background.FillColor = fyneTheme.Color(r2theme.ColorRowHover)
		r.accent.FillColor = color.Transparent
	default:
		r.background.FillColor = color.Transparent
		r.accent.FillColor = color.Transparent
	}
	r.background.Refresh()
	r.accent.Refresh()
	r.BaseWidget.Refresh()
}

func (r *sidebarBucketRow) Tapped(*fyne.PointEvent) {
	if r.onTapped != nil {
		r.onTapped()
	}
}

func (r *sidebarBucketRow) MouseIn(*desktop.MouseEvent) {
	r.hovered = true
	r.Refresh()
}
func (r *sidebarBucketRow) MouseMoved(*desktop.MouseEvent) {}
func (r *sidebarBucketRow) MouseOut() {
	r.hovered = false
	r.Refresh()
}
