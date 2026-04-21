package ui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/config"
	"r2go/internal/mount"
	"r2go/internal/s3client"
	"r2go/internal/theme"
	"r2go/internal/transfer"
)

type MainWindow struct {
	app   fyne.App
	win   fyne.Window
	cfg   *config.Manager
	theme *theme.Theme

	sidebar   *Sidebar
	toolbar   *Toolbar
	fileView  *FileView
	statusBar *StatusBar

	transfers      *transfer.Manager
	transfersPanel *TransfersPanel
	mounts         *mount.Manager

	clients map[string]*s3client.Client
}

func NewMainWindow(f fyne.App, cfg *config.Manager, th *theme.Theme) fyne.Window {
	mw := &MainWindow{
		app:     f,
		win:     f.NewWindow("r2go — Cloudflare R2 Explorer"),
		cfg:     cfg,
		theme:   th,
		clients: map[string]*s3client.Client{},
	}

	mw.statusBar = NewStatusBar()
	mw.transfers = transfer.New(cfg.Settings().Concurrency, func() {
		fyne.Do(mw.transfersPanel.Refresh)
	})
	mw.transfersPanel = NewTransfersPanel(mw)
	mw.mounts = mount.NewManager()
	mw.fileView = NewFileView(mw)
	mw.toolbar = NewToolbar(mw)
	mw.sidebar = NewSidebar(mw)

	center := container.NewBorder(mw.toolbar.Container, mw.statusBar.Container, nil, nil,
		container.NewVBox(mw.fileView.Container, mw.transfersPanel.Container))
	split := container.NewHSplit(mw.sidebar.Container(), center)
	split.SetOffset(0.22)

	mw.win.Resize(fyne.NewSize(1280, 800))
	mw.win.SetContent(split)
	mw.win.SetMaster()
	mw.registerShortcuts()
	mw.transfers.OnFail(func(it *transfer.Item) {
		fyne.Do(func() {
			verb := "Upload"
			if it.Kind == transfer.Download {
				verb = "Download"
			}
			mw.notifyStatus(verb + " failed: " + it.Key)
			mw.app.SendNotification(fyne.NewNotification(verb+" failed", it.Key+": "+it.Err.Error()))
		})
	})
	mw.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		mw.onDrop(uris)
	})
	mw.win.SetOnClosed(func() {
		mw.mounts.UnmountAll()
	})
	return mw.win
}

func (mw *MainWindow) mountBucket(account, bucket string) {
	client, err := mw.clientFor(account)
	if err != nil {
		ShowError(mw.win, "Mount", err)
		return
	}
	target := widget.NewEntry()
	target.SetText(mount.DefaultMountTarget(bucket))
	readOnly := widget.NewCheck("Read-only", nil)

	d := dialog.NewForm("Mount bucket", "Mount", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Drive at", wideEntry(360, target)),
		widget.NewFormItem("", readOnly),
	}, func(ok bool) {
		if !ok {
			return
		}
		mw.notifyStatus("Mounting " + bucket + "…")
		mo, err := mw.mounts.Mount(client, account, bucket, "", target.Text, readOnly.Checked)
		if err != nil {
			ShowError(mw.win, "Mount", err)
			return
		}
		mw.notifyStatus("Mounted " + bucket + " at " + mo.Path)
	}, mw.win)
	d.Show()
}

func (mw *MainWindow) Config() *config.Manager { return mw.cfg }
func (mw *MainWindow) Theme() *theme.Theme     { return mw.theme }
func (mw *MainWindow) Window() fyne.Window     { return mw.win }

func (mw *MainWindow) toggleTheme() {
	v := fyneTheme.VariantDark
	if mw.theme.Variant() == fyneTheme.VariantDark {
		v = fyneTheme.VariantLight
	}
	mw.theme.SetVariant(v)
	mw.app.Settings().SetTheme(mw.theme)
	mw.cfg.Settings().Theme = "dark"
	if v == fyneTheme.VariantLight {
		mw.cfg.Settings().Theme = "light"
	}
	_ = mw.cfg.Save()
}

func (mw *MainWindow) notifyStatus(msg string) {
	mw.statusBar.SetMessage(msg)
}

func (mw *MainWindow) clientFor(account string) (*s3client.Client, error) {
	if c, ok := mw.clients[account]; ok {
		return c, nil
	}
	acc, ok := mw.cfg.FindAccount(account)
	if !ok {
		return nil, fmt.Errorf("account %q not found", account)
	}
	secret, ok := mw.cfg.GetSecret(account)
	if !ok {
		return nil, fmt.Errorf("no stored secret for account %q — edit the account to re-enter it", account)
	}
	c := s3client.New(acc, secret)
	mw.clients[account] = c
	return c, nil
}

