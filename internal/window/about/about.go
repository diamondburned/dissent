package about

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/logui"
)

// New creates a new about window.
func New(ctx context.Context) *adw.AboutDialog {
	about := adw.NewAboutDialog()
	about.SetApplicationName("Dissent")
	about.SetApplicationIcon("logo")
	about.SetVersion("git") // TODO: version
	about.SetWebsite("https://libdb.so/dissent")
	about.SetCopyright("© 2023 diamondburned and contributors")
	about.SetLicenseType(gtk.LicenseGPL30)

	about.SetDevelopers([]string{
		"diamondburned",
		"Dissent contributors",
	})

	if contributors != nil {
		about.SetDevelopers(contributors)
	} else {
		ctx, cancel := context.WithCancel(ctx)
		about.ConnectClosed(cancel)
		go func() {
			developers, err := fetchContributors(ctx)
			if err != nil {
				slog.Warn("cannot fetch contributors for the about dialog", "err", err)
				return
			}
			glib.IdleAdd(func() {
				contributors = developers
				about.SetDevelopers(developers)
			})
		}()
	}

	about.AddCreditSection("Sound Files", []string{
		"freedesktop.org https://www.freedesktop.org/wiki/",
		"Lennart Poettering",
	})

	build, ok := debug.ReadBuildInfo()
	if ok {
		about.AddCreditSection("Dependency Authors", modAuthors(build.Deps))
		about.SetDebugInfo(debugInfo(build))
		about.SetDebugInfoFilename("dissent-debuginfo")

		version := buildVersion(build.Settings)
		about.SetVersion(version)

		if strings.HasSuffix(version, "(dirty)") {
			about.AddCSSClass("devel")
			about.SetApplicationIcon("logo-nightly")
		}
	}

	return about
}

// contributors is the list of contributors once it has been fetched. It is kept
// for the rest of the session, since fetching it takes a request per
// contributor and GitHub only allows 60 requests an hour without signing in.
var contributors []string

// fetchContributors fetches the contributors of Dissent from GitHub, ordered by
// their number of commits.
func fetchContributors(ctx context.Context) ([]string, error) {
	var users []struct {
		Login   string `json:"login"`
		URL     string `json:"url"`
		HTMLURL string `json:"html_url"`
		Type    string `json:"type"`
	}
	err := getJSON(ctx, "https://api.github.com/repos/diamondburned/dissent/contributors?per_page=100", &users)
	if err != nil {
		return nil, err
	}

	developers := make([]string, len(users))
	var wg sync.WaitGroup
	for i, user := range users {
		if user.Type == "Bot" {
			continue
		}
		wg.Go(func() {
			name := user.Login
			var profile struct {
				Name string `json:"name"`
			}
			// Fall back to the username if the profile cannot be fetched.
			if getJSON(ctx, user.URL, &profile) == nil &&
				profile.Name != "" && !strings.EqualFold(profile.Name, user.Login) {
				name = fmt.Sprintf("%s (%s)", profile.Name, user.Login)
			}
			// AdwAboutDialog links names that are followed by a URL.
			developers[i] = name + " " + user.HTMLURL
		})
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(developers, func(s string) bool { return s == "" }), nil
}

func getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

var customVersion string

// SetVersion sets the custom version string. It overrides the version string
// that's automatically generated from the build info.
func SetVersion(version string) {
	customVersion = version
}

func buildVersion(settings []debug.BuildSetting) string {
	if customVersion != "" {
		return customVersion
	}

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

func debugInfo(build *debug.BuildInfo) string {
	var s strings.Builder
	fmt.Fprintf(&s, "Version: %s", buildVersion(build.Settings))
	s.WriteString("\n")

	s.WriteString("Build Info:\n")
	s.WriteString(build.String())
	s.WriteString("\n\n")

	s.WriteString("Last 50 log lines:\n")
	s.WriteString(lastNLogLines(50))
	s.WriteString("\n\n")

	return strings.TrimSpace(s.String())
}

func lastNLogLines(n int) string {
	handler := logui.DefaultLogHandler()

	logModel := handler.ListModel()
	nLogs := logModel.Len()

	iter := logModel.RangeItems(max(0, nLogs-n), nLogs)
	logs := logui.RecordsToString(iter)
	return logs
}
