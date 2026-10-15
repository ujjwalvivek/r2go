package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"r2go/internal/cloudflare"
	"r2go/internal/config"
	"r2go/internal/model"
	"r2go/internal/r2"
	t "r2go/internal/theme"
	"r2go/internal/transfer"
)

type remoteClipboard struct {
	session model.Session
	entries []model.Entry
	cut     bool
}

type MainWindow struct {
	app  fyne.App
	win  fyne.Window
	cfg  *config.Manager
	cf   *cloudflare.Client
	icon fyne.Resource

	clientsMu sync.Mutex
	clients   map[string]*r2.Client

	downloadsMu sync.Mutex
	downloads   map[string]struct{}

	transfers     *transfer.Manager
	thumbs        *thumbCache
	sidebar       *Sidebar
	toolbar       *Toolbar
	browser       *Browser
	details       *Details
	selection     *SelectionBar
	status        *StatusBar
	transferPanel *TransferPanel
	progress      *progressLine
	banner        *banner
	welcome       *Welcome
	dashboard     *Dashboard

	center    *fyne.Container
	root      *fyne.Container
	workspace *fyne.Container
	split     *fyne.Container

	// dashAccount is set while an account's overview is on screen instead of
	// a bucket.
	dashAccount   string
	back, forward []model.Session
	clipboard     remoteClipboard
	operations    int
	transferState StatusState
	shift         bool
	typed         string
	typedAt       time.Time
}

func NewMainWindow(app fyne.App, cfg *config.Manager, icon fyne.Resource) fyne.Window {
	mw := &MainWindow{
		app:       app,
		win:       app.NewWindow("R2GO"),
		cfg:       cfg,
		cf:        cloudflare.New(),
		icon:      icon,
		clients:   map[string]*r2.Client{},
		downloads: map[string]struct{}{},
		transfers: transfer.New(3),
		thumbs:    newThumbCache(),
	}
	mw.win.SetPadded(false)

	mw.progress = newProgressLine()
	mw.status = NewStatusBar()
	mw.banner = newBanner()
	mw.browser = NewBrowser(mw)
	mw.details = NewDetails(mw)
	mw.selection = NewSelectionBar(mw)
	mw.toolbar = NewToolbar(mw)
	mw.transferPanel = NewTransferPanel(mw, mw.transfers)
	mw.welcome = NewWelcome(mw)
	mw.dashboard = NewDashboard(mw)
	mw.sidebar = NewSidebar(mw)

	settings := cfg.Settings()
	if settings.HideDetails {
		mw.details.Container.Hide()
	}
	mw.browser.SetView(settings.View)
	mw.toolbar.SetView(settings.View, !settings.HideDetails)

	toolbar := container.New(&bottomEdgeLayout{height: t.ProgressLineH}, mw.toolbar.Container, mw.progress)
	mw.split = container.New(&detailsSplitLayout{}, mw.browser.Container, mw.details.Container)
	mw.workspace = container.NewStack(mw.split, mw.dashboard.Container, mw.welcome.Container)
	mw.center = container.NewBorder(vbox(0, toolbar, mw.banner.Container), nil, nil, nil, mw.workspace)

	// The sidebar floats on the window background with a margin all round.
	// The selection strip, transfers and status bar run the full width under
	// it, so the sidebar gets shorter while they are showing.
	sidebar := container.New(&fixedWidthLayout{width: t.WidthSidebar + 2*t.SpaceGap},
		inset(t.SpaceGap, t.SpaceGap, t.SpaceGap, t.SpaceGap, mw.sidebar.Container))
	shell := container.NewBorder(
		hRule(t.ColorBorderStrong), // the title bar's bottom edge
		vbox(0, mw.selection.Container, mw.transferPanel.Container, mw.status.Container),
		sidebar, nil,
		mw.center)
	mw.root = shell

	tipLayer := newTipLayer()
	tips.push(tipLayer)
	mw.win.SetContent(container.NewStack(panelBox(t.ColorBase, shell), tipLayer.Container))
	mw.win.Resize(fyne.NewSize(1360, 840))
	mw.win.SetMaster()
	mw.registerShortcuts()
	mw.accountsChanged()
	mw.updateLocation()

	mw.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) { mw.handleDrop(uris) })
	// The native window only exists once the app has started.
	app.Lifecycle().SetOnStarted(func() {
		styleNativeWindow(mw.win)
		mw.runDevScript()
	})
	return mw.win
}

