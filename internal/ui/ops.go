package ui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"r2go/internal/explorer"
	"r2go/internal/model"
	"r2go/internal/util"
)

func (mw *MainWindow) showUploadMenu() {
	if mw.currentSession().Bucket == "" {
		return
	}
	var popup *dialog.CustomDialog
	fileButton := widget.NewButtonWithIcon("Choose a file", fyneTheme.Icon(fyneTheme.IconNameFile), func() {
		popup.Hide()
		picker := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			_ = reader.Close()
			mw.enqueueUploadPaths([]string{path})
		}, mw.win)
		picker.SetView(dialog.ListView)
		picker.SetTitleText("Select file to upload")
		picker.Resize(fyne.NewSize(760, 520))
		picker.Show()
	})
	fileButton.Importance = widget.HighImportance
	folderButton := widget.NewButtonWithIcon("Choose a folder", fyneTheme.Icon(fyneTheme.IconNameFolderOpen), func() {
		popup.Hide()
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			mw.enqueueUploadPaths([]string{uri.Path()})
		}, mw.win)
		picker.SetView(dialog.ListView)
		picker.SetTitleText("Select folder to upload")
		picker.Resize(fyne.NewSize(760, 520))
		picker.Show()
	})
	folderButton.Importance = widget.MediumImportance
	content := container.NewVBox(
		widget.NewLabelWithStyle("Upload to "+mw.currentSession().Bucket, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		mutedLabel("Drag and drop also supports multiple files and folders."),
		fileButton,
		folderButton,
	)
	popup = dialog.NewCustom("Upload", "Cancel", inset(4, 2, 4, 0, content), mw.win)
	popup.Resize(fyne.NewSize(420, 200))
	popup.Show()
}

func (mw *MainWindow) handleDrop(uris []fyne.URI) {
	if mw.currentSession().Bucket == "" || len(uris) == 0 {
		return
	}
	paths := make([]string, 0, len(uris))
	for _, uri := range uris {
		if uri == nil || uri.Path() == "" {
			continue
		}
		paths = append(paths, uri.Path())
	}
	mw.enqueueUploadPaths(paths)
}

func (mw *MainWindow) enqueueUploadPaths(paths []string) {
	session := mw.currentSession()
	client, err := mw.clientFor(session.Account)
	if err != nil {
		mw.showError("Upload", err)
		return
	}
	if len(paths) == 0 {
		return
	}

	groupID := mw.transfers.NewGroup("upload")
	label := fmt.Sprintf("Uploading %d item%s", len(paths), plural(len(paths)))
	if len(paths) == 1 {
		label = "Uploading " + filepath.Base(paths[0])
	}
	mw.status.SetMessage("Preparing upload…")

	// Walking a large local folder can be slow (and can exceed the transfer
	// queue's backpressure limit), so preparation never runs on Fyne's UI goroutine.
	go func() {
		defer mw.transfers.CloseGroup(groupID)
		queued := 0
		emptyFolders := 0
		var firstErr error

		for _, localPath := range paths {
			info, err := os.Stat(localPath)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("inspect %s: %w", localPath, err)
				}
				continue
			}
			if !info.IsDir() {
				key := session.Prefix + filepath.Base(localPath)
				mw.transfers.AddUpload(client, session.Account, session.Bucket, key, localPath, groupID, label)
				queued++
				continue
			}

			rootName := filepath.Base(filepath.Clean(localPath))
			rootPrefix := session.Prefix + rootName + "/"
			files := 0
			walkErr := filepath.WalkDir(localPath, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				relative, err := filepath.Rel(localPath, path)
				if err != nil {
					return err
				}
				key := rootPrefix + filepath.ToSlash(relative)
				mw.transfers.AddUpload(client, session.Account, session.Bucket, key, path, groupID, label)
				files++
				queued++
				return nil
			})
			if walkErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("scan %s: %w", localPath, walkErr)
			}
			if files == 0 && walkErr == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := client.CreateFolder(ctx, session.Bucket, rootPrefix)
				cancel()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
				} else {
					emptyFolders++
				}
			}
		}

		fyne.Do(func() {
			switch {
			case firstErr != nil && queued == 0 && emptyFolders == 0:
				mw.showError("Upload", firstErr)
			case firstErr != nil:
				mw.status.SetMessage(fmt.Sprintf("Queued %d upload%s; some items could not be prepared", queued, plural(queued)))
				mw.notify("Upload preparation warning", firstErr.Error())
			case queued > 0:
				mw.status.SetMessage(fmt.Sprintf("Queued %d upload%s", queued, plural(queued)))
			case emptyFolders > 0:
				mw.status.SetMessage(fmt.Sprintf("Created %d empty folder%s", emptyFolders, plural(emptyFolders)))
				if mw.currentSession() == session {
					mw.browser.Reload()
				}
			default:
				mw.status.SetMessage("Nothing to upload")
			}
		})
	}()
}

