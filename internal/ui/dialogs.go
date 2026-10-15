package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"

	"r2go/internal/config"
	"r2go/internal/r2"
	t "r2go/internal/theme"
	"r2go/internal/util"
)

// ---------------------------------------------------------------- account form

// accountForm collects an account's credentials. It is used by the account
// dialog and by the welcome card. Every credential field is masked; the eye
// in each shows the value only while it is held down.
type accountForm struct {
	mw       *MainWindow
	existing *config.Account

	name, accountID, accessKey, secret, apiToken *field
	clearToken                                   *checkbox
	statusIcon                                   *fyne.Container
	status                                       *textView
	statusTip                                    string
	test                                         *button
	Content                                      fyne.CanvasObject
	onSubmit                                     func()
}

func newAccountForm(mw *MainWindow, existing *config.Account) *accountForm {
	f := &accountForm{mw: mw, existing: existing}
	f.name = newField("e.g. Personal")
	f.accountID = newSecretField("32-character account ID")
	f.accessKey = newSecretField("R2 access key ID")
	f.secret = newSecretField("R2 secret access key")
	f.apiToken = newSecretField("Optional")
	f.clearToken = newCheckbox("Remove the saved API token", false, nil)
	f.clearToken.Hide()

	if existing != nil {
		f.name.SetText(existing.Name)
		f.accountID.SetText(existing.AccountID)
		f.accessKey.SetText(existing.AccessKeyID)
		f.secret.SetPlaceHolder("Saved · leave blank to keep")
		f.apiToken.SetPlaceHolder("Leave blank to keep")
		if _, ok := mw.cfg.APIToken(existing.Name); ok {
			f.clearToken.Show()
			f.apiToken.SetPlaceHolder("Saved · leave blank to keep")
		}
	}
	for _, entry := range []*field{f.name, f.accountID, f.accessKey, f.secret, f.apiToken} {
		entry.OnSubmitted = func(string) {
			if f.onSubmit != nil {
				f.onSubmit()
			}
		}
	}

	f.status = newElided("", t.TextSmall, t.ColorMuted, styleNormal)
	f.statusIcon = container.NewCenter()
	f.test = newButton(buttonGhost, "Test connection", t.IconRefresh, f.Test).withTip("Test connection\nList this account's buckets with these credentials, without saving")
	statusLine := container.NewBorder(nil, nil, f.statusIcon, nil, withTip(f.status, func() string {
		if f.statusTip != "" {
			return f.statusTip
		}
		return elidedTip(f.status)()
	}))

	f.Content = vbox(10,
		labelled("Name", f.name, "Only shown in r2go."),
		labelled("Account ID", f.accountID, "Cloudflare dashboard → R2 → Account details."),
		container.NewGridWithColumns(2,
			labelled("Access key ID", f.accessKey),
			labelled("Secret access key", f.secret),
		),
		labelled("Cloudflare API token", f.apiToken, "Optional. Lets r2go find a bucket's public domain; browsing never needs it."),
		f.clearToken,
		container.NewBorder(nil, nil, f.test, nil, inset(10, 0, 0, 0, statusLine)),
	)
	return f
}

func (f *accountForm) setStatus(icon string, colorName fyne.ThemeColorName, text, tip string) {
	f.status.SetText(text)
	f.status.SetColorName(colorName)
	f.statusTip = tip
	if icon == "" {
		f.statusIcon.Objects = nil
	} else {
		f.statusIcon.Objects = []fyne.CanvasObject{inset(0, 0, 6, 0, newIcon(icon, colorName))}
	}
	f.statusIcon.Refresh()
}

func (f *accountForm) values() (config.Account, string) {
	account := config.Account{
		Name:        strings.TrimSpace(f.name.Text),
		AccountID:   strings.TrimSpace(f.accountID.Text),
		AccessKeyID: strings.TrimSpace(f.accessKey.Text),
	}
	secret := strings.TrimSpace(f.secret.Text)
	if secret == "" && f.existing != nil {
		secret, _ = f.mw.cfg.S3Secret(f.existing.Name)
	}
	return account, secret
}

