package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneTheme "fyne.io/fyne/v2/theme"

	"r2go/internal/cloudflare"
	"r2go/internal/config"
	"r2go/internal/model"
	"r2go/internal/r2"
	r2theme "r2go/internal/theme"
	"r2go/internal/transfer"
)

type remoteClipboard struct {
	session model.Session
	entries []model.Entry
	cut     bool
}

type MainWindow struct {
	app   fyne.App
	win   fyne.Window
	cfg   *config.Manager
	theme *r2theme.Theme
	cf    *cloudflare.Client

	clientsMu sync.Mutex
	clients   map[string]*r2.Client

	downloadsMu sync.Mutex
	downloads   map[string]struct{}

	transfers     *transfer.Manager
	sidebar       *Sidebar
	toolbar       *Toolbar
	browser       *Browser
	status        *StatusBar
	transferPanel *TransferPanel
	bottom        *fyne.Container
	center        *fyne.Container
	shell         *fyne.Container

	clipboard remoteClipboard
}

func NewMainWindow(app fyne.App, cfg *config.Manager, theme *r2theme.Theme) fyne.Window {
	mw := &MainWindow{
		app:       app,
		win:       app.NewWindow("R2GO — Cloudflare R2 Explorer"),
		cfg:       cfg,
		theme:     theme,
		cf:        cloudflare.New(),
		clients:   map[string]*r2.Client{},
		downloads: map[string]struct{}{},
		transfers: transfer.New(3),
	}
	mw.win.SetPadded(false)

	mw.status = NewStatusBar()
	mw.browser = NewBrowser(mw)
	mw.toolbar = NewToolbar(mw)
	mw.sidebar = NewSidebar(mw)
	mw.transferPanel = NewTransferPanel(mw, mw.transfers)

	mw.bottom = fyne.NewContainerWithLayout(newBottomLayout(), mw.transferPanel.Container, mw.status.Container)
	mw.center = fyne.NewContainerWithLayout(newCenterLayout(), mw.toolbar.Container, mw.browser.Container, mw.bottom)
	mw.shell = newExplorerSplit(mw.sidebar.Container, mw.center)
	mw.win.SetContent(mw.shell)
	mw.win.Resize(fyne.NewSize(1280, 760))
	mw.win.SetMaster()
	mw.registerShortcuts()

	mw.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		mw.handleDrop(uris)
	})
	return mw.win
}

func (mw *MainWindow) clientFor(accountName string) (*r2.Client, error) {
	account, ok := mw.cfg.FindAccount(accountName)
	if !ok {
		return nil, fmt.Errorf("account %q no longer exists", accountName)
	}
	mw.clientsMu.Lock()
	if client, ok := mw.clients[accountName]; ok {
		mw.clientsMu.Unlock()
		return client, nil
	}
	mw.clientsMu.Unlock()

	secret, ok := mw.cfg.S3Secret(accountName)
	if !ok || secret == "" {
		return nil, fmt.Errorf("no R2 secret stored for %q — edit the account and enter it again", accountName)
	}
	client := r2.New(account, secret)
	mw.clientsMu.Lock()
	mw.clients[accountName] = client
	mw.clientsMu.Unlock()
	return client, nil
}

func (mw *MainWindow) invalidateClient(accountName string) {
	mw.clientsMu.Lock()
	delete(mw.clients, accountName)
	mw.clientsMu.Unlock()
}

func (mw *MainWindow) currentSession() model.Session {
	if mw.browser == nil {
		return model.Session{}
	}
	return mw.browser.Session()
}

func (mw *MainWindow) selectBucket(account, bucket string) {
	session := model.Session{Account: account, Bucket: bucket}
	mw.toolbar.SetEnabled(true)
	mw.toolbar.SetSearchText("")
	mw.toolbar.SetPath(bucket, "")
	mw.toolbar.SetSelection(0)
	mw.browser.SetSession(session)
	mw.status.SetMessage("Opened " + bucket)
	mw.updatePublicURLState()
	mw.discoverPublicDomain(session)
}

func (mw *MainWindow) clearSession() {
	mw.browser.SetSession(model.Session{})
	mw.toolbar.SetEnabled(false)
	mw.toolbar.SetPath("", "")
	mw.toolbar.SetSelection(0)
	mw.toolbar.SetPublicAvailable(false)
	mw.status.SetRight("")
}

