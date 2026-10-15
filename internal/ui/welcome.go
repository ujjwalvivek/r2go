package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	t "r2go/internal/theme"
	"r2go/internal/util"
)

const tokenDocsURL = "https://developers.cloudflare.com/r2/api/tokens/"

// Welcome replaces the browser while no account exists: a centred card that
// says what r2go needs and takes the first account inline.
type Welcome struct {
	mw        *MainWindow
	Container fyne.CanvasObject
	card      *fyne.Container
	form      *accountForm
}

func NewWelcome(mw *MainWindow) *Welcome {
	w := &Welcome{mw: mw}
	w.card = container.NewStack()
	scroll := container.NewVScroll(container.NewCenter(inset(0, t.SpacePageY*2, 0, t.SpacePageY*2, w.card)))
	w.Container = panelBox(t.ColorBase, scroll)
	w.Container.Hide()
	return w
}

func (w *Welcome) Show() {
	w.build()
	w.Container.Show()
}

func (w *Welcome) Hide() {
	w.Container.Hide()
	w.form = nil
}

func (w *Welcome) build() {
	mw := w.mw
	w.form = newAccountForm(mw, nil)
	w.form.onSubmit = w.add

	mark := canvas.NewImageFromResource(mw.icon)
	mark.FillMode = canvas.ImageFillContain
	mark.SetMinSize(fyne.NewSize(28, 28))
	title := newText("Connect a Cloudflare R2 account", t.TextDisplay, t.ColorText, styleStrong)
	_, intro := wrapped("r2go needs an R2 API token: an access key ID and its secret, plus your account ID. "+
		"Create one in the Cloudflare dashboard under R2 → Manage API tokens. Scope it to the buckets you want to browse.",
		t.SizeBody, t.ColorSubtle, styleNormal)
	docs := newButton(buttonQuiet, "How to create an R2 token", t.IconExternal, func() {
		if err := util.Open(tokenDocsURL); err != nil {
			mw.reportError("Could not open the link", err)
		}
	})

	add := newButton(buttonPrimary, "Add account", t.IconPlus, w.add)
	footer := container.NewBorder(nil, nil, docs, add)

	facts := vbox(2,
		factRow("Keys", "Saved in "+mw.cfg.SecretsLocation(), false),
		factRow("Settings", mw.cfg.Dir(), true),
	)

	body := vbox(14,
		container.New(&rowLayout{gap: 10}, mark, title),
		intro,
		hRule(t.ColorBorder),
		w.form.Content,
		hRule(t.ColorBorder),
		footer,
	)
	frame := newRect(t.ColorPanel)
	frame.StrokeColor = tc(t.ColorBorder)
	frame.StrokeWidth = 1
	card := container.NewStack(frame, inset(24, 20, 24, 20, body))
	w.card.Objects = []fyne.CanvasObject{vbox(12, fixedWidth(560, card), fixedWidth(560, inset(4, 0, 4, 0, facts)))}
	w.card.Refresh()
}

func (w *Welcome) add() {
	if w.form != nil && w.form.Save() {
		w.mw.status.SetMessage("Account added · loading buckets")
	}
}

// fixedWidth gives content an exact width and its natural height.
func fixedWidth(width float32, object fyne.CanvasObject) *fyne.Container {
	return container.New(&fixedWidthLayout{width: width}, object)
}

type fixedWidthLayout struct{ width float32 }

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(fyne.NewSize(l.width, size.Height))
	}
}

func (l *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, object := range objects {
		h = fyne.Max(h, object.MinSize().Height)
	}
	return fyne.NewSize(l.width, h)
}
