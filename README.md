<div align="center">

![Logo](./internal/icons/svg/logo.svg)

# gtkcord4

</div>

![Screenshot](./.github/screenshot4.png)

## Installation

### Dependencies

gtkcord4 needs GTK4, gobject-introspection, and optionally libcanberra. If compiling, then the library
headers are also required.

### Pre-built Binary

gtkcord4's CI automatically builds each release for Linux x86_64 and aarch64.
See the [Releases](https://github.com/diamondburned/gtkcord4/releases) page for
the binaries.

### Compiling

You need Go 1.18+ for this step.

To compile from scratch, run

```sh
go install -v github.com/diamondburned/gtkcord4@latest
```