func (mw *MainWindow) loadPage(sess *Session, token string) (s3client.ListPage, error) {
	dbgLog("loadPage: acc=%q bucket=%q prefix=%q token=%q", sess.AccountName, sess.Bucket, sess.Prefix, token)
	client, err := mw.clientFor(sess.AccountName)
	if err != nil {
		return s3client.ListPage{}, err
	}
	page, err := client.ListPage(context.Background(), sess.Bucket, sess.Prefix, token)
	dbgLog("loadPage done: objects=%d prefixes=%d err=%v", len(page.Objects), len(page.Prefixes), err)
	return page, err
}

func (mw *MainWindow) onBucketSelected(account, bucket string) {
	dbgLog("onBucketSelected: acc=%q bucket=%q", account, bucket)
	if _, err := mw.clientFor(account); err != nil {
		ShowError(mw.win, "Account error", err)
		return
	}
	mw.toolbar.SetActionsEnabled(true)
	mw.fileView.SetSession(Session{AccountName: account, Bucket: bucket})
	mw.notifyStatus("Bucket: " + bucket)
}

func (mw *MainWindow) onNavigatePrefix(prefix string) {
	mw.fileView.NavigateTo(prefix)
}

func (mw *MainWindow) onOpenEntry(e Entry) {
	sess := mw.fileView.session
	client, err := mw.clientFor(sess.AccountName)
	if err != nil {
		ShowError(mw.win, "Open", err)
		return
	}
	dest := mw.tempPath(sess.Bucket, e.Key)
	it := mw.transfers.AddDownload(client, sess.AccountName, sess.Bucket, e.Key, dest)
	it.AutoOpen = true
	mw.notifyStatus("Downloading " + e.Name + "…")
}

func (mw *MainWindow) tempPath(bucket, key string) string {
	base := filepath.Join(os.TempDir(), "r2go", bucket)
	path := filepath.Join(base, filepath.FromSlash(key))
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return path
}

func (mw *MainWindow) onDrop(uris []fyne.URI) {
	sess := mw.fileView.session
	if sess.Bucket == "" {
		mw.notifyStatus("Select a bucket first to upload")
		return
	}
	client, err := mw.clientFor(sess.AccountName)
	if err != nil {
		ShowError(mw.win, "Upload", err)
		return
	}
	count := 0
	for _, u := range uris {
		path := u.Path()
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		if st.IsDir() {
			count += mw.enqueueFolder(client, sess, path)
		} else {
			mw.enqueueFile(client, sess, path)
			count++
		}
	}
	if count > 0 {
		mw.notifyStatus("Enqueued " + itoa(count) + " upload" + plural(count))
	}
}

func (mw *MainWindow) onUploadClick() {
	sess := mw.fileView.session
	if sess.Bucket == "" {
		return
	}
	menu := fyne.NewMenu("Upload",
		fyne.NewMenuItem("New folder…", func() { mw.newFolder() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Files…", func() {
			dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
				if err != nil || rc == nil {
					return
				}
				path := rc.URI().Path()
				_ = rc.Close()
				mw.enqueuePath(path)
			}, mw.win).Show()
		}),
		fyne.NewMenuItem("Folder…", func() {
			dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
				if err != nil || lu == nil {
					return
				}
				mw.enqueuePath(lu.Path())
			}, mw.win).Show()
		}),
	)
	pop := widget.NewPopUpMenu(menu, mw.win.Canvas())
	pop.ShowAtPosition(fyne.NewPos(110, 60))
}

func (mw *MainWindow) enqueuePath(path string) {
	sess := mw.fileView.session
	client, err := mw.clientFor(sess.AccountName)
	if err != nil {
		ShowError(mw.win, "Upload", err)
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		return
	}
	n := 0
	if st.IsDir() {
		n = mw.enqueueFolder(client, sess, path)
	} else {
		mw.enqueueFile(client, sess, path)
		n = 1
	}
	if n > 0 {
		mw.notifyStatus("Enqueued " + itoa(n) + " upload" + plural(n))
	}
}

func (mw *MainWindow) enqueueFile(client *s3client.Client, sess Session, path string) {
	name := filepath.Base(path)
	key := sess.Prefix + name
	mw.transfers.AddUpload(client, sess.AccountName, sess.Bucket, key, path)
}

func (mw *MainWindow) enqueueFolder(client *s3client.Client, sess Session, dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		key := sess.Prefix + filepath.ToSlash(rel)
		mw.transfers.AddUpload(client, sess.AccountName, sess.Bucket, key, path)
		n++
		return nil
	})
	return n
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func (mw *MainWindow) onRefresh() {
	mw.fileView.Reload()
}
