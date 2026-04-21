package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const bucketSep = "\x00"

type Sidebar struct {
	mw        *MainWindow
	container *fyne.Container

	tree  *widget.Tree
	empty *widget.Label

	buckets map[string][]string
	loaded  map[string]bool
}

func NewSidebar(mw *MainWindow) *Sidebar {
	s := &Sidebar{
		mw:      mw,
		buckets: map[string][]string{},
		loaded:  map[string]bool{},
	}

	s.tree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			if uid == "" {
				names := make([]widget.TreeNodeID, 0, 8)
				for _, a := range mw.Config().Accounts() {
					names = append(names, a.Name)
				}
				return names
			}
			if _, ok := s.buckets[uid]; ok {
				ids := make([]widget.TreeNodeID, len(s.buckets[uid]))
				for i, b := range s.buckets[uid] {
					ids[i] = uid + bucketSep + b
				}
				return ids
			}
			return nil
		},
		func(uid widget.TreeNodeID) bool {
			if uid == "" {
				return true
			}
			_, isAccount := s.buckets[uid]
			return isAccount
		},
		func(bool) fyne.CanvasObject {
			return newTreeItem()
		},
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			item := obj.(*treeItem)
			item.onMenu = nil
			switch {
			case uid == "":
				item.Set(fyneTheme.Icon(fyneTheme.IconNameStorage), "Buckets")
			case isBucketID(uid):
				acc, bucket := splitBucketID(uid)
				item.Set(fyneTheme.Icon(fyneTheme.IconNameFolder), bucket)
				item.onMenu = func(pos fyne.Position) {
					s.showBucketMenu(acc, bucket, pos)
				}
			default:
				accName := uid
				item.Set(fyneTheme.Icon(fyneTheme.IconNameAccount), accName)
				item.onMenu = func(pos fyne.Position) {
					s.showAccountMenu(accName, pos)
				}
			}
		},
	)
	s.tree.OnSelected = func(uid widget.TreeNodeID) {
		dbgLog("OnSelected: uid=%q", uid)
		if uid == "" {
			return
		}
		if isBucketID(uid) {
			acc, bucket := splitBucketID(uid)
			s.mw.onBucketSelected(acc, bucket)
			return
		}
		s.loadBuckets(uid)
	}

	s.empty = widget.NewLabel("No accounts yet.\nClick + to add one.")
	s.empty.Alignment = fyne.TextAlignCenter
	s.empty.Wrapping = fyne.TextWrapWord
	s.empty.TextStyle = fyne.TextStyle{Italic: true}

	add := widget.NewButtonWithIcon("Add", fyneTheme.Icon(fyneTheme.IconNameContentAdd), func() {
		ShowAccountDialog(mw.Window(), mw.Config(), nil, s.Refresh)
	})
	header := container.NewHBox(
		widget.NewLabelWithStyle("Accounts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewPadded(container.NewHBox()),
		add,
	)

	body := container.NewStack(fixedMin(280, s.tree), s.empty)
	root := container.NewBorder(header, nil, nil, nil, body)
	s.container = container.NewPadded(root)
	s.Refresh()
	if accs := mw.Config().Accounts(); len(accs) > 0 {
		s.tree.Select(accs[0].Name)
	}
	dbgDump("sidebar-built", s)
	go dbgDumpLater(s, 2*time.Second, "t+2s")
	go dbgDumpLater(s, 4*time.Second, "t+4s")
	return s
}

func (s *Sidebar) Container() *fyne.Container { return s.container }

func (s *Sidebar) Refresh() {
	has := len(s.mw.Config().Accounts()) > 0
	s.tree.Refresh()
	if has {
		s.empty.Hide()
		s.tree.Show()
	} else {
		s.tree.Hide()
		s.empty.Show()
	}
}

func (s *Sidebar) loadBuckets(account string) {
	if s.loaded[account] {
		s.tree.OpenBranch(account)
		return
	}
	s.mw.statusBar.SetMessage("Loading buckets for " + account + "…")
	go func() {
		client, err := s.mw.clientFor(account)
		var names []string
		if err == nil {
			names, err = client.ListBuckets(context.Background())
		}
		fyne.Do(func() {
			if err != nil {
				ShowError(s.mw.Window(), "Load buckets", fmt.Errorf("%s: %w", account, err))
				s.mw.notifyStatus("Buckets failed for " + account)
				return
			}
			s.loaded[account] = true
			s.buckets[account] = names
			s.tree.RefreshItem(account)
			s.tree.OpenBranch(account)
			s.mw.notifyStatus("Buckets loaded for " + account)
		})
	}()
}

func (s *Sidebar) showBucketMenu(account, bucket string, pos fyne.Position) {
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Mount as drive…", func() { s.mw.mountBucket(account, bucket) }),
	)
	if s.mw.mounts.Find(account, bucket) != nil {
		menu = fyne.NewMenu("",
			fyne.NewMenuItem("Unmount", func() {
				if err := s.mw.mounts.Unmount(account, bucket); err != nil {
					ShowError(s.mw.Window(), "Unmount", err)
					return
				}
				s.mw.notifyStatus("Unmounted " + bucket)
			}),
		)
	}
	pop := widget.NewPopUpMenu(menu, s.mw.Window().Canvas())
	pop.ShowAtPosition(pos)
}

func (s *Sidebar) showAccountMenu(name string, pos fyne.Position) {
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Edit", func() {
			if acc, ok := s.mw.Config().FindAccount(name); ok {
				ShowAccountDialog(s.mw.Window(), s.mw.Config(), &acc, s.Refresh)
			}
		}),
		fyne.NewMenuItem("Delete", func() {
			dialog.ShowConfirm("Delete account",
				"Delete account \""+name+"\"? Credentials will be removed from the keychain.",
				func(ok bool) {
					if !ok {
						return
					}
					if err := s.mw.Config().DeleteAccount(name); err != nil {
						ShowError(s.mw.Window(), "Delete account", err)
						return
					}
					delete(s.buckets, name)
					delete(s.loaded, name)
					s.mw.notifyStatus("Account deleted: " + name)
					s.Refresh()
				}, s.mw.Window())
		}),
	)
	pop := widget.NewPopUpMenu(menu, s.mw.Window().Canvas())
	pop.ShowAtPosition(pos)
}

func isBucketID(uid string) bool {
	return strings.Contains(uid, bucketSep)
}

func splitBucketID(uid string) (account, bucket string) {
	parts := strings.SplitN(uid, bucketSep, 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

type treeItem struct {
	widget.BaseWidget
	icon  *widget.Icon
	label *widget.Label
	onMenu func(fyne.Position)
}

func newTreeItem() *treeItem {
	i := &treeItem{
		icon:  widget.NewIcon(nil),
		label: widget.NewLabel(""),
	}
	i.label.Truncation = fyne.TextTruncateEllipsis
	i.ExtendBaseWidget(i)
	return i
}

func (i *treeItem) Set(icon fyne.Resource, text string) {
	i.icon.SetResource(icon)
	i.label.SetText(text)
}

func (i *treeItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, i.icon, nil, i.label))
}

func (i *treeItem) TappedSecondary(e *fyne.PointEvent) {
	if i.onMenu != nil {
		i.onMenu(e.AbsolutePosition)
	}
}
