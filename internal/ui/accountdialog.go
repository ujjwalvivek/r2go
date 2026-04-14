package ui

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/config"
)

func ShowAccountDialog(win fyne.Window, cfg *config.Manager, existing *config.Account, onDone func()) {
	title, confirm := "Add Account", "Save"
	if existing != nil {
		title, confirm = "Edit Account", "Save changes"
	}

	name := widget.NewEntry()
	name.SetPlaceHolder("e.g. Production")
	accID := widget.NewEntry()
	accID.SetPlaceHolder("Cloudflare Account ID (dash URL segment)")
	accID.Validator = requiredValidator("account id")
	key := widget.NewEntry()
	key.SetPlaceHolder("R2 API Token Access Key ID")
	key.Validator = requiredValidator("access key id")
	secret := widget.NewPasswordEntry()
	secret.SetPlaceHolder("R2 API Token Secret Access Key")

	name.SetText(existingName(existing))
	accID.SetText(existingField(existing, func(a config.Account) string { return a.AccountID }))
	key.SetText(existingField(existing, func(a config.Account) string { return a.AccessKeyID }))

	if existing != nil {
		secret.SetPlaceHolder("Leave blank to keep the stored secret")
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Name", wideEntry(360, name)),
		widget.NewFormItem("Account ID", wideEntry(360, accID)),
		widget.NewFormItem("Access Key ID", wideEntry(360, key)),
		widget.NewFormItem("Secret Access Key", wideEntry(360, secret)),
	}

	d := dialog.NewForm(title, confirm, "Cancel", items, func(ok bool) {
		if !ok {
			onDone()
			return
		}
		if existing == nil && secret.Text == "" {
			ShowError(win, title, errors.New("secret access key is required"))
			return
		}
		acc := config.Account{
			Name:        name.Text,
			AccountID:   accID.Text,
			AccessKeyID: key.Text,
		}
		var err error
		if existing == nil {
			err = cfg.AddAccount(acc, secret.Text)
		} else {
			setSecret := secret.Text != ""
			err = cfg.UpdateAccount(existing.Name, acc, secret.Text, setSecret)
		}
		if err != nil {
			ShowError(win, title, err)
			return
		}
		onDone()
	}, win)
	d.Show()
}

func requiredValidator(field string) fyne.StringValidator {
	return func(s string) error {
		if s == "" {
			return errors.New(field + " is required")
		}
		return nil
	}
}

func existingName(existing *config.Account) string {
	if existing == nil {
		return ""
	}
	return existing.Name
}

func existingField(existing *config.Account, get func(config.Account) string) string {
	if existing == nil {
		return ""
	}
	return get(*existing)
}

func ShowError(win fyne.Window, title string, err error) {
	dialog.NewError(err, win).Show()
}
