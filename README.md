# gtkcord4

![Screenshot](./.github/screenshot2.png)

## Installation

### Dependencies

gtkcord4 needs GTK4, gobject-introspection, and optionally libcanberra. If compiling, then the library
headers are also required.

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