// Test lists buckets with the entered credentials.
func (f *accountForm) Test() {
	account, secret := f.values()
	if account.AccountID == "" || account.AccessKeyID == "" || secret == "" {
		f.setStatus(t.IconAlert, t.ColorWarning, "Enter the account ID, access key and secret first.", "")
		return
	}
	f.setStatus("", t.ColorMuted, "Testing…", "")
	f.test.Disable()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		buckets, err := r2.New(account, secret).ListBuckets(ctx)
		fyne.Do(func() {
			f.test.Enable()
			if err != nil {
				e := explainError(err)
				f.setStatus(t.IconAlert, t.ColorError, e.Title+" · "+e.Detail, e.Title+"\n"+e.Detail+"\n"+e.Raw)
				return
			}
			f.setStatus(t.IconCheck, t.ColorSuccess, fmt.Sprintf("Connected · %s found", countNoun(len(buckets), "bucket")), "")
		})
	}()
}

// Save validates and stores the account. It reports success; on failure the
// reason is shown in the form.
func (f *accountForm) Save() bool {
	account, _ := f.values()
	secret := strings.TrimSpace(f.secret.Text)
	switch {
	case account.Name == "":
		f.setStatus(t.IconAlert, t.ColorWarning, "Give the account a name.", "")
		f.mw.win.Canvas().Focus(f.name)
		return false
	case account.AccountID == "":
		f.setStatus(t.IconAlert, t.ColorWarning, "Enter the Cloudflare account ID.", "")
		f.mw.win.Canvas().Focus(f.accountID)
		return false
	case account.AccessKeyID == "":
		f.setStatus(t.IconAlert, t.ColorWarning, "Enter the R2 access key ID.", "")
		f.mw.win.Canvas().Focus(f.accessKey)
		return false
	case f.existing == nil && secret == "":
		f.setStatus(t.IconAlert, t.ColorWarning, "Enter the R2 secret access key.", "")
		f.mw.win.Canvas().Focus(f.secret)
		return false
	}

	mw := f.mw
	if f.existing != nil {
		wasCurrent := mw.currentSession().Account == f.existing.Name
		token := strings.TrimSpace(f.apiToken.Text)
		replaceAPI := token != "" || f.clearToken.Checked
		if f.clearToken.Checked {
			token = ""
		}
		if err := mw.cfg.UpdateAccount(f.existing.Name, account, secret, token, secret != "", replaceAPI); err != nil {
			f.setStatus(t.IconAlert, t.ColorError, "Could not save: "+err.Error(), "")
			return false
		}
		mw.invalidateClient(f.existing.Name)
		mw.invalidateClient(account.Name)
		if wasCurrent {
			mw.clearSession()
		}
	} else {
		if err := mw.cfg.AddAccount(account, secret, strings.TrimSpace(f.apiToken.Text)); err != nil {
			f.setStatus(t.IconAlert, t.ColorError, "Could not save: "+err.Error(), "")
			return false
		}
	}
	mw.sidebar.Reload()
	mw.status.SetMessage("Saved " + account.Name)
	return true
}

func (mw *MainWindow) showAccountDialog(existing *config.Account) {
	form := newAccountForm(mw, existing)
	title := "Add R2 account"
	confirm := "Add account"
	var footer []fyne.CanvasObject
	if existing != nil {
		title = "Edit " + existing.Name
		confirm = "Save"
	}
	var m *modal
	if existing != nil {
		name := existing.Name
		footer = append(footer, newButton(buttonDanger, "Remove account", t.IconTrash, func() {
			mw.confirmRemoveAccount(name, func() { m.Cancel() })
		}))
	}
	m = showModal(mw.win, modalSpec{
		title:       title,
		width:       520,
		body:        form.Content,
		footerLeft:  footer,
		confirmText: confirm,
		onConfirm:   form.Save,
		focus:       form.name,
	})
	form.onSubmit = m.Confirm
	for _, entry := range form.fields() {
		entry.onEscape = m.Cancel
	}
}

func (f *accountForm) fields() []*field {
	return []*field{f.name, f.accountID, f.accessKey, f.secret, f.apiToken}
}

func (mw *MainWindow) confirmRemoveAccount(name string, after func()) {
	mw.confirm(confirmSpec{
		title:       "Remove " + name + "?",
		message:     "r2go forgets this account and deletes its saved keys from this computer. Nothing in R2 is touched.",
		confirmText: "Remove account",
		danger:      true,
		onConfirm: func() {
			if err := mw.cfg.DeleteAccount(name); err != nil {
				mw.reportError("Could not remove the account", err)
				return
			}
			mw.invalidateClient(name)
			if mw.currentSession().Account == name {
				mw.clearSession()
			}
			mw.sidebar.Reload()
			mw.status.SetMessage("Removed " + name)
			if after != nil {
				after()
			}
		},
	})
}

// ---------------------------------------------------------------- generic dialogs

