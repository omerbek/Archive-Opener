# Arşiv Açıcı

*English | [Türkçe](README.tr.md)*

A WinRAR-style archive extractor for Windows. No installation needed: it is a single `.exe` file. The user interface is in Turkish.

## Supported formats

- **RAR** and **RAR5**, including multi-volume archives (`.part1.rar`, `.r00`) and self-extracting `.exe` archives
- **ZIP**, including legacy Turkish Windows file names (CP857) and encrypted ZIP (ZipCrypto and AES)
- **7z**, including encrypted content and encrypted file lists
- **TAR**, **GZ**, **BZ2**, **XZ**, **ZST** (and combined forms such as `.tar.gz` and `.tgz`)

The program only extracts archives; it does not create them. Very old RAR 1.5 archives are not supported.

## Features

- Browse folders inside the archive, sort by any column, Explorer file icons
- Double-click a file to open it with its default program; archives inside archives open in the app itself
- **Extract…**: choose the destination folder and what to do with existing files (ask / overwrite / skip)
- **Extract to folder**: one click, extracts next to the archive into a folder with the same name
- **Test**: finds damaged files in the archive
- Progress window with speed and cancel
- Drag and drop to open
- Optional Windows integration: "Open with" entry and "Extract here" / "Extract to folder" in the right-click menu. Written to the current user only and removable from the menu.
- Safety: paths that try to escape the destination folder (`../`) and symbolic links are never extracted

## Download

Get `ArsivAcici.exe` from the [Releases](../../releases) page. The program is not code-signed, so Windows SmartScreen may warn on first launch; choose "More info → Run anyway".

Command line:

```
ArsivAcici.exe file.rar        open the archive in the window
ArsivAcici.exe -x file.rar     extract into a "file" folder next to the archive
ArsivAcici.exe -xh file.rar    extract into the archive's own folder
```

## Building

Requires Go 1.21 or later. Builds on Linux, macOS or Windows:

```
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o ArsivAcici.exe ./app
```

The icon and manifest are embedded through `app/rsrc_windows_amd64.syso`. To change them:

```
go run ./cmd/mkicon app/app.ico
go install github.com/akavel/rsrc@latest
rsrc -arch amd64 -manifest app/app.manifest -ico app/app.ico -o app/rsrc_windows_amd64.syso
```

The `replace` lines in `go.mod` fetch some dependencies straight from their GitHub sources and satisfy test-only modules with empty placeholders (`stubs/`).

## Project layout

| Folder | Contents |
|---|---|
| `arc/` | Archive reading and safe extraction (platform independent) |
| `app/` | Windows user interface ([lxn/walk](https://github.com/lxn/walk)) |
| `cmd/arctest/` | Small command-line tool for testing archives |
| `cmd/mkicon/` | Tool that draws the application icon |

## Libraries used

- [nwaples/rardecode](https://github.com/nwaples/rardecode): RAR and RAR5
- [bodgit/sevenzip](https://github.com/bodgit/sevenzip): 7z
- [ulikunitz/xz](https://github.com/ulikunitz/xz), [klauspost/compress](https://github.com/klauspost/compress): XZ, LZMA, Zstandard
- [lxn/walk](https://github.com/lxn/walk): Windows user interface

## License

MIT. See [LICENSE](LICENSE).
