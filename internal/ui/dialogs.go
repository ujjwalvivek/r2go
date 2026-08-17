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

	"r2go/internal/config"
	"r2go/internal/r2"
)

func (mw *MainWindow) showError(title string, err error) {
	if err == nil {
		return
	}
	dialog.ShowError(fmt.Errorf("%s: %w", title, err), mw.win)
}

func (mw *MainWindow) showAccountDialog(existing *config.Account) {
	editing := existing != nil
	name := widget.NewEntry()
	accountID := widget.NewEntry()
	accessKey := widget.NewEntry()
	secret := widget.NewPasswordEntry()
	apiToken := widget.NewPasswordEntry()
	clearToken := widget.NewCheck("Remove saved API token", nil)
	clearToken.Hide()

	if editing {
		name.SetText(existing.Name)
		accountID.SetText(existing.AccountID)
		accessKey.SetText(existing.AccessKeyID)
		secret.SetPlaceHolder("Leave blank to keep saved secret")
		apiToken.SetPlaceHolder("Leave blank to keep saved token")
		if _, ok := mw.cfg.APIToken(existing.Name); ok {
			clearToken.Show()
		}
	} else {
		name.SetPlaceHolder("e.g. Personal")
		accountID.SetPlaceHolder("Cloudflare Account ID")
		accessKey.SetPlaceHolder("R2 Access Key ID")
		secret.SetPlaceHolder("R2 Secret Access Key")
		apiToken.SetPlaceHolder("Optional, for automatic CDN discovery")
	}

	status := mutedLabel("")
	var test *widget.Button
	test = widget.NewButtonWithIcon("Test connection", fyneTheme.Icon(fyneTheme.IconNameViewRefresh), func() {
		account := config.Account{Name: strings.TrimSpace(name.Text), AccountID: strings.TrimSpace(accountID.Text), AccessKeyID: strings.TrimSpace(accessKey.Text)}
		value := secret.Text
		if editing && value == "" {
			value, _ = mw.cfg.S3Secret(existing.Name)
		}
		if account.AccountID == "" || account.AccessKeyID == "" || value == "" {
			status.SetText("Enter Account ID, access key and secret first.")
			return
		}
		status.SetText("Testing…")
		test.Disable()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			buckets, err := r2.New(account, value).ListBuckets(ctx)
			fyne.Do(func() {
				test.Enable()
				if err != nil {
					status.SetText("Connection failed: " + err.Error())
					return
				}
				status.SetText(fmt.Sprintf("Connected: %d bucket%s found.", len(buckets), plural(len(buckets))))
			})
		}()
	})
	test.Importance = widget.LowImportance

	form := container.NewVBox(
		formRow("Name", name),
		formRow("Account ID", accountID),
		formRow("Access Key ID", accessKey),
		formRow("Secret Access Key", secret),
		widget.NewSeparator(),
		formRow("Cloudflare API token", apiToken),
		mutedLabel("Optional. Used only to discover a bucket's custom/r2.dev public domain."),
		clearToken,
		container.NewBorder(nil, nil, test, status),
	)

	closeAccountDialog := func() {}
	if editing {
		deleteButton := widget.NewButton("Delete account", func() {
			dialog.ShowConfirm("Delete account", "Remove this account and its saved credentials from R2GO?", func(ok bool) {
				if !ok {
					return
				}
				if err := mw.cfg.DeleteAccount(existing.Name); err != nil {
					mw.showError("Delete account", err)
					return
				}
				mw.invalidateClient(existing.Name)
				if mw.currentSession().Account == existing.Name {
					mw.clearSession()
				}
				mw.sidebar.Reload()
				closeAccountDialog()
			}, mw.win)
		})
		deleteButton.Importance = widget.DangerImportance
		form.Add(container.NewBorder(nil, nil, deleteButton, nil))
	}

	title := "Add R2 account"
	if editing {
		title = "Edit R2 account"
	}
	confirm := dialog.NewCustomConfirm(title, "Save", "Cancel", inset(4, 2, 4, 0, form), func(ok bool) {
		if !ok {
			return
		}
		account := config.Account{
			Name:        strings.TrimSpace(name.Text),
			AccountID:   strings.TrimSpace(accountID.Text),
			AccessKeyID: strings.TrimSpace(accessKey.Text),
		}
		if editing {
			wasCurrent := mw.currentSession().Account == existing.Name
			replaceAPI := apiToken.Text != "" || clearToken.Checked
			token := apiToken.Text
			if clearToken.Checked {
				token = ""
			}
			if err := mw.cfg.UpdateAccount(existing.Name, account, secret.Text, token, secret.Text != "", replaceAPI); err != nil {
				mw.showError("Save account", err)
				return
			}
			mw.invalidateClient(existing.Name)
			mw.invalidateClient(account.Name)
			if wasCurrent {
				mw.clearSession()
			}
		} else {
			if secret.Text == "" {
				mw.showError("Save account", fmt.Errorf("Secret Access Key is required"))
				return
			}
			if err := mw.cfg.AddAccount(account, secret.Text, apiToken.Text); err != nil {
				mw.showError("Save account", err)
				return
			}
		}
		mw.sidebar.Reload()
		mw.status.SetMessage("Account saved")
	}, mw.win)
	closeAccountDialog = confirm.Hide
	confirm.Resize(fyne.NewSize(560, 500))
	confirm.Show()
}