func (mw *MainWindow) downloadSelected() {
	entries := mw.browser.SelectedEntries()
	if len(entries) == 0 {
		return
	}
	mw.downloadEntries(entries)
}

func (mw *MainWindow) downloadEntries(entries []model.Entry) {
	session := mw.currentSession()
	client, err := mw.clientFor(session.Account)
	if err != nil {
		mw.showError("Download", err)
		return
	}
	settings := mw.cfg.Settings()
	groupID := mw.transfers.NewGroup("download")
	label := fmt.Sprintf("Downloading %d item%s", len(entries), plural(len(entries)))
	if len(entries) == 1 {
		label = "Downloading " + entries[0].Name
	}
	mw.status.SetMessage("Preparing download…")
	go func() {
		defer mw.transfers.CloseGroup(groupID)
		queued := 0
		createdFolders := 0
		for _, entry := range entries {
			if !entry.IsDir {
				dest := mw.reserveLocalPath(filepath.Join(settings.DownloadDir, entry.Name), false)
				mw.transfers.AddDownload(client, session.Account, session.Bucket, entry.Key, dest, groupID, label)
				queued++
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			objects, err := client.ListAll(ctx, session.Bucket, entry.Key)
			cancel()
			if err != nil {
				fyne.Do(func() { mw.showError("Download folder", err) })
				return
			}
			root := mw.reserveLocalPath(filepath.Join(settings.DownloadDir, entry.Name), true)
			if err := os.MkdirAll(root, 0o755); err != nil {
				fyne.Do(func() { mw.showError("Download folder", err) })
				return
			}
			createdFolders++
			for _, object := range objects {
				relative := strings.TrimPrefix(object.Key, entry.Key)
				if relative == "" || strings.HasSuffix(relative, "/") {
					continue
				}
				dest := filepath.Join(root, filepath.FromSlash(relative))
				mw.transfers.AddDownload(client, session.Account, session.Bucket, object.Key, dest, groupID, label)
				queued++
			}
		}
		fyne.Do(func() {
			switch {
			case queued > 0:
				mw.status.SetMessage(fmt.Sprintf("Queued %d download%s", queued, plural(queued)))
			case createdFolders > 0:
				mw.status.SetMessage(fmt.Sprintf("Created %d local folder%s", createdFolders, plural(createdFolders)))
			default:
				mw.status.SetMessage("Nothing to download")
			}
		})
	}()
}

func (mw *MainWindow) reserveLocalPath(target string, directory bool) string {
	mw.downloadsMu.Lock()
	defer mw.downloadsMu.Unlock()

	available := func(candidate string) bool {
		key := strings.ToLower(filepath.Clean(candidate))
		if _, reserved := mw.downloads[key]; reserved {
			return false
		}
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			return false
		}
		mw.downloads[key] = struct{}{}
		return true
	}

	if available(target) {
		return target
	}
	if directory {
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s (%d)", target, i)
			if available(candidate) {
				return candidate
			}
		}
	}
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(target, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if available(candidate) {
			return candidate
		}
	}
}

