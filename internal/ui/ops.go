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

	"r2go/internal/explorer"
	"r2go/internal/model"
	t "r2go/internal/theme"
	"r2go/internal/transfer"
	"r2go/internal/util"
)

// ---------------------------------------------------------------- upload

func (mw *MainWindow) showUploadMenu(pos fyne.Position) {
	if mw.currentSession().Bucket == "" {
		return
	}
	showMenu(mw.win, pos,
		menuItem{label: "Upload files…", icon: t.IconFile, action: mw.pickUploadFile},
		menuItem{label: "Upload a folder…", icon: t.IconFolder, action: mw.pickUploadFolder},
	)
}

// showUploadMenuAtToolbar opens the upload menu under the toolbar's button,
// for the keyboard shortcut.
func (mw *MainWindow) showUploadMenuAtToolbar() {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(mw.toolbar.upload)
	mw.showUploadMenu(pos.Add(fyne.NewPos(0, mw.toolbar.upload.Size().Height+2)))
}

func (mw *MainWindow) pickUploadFile() {
	mw.pickLocal(false, "Upload files to "+mw.locationLabel(), "", mw.enqueueUploadPaths)
}

func (mw *MainWindow) pickUploadFolder() {
	mw.pickLocal(true, "Upload a folder to "+mw.locationLabel(), "", mw.enqueueUploadPaths)
}

func (mw *MainWindow) handleDrop(uris []fyne.URI) {
	if mw.currentSession().Bucket == "" || len(uris) == 0 {
		if len(uris) > 0 {
			mw.status.SetMessage("Open a bucket first, then drop files to upload them")
		}
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
		mw.reportError("Could not upload", err)
		return
	}
	if len(paths) == 0 {
		return
	}

	groupID := mw.transfers.NewGroup("upload")
	label := fmt.Sprintf("Uploading %s to %s", countNoun(len(paths), "item"), mw.locationLabel())
	if len(paths) == 1 {
		label = fmt.Sprintf("Uploading %s to %s", filepath.Base(paths[0]), mw.locationLabel())
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
				mw.reportError("Could not upload", firstErr)
			case firstErr != nil:
				mw.status.SetMessage(fmt.Sprintf("Queued %s; some items could not be read", countNoun(queued, "upload")))
				mw.reportWarning("Some items were skipped", firstErr)
			case queued > 0:
				mw.status.SetMessage(fmt.Sprintf("Queued %s", countNoun(queued, "upload")))
			case emptyFolders > 0:
				mw.status.SetMessage(fmt.Sprintf("Created %s", countNoun(emptyFolders, "empty folder")))
				if mw.currentSession() == session {
					mw.browser.Reload()
				}
			default:
				mw.status.SetMessage("Nothing to upload")
			}
		})
	}()
}

// uploadsFinished reloads the listing when finished uploads landed in the
// folder on screen.
func (mw *MainWindow) uploadsFinished(items []transfer.Item) {
	session := mw.currentSession()
	for _, item := range items {
		if item.Account == session.Account && item.Bucket == session.Bucket && strings.HasPrefix(item.Key, session.Prefix) {
			mw.browser.Reload()
			return
		}
	}
}

