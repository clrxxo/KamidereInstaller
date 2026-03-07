# Kamidere Installer

The Kamidere Installer patches Discord Desktop with the `Kamidere` desktop build from the sibling `Kamidere` project or from release assets.

## Usage

Windows
- GUI: `KamidereInstaller.exe`
- CLI: `KamidereCli.exe`

macOS
- GUI bundle archive: `KamidereInstaller.MacOS.zip`

Linux
- GUI: `KamidereInstaller-x11`
- CLI: `KamidereCli-linux`

## Building

Prerequisites:
- Go 1.24+
- GCC / MinGW

Install dependencies:

```sh
go mod tidy
```

Build the GUI:

```sh
go build -o KamidereInstaller
```

Build the CLI:

```sh
go build --tags cli -o KamidereCli
```

Linux Wayland:

```sh
go build --tags wayland -o KamidereInstaller-x11
```

## Environment

The installer accepts both the new `KAMIDERE_*` environment variables and the legacy `EQUICORD_*` names for compatibility:

- `KAMIDERE_USER_DATA_DIR`
- `KAMIDERE_DIRECTORY`
- `KAMIDERE_DEV_INSTALL`

## Release Notes

The GitHub workflow in [.github/workflows/release.yml](./.github/workflows/release.yml) builds the branded Kamidere installer artifacts for all supported platforms.