func (mw *MainWindow) newFolder() {
	session := mw.currentSession()
	if session.Bucket == "" {
		return
	}
	name := widget.NewEntry()
	name.SetPlaceHolder("Folder name")
	confirm := dialog.NewCustomConfirm("New folder", "Create", "Cancel", inset(4, 2, 4, 0, name), func(ok bool) {
		if !ok {
			return
		}
		value := strings.TrimSpace(name.Text)
		if value == "" || strings.ContainsAny(value, "/\\") {
			mw.showError("Create folder", fmt.Errorf("enter a single folder name"))
			return
		}
		client, err := mw.clientFor(session.Account)
		if err != nil {
			mw.showError("Create folder", err)
			return
		}
		key := model.JoinPrefix(session.Prefix, value, true)
		go func() {
			err := client.CreateFolder(context.Background(), session.Bucket, key)
			fyne.Do(func() {
				if err != nil {
					mw.showError("Create folder", err)
					return
				}
				mw.status.SetMessage("Created " + value)
				if mw.currentSession() == session {
					mw.browser.Reload()
				}
			})
		}()
	}, mw.win)
	confirm.Resize(fyne.NewSize(420, 150))
	confirm.Show()
}

func (mw *MainWindow) renameSelected() {
	entry, ok := mw.browser.SelectedEntry()
	if !ok {
		return
	}
	name := widget.NewEntry()
	name.SetText(entry.Name)
	confirm := dialog.NewCustomConfirm("Rename", "Rename", "Cancel", inset(4, 2, 4, 0, name), func(ok bool) {
		if !ok {
			return
		}
		newName := strings.TrimSpace(name.Text)
		if newName == entry.Name {
			return
		}
		if newName == "" || strings.ContainsAny(newName, "/\\") {
			mw.showError("Rename", fmt.Errorf("enter a single file or folder name"))
			return
		}
		session := mw.currentSession()
		client, err := mw.clientFor(session.Account)
		if err != nil {
			mw.showError("Rename", err)
			return
		}
		mw.runOperation("Renaming "+entry.Name, session, func(ctx context.Context, progress explorer.Progress) error {
			return explorer.Rename(ctx, client, session.Bucket, entry, newName, progress)
		})
	}, mw.win)
	confirm.Resize(fyne.NewSize(420, 150))
	confirm.Show()
}

func (mw *MainWindow) deleteSelected() {
	entries := mw.browser.SelectedEntries()
	if len(entries) == 0 {
		return
	}
	label := entries[0].Name
	if len(entries) > 1 {
		label = fmt.Sprintf("%d selected items", len(entries))
	}
	dialog.ShowConfirm("Delete permanently", "Delete "+label+" from R2? This cannot be undone.", func(ok bool) {
		if !ok {
			return
		}
		session := mw.currentSession()
		client, err := mw.clientFor(session.Account)
		if err != nil {
			mw.showError("Delete", err)
			return
		}
		mw.runOperation("Deleting "+label, session, func(ctx context.Context, progress explorer.Progress) error {
			return explorer.Delete(ctx, client, session.Bucket, entries, progress)
		})
	}, mw.win)
}

func (mw *MainWindow) copySelection(cut bool) {
	entries := mw.browser.SelectedEntries()
	if len(entries) == 0 {
		return
	}
	mw.clipboard = remoteClipboard{
		session: mw.currentSession(),
		entries: append([]model.Entry(nil), entries...),
		cut:     cut,
	}
	verb := "Copied"
	if cut {
		verb = "Cut"
	}
	mw.status.SetMessage(fmt.Sprintf("%s %d item%s - navigate and paste", verb, len(entries), plural(len(entries))))
}

func (mw *MainWindow) pasteClipboard() {
	clip := mw.clipboard
	destination := mw.currentSession()
	if len(clip.entries) == 0 || destination.Bucket == "" {
		return
	}
	if clip.session.Account != destination.Account || clip.session.Bucket != destination.Bucket {
		mw.showError("Paste", fmt.Errorf("copy/move currently works within the same R2 bucket"))
		return
	}
	mw.runCopyMove(clip.entries, clip.session, destination.Prefix, clip.cut, explorer.SkipExisting)
	if clip.cut {
		mw.clipboard = remoteClipboard{}
	}
}

