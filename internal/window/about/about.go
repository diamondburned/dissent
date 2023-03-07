package about

import (
	"context"
	"fmt"
	"path"
	"runtime/debug"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gtkcord4/internal/icons"
)

// New creates a new about window.
func New(ctx context.Context) *gtk.AboutDialog {
	about := gtk.NewAboutDialog()
	about.SetTransientFor(app.GTKWindowFromContext(ctx))
	about.SetModal(true)
	about.SetProgramName("gtkcord4")
	about.SetLogo(icons.Paintable("logo.png"))
	about.SetVersion("git") // TODO version
	about.SetWebsite("https://libdb.so/gtkcord4")
	about.SetLicenseType(gtk.LicenseGPL30)

	about.SetAuthors([]string{
		"diamondburned",
		"gtkcord4 contributors",
	})

	about.AddCreditSection("Sound Files", []string{
		// https://directory.fsf.org/wiki/Sound-theme-freedesktop
		"freedesktop.org",
		"Lennart Poettering",
	})

	build, ok := debug.ReadBuildInfo()
	if ok {
		about.AddCreditSection("Dependency Authors", modAuthors(build.Deps))
		about.SetSystemInformation(build.String())

		version := buildVersion(build.Settings)
		about.SetVersion(version)

		if strings.HasSuffix(version, "(dirty)") {
			about.AddCSSClass("devel")
			about.SetLogo(icons.Paintable("logo_nightly.png"))
		}
	}

	return about
}

func buildVersion(settings []debug.BuildSetting) string {
	find := func(name string) string {
		for _, setting := range settings {
			if setting.Key == name {
				return setting.Value
			}
		}
		return ""
	}

	vcs := find("vcs")
	rev := find("vcs.revision")
	modified := find("vcs.modified")

	if vcs == "" {
		return ""
	}

	if rev == "" {
		return vcs
	}

	if len(rev) > 7 {
		rev = rev[:7]
	}

	version := fmt.Sprintf("%s (%s)", vcs, rev)
	if modified == "true" {
		version += " (dirty)"
	}

	return version
}

func modAuthors(mods []*debug.Module) []string {
	authors := make([]string, 0, len(mods))
	authMap := make(map[string]struct{}, len(mods))

	for _, mod := range mods {
		author := path.Dir(mod.Path)
		if _, ok := authMap[author]; !ok {
			authors = append(authors, author)
			authMap[author] = struct{}{}
		}
	}

	return authors
}