// ---------------------------------------------------------------- download

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
		mw.reportError("Could not download", err)
		return
	}
	settings := mw.cfg.Settings()
	groupID := mw.transfers.NewGroup("download")
	label := fmt.Sprintf("Downloading %s", countNoun(len(entries), "item"))
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
				fyne.Do(func() { mw.reportError("Could not download "+entry.Name, err) })
				return
			}
			root := mw.reserveLocalPath(filepath.Join(settings.DownloadDir, entry.Name), true)
			if err := os.MkdirAll(root, 0o755); err != nil {
				fyne.Do(func() { mw.reportError("Could not download "+entry.Name, err) })
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
				mw.status.SetMessage(fmt.Sprintf("Downloading %s to %s", countNoun(queued, "file"), settings.DownloadDir))
			case createdFolders > 0:
				mw.status.SetMessage(fmt.Sprintf("Created %s", countNoun(createdFolders, "local folder")))
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

// ---------------------------------------------------------------- folders, rename, delete

// validName checks a single file or folder name typed by the user.
func validName(existing []model.Entry, current string) func(string) string {
	return func(value string) string {
		switch {
		case value == "":
			return "Enter a name."
		case strings.ContainsAny(value, "/\\"):
			return "A name cannot contain / or \\."
		case value == "." || value == "..":
			return "That name is reserved."
		}
		if value == current {
			return ""
		}
		for _, entry := range existing {
			if strings.EqualFold(entry.Name, value) {
				return "Something with that name is already here."
			}
		}
		return ""
	}
}

func (mw *MainWindow) newFolder() {
	session := mw.currentSession()
	if session.Bucket == "" {
		return
	}
	mw.prompt(promptSpec{
		title:       "New folder",
		caption:     "Folder name",
		placeholder: "e.g. images",
		hint:        "Created in " + mw.locationLabel() + ". R2 has no real folders, so r2go stores an empty marker object.",
		confirmText: "Create",
		validate:    validName(mw.browser.all, ""),
		onConfirm: func(value string) {
			client, err := mw.clientFor(session.Account)
			if err != nil {
				mw.reportError("Could not create the folder", err)
				return
			}
			key := model.JoinPrefix(session.Prefix, value, true)
			mw.progress.Begin()
			go func() {
				err := client.CreateFolder(context.Background(), session.Bucket, key)
				fyne.Do(func() {
					mw.progress.End()
					if err != nil {
						mw.reportError("Could not create the folder", err)
						return
					}
					mw.status.SetMessage("Created " + value)
					if mw.currentSession() == session {
						mw.browser.selected = map[string]struct{}{key: {}}
						mw.browser.focusKey = key
						mw.browser.Reload()
					}
				})
			}()
		},
	})
}

func (mw *MainWindow) renameSelected() {
	entry, ok := mw.browser.SelectedEntry()
	if !ok {
		return
	}
	caption := "New name"
	hint := ""
	if entry.IsDir {
		hint = "Renaming a folder copies every object inside it to the new name, then deletes the old ones."
	}
	mw.prompt(promptSpec{
		title:       "Rename " + entry.Name,
		caption:     caption,
		value:       entry.Name,
		hint:        hint,
		confirmText: "Rename",
		selectStem:  !entry.IsDir,
		validate:    validName(mw.browser.all, entry.Name),
		onConfirm: func(newName string) {
			if newName == entry.Name {
				return
			}
			session := mw.currentSession()
			client, err := mw.clientFor(session.Account)
			if err != nil {
				mw.reportError("Could not rename", err)
				return
			}
			newKey := model.JoinPrefix(session.Prefix, newName, entry.IsDir)
			mw.runOperation("Renaming "+entry.Name, session, newKey, func(ctx context.Context, progress explorer.Progress) error {
				return explorer.Rename(ctx, client, session.Bucket, entry, newName, progress)
			})
		},
	})
}

func (mw *MainWindow) deleteSelected() {
	entries := mw.browser.SelectedEntries()
	if len(entries) == 0 {
		return
	}
	files, folders := 0, 0
	var size int64
	pending := false
	for _, entry := range entries {
		if entry.IsDir {
			folders++
			pending = pending || !entry.StatsKnown
		} else {
			files++
		}
		size += entry.Size
	}
	sizeText := formatSize(size)
	if pending {
		sizeText = "at least " + sizeText
	}
	title := "Delete " + entries[0].Name + "?"
	if len(entries) > 1 {
		title = fmt.Sprintf("Delete %s?", countNoun(len(entries), "item"))
	}
	parts := []string{}
	if files > 0 {
		parts = append(parts, countNoun(files, "file"))
	}
	if folders > 0 {
		parts = append(parts, countNoun(folders, "folder")+" and everything inside")
	}
	summary := newText(joinDot(strings.Join(parts, ", "), sizeText), t.TextSmall, t.ColorSubtle, styleMono)
	mw.confirm(confirmSpec{
		title:       title,
		message:     "This permanently removes them from " + mw.locationLabel() + ". R2 has no recycle bin, so this cannot be undone.",
		detail:      summary,
		confirmText: "Delete",
		danger:      true,
		onConfirm: func() {
			session := mw.currentSession()
			client, err := mw.clientFor(session.Account)
			if err != nil {
				mw.reportError("Could not delete", err)
				return
			}
			label := "Deleting " + entries[0].Name
			if len(entries) > 1 {
				label = "Deleting " + countNoun(len(entries), "item")
			}
			mw.browser.ClearSelection()
			mw.runOperation(label, session, "", func(ctx context.Context, progress explorer.Progress) error {
				return explorer.Delete(ctx, client, session.Bucket, entries, progress)
			})
		},
	})
}

// ---------------------------------------------------------------- copy and move

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
	mw.status.SetMessage(fmt.Sprintf("%s %s · open a folder and press Ctrl+V to paste", verb, countNoun(len(entries), "item")))
}