func formRow(label string, object fyne.CanvasObject) fyne.CanvasObject {
	caption := widget.NewLabelWithStyle(label, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
	return container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(132, 34), caption), nil,
		container.NewGridWrap(fyne.NewSize(360, 34), object))
}

func (mw *MainWindow) showSettings() {
	settings := mw.cfg.Settings()
	themeChoice := widget.NewRadioGroup([]string{"AMOLED Dark", "Light"}, nil)
	if settings.Theme == "light" {
		themeChoice.SetSelected("Light")
	} else {
		themeChoice.SetSelected("AMOLED Dark")
	}

	downloadDir := widget.NewEntry()
	downloadDir.SetText(settings.DownloadDir)
	browse := widget.NewButtonWithIcon("Browse…", fyneTheme.Icon(fyneTheme.IconNameFolderOpen), func() {
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				downloadDir.SetText(uri.Path())
			}
		}, mw.win)
		picker.SetView(dialog.ListView)
		picker.SetTitleText("Choose download folder")
		picker.Resize(fyne.NewSize(760, 520))
		picker.Show()
	})
	browse.Importance = widget.LowImportance

	appearanceTitle := widget.NewLabelWithStyle("Appearance", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	downloadTitle := widget.NewLabelWithStyle("Downloads", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content := container.NewVBox(
		appearanceTitle,
		mutedLabel("AMOLED Dark uses near-black surfaces with an orange accent."),
		themeChoice,
		widget.NewSeparator(),
		downloadTitle,
		container.NewBorder(nil, nil, nil, browse, downloadDir),
		mutedLabel("Folder downloads preserve their directory structure."),
	)

	confirm := dialog.NewCustomConfirm("Settings", "Save", "Cancel", inset(4, 2, 4, 0, content), func(ok bool) {
		if !ok {
			return
		}
		settings.DownloadDir = strings.TrimSpace(downloadDir.Text)
		if themeChoice.Selected == "Light" {
			settings.Theme = "light"
		} else {
			settings.Theme = "dark"
		}
		if err := mw.cfg.UpdateSettings(settings); err != nil {
			mw.showError("Save settings", err)
			return
		}
		mw.applyTheme(settings.Theme)
		mw.status.SetMessage("Settings saved")
	}, mw.win)
	confirm.Resize(fyne.NewSize(540, 330))
	confirm.Show()
}

func (mw *MainWindow) showPublicDomainDialog() {
	session := mw.currentSession()
	if session.Bucket == "" {
		return
	}
	domainEntry := widget.NewEntry()
	domainEntry.SetText(mw.cfg.PublicDomain(session.Account, session.Bucket))
	domainEntry.SetPlaceHolder("cdn.example.com")
	status := mutedLabel("Public URLs are built as https://domain/object-key")

	var discover *widget.Button
	discover = widget.NewButton("Discover from Cloudflare", func() {
		token, ok := mw.cfg.APIToken(session.Account)
		if !ok || token == "" {
			status.SetText("Add a Cloudflare API token to this account to auto-discover domains.")
			return
		}
		account, ok := mw.cfg.FindAccount(session.Account)
		if !ok {
			return
		}
		discover.Disable()
		status.SetText("Discovering public domains…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			domains, err := mw.cf.PublicDomains(ctx, account.AccountID, token, session.Bucket)
			fyne.Do(func() {
				discover.Enable()
				if err != nil {
					status.SetText("Discovery failed: " + err.Error())
					return
				}
				if len(domains) == 0 {
					status.SetText("No enabled public domain found for this bucket.")
					return
				}
				domainEntry.SetText(domains[0].Domain)
				status.SetText(fmt.Sprintf("Found %d public domain%s.", len(domains), plural(len(domains))))
			})
		}()
	})
	discover.Importance = widget.LowImportance

	content := container.NewVBox(
		widget.NewLabelWithStyle(session.Bucket, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		mutedLabel("Use the bucket's CDN/custom domain so Copy URL produces your real production link."),
		domainEntry,
		container.NewBorder(nil, nil, discover, status),
	)
	confirm := dialog.NewCustomConfirm("Public URL", "Save", "Cancel", inset(4, 2, 4, 0, content), func(ok bool) {
		if !ok {
			return
		}
		if err := mw.cfg.SetPublicDomain(session.Account, session.Bucket, domainEntry.Text); err != nil {
			mw.showError("Save public domain", err)
			return
		}
		mw.updatePublicURLState()
	}, mw.win)
	confirm.Resize(fyne.NewSize(560, 270))
	confirm.Show()
}