func (mw *MainWindow) navigate(prefix string) {
	session := mw.currentSession()
	if session.Bucket == "" {
		return
	}
	mw.toolbar.SetSearchText("")
	mw.browser.Navigate(prefix)
	mw.toolbar.SetPath(session.Bucket, prefix)
}

func (mw *MainWindow) refresh() {
	if mw.currentSession().Bucket == "" {
		mw.sidebar.Reload()
		return
	}
	mw.browser.Reload()
}

func (mw *MainWindow) selectionChanged(count int) {
	mw.toolbar.SetSelection(count)
	mw.updatePublicURLState()
}

func (mw *MainWindow) updatePublicURLState() {
	session := mw.currentSession()
	domain := mw.cfg.PublicDomain(session.Account, session.Bucket)
	entry, one := mw.browser.SelectedEntry()
	mw.toolbar.SetPublicAvailable(domain != "" && one && !entry.IsDir)
}

func (mw *MainWindow) discoverPublicDomain(session model.Session) {
	if session.Account == "" || session.Bucket == "" || mw.cfg.PublicDomain(session.Account, session.Bucket) != "" {
		return
	}
	token, ok := mw.cfg.APIToken(session.Account)
	if !ok || token == "" {
		return
	}
	account, ok := mw.cfg.FindAccount(session.Account)
	if !ok {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		domains, err := mw.cf.PublicDomains(ctx, account.AccountID, token, session.Bucket)
		if err != nil || len(domains) == 0 {
			return
		}
		preferred := domains[0].Domain
		_ = mw.cfg.SetPublicDomain(session.Account, session.Bucket, preferred)
		fyne.Do(func() {
			if mw.currentSession() == session {
				mw.updatePublicURLState()
				mw.status.SetMessage("Public URL detected: " + preferred)
			}
		})
	}()
}

func (mw *MainWindow) applyTheme(value string) {
	variant := fyneTheme.VariantDark
	if value == "light" {
		variant = fyneTheme.VariantLight
	}
	mw.theme.SetVariant(variant)
	mw.app.Settings().SetTheme(mw.theme)
	mw.win.Canvas().Refresh(mw.win.Content())
}

func (mw *MainWindow) notify(title, body string) {
	mw.app.SendNotification(fyne.NewNotification(title, body))
}

// Simple layouts keep the main shell deterministic and avoid nested VBox
// minimum-size feedback loops that caused the original UI to overlap.
type centerLayout struct{}

func newCenterLayout() *centerLayout { return &centerLayout{} }

func (l *centerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 3 {
		return
	}
	topH := objects[0].MinSize().Height
	bottomH := objects[2].MinSize().Height
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, topH))
	objects[2].Move(fyne.NewPos(0, fyne.Max(topH, size.Height-bottomH)))
	objects[2].Resize(fyne.NewSize(size.Width, bottomH))
	bodyH := fyne.Max(0, size.Height-topH-bottomH)
	objects[1].Move(fyne.NewPos(0, topH))
	objects[1].Resize(fyne.NewSize(size.Width, bodyH))
}

func (l *centerLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 3 {
		return fyne.NewSize(720, 500)
	}
	return fyne.NewSize(720, objects[0].MinSize().Height+objects[1].MinSize().Height+objects[2].MinSize().Height)
}

type bottomLayout struct{}

func newBottomLayout() *bottomLayout { return &bottomLayout{} }

func (l *bottomLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	statusH := objects[1].MinSize().Height
	transferH := float32(0)
	if objects[0].Visible() {
		transferH = objects[0].MinSize().Height
	}
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, transferH))
	objects[1].Move(fyne.NewPos(0, transferH))
	objects[1].Resize(fyne.NewSize(size.Width, statusH))
}

func (l *bottomLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 2 {
		return fyne.NewSize(100, 28)
	}
	height := objects[1].MinSize().Height
	if objects[0].Visible() {
		height += objects[0].MinSize().Height
	}
	return fyne.NewSize(100, height)
}

func newExplorerSplit(sidebar, center fyne.CanvasObject) *fyne.Container {
	return newHSplit(sidebar, center)
}

func (mw *MainWindow) relayoutShell() {
	// Visibility changes (notably the transfer drawer) change MinSize. Fyne
	// does not always propagate that through custom layouts immediately, so
	// explicitly relayout the three shell layers in order.
	for _, c := range []*fyne.Container{mw.bottom, mw.center, mw.shell} {
		if c == nil || c.Layout == nil {
			continue
		}
		c.Layout.Layout(c.Objects, c.Size())
		c.Refresh()
	}
}