// ---------------------------------------------------------------- clients

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
		return nil, fmt.Errorf("no R2 secret stored for %q - edit the account and enter it again", accountName)
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

// ---------------------------------------------------------------- navigation

// currentSession is the location on screen. On an account's overview it
// names only the account.
func (mw *MainWindow) currentSession() model.Session {
	if mw.dashAccount != "" {
		return model.Session{Account: mw.dashAccount}
	}
	if mw.browser == nil {
		return model.Session{}
	}
	return mw.browser.Session()
}

func (mw *MainWindow) showingDashboard() bool { return mw.dashAccount != "" }

func (mw *MainWindow) pushHistory() {
	current := mw.currentSession()
	if current.Account == "" {
		return
	}
	mw.back = append(mw.back, current)
	if len(mw.back) > 100 {
		mw.back = mw.back[1:]
	}
	mw.forward = nil
}

func (mw *MainWindow) selectBucket(account, bucket string) {
	current := mw.currentSession()
	if current.Account == account && current.Bucket == bucket && current.Prefix == "" {
		return
	}
	mw.pushHistory()
	mw.open(model.Session{Account: account, Bucket: bucket})
}

// openDashboard shows an account's overview.
func (mw *MainWindow) openDashboard(account string) {
	if mw.dashAccount == account {
		return
	}
	mw.pushHistory()
	mw.open(model.Session{Account: account})
}

// open shows a location without touching the history. A session without a
// bucket is the account's overview.
func (mw *MainWindow) open(session model.Session) {
	mw.toolbar.SetSearchText("")
	mw.banner.Hide()
	if session.Bucket == "" {
		mw.browser.SetSession(model.Session{})
		mw.dashAccount = session.Account
		mw.split.Hide()
		mw.dashboard.Show(session.Account)
		mw.toolbar.SetEnabled(false)
		mw.workspace.Refresh()
		mw.updateLocation()
		mw.sidebar.render()
		return
	}
	if mw.dashAccount != "" {
		mw.dashAccount = ""
		mw.dashboard.Hide()
		mw.split.Show()
		mw.workspace.Refresh()
	}
	current := mw.browser.Session()
	if current.Account == session.Account && current.Bucket == session.Bucket {
		mw.browser.Navigate(session.Prefix)
	} else {
		mw.browser.SetSession(session)
		mw.discoverPublicDomain(session)
	}
	mw.toolbar.SetEnabled(true)
	mw.updateLocation()
	mw.sidebar.render()
}

// clearSession leaves the current location (after its account changed or
// went away) and goes home: the first account's overview, if there is one.
func (mw *MainWindow) clearSession() {
	mw.back, mw.forward = nil, nil
	mw.dashAccount = ""
	mw.browser.SetSession(model.Session{})
	mw.goHome()
}

// goHome opens the first account's overview, the app's home page.
func (mw *MainWindow) goHome() {
	accounts := mw.cfg.Accounts()
	if len(accounts) == 0 {
		mw.dashAccount = ""
		mw.dashboard.Hide()
		mw.toolbar.SetEnabled(false)
		mw.updateLocation()
		mw.sidebar.render()
		return
	}
	mw.dashAccount = ""
	mw.open(model.Session{Account: accounts[0].Name})
}

func (mw *MainWindow) navigate(prefix string) {
	session := mw.currentSession()
	if session.Bucket == "" || session.Prefix == prefix {
		return
	}
	mw.pushHistory()
	session.Prefix = prefix
	mw.open(session)
}

func (mw *MainWindow) goBack() {
	if len(mw.back) == 0 {
		return
	}
	target := mw.back[len(mw.back)-1]
	mw.back = mw.back[:len(mw.back)-1]
	mw.forward = append(mw.forward, mw.currentSession())
	mw.openFromHistory(target)
}