func (mw *MainWindow) showCopyMoveDialog(entries []model.Entry, move bool) {
	if len(entries) == 0 {
		return
	}
	destination := widget.NewEntry()
	destination.SetText(mw.currentSession().Prefix)
	destination.SetPlaceHolder("folder/subfolder/  (blank = bucket root)")
	replace := widget.NewCheck("Replace objects that already exist", nil)
	content := container.NewVBox(
		mutedLabel("Destination prefix inside this bucket"),
		destination,
		replace,
		mutedLabel("By default existing objects are skipped; sources are never deleted when a move is skipped."),
	)
	title := "Copy to…"
	confirmText := "Copy"
	if move {
		title = "Move to…"
		confirmText = "Move"
	}
	source := mw.currentSession()
	confirm := dialog.NewCustomConfirm(title, confirmText, "Cancel", inset(4, 2, 4, 0, content), func(ok bool) {
		if !ok {
			return
		}
		policy := explorer.SkipExisting
		if replace.Checked {
			policy = explorer.ReplaceExisting
		}
		mw.runCopyMove(entries, source, destination.Text, move, policy)
	}, mw.win)
	confirm.Resize(fyne.NewSize(540, 250))
	confirm.Show()
}

func (mw *MainWindow) runCopyMove(entries []model.Entry, source model.Session, destinationPrefix string, move bool, policy explorer.ConflictPolicy) {
	client, err := mw.clientFor(source.Account)
	if err != nil {
		mw.showError("Copy/move", err)
		return
	}
	destinationPrefix = strings.Trim(strings.TrimSpace(destinationPrefix), "/")
	if destinationPrefix != "" {
		destinationPrefix += "/"
	}
	verb := "Copying"
	if move {
		verb = "Moving"
	}
	label := fmt.Sprintf("%s %d item%s", verb, len(entries), plural(len(entries)))
	mw.runOperation(label, source, func(ctx context.Context, progress explorer.Progress) error {
		if move {
			return explorer.Move(ctx, client, source.Bucket, entries, destinationPrefix, policy, progress)
		}
		return explorer.Copy(ctx, client, source.Bucket, entries, destinationPrefix, policy, progress)
	})
}

func (mw *MainWindow) runOperation(label string, session model.Session, operation func(context.Context, explorer.Progress) error) {
	mw.status.SetMessage(label + "…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		lastUpdate := time.Now().Add(-time.Second)
		err := operation(ctx, func(done, total int, current string) {
			if time.Since(lastUpdate) < 200*time.Millisecond && done != total {
				return
			}
			lastUpdate = time.Now()
			fyne.Do(func() {
				mw.status.SetMessage(fmt.Sprintf("%s - %d/%d", label, done, total))
			})
		})
		fyne.Do(func() {
			if err != nil {
				mw.showError(label, err)
				mw.notify("R2 operation failed", label+": "+err.Error())
				return
			}
			mw.status.SetMessage(label + " - done")
			mw.notify("R2 operation complete", label)
			if mw.currentSession().Account == session.Account && mw.currentSession().Bucket == session.Bucket {
				mw.browser.Reload()
			}
		})
	}()
}

func (mw *MainWindow) openEntry(entry model.Entry) {
	if entry.IsDir {
		mw.navigate(entry.Key)
		return
	}
	session := mw.currentSession()
	client, err := mw.clientFor(session.Account)
	if err != nil {
		mw.showError("Open", err)
		return
	}
	mw.status.SetMessage("Opening " + entry.Name + "…")
	go func() {
		cacheRoot, tempErr := os.MkdirTemp("", "r2go-open-*")
		if tempErr != nil {
			fyne.Do(func() { mw.showError("Open "+entry.Name, tempErr) })
			return
		}
		// Do not delete this temp directory immediately after launching the file:
		// many desktop applications keep reading it after Open returns. The OS temp
		// directory can safely reclaim these files later.
		destination := filepath.Join(cacheRoot, safeComponent(entry.Name))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		err := client.DownloadFile(ctx, session.Bucket, entry.Key, destination, nil)
		if err == nil {
			err = util.Open(destination)
		}
		fyne.Do(func() {
			if err != nil {
				mw.showError("Open "+entry.Name, err)
				return
			}
			mw.status.SetMessage("Opened " + entry.Name)
		})
	}()
}

