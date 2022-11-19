<div align="center">

![Logo](./internal/icons/svg/logo.svg)

# gtkcord4

</div>

![Screenshot](./.github/screenshot2.png)

## Installation

### Dependencies

Gtkcord4 needs the following dependencies met:

```
golang git gtk4-devel gobject-introspection gobject-introspection-devel
```

Optionally (but recommended) install `libcanberra`

To build with libadwaita, you will need `libadwaita-devel`

### Pre-built Binary

gtkcord4's CI automatically builds each release for Linux x86_64 and aarch64.
See the [Releases](https://github.com/diamondburned/gtkcord4/releases) page for
the binaries.

### Compiling

You need the Go compiler that's 1.17 or newer for this step.

To compile from scratch, run

```sh
go install -v github.com/diamondburned/gtkcord4@latest
```

## libadwaita

gtkcord4 doesn't use libadwaita by default, and the releases will not be built
against libadwaita.

To build gtkcord4 against libadwaita, use `go install -tags libadwaita`.
