<div align="center">

![Logo](./internal/icons/hicolor/scalable/apps/logo.svg)

# gtkcord4

[![Go Report Card](https://goreportcard.com/badge/github.com/diamondburned/gtkcord4)](https://goreportcard.com/report/github.com/diamondburned/gtkcord4)
[![Packaging status](https://img.shields.io/repology/repositories/gtkcord4?label=in%20repositories)](https://repology.org/project/gtkcord4/versions)
![GitHub download count](https://img.shields.io/github/downloads/diamondburned/gtkcord4/total?label=GitHub%20Downloads&logo=github)
![Flathub download count](https://img.shields.io/flathub/downloads/so.libdb.gtkcord4?logo=flatpak&logoColor=orange&label=Flatpak%20Installs&color=orange)
![SourceForge download count](https://img.shields.io/sourceforge/dt/gtkcord4.mirror?label=SourceForge%20Downloads&logo=sourceforge&color=orange)
[![Nightly release status](https://img.shields.io/github/deployments/diamondburned/gtkcord4/Nightly%20release?logo=github&label=Nightly%20Build)](https://github.com/diamondburned/gtkcord4/deployments/Nightly%20release)
[![Stable release status](https://img.shields.io/github/deployments/diamondburned/gtkcord4/Stable%20release?logo=github&label=Stable%20Build)](https://github.com/diamondburned/gtkcord4/deployments/Stable%20release)
![Latest release](https://img.shields.io/github/v/tag/diamondburned/gtkcord4?filter=!nightly&label=Latest%20Release&color=blue)

<img src="./.github/screenshot4.png" alt="Screenshot" width="800">

</div>

## Installation

### Dependencies

gtkcord4 needs GTK4, gobject-introspection, and optionally libcanberra. If compiling, then the library
headers are also required.

### Pre-built Binary

gtkcord4's CI automatically builds each release for Linux x86_64 and aarch64.
See the [Releases](https://github.com/diamondburned/gtkcord4/releases) page for
the binaries.

### Distribution Packages

gtkcord4 is available in the following distributions:

<a href="https://repology.org/project/gtkcord4/versions">
    <img src="https://repology.org/badge/vertical-allrepos/gtkcord4.svg" alt="Packaging status">
</a>

### Flatpak

gtkcord4 is available on Flathub:

[![Flathub Version](https://img.shields.io/flathub/v/so.libdb.gtkcord4?logo=flatpak&logoColor=orange&label=Flathub)](https://flathub.org/apps/details/so.libdb.gtkcord4)

<a href="https://flathub.org/apps/details/so.libdb.gtkcord4">
    <img src="https://flathub.org/assets/badges/flathub-badge-en.svg" alt="Download on Flathub" width="180">
</a>

### Compiling

You need Go 1.18+ for this step.

To compile from scratch, run

```sh
go install -v github.com/diamondburned/gtkcord4@latest
```
