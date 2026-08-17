# R2GO

R2GO is a small desktop explorer for **Cloudflare R2**, written in Go with Fyne.

It is built around the boring day-to-day stuff: browse buckets, upload and download files, move things around, and get out of the way.

## Features

- Multiple R2 accounts
- Bucket and folder navigation with breadcrumbs
- Sortable file table with multi-select
- Current-folder filtering
- File and folder upload, including drag and drop
- File and folder download with directory structure preserved
- Batch download, copy, move, and delete
- Rename with conflict checks
- Background transfers with progress and notifications
- Multipart uploads for large files
- Keyboard-first navigation
- Optional public/CDN URL discovery for R2 buckets
- Custom dark/light theme

## Requirements

- Go **1.25+**
- Cloudflare R2 S3 credentials
- A C compiler / graphics toolchain required by Fyne

If you already build Fyne apps on Windows, your existing setup should be enough.

On Debian/Ubuntu:

```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev
```

## Development

```bash
go test ./...
go vet ./...
```

## Build

Windows PowerShell:

```powershell
./scripts/build.ps1
```

Linux/macOS:

```bash
./scripts/build.sh
```

Or run it directly:

```bash
go run ./cmd/r2go
```

## Add an account

R2GO asks for:

- **Name**: local display name, e.g. `Personal`
- **Account ID**: Cloudflare account ID
- **Access Key ID**: R2 S3 access key
- **Secret Access Key**: R2 S3 secret
- **Cloudflare API token**: optional, only used for automatic public-domain discovery

The API token is not required for normal R2 browsing or file operations.

Use least-privilege credentials and scope them to only the buckets and permissions you need.

## Keyboard controls

| Shortcut                 | Action                              |
| ------------------------ | ----------------------------------- |
| `Ctrl+F`                 | Focus filter                        |
| `Ctrl+R` / `F5`          | Refresh                             |
| `↑` / `↓`                | Move selection                      |
| `Shift+↑` / `Shift+↓`    | Extend selection                    |
| `Ctrl+A`                 | Select all visible items            |
| `Enter`                  | Open file / enter folder            |
| `Backspace` / `Alt+Left` | Go to parent folder                 |
| `F2`                     | Rename                              |
| `Delete`                 | Delete selected items               |
| `Ctrl+C`                 | Copy                                |
| `Ctrl+X`                 | Cut / move                          |
| `Ctrl+V`                 | Paste                               |
| `Ctrl+Shift+N`           | New folder                          |
| `Ctrl+U`                 | Upload                              |
| `Esc`                    | Clear selection / leave input focus |

Mouse selection follows normal desktop behavior: click, Ctrl/Cmd-click, Shift-click, double-click to open, right-click for actions.

## Public URLs

R2GO can build normal public URLs for buckets exposed through a custom domain or `r2.dev`.

You can either:

- let R2GO discover the domain using an optional Cloudflare API token, or
- configure the domain manually.

These are ordinary public URLs, not presigned links.

## Secrets

R2GO stores non-secret settings in the OS user config directory and prefers the native credential store for secrets:

- Windows Credential Manager
- macOS Keychain
- Secret Service / KWallet on Linux

If no supported keyring is available, R2GO falls back to a local `secrets.bin` file with user-only permissions where supported.

That fallback is **not encrypted**. Treat it like plaintext protected by your OS account permissions.

## Reliability notes

A few things are deliberately kept simple:

- selection is tracked by object key, not row number
- remote listing is asynchronous and ignores stale responses after navigation
- downloads use partial files and only rename on success
- folder downloads keep their directory structure
- transfer concurrency is fixed internally instead of exposed as a setting
- destructive copy/move/rename logic is kept outside the UI and covered by tests

## Project layout

```text
cmd/r2go/              app entry point
internal/app/          bootstrap
internal/cloudflare/   optional public-domain discovery
internal/config/       profiles, settings, secrets
internal/explorer/     copy/move/delete/rename logic
internal/model/        object and view models
internal/r2/           R2 S3 client
internal/theme/        app theme
internal/transfer/     background transfer queue
internal/ui/           Fyne UI
internal/util/         OS helpers
```

## License

MIT. See [LICENSE](LICENSE).