func (mw *MainWindow) pasteClipboard() {
	clip := mw.clipboard
	destination := mw.currentSession()
	if len(clip.entries) == 0 || destination.Bucket == "" {
		return
	}
	if clip.session.Account != destination.Account || clip.session.Bucket != destination.Bucket {
		mw.status.SetMessage("Paste works within one bucket. Go back to " + clip.session.Bucket + " to paste there.")
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
	source := mw.currentSession()
	picker := newFolderPicker(mw, source, source.Prefix, entries)
	replace := newCheckbox("Replace objects that already exist there", false, nil)
	title := "Copy"
	if move {
		title = "Move"
	}
	what := entries[0].Name
	if len(entries) > 1 {
		what = countNoun(len(entries), "item")
	}
	body := vbox(10,
		labelled("Destination in "+source.Bucket, picker.Container,
			"Open a folder below, or type a path. Folders that do not exist yet are created."),
		replace,
		wrappedHint("Without this, objects that already exist are skipped. When a move skips an object, its original is kept."),
	)
	m := showModal(mw.win, modalSpec{
		title:       title + " " + what,
		width:       520,
		body:        body,
		confirmText: title + " here",
		onConfirm: func() bool {
			policy := explorer.SkipExisting
			if replace.Checked {
				policy = explorer.ReplaceExisting
			}
			mw.runCopyMove(entries, source, picker.Value(), move, policy)
			return true
		},
		focus: picker.Path,
	})
	picker.Path.OnSubmitted = func(string) { m.Confirm() }
	picker.Path.onEscape = m.Cancel
}

func (mw *MainWindow) runCopyMove(entries []model.Entry, source model.Session, destinationPrefix string, move bool, policy explorer.ConflictPolicy) {
	client, err := mw.clientFor(source.Account)
	if err != nil {
		mw.reportError("Could not copy", err)
		return
	}
	destinationPrefix = strings.Trim(strings.TrimSpace(strings.ReplaceAll(destinationPrefix, "\\", "/")), "/")
	if destinationPrefix != "" {
		destinationPrefix += "/"
	}
	verb := "Copying"
	if move {
		verb = "Moving"
	}
	label := fmt.Sprintf("%s %s", verb, countNoun(len(entries), "item"))
	mw.runOperation(label, source, "", func(ctx context.Context, progress explorer.Progress) error {
		if move {
			return explorer.Move(ctx, client, source.Bucket, entries, destinationPrefix, policy, progress)
		}
		return explorer.Copy(ctx, client, source.Bucket, entries, destinationPrefix, policy, progress)
	})
}

// runOperation runs a bucket operation in the background. Progress shows in
// the status bar and the progress line; a failure shows as a banner. When it
// finishes, the folder reloads and selectKey (if set) is selected.
func (mw *MainWindow) runOperation(label string, session model.Session, selectKey string, operation func(context.Context, explorer.Progress) error) {
	mw.status.SetMessage(label + "…")
	mw.progress.Begin()
	mw.operations++
	mw.updateStatusState()
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
				mw.status.SetMessage(fmt.Sprintf("%s · %s of %s", label, formatCount(done), formatCount(total)))
			})
		})
		fyne.Do(func() {
			mw.progress.End()
			mw.operations--
			mw.updateStatusState()
			if err != nil {
				mw.reportError(label+" failed", err)
				mw.notify("R2 operation failed", label)
			} else {
				mw.status.SetMessage(label + " · done")
			}
			current := mw.currentSession()
			if current.Account == session.Account && current.Bucket == session.Bucket {
				if selectKey != "" {
					mw.browser.selected = map[string]struct{}{selectKey: {}}
					mw.browser.focusKey = selectKey
				}
				mw.browser.Reload()
			}
		})
	}()
}

// ---------------------------------------------------------------- open and links