type confirmSpec struct {
	title       string
	message     string
	detail      fyne.CanvasObject
	confirmText string
	danger      bool
	onConfirm   func()
}

// confirm asks a yes/no question. Enter confirms and Escape cancels.
func (mw *MainWindow) confirm(spec confirmSpec) {
	_, message := wrapped(spec.message, t.SizeBody, t.ColorSubtle, styleNormal)
	body := []fyne.CanvasObject{message}
	if spec.detail != nil {
		body = append(body, spec.detail)
	}
	kind := buttonPrimary
	if spec.danger {
		kind = buttonDanger
	}
	showModal(mw.win, modalSpec{
		title:       spec.title,
		width:       440,
		body:        vbox(10, body...),
		confirmText: spec.confirmText,
		confirmKind: kind,
		onConfirm: func() bool {
			spec.onConfirm()
			return true
		},
	})
}

type promptSpec struct {
	title       string
	caption     string
	value       string
	placeholder string
	hint        string
	confirmText string
	selectStem  bool                // select the name without its extension
	validate    func(string) string // returns a reason, or "" when valid
	onConfirm   func(string)
}

// prompt asks for one line of text, validating as the user types.
func (mw *MainWindow) prompt(spec promptSpec) {
	entry := newField(spec.placeholder)
	entry.SetText(spec.value)
	problem := newTextView("", t.TextSmall, t.ColorError, styleNormal)
	controls := []fyne.CanvasObject{labelled(spec.caption, entry), problem}
	if spec.hint != "" {
		_, hint := wrapped(spec.hint, t.SizeSmall, t.ColorMuted, styleNormal)
		controls = append(controls, hint)
	}
	var m *modal
	check := func(value string) bool {
		reason := ""
		if spec.validate != nil {
			reason = spec.validate(strings.TrimSpace(value))
		}
		problem.SetText(reason)
		if m != nil && m.confirm != nil {
			m.confirm.SetDisabled(reason != "")
		}
		return reason == ""
	}
	entry.OnChanged = func(value string) { check(value) }
	m = showModal(mw.win, modalSpec{
		title:       spec.title,
		width:       420,
		body:        vbox(4, controls...),
		confirmText: spec.confirmText,
		onConfirm: func() bool {
			if !check(entry.Text) {
				return false
			}
			spec.onConfirm(strings.TrimSpace(entry.Text))
			return true
		},
		focus: entry,
	})
	entry.OnSubmitted = func(string) { m.Confirm() }
	entry.onEscape = m.Cancel
	if spec.value != "" {
		check(spec.value)
	} else if spec.validate != nil {
		// An empty field is not an error yet; just keep the button off.
		m.confirm.SetDisabled(spec.validate("") != "")
	}
	if spec.selectStem {
		selectStem(entry)
	}
}

// selectStem selects a file name up to its extension, like a desktop file
// manager does on rename.
func selectStem(entry *field) {
	text := entry.Text
	stem := []rune(text)
	if i := strings.LastIndex(text, "."); i > 0 {
		stem = []rune(text[:i])
	}
	// SetText drops any selection; then, with the cursor placed right after
	// the stem, Shift+Home selects exactly the stem.
	entry.SetText(text)
	entry.CursorRow, entry.CursorColumn = 0, len(stem)
	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyHome})
	entry.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
}

// ---------------------------------------------------------------- settings

