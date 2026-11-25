package login

import (
	"context"
	_ "embed"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:embed loggingin.ui
var loggingInUI string

// LoadingPage is the busy spinner screen that's shown while the application is
// trying to log in.
type LoadingPage struct {
	*adw.ToolbarView
	statusPage   *adw.StatusPage
	logoutButton *gtk.Button
}

// NewLoadingPage creates a new logging in loading screen.
func NewLoadingPage(ctx context.Context) *LoadingPage {
	ui := gtk.NewBuilderFromString(loggingInUI)
	get := func(name string) glib.Objector { return ui.GetObject(name).Cast() }

	statusPage := get("statusPage").(*adw.StatusPage)
	statusPage.SetPaintable(adw.NewSpinnerPaintable(statusPage))

	return &LoadingPage{
		ToolbarView:  get("rootContent").(*adw.ToolbarView),
		statusPage:   statusPage,
		logoutButton: get("logoutButton").(*gtk.Button),
	}
}

// SetText sets the text that's shown underneath the spinner.
func (l *LoadingPage) SetText(text string) {
	l.statusPage.SetTitle(text)
}