func (mw *MainWindow) openEntry(entry model.Entry) {
	if entry.IsDir {
		mw.navigate(entry.Key)
		return
	}
	session := mw.currentSession()
	client, err := mw.clientFor(session.Account)
	if err != nil {
		mw.reportError("Could not open "+entry.Name, err)
		return
	}
	mw.status.SetMessage("Opening " + entry.Name + "…")
	mw.progress.Begin()
	go func() {
		cacheRoot, tempErr := os.MkdirTemp("", "r2go-open-*")
		if tempErr != nil {
			fyne.Do(func() {
				mw.progress.End()
				mw.reportError("Could not open "+entry.Name, tempErr)
			})
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
			mw.progress.End()
			if err != nil {
				mw.reportError("Could not open "+entry.Name, err)
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
	mw.win.Clipboard().SetContent(publicURL(domain, entry.Key))
	mw.status.SetMessage("Copied the public URL for " + entry.Name)
}

func (mw *MainWindow) copySelectedKeys() {
	entries := mw.browser.SelectedEntries()
	if len(entries) == 0 {
		return
	}
	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	mw.win.Clipboard().SetContent(strings.Join(keys, "\n"))
	if len(keys) == 1 {
		mw.status.SetMessage("Copied the key")
	} else {
		mw.status.SetMessage(fmt.Sprintf("Copied %s", countNoun(len(keys), "key")))
	}
}

func (mw *MainWindow) openPublicURL(entry model.Entry) {
	session := mw.currentSession()
	domain := mw.cfg.PublicDomain(session.Account, session.Bucket)
	if domain == "" || entry.IsDir {
		return
	}
	if err := util.Open(publicURL(domain, entry.Key)); err != nil {
		mw.reportError("Could not open the link", err)
	}
}

// ---------------------------------------------------------------- context menu

func (mw *MainWindow) showContextMenu(entries []model.Entry, position fyne.Position) {
	if len(entries) == 0 {
		return
	}
	if len(entries) > 1 {
		showMenu(mw.win, position,
			menuItem{label: "Download " + countNoun(len(entries), "item"), icon: t.IconDownload, action: func() { mw.downloadEntries(entries) }},
			menuItem{label: "Copy keys", icon: t.IconCopy, action: mw.copySelectedKeys},
			menuSeparator,
			menuItem{label: "Copy to…", icon: t.IconCopy, action: func() { mw.showCopyMoveDialog(entries, false) }},
			menuItem{label: "Move to…", icon: t.IconMove, action: func() { mw.showCopyMoveDialog(entries, true) }},
			menuSeparator,
			menuItem{label: "Delete…", icon: t.IconTrash, danger: true, action: mw.deleteSelected},
		)
		return
	}

	entry := entries[0]
	items := []menuItem{}
	if entry.IsDir {
		items = append(items, menuItem{label: "Open", icon: t.IconFolderOpen, action: func() { mw.openEntry(entry) }})
	} else {
		items = append(items, menuItem{label: "Open", icon: t.IconExternal, action: func() { mw.openEntry(entry) }})
	}
	items = append(items, menuItem{label: "Download", icon: t.IconDownload, action: func() { mw.downloadEntries(entries) }})
	items = append(items, menuSeparator, menuItem{label: "Copy key", icon: t.IconCopy, action: mw.copySelectedKeys})
	if !entry.IsDir {
		session := mw.currentSession()
		if mw.cfg.PublicDomain(session.Account, session.Bucket) != "" {
			items = append(items,
				menuItem{label: "Copy public URL", icon: t.IconLink, action: mw.copySelectedPublicURL},
				menuItem{label: "Open public URL", icon: t.IconGlobe, action: func() { mw.openPublicURL(entry) }},
			)
		} else {
			items = append(items, menuItem{label: "Set up public URL…", icon: t.IconGlobe, action: mw.showPublicDomainDialog})
		}
	}
	items = append(items,
		menuSeparator,
		menuItem{label: "Rename…", icon: t.IconPencil, action: mw.renameSelected},
		menuItem{label: "Copy to…", icon: t.IconCopy, action: func() { mw.showCopyMoveDialog(entries, false) }},
		menuItem{label: "Move to…", icon: t.IconMove, action: func() { mw.showCopyMoveDialog(entries, true) }},
		menuSeparator,
		menuItem{label: "Delete…", icon: t.IconTrash, danger: true, action: mw.deleteSelected},
	)
	showMenu(mw.win, position, items...)
}
