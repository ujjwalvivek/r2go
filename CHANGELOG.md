# Changelog

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