func (mw *MainWindow) goForward() {
	if len(mw.forward) == 0 {
		return
	}
	target := mw.forward[len(mw.forward)-1]
	mw.forward = mw.forward[:len(mw.forward)-1]
	mw.back = append(mw.back, mw.currentSession())
	mw.openFromHistory(target)
}

// openFromHistory opens a remembered location and then selects the folder
// you came out of, so going back up lands where you were.
func (mw *MainWindow) openFromHistory(target model.Session) {
	if _, ok := mw.cfg.FindAccount(target.Account); !ok {
		return
	}
	from := mw.currentSession()
	mw.open(target)
	mw.selectChildOf(from, target)
}

func (mw *MainWindow) goParent() {
	from := mw.currentSession()
	if from.Bucket == "" || from.Prefix == "" {
		return
	}
	mw.navigate(model.ParentPrefix(from.Prefix))
	mw.selectChildOf(from, mw.currentSession())
}

// selectChildOf marks the folder that leads from "to" back down to "from",
// once the listing arrives.
func (mw *MainWindow) selectChildOf(from, to model.Session) {
	if from.Account != to.Account || from.Bucket != to.Bucket || !strings.HasPrefix(from.Prefix, to.Prefix) || from.Prefix == to.Prefix {
		return
	}
	rest := strings.TrimPrefix(from.Prefix, to.Prefix)
	child := to.Prefix + strings.SplitN(rest, "/", 2)[0] + "/"
	mw.browser.focusKey = child
	mw.browser.anchorKey = child
	mw.browser.selected = map[string]struct{}{child: {}}
}

func (mw *MainWindow) refresh() {
	if mw.dashAccount != "" {
		mw.dashboard.Refresh()
		return
	}
	if mw.currentSession().Bucket == "" {
		mw.sidebar.Reload()
		return
	}
	mw.browser.Reload()
	mw.status.SetMessage("Refreshed")
}

// updateLocation refreshes everything that names the current location: the
// breadcrumb, history buttons, window title and status bar.
func (mw *MainWindow) updateLocation() {
	session := mw.currentSession()
	mw.toolbar.SetHistory(len(mw.back) > 0, len(mw.forward) > 0, session.Prefix != "")
	switch {
	case mw.dashAccount != "":
		mw.toolbar.SetOverview(mw.dashAccount)
		mw.win.SetTitle(mw.dashAccount + " | R2GO")
		mw.status.SetSource(mw.dashAccount + " · overview")
	case session.Bucket == "":
		mw.toolbar.SetPath("", "")
		mw.win.SetTitle("R2GO")
		mw.status.SetSource("")
	default:
		mw.toolbar.SetPath(session.Bucket, session.Prefix)
		mw.win.SetTitle(mw.locationLabel() + " | R2GO")
		mw.status.SetSource(session.Account + " · " + session.Bucket + "/" + session.Prefix)
	}
	mw.selectionChanged()
	mw.updateTotals()
}

// locationLabel is bucket/folder, short enough for titles and messages.
func (mw *MainWindow) locationLabel() string {
	session := mw.currentSession()
	if session.Prefix == "" {
		return session.Bucket
	}
	parts := strings.Split(strings.TrimSuffix(session.Prefix, "/"), "/")
	if len(parts) > 2 {
		return session.Bucket + "/…/" + parts[len(parts)-1]
	}
	return session.Bucket + "/" + strings.TrimSuffix(session.Prefix, "/")
}

// accountsChanged shows the welcome card when there is nothing to browse.
func (mw *MainWindow) accountsChanged() {
	if mw.welcome == nil || mw.workspace == nil {
		return
	}
	if len(mw.cfg.Accounts()) == 0 {
		mw.welcome.Show()
		mw.split.Hide()
		mw.dashboard.Hide()
		mw.dashAccount = ""
	} else {
		mw.welcome.Hide()
		if session := mw.currentSession(); session.Account == "" {
			mw.goHome()
		} else if mw.dashAccount == "" {
			mw.split.Show()
		}
	}
	mw.workspace.Refresh()
}

// ---------------------------------------------------------------- selection and view

