# Changelog

## v2.0.0

- Redesigned the whole UI experience.
- Added an account overview dashboard, opened by clicking an account and shown at startup: totals, storage by bucket and by file type, recently changed and largest files.
- Added a grid view with cached image previews (including SVG and ICO, up to 100 MB) and format-coloured icons and badges.
- Added a details panel: preview, content type, ETag, metadata, public URL and quick actions.
- Added `Ctrl+K` quick jump, `Ctrl+L` path editing, back/forward history, type-ahead and a `?` keyboard reference.
- Added a folder browser to the copy and move dialogs.
- Added retry for failed transfers, with ETA and speed per batch.
- Failures now show as plain-words banners instead of modal error boxes.
- The sidebar floats beside the content; the selection strip, transfers and status bar run the full window width.
- Credential fields are masked, with a hold-to-reveal eye.
- Native file and folder pickers replace Fyne's. This fixes crashes when uploading and when choosing a download folder.
- Fixed `Ctrl+A/C/X/V` and the plain-key shortcuts (arrows, Enter, Delete, F2, F5), which never reached the app before.
- Opted in to Fyne's `fyne.Do` threading model.
- Removed the light theme.

## v1.1.0

- Reworked the UI into a denser, edge-to-edge Windows 11-style layout.
- Switched the dark theme to AMOLED blacks with tighter spacing, smaller radii, and themed scrollbars.
- Rebuilt the file list for full-width rows, aligned columns, cleaner selection/hover states, and no horizontal scrollbar.
- Fixed toolbar and transfer-panel layout shifts so the browser no longer slides underneath them.
- Fixed collapsed account names in the sidebar.
- Added recursive folder size and latest-modified stats.
- Added a real app/window icon and Windows GUI builds no longer open a console window.
- Compacted dialogs, context menus, file pickers, loading states, search, status bar, and transfer UI.
- Build scripts now stop on the first failure and show verbose linking progress.

## v1.0.0

- Removed filesystem/WinFsp mounting completely.
- Removed metadata editing and presigned-link workflow.
- Rebuilt the file table with stable object-key selection and keyboard navigation.
- Added safe batch download, copy, move, rename, and delete behavior.
- Preserved folder structure during downloads and copy/move operations.
- Replaced the transfer scheduler with a fixed worker queue and batch-level progress UI.
- Added multipart upload progress, async navigation, and cleaner completion/failure notifications.
- Added safer client caching, credential storage, config migration, tests, CI, and public CDN URL discovery.
