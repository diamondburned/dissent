package login

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// LoadingPage is the busy spinner screen that's shown while the application is
// trying to log in.
type LoadingPage struct {
	*gtk.Box
	Header *gtk.HeaderBar

	Content struct { // whole page is draggable
		*gtk.WindowHandle
		Box     *gtk.Box
		Spinner *gtk.Spinner
		Text    *gtk.Label
	}
}

var loggingInCSS = cssutil.Applier("login-loading", `
	.login-loading headerbar {
		background: none;
	}
	.login-loading-text {
		margin-top:  8px;
		font-size: 1.2em;
	}
`)

// NewLoadingPage creates a new logging in loading screen.
func NewLoadingPage(ctx context.Context) *LoadingPage {
	l := LoadingPage{}
	l.Content.Spinner = gtk.NewSpinner()
	l.Content.Spinner.SetSizeRequest(64, 64)

	l.Content.Text = gtk.NewLabel("Connecting...")
	l.Content.Text.AddCSSClass("login-loading-text")

	l.Content.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	l.Content.Box.SetVAlign(gtk.AlignCenter)
	l.Content.Box.SetHAlign(gtk.AlignCenter)
	l.Content.Box.Append(l.Content.Spinner)
	l.Content.Box.Append(l.Content.Text)

	l.Content.WindowHandle = gtk.NewWindowHandle()
	l.Content.WindowHandle.SetVExpand(true)
	l.Content.WindowHandle.SetChild(l.Content.Box)

	l.Header = gtk.NewHeaderBar()
	l.Header.SetShowTitleButtons(true)

	l.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	l.Box.Append(l.Header)
	l.Box.Append(l.Content)

	l.ConnectMap(l.Content.Spinner.Start)
	l.ConnectUnmap(l.Content.Spinner.Stop)
	loggingInCSS(l)

	return &l
}

// SetText sets the text that's shown underneath the spinner.
func (l *LoadingPage) SetText(text string) {
	l.Content.Text.SetText(text)
}
