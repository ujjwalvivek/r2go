package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/s3client"
)

func (mw *MainWindow) showItemMenu(e Entry, pos fyne.Position) {
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Open", func() { mw.onOpenEntry(e) }),
	)
	if !e.IsDir {
		menu.Items = append(menu.Items,
			fyne.NewMenuItem("Download", func() { mw.downloadEntries([]Entry{e}) }),
			fyne.NewMenuItem("Copy URL", func() { mw.copyURL(e) }),
			fyne.NewMenuItem("Edit metadata…", func() { mw.editMetadata(e) }),
		)
	}
	menu.Items = append(menu.Items,
		fyne.NewMenuItem("Rename…", func() { mw.renameEntry(e) }),
		fyne.NewMenuItem("Copy to folder…", func() { mw.copyMoveEntries([]Entry{e}, false) }),
		fyne.NewMenuItem("Move to folder…", func() { mw.copyMoveEntries([]Entry{e}, true) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Delete", func() { mw.deleteEntries([]Entry{e}) }),
	)
	pop := widget.NewPopUpMenu(menu, mw.win.Canvas())
	pop.ShowAtPosition(pos)
}

func (mw *MainWindow) showBatchMenu(entries []Entry, pos fyne.Position) {
	n := len(entries)
	label := func(s string) string { return fmt.Sprintf("%s (%d)", s, n) }
	menu := fyne.NewMenu("",
		fyne.NewMenuItem(label("Download"), func() { mw.downloadEntries(entries) }),
		fyne.NewMenuItem(label("Copy to folder"), func() { mw.copyMoveEntries(entries, false) }),
		fyne.NewMenuItem(label("Move to folder"), func() { mw.copyMoveEntries(entries, true) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(label("Delete"), func() { mw.deleteEntries(entries) }),
	)
	pop := widget.NewPopUpMenu(menu, mw.win.Canvas())
	pop.ShowAtPosition(pos)
}

func (mw *MainWindow) currentSession() (Session, *s3client.Client, error) {
	sess := mw.fileView.session
	if sess.Bucket == "" {
		return sess, nil, fmt.Errorf("no bucket selected")
	}
	client, err := mw.clientFor(sess.AccountName)
	return sess, client, err
}

func (mw *MainWindow) downloadEntries(entries []Entry) {
	sess, client, err := mw.currentSession()
	if err != nil {
		ShowError(mw.win, "Download", err)
		return
	}
	dir := mw.cfg.Settings().DownloadDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		ShowError(mw.win, "Download", err)
		return
	}

	var targets []Entry
	for _, e := range entries {
		if e.IsDir {
			objs, err := client.ListAll(context.Background(), sess.Bucket, e.Key)
			if err != nil {
				ShowError(mw.win, "Download", err)
				return
			}
			for _, o := range objs {
				targets = append(targets, Entry{Key: o.Key, Name: filepath.Base(o.Key), IsDir: false})
			}
		} else {
			targets = append(targets, e)
		}
	}
	mw.notifyStatus(fmt.Sprintf("Downloading %d file(s)…", len(targets)))
	for _, e := range targets {
		dest := mw.uniqueDest(filepath.Join(dir, filepath.Base(e.Key)))
		mw.transfers.AddDownload(client, sess.AccountName, sess.Bucket, e.Key, dest)
	}
}

func (mw *MainWindow) uniqueDest(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func (mw *MainWindow) renameEntry(e Entry) {
	name := widget.NewEntry()
	name.SetText(e.Name)
	name.SetPlaceHolder("New name")
	d := dialog.NewForm("Rename", "Rename", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", wideEntry(360, name)),
	}, func(ok bool) {
		if !ok || name.Text == "" || name.Text == e.Name {
			return
		}
		sess, client, err := mw.currentSession()
		if err != nil {
			ShowError(mw.win, "Rename", err)
			return
		}
		parent := strings.TrimSuffix(e.Key, e.Name)
		newKey := parent + name.Text
		if e.IsDir {
			newKey = parent + name.Text + "/"
		}
		mw.notifyStatus("Renaming " + e.Name + "…")
		go func() {
			var err error
			if e.IsDir {
				err = mw.renameFolder(client, sess.Bucket, e.Key, newKey)
			} else {
				if cerr := client.CopyObject(context.Background(), sess.Bucket, e.Key, newKey); cerr == nil {
					err = client.DeleteObject(context.Background(), sess.Bucket, e.Key)
				} else {
					err = cerr
				}
			}
			fyne.Do(func() {
				if err != nil {
					ShowError(mw.win, "Rename", err)
					return
				}
				mw.notifyStatus("Renamed to " + name.Text)
				mw.fileView.Reload()
			})
		}()
	}, mw.win)
	d.Show()
}

func (mw *MainWindow) renameFolder(client *s3client.Client, bucket, srcPrefix, dstPrefix string) error {
	objs, err := client.ListAll(context.Background(), bucket, srcPrefix)
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return nil
	}
	for _, o := range objs {
		dst := dstPrefix + strings.TrimPrefix(o.Key, srcPrefix)
		if err := client.CopyObject(context.Background(), bucket, o.Key, dst); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(objs))
	for _, o := range objs {
		keys = append(keys, o.Key)
	}
	return client.DeleteObjects(context.Background(), bucket, keys)
}

func (mw *MainWindow) copyMoveEntries(entries []Entry, move bool) {
	verb := "Copy"
	if move {
		verb = "Move"
	}
	target := widget.NewEntry()
	target.SetPlaceHolder("folder/prefix (empty = bucket root)")
	d := dialog.NewForm(verb+" to folder", verb, "Cancel", []*widget.FormItem{
		widget.NewFormItem("Target prefix", wideEntry(360, target)),
	}, func(ok bool) {
		if !ok {
			return
		}
		sess, client, err := mw.currentSession()
		if err != nil {
			ShowError(mw.win, verb, err)
			return
		}
		targetPrefix := strings.TrimPrefix(target.Text, "/")
		if targetPrefix != "" && !strings.HasSuffix(targetPrefix, "/") {
			targetPrefix += "/"
		}
		if targetPrefix == sess.Prefix {
			ShowError(mw.win, verb, fmt.Errorf("target is the current folder"))
			return
		}
		mw.notifyStatus(fmt.Sprintf("%s %d item(s)…", verb, len(entries)))
		go func() {
			err := mw.copyMove(client, sess.Bucket, entries, targetPrefix, move)
			fyne.Do(func() {
				if err != nil {
					ShowError(mw.win, verb, err)
					return
				}
				mw.notifyStatus(fmt.Sprintf("%s done", verb))
				mw.fileView.Reload()
			})
		}()
	}, mw.win)
	d.Show()
}

func (mw *MainWindow) copyMove(client *s3client.Client, bucket string, entries []Entry, targetPrefix string, move bool) error {
	for _, e := range entries {
		if e.IsDir {
			objs, err := client.ListAll(context.Background(), bucket, e.Key)
			if err != nil {
				return err
			}
			if len(objs) == 0 {
				continue
			}
			for _, o := range objs {
				dst := targetPrefix + strings.TrimPrefix(o.Key, e.Key)
				if err := client.CopyObject(context.Background(), bucket, o.Key, dst); err != nil {
					return err
				}
			}
			if move {
				keys := make([]string, 0, len(objs))
				for _, o := range objs {
					keys = append(keys, o.Key)
				}
				if err := client.DeleteObjects(context.Background(), bucket, keys); err != nil {
					return err
				}
			}
			continue
		}
		dst := targetPrefix + e.Name
		if err := client.CopyObject(context.Background(), bucket, e.Key, dst); err != nil {
			return err
		}
		if move {
			if err := client.DeleteObject(context.Background(), bucket, e.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mw *MainWindow) deleteEntries(entries []Entry) {
	n := len(entries)
	what := "item"
	if n > 1 {
		what = "items"
	}
	dialog.ShowConfirm("Delete",
		fmt.Sprintf("Delete %d %s? This cannot be undone.", n, what),
		func(ok bool) {
			if !ok {
				return
			}
			sess, client, err := mw.currentSession()
			if err != nil {
				ShowError(mw.win, "Delete", err)
				return
			}
			mw.notifyStatus(fmt.Sprintf("Deleting %d %s…", n, what))
			go func() {
				err := mw.deleteKeys(client, sess.Bucket, entries)
				fyne.Do(func() {
					if err != nil {
						ShowError(mw.win, "Delete", err)
						return
					}
					mw.notifyStatus(fmt.Sprintf("Deleted %d %s", n, what))
					mw.fileView.Reload()
				})
			}()
		}, mw.win)
}

func (mw *MainWindow) deleteKeys(client *s3client.Client, bucket string, entries []Entry) error {
	var keys []string
	for _, e := range entries {
		if e.IsDir {
			objs, err := client.ListAll(context.Background(), bucket, e.Key)
			if err != nil {
				return err
			}
			for _, o := range objs {
				keys = append(keys, o.Key)
			}
		} else {
			keys = append(keys, e.Key)
		}
	}
	return client.DeleteObjects(context.Background(), bucket, keys)
}

func (mw *MainWindow) newFolder() {
	sess, client, err := mw.currentSession()
	if err != nil {
		ShowError(mw.win, "New folder", err)
		return
	}
	name := widget.NewEntry()
	name.SetPlaceHolder("Folder name")
	d := dialog.NewForm("New folder", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", wideEntry(360, name)),
	}, func(ok bool) {
		if !ok || name.Text == "" {
			return
		}
		key := sess.Prefix + strings.TrimSuffix(name.Text, "/") + "/"
		go func() {
			err := client.PutObject(context.Background(), sess.Bucket, key, strings.NewReader(""), 0, "", nil)
			fyne.Do(func() {
				if err != nil {
					ShowError(mw.win, "New folder", err)
					return
				}
				mw.notifyStatus("Folder created: " + name.Text)
				mw.fileView.Reload()
			})
		}()
	}, mw.win)
	d.Show()
}

func (mw *MainWindow) copyURL(e Entry) {
	sess, client, err := mw.currentSession()
	if err != nil {
		ShowError(mw.win, "Copy URL", err)
		return
	}
	if e.IsDir {
		mw.notifyStatus("Folders don't have URLs")
		return
	}
	url, err := client.PresignGet(context.Background(), sess.Bucket, e.Key, 7*24*time.Hour)
	if err != nil {
		ShowError(mw.win, "Copy URL", err)
		return
	}
	mw.win.Clipboard().SetContent(url)
	mw.notifyStatus("Signed URL copied to clipboard (expires in 7 days)")
}

func (mw *MainWindow) editMetadata(e Entry) {
	sess, client, err := mw.currentSession()
	if err != nil {
		ShowError(mw.win, "Metadata", err)
		return
	}
	head, err := client.HeadObject(context.Background(), sess.Bucket, e.Key)
	if err != nil {
		ShowError(mw.win, "Metadata", err)
		return
	}

	ct := widget.NewEntry()
	ct.SetText(derefStr(head.ContentType))
	cc := widget.NewEntry()
	cc.SetText(derefStr(head.CacheControl))

	var metaLines []string
	for k, v := range head.Metadata {
		metaLines = append(metaLines, k+"="+v)
	}
	sort.Strings(metaLines)
	meta := widget.NewMultiLineEntry()
	meta.SetText(strings.Join(metaLines, "\n"))
	meta.SetPlaceHolder("key=value, one per line")
	meta.Wrapping = fyne.TextWrapOff

	d := dialog.NewForm("Metadata — "+e.Name, "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Content-Type", wideEntry(300, ct)),
		widget.NewFormItem("Cache-Control", wideEntry(300, cc)),
		widget.NewFormItem("Custom metadata", sized(360, 120, meta)),
	}, func(ok bool) {
		if !ok {
			return
		}
		parsed := map[string]string{}
		lines := strings.Split(meta.Text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			kv := strings.SplitN(line, "=", 2)
			if len(kv) != 2 {
				ShowError(mw.win, "Metadata", fmt.Errorf("invalid metadata line: %q (expected key=value)", line))
				return
			}
			parsed[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
		mw.notifyStatus("Updating metadata for " + e.Name + "…")
		go func() {
			err := client.UpdateMetadata(context.Background(), sess.Bucket, e.Key,
				ct.Text, cc.Text, parsed)
			fyne.Do(func() {
				if err != nil {
					ShowError(mw.win, "Metadata", err)
					return
				}
				mw.notifyStatus("Metadata saved for " + e.Name)
			})
		}()
	}, mw.win)
	d.Show()
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
