<div align="center">

![Dissent logo](./internal/icons/hicolor/scalable/apps/so.libdb.dissent.svg)

<h1>Dissent</h1>

<img src="./.github/screenshots/03.png" alt="Screenshot 3" width="800">

<div>
  <a href="./.github/screenshots/03.png"><img src="./.github/screenshots/03.png" alt="Screenshot 3" width="150"></a>
  <a href="./.github/screenshots/01.png"><img src="./.github/screenshots/01.png" alt="Screenshot 1" width="150"></a>
  <a href="./.github/screenshots/02.png"><img src="./.github/screenshots/02.png" alt="Screenshot 2" width="150"></a>
  <a href="./.github/screenshots/04.png"><img src="./.github/screenshots/04.png" alt="Screenshot 4" width="150"></a>
</div>

</div>

<br>

<div>
  <a href="https://github.com/diamondburned/dissent/releases/latest"><img height="22" src="https://img.shields.io/github/downloads/diamondburned/dissent/total?label=GitHub%20Downloads&amp;logo=github" alt="GitHub download count"></a>
  <a href="https://flathub.org/apps/so.libdb.dissent"><img height="22" src="https://img.shields.io/flathub/downloads/so.libdb.dissent?logo=flathub&amp;logoColor=white&amp;label=Flatpak%20Installs&amp;color=%233d7fcd" alt="Flathub download count"></a>
  <a href="https://github.com/diamondburned/dissent/releases/latest"><img height="22" src="https://img.shields.io/github/v/tag/diamondburned/dissent?filter=!nightly&amp;label=Latest%20Release&amp;color=blue" alt="Latest release"></a>
  <a href="https://repology.org/project/dissent/versions"><img height="22" src="https://img.shields.io/repology/repositories/dissent?label=Packaged Distros" alt="Packaging status"></a>
  <a href="https://goreportcard.com/report/github.com/diamondburned/dissent"><img height="22" src="https://goreportcard.com/badge/github.com/diamondburned/dissent" alt="Go Report Card"></a>
  <a href="https://github.com/diamondburned/dissent/deployments/Nightly%20release"><img height="22" src="https://img.shields.io/github/deployments/diamondburned/dissent/Nightly%20release?logo=github&amp;label=Nightly%20Build" alt="Nightly release status"></a>
  <a href="https://github.com/diamondburned/dissent/deployments/Stable%20release"><img height="22" src="https://img.shields.io/github/deployments/diamondburned/dissent/Stable%20release?logo=github&amp;label=Stable%20Build" alt="Stable release status"></a>
</div>

<br>

Dissent (previously gtkcord4) is a third-party Discord client designed for a
smooth, native experience on Linux desktops.

Built with the GTK4 and libadwaita for a modern look and feel, it delivers your
favorite Discord app in a lightweight and visually appealing package.

## Features

Dissent offers a streamlined Discord experience, prioritizing simplicity and
speed over feature completeness on par with the official client. Here's what
you can expect:

- Text chat with complete Markdown and custom emoji support
- Guild folders and channel categories
- Tabbed chat interface
- Quick switcher for channels and servers
- Image and file uploads, previews, and downloads
- User theming via custom CSS
- Partial thread/forum support
- Partial message reaction support
- Partial AI summary support (provided by Discord)

It does not aim to support voice chat and other advanced features, as these are
best handled by the official client or the web app.

## Installation

### Flatpak

Dissent is available on Flathub:

<a href="https://flathub.org/apps/details/so.libdb.dissent">
  <img src="https://flathub.org/api/badge?svg&locale=en" alt="Download on Flathub" width="220">
</a>

### Pre-built Downloads

You can download Dissent as a pre-built binary for the following platforms by
clicking on the below badges. These are automatically built and uploaded by
GitHub Actions on each release.

<div>
  <a href="https://github.com/diamondburned/dissent/releases/download/latest/dissent-windows-amd64.exe">
    <img height="24" alt="Windows x86_64" src="https://img.shields.io/badge/Windows-Download%20for%20x86__64-grey?style=flat-square&logo=windows11&labelColor=%23357EC7&cacheSeconds=999999999" />
  </a>
  <br>
  <a href="https://github.com/diamondburned/dissent/releases/download/latest/dissent-linux-amd64.tar.zst">
    <img height="24" alt="Linux x86_64" src="https://img.shields.io/badge/Linux-Download%20for%20x86__64-grey?style=flat-square&logo=linux&logoColor=black&labelColor=%23ffcc33&cacheSeconds=999999999" />
  </a>
  <br>
  <a href="https://github.com/diamondburned/dissent/releases/download/latest/dissent-linux-arm64.tar.zst">
    <img height="24" alt="Linux Aarch64" src="https://img.shields.io/badge/Linux-Download%20for%20AArch64-grey?style=flat-square&logo=linux&logoColor=black&labelColor=%23ffcc33&cacheSeconds=999999999" />
  </a>
</div>

#### Dependencies

- Linux: Dissent needs GTK4, gobject-introspection, and optionally
  libcanberra. If compiling, then the library headers are also required.
- Windows: all the needed dependencies are bundled in the executable.

### Distribution Packages

Dissent is available in the distribution repositories below. Click on the badge
to see the available versions and installation instructions.

<a href="https://repology.org/project/dissent/versions">
  <img src="https://repology.org/badge/vertical-allrepos/dissent.svg" alt="Packaging status" width="200">
</a>

### Compiling

You need Go 1.21+ for this step. To compile Dissent and install it into `$GOBIN`, run:

```sh
go install -v libdb.so/dissent@latest
```

> [!NOTE]
> Compiling is known to take at least 20 minutes on a modern system due
> to CGo. This is normal and expected, but it is still recommended to use a
> pre-built binary if available.

## Logging In

To log into Dissent, you can either use your token (recommended) or login using
your username and password. Here's how you can obtain your token:

1. Open the Discord web app in your browser and log in.
2. Press <kbd>F12</kbd> to open the Inspector.
3. Go to the Network tab then press <kbd>F5</kbd> to refresh the page.
4. In the 'Filter URLs' text box, search `discord api`.
5. Click on any HTTP message entry and inspect its message headers. Under
   the 'Request Headers' section, search for the `Authorization` header.
6. Copy its value (the token) into the Token field, then click Login.

> [!WARNING]
> Logging in using username/email and password is strongly discouraged. This
> method is untested and may cause your account to be banned! Prefer using the
> token method above.

> [!IMPORTANT]
> Using an unofficial client at all is against Discord's Terms of Service and
> may cause your account to be banned! While Dissent tries its best to not use
> the REST API at all unless necessary to reduce the risk of abuse, it is still
> possible that Discord may ban your account for using it.
>
> **Please use Dissent at your own risk!**
