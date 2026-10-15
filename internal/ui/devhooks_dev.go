//go:build dev

package ui

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"r2go/internal/model"
)

// runDevScript drives the window into a given state for screenshots during
// development. It reads R2GO_DEV, a ";"-separated list of steps such as
// "open:Account/bucket/prefix/;wait:1500;select:2;view:grid;help".
// It is only compiled with -tags dev.
func (mw *MainWindow) runDevScript() {
	script := os.Getenv("R2GO_DEV")
	if script == "" {
		return
	}
	go func() {
		time.Sleep(800 * time.Millisecond)
		for _, step := range strings.Split(script, ";") {
			name, arg, _ := strings.Cut(strings.TrimSpace(step), ":")
			if name == "wait" {
				ms, _ := strconv.Atoi(arg)
				time.Sleep(time.Duration(ms) * time.Millisecond)
				continue
			}
			done := make(chan struct{})
			fyne.Do(func() {
				defer close(done)
				mw.devStep(name, arg)
			})
			<-done
			time.Sleep(250 * time.Millisecond)
		}
	}()
}

func (mw *MainWindow) devStep(name, arg string) {
	switch name {
	case "open":
		parts := strings.SplitN(arg, "/", 3)
		session := model.Session{Account: parts[0]}
		if len(parts) > 1 {
			session.Bucket = parts[1]
		}
		if len(parts) > 2 {
			session.Prefix = parts[2]
		}
		mw.pushHistory()
		mw.open(session)
	case "select":
		n, _ := strconv.Atoi(arg)
		mw.browser.ClearSelection()
		for i := 0; i < n; i++ {
			mw.browser.MoveFocus(1, i > 0)
		}
	case "selectname":
		for _, entry := range mw.browser.visible {
			if entry.Name == arg {
				mw.browser.SelectKey(entry.Key)
			}
		}
	case "view":
		mw.setView(arg)
	case "details":
		if (arg == "on") != mw.details.Container.Visible() {
			mw.toggleDetails()
		}
	case "filter":
		mw.toolbar.SetSearchText(arg)
	case "palette":
		mw.showPalette()
	case "type":
		if focused, ok := mw.win.Canvas().Focused().(*field); ok {
			focused.SetText(arg)
		}
	case "help":
		mw.showHelp()
	case "settings":
		mw.showSettings()
	case "newfolder":
		mw.newFolder()
	case "rename":
		mw.renameSelected()
	case "delete":
		mw.deleteSelected()
	case "copyto":
		mw.showCopyMoveDialog(mw.browser.SelectedEntries(), false)
	case "addaccount":
		mw.showAccountDialog(nil)
	case "error":
		mw.reportError("Could not delete 2 items", os.ErrPermission)
	case "tip":
		b := mw.toolbar.upload
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(b)
		tips.Show(b, b.tip, pos.Add(fyne.NewPos(20, 14)))
	case "transfers":
		// Fake uploads that never touch R2: one batch that progresses, one
		// that fails, for checking the transfers drawer.
		group := mw.transfers.NewGroup("upload")
		for i := 0; i < 4; i++ {
			mw.transfers.AddUpload(fakeClient{}, "dev", "bucket", "slow/file"+strconv.Itoa(i), "file"+strconv.Itoa(i), group, "Uploading 4 items to bucket/slow")
		}
		mw.transfers.CloseGroup(group)
		failing := mw.transfers.NewGroup("upload")
		mw.transfers.AddUpload(fakeClient{fail: true}, "dev", "bucket", "broken.bin", "broken.bin", failing, "Uploading broken.bin to bucket")
		mw.transfers.CloseGroup(failing)
	case "path":
		mw.toolbar.EditPath()
	}
}

type fakeClient struct{ fail bool }

func (f fakeClient) UploadFile(ctx context.Context, _, _, _ string, progress func(done, total int64)) error {
	const total = 40 << 20
	for done := int64(0); done <= total; done += 2 << 20 {
		if f.fail && done > total/3 {
			return os.ErrPermission
		}
		progress(done, total)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return nil
}

func (f fakeClient) DownloadFile(ctx context.Context, bucket, key, destination string, progress func(done, total int64)) error {
	return f.UploadFile(ctx, bucket, key, destination, progress)
}