func (mw *MainWindow) selectionChanged() {
	if mw.selection == nil || mw.details == nil {
		return
	}
	session := mw.currentSession()
	entries := mw.browser.SelectedEntries()
	mw.selection.Update(entries, mw.cfg.PublicDomain(session.Account, session.Bucket) != "")
	mw.details.Update()
	mw.relayout()
}

func (mw *MainWindow) updateTotals() {
	mw.status.SetTotals(mw.browser.Totals())
}

func (mw *MainWindow) setView(view string) {
	settings := mw.cfg.Settings()
	settings.View = view
	_ = mw.cfg.UpdateSettings(settings)
	mw.browser.SetView(view)
	mw.toolbar.SetView(view, mw.details.Container.Visible())
}

func (mw *MainWindow) toggleDetails() {
	settings := mw.cfg.Settings()
	if mw.details.Container.Visible() {
		mw.details.Container.Hide()
		settings.HideDetails = true
	} else {
		mw.details.Container.Show()
		settings.HideDetails = false
		mw.details.Update()
	}
	_ = mw.cfg.UpdateSettings(settings)
	mw.toolbar.SetView(mw.browser.view, mw.details.Container.Visible())
	mw.split.Refresh()
}

// relayout re-runs the shell layout after a bar appeared or disappeared.
func (mw *MainWindow) relayout() {
	if mw.root != nil {
		mw.root.Refresh()
	}
	if mw.center != nil {
		mw.center.Refresh()
	}
}

// ---------------------------------------------------------------- status and errors

func (mw *MainWindow) setTransferState(state StatusState) {
	mw.transferState = state
	mw.updateStatusState()
}

func (mw *MainWindow) updateStatusState() {
	state := mw.transferState
	if state == StatusReady && mw.operations > 0 {
		state = StatusWorking
	}
	mw.status.SetState(state)
}

// reportError shows a failure as a slim banner above the list, in plain
// words, with the raw error in its tooltip. It stays until dismissed or the
// next location change.
func (mw *MainWindow) reportError(title string, err error) {
	mw.showProblem(toneError, title, err)
}

func (mw *MainWindow) reportWarning(title string, err error) {
	mw.showProblem(toneWarning, title, err)
}

func (mw *MainWindow) showProblem(tone bannerTone, title string, err error) {
	if err == nil {
		return
	}
	e := explainError(err)
	mw.banner.ShowWithTip(tone, title, e.Title+" · "+e.Detail, e.Title+"\n"+e.Raw,
		newButton(buttonQuiet, "Dismiss", "", mw.banner.Hide))
	if tone == toneError {
		mw.status.SetFailure(e)
	}
	mw.relayout()
}

func (mw *MainWindow) notify(title, body string) {
	mw.app.SendNotification(fyne.NewNotification(title, body))
}

// ---------------------------------------------------------------- public domains

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
			if mw.currentSession().Bucket == session.Bucket {
				mw.selectionChanged()
				mw.status.SetMessage("Found the public domain " + preferred)
			}
		})
	}()
}

// ---------------------------------------------------------------- layouts

// bottomEdgeLayout lays the first object over the whole area and pins the
// second to its bottom edge at a fixed height (the progress line under the
// toolbar).
type bottomEdgeLayout struct{ height float32 }

func (l *bottomEdgeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
	objects[1].Move(fyne.NewPos(0, size.Height-l.height))
	objects[1].Resize(fyne.NewSize(size.Width, l.height))
}

func (l *bottomEdgeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return objects[0].MinSize()
}

// detailsSplitLayout gives the details panel a fixed width on the right
// while it is visible.
type detailsSplitLayout struct{}

func (detailsSplitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	main, details := objects[0], objects[1]
	width := float32(0)
	if details.Visible() {
		width = fyne.Min(t.WidthDetails, size.Width*0.45)
	}
	main.Move(fyne.NewPos(0, 0))
	main.Resize(fyne.NewSize(size.Width-width, size.Height))
	details.Move(fyne.NewPos(size.Width-width, 0))
	details.Resize(fyne.NewSize(width, size.Height))
}

func (detailsSplitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(480, 200)
}