func (mw *MainWindow) showSettings() {
	settings := mw.cfg.Settings()

	downloadDir := newField("")
	downloadDir.SetText(settings.DownloadDir)
	browse := newButton(buttonGhost, "Browse…", t.IconFolderOpen, func() {
		start := strings.TrimSpace(downloadDir.Text)
		if start != "" {
			start += string(filepath.Separator)
		}
		mw.pickLocal(true, "Choose the download folder", start, func(paths []string) {
			if len(paths) > 0 && paths[0] != "" {
				downloadDir.SetText(paths[0])
			}
		})
	})

	previews := newCheckbox("Show image previews", !settings.NoThumbnails, nil)
	cacheInfo := newTextView("Measuring…", t.TextSmall, t.ColorMuted, styleMono)
	clearCache := newButton(buttonQuiet, "Clear cache", t.IconTrash, nil)
	clearCache.OnTapped = func() {
		if err := mw.thumbs.Clear(); err != nil {
			cacheInfo.SetText("Could not clear: " + err.Error())
			return
		}
		cacheInfo.SetText("Cache is empty")
		clearCache.Disable()
	}
	go func() {
		size := mw.thumbs.DiskUsage()
		fyne.Do(func() {
			cacheInfo.SetText("Preview cache: " + formatSize(size))
			clearCache.SetDisabled(size == 0)
		})
	}()

	openConfig := newButton(buttonQuiet, "Open folder", t.IconExternal, func() {
		if err := util.Open(mw.cfg.Dir()); err != nil {
			mw.reportError("Could not open the folder", err)
		}
	})

	body := vbox(14,
		section("Downloads",
			labelled("Save downloads to", container.NewBorder(nil, nil, nil, inset(6, 0, 0, 0, browse), downloadDir),
				"Folder downloads keep their folder structure. A name that already exists gets a number added."),
		),
		section("Previews",
			previews,
			wrappedHint("Images up to 100 MB (and SVG and ICO files) are downloaded once to make a small preview for the grid and the details panel. Previews are cached on this computer by version."),
			container.NewBorder(nil, nil, nil, clearCache, cacheInfo),
		),
		section("Storage",
			factRowWithAction("Settings", mw.cfg.Dir(), openConfig),
			factRow("Keys", mw.cfg.SecretsLocation(), false),
		),
	)

	showModal(mw.win, modalSpec{
		title:       "Settings",
		width:       520,
		body:        body,
		confirmText: "Save",
		onConfirm: func() bool {
			settings := mw.cfg.Settings()
			settings.DownloadDir = strings.TrimSpace(downloadDir.Text)
			settings.NoThumbnails = !previews.Checked
			if err := mw.cfg.UpdateSettings(settings); err != nil {
				mw.reportError("Could not save settings", err)
				return false
			}
			mw.browser.refreshAll()
			mw.details.Update()
			mw.status.SetMessage("Settings saved")
			return true
		},
	})
}

func wrappedHint(text string) fyne.CanvasObject {
	_, hint := wrapped(text, t.SizeSmall, t.ColorMuted, styleNormal)
	return hint
}

// ---------------------------------------------------------------- public domain

func (mw *MainWindow) showPublicDomainDialog() {
	session := mw.currentSession()
	if session.Bucket == "" {
		return
	}
	domain := newField("cdn.example.com")
	domain.SetText(mw.cfg.PublicDomain(session.Account, session.Bucket))
	status := newElided("", t.TextSmall, t.ColorMuted, styleNormal)

	var discover *button
	discover = newButton(buttonGhost, "Find it for me", t.IconSearch, func() {
		token, ok := mw.cfg.APIToken(session.Account)
		if !ok || token == "" {
			status.SetText("Add a Cloudflare API token to this account first.")
			status.SetColorName(t.ColorWarning)
			return
		}
		account, ok := mw.cfg.FindAccount(session.Account)
		if !ok {
			return
		}
		discover.Disable()
		status.SetText("Asking Cloudflare…")
		status.SetColorName(t.ColorMuted)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			domains, err := mw.cf.PublicDomains(ctx, account.AccountID, token, session.Bucket)
			fyne.Do(func() {
				discover.Enable()
				switch {
				case err != nil:
					e := explainError(err)
					status.SetText(e.Title + " · " + e.Detail)
					status.SetColorName(t.ColorError)
				case len(domains) == 0:
					status.SetText("This bucket has no public domain turned on.")
					status.SetColorName(t.ColorWarning)
				default:
					domain.SetText(domains[0].Domain)
					status.SetText(fmt.Sprintf("Found %s.", countNoun(len(domains), "domain")))
					status.SetColorName(t.ColorSuccess)
				}
			})
		}()
	}).withTip("Find it for me\nUses the account's Cloudflare API token to look up the bucket's custom or r2.dev domain")

	body := vbox(10,
		labelled("Public domain for "+session.Bucket, domain, "Links are built as https://domain/object-key. Use the bucket's custom domain or its r2.dev address."),
		container.NewBorder(nil, nil, discover, nil, inset(10, 0, 0, 0, withTip(status, elidedTip(status)))),
	)
	m := showModal(mw.win, modalSpec{
		title:       "Public URL",
		width:       480,
		body:        body,
		confirmText: "Save",
		onConfirm: func() bool {
			if err := mw.cfg.SetPublicDomain(session.Account, session.Bucket, domain.Text); err != nil {
				status.SetText(err.Error())
				status.SetColorName(t.ColorError)
				return false
			}
			mw.selectionChanged()
			mw.status.SetMessage("Public domain saved")
			return true
		},
		focus: domain,
	})
	domain.OnSubmitted = func(string) { m.Confirm() }
	domain.onEscape = m.Cancel
}