func safeComponent(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")
	return replacer.Replace(value)
}

func (mw *MainWindow) copySelectedPublicURL() {
	entry, ok := mw.browser.SelectedEntry()
	if !ok || entry.IsDir {
		return
	}
	session := mw.currentSession()
	domain := mw.cfg.PublicDomain(session.Account, session.Bucket)
	if domain == "" {
		mw.showPublicDomainDialog()
		return
	}
	value := publicURL(domain, entry.Key)
	mw.win.Clipboard().SetContent(value)
	mw.status.SetMessage("Public URL copied")
}

func (mw *MainWindow) openPublicURL(entry model.Entry) {
	session := mw.currentSession()
	domain := mw.cfg.PublicDomain(session.Account, session.Bucket)
	if domain == "" || entry.IsDir {
		return
	}
	if err := util.Open(publicURL(domain, entry.Key)); err != nil {
		mw.showError("Open public URL", err)
	}
}

func (mw *MainWindow) showContextMenu(entries []model.Entry, position fyne.Position) {
	if len(entries) == 0 {
		return
	}
	if len(entries) > 1 {
		menu := fyne.NewMenu("Selection",
			fyne.NewMenuItemWithIcon("Download", fyneTheme.Icon(fyneTheme.IconNameDownload), func() { mw.downloadEntries(entries) }),
			fyne.NewMenuItemWithIcon("Copy to…", fyneTheme.Icon(fyneTheme.IconNameContentCopy), func() { mw.showCopyMoveDialog(entries, false) }),
			fyne.NewMenuItemWithIcon("Move to…", fyneTheme.Icon(fyneTheme.IconNameContentCut), func() { mw.showCopyMoveDialog(entries, true) }),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItemWithIcon("Delete", fyneTheme.Icon(fyneTheme.IconNameDelete), mw.deleteSelected),
		)
		widget.NewPopUpMenu(menu, mw.win.Canvas()).ShowAtPosition(position)
		return
	}

	entry := entries[0]
	openIcon := entryIcon(entry)
	openLabel := "Open"
	if entry.IsDir {
		openLabel = "Open folder"
		openIcon = fyneTheme.Icon(fyneTheme.IconNameFolderOpen)
	}
	items := []*fyne.MenuItem{
		fyne.NewMenuItemWithIcon(openLabel, openIcon, func() { mw.openEntry(entry) }),
		fyne.NewMenuItemWithIcon("Download", fyneTheme.Icon(fyneTheme.IconNameDownload), func() { mw.downloadEntries(entries) }),
	}
	if !entry.IsDir {
		domain := mw.cfg.PublicDomain(mw.currentSession().Account, mw.currentSession().Bucket)
		if domain != "" {
			items = append(items,
				fyne.NewMenuItemWithIcon("Copy public URL", fyneTheme.Icon(fyneTheme.IconNameContentCopy), mw.copySelectedPublicURL),
				fyne.NewMenuItemWithIcon("Open public URL", fyneTheme.Icon(fyneTheme.IconNameComputer), func() { mw.openPublicURL(entry) }),
			)
		} else {
			items = append(items, fyne.NewMenuItemWithIcon("Configure public URL…", fyneTheme.Icon(fyneTheme.IconNameSettings), mw.showPublicDomainDialog))
		}
	}
	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Copy to…", fyneTheme.Icon(fyneTheme.IconNameContentCopy), func() { mw.showCopyMoveDialog(entries, false) }),
		fyne.NewMenuItemWithIcon("Move to…", fyneTheme.Icon(fyneTheme.IconNameContentCut), func() { mw.showCopyMoveDialog(entries, true) }),
		fyne.NewMenuItemWithIcon("Rename", fyneTheme.Icon(fyneTheme.IconNameDocumentCreate), mw.renameSelected),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Delete", fyneTheme.Icon(fyneTheme.IconNameDelete), mw.deleteSelected),
	)
	widget.NewPopUpMenu(fyne.NewMenu(entry.Name, items...), mw.win.Canvas()).ShowAtPosition(position)
}
