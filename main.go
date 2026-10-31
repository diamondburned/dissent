package main

import (
	"context"
	"embed"
	"io/fs"
	"os"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/session"
	"libdb.so/dissent/internal/window"
	"libdb.so/dissent/internal/window/about"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"

	_ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
	_ "libdb.so/dissent/internal/icons"
)

//go:embed po/*
var po embed.FS

func init() {
	po, _ := fs.Sub(po, "po")
	locale.LoadLocale(po)
}

// Version is connected to about.SetVersion.
var Version string

func init() { about.SetVersion(Version) }

var _ = cssutil.WriteCSS(`
	window.background,
	window.background.solid-csd {
		background-color: @theme_bg_color;
	}

	avatar > image {
		background: none;
	}
	avatar > label {
		background: @borders;
	}

	.md-textblock {
		line-height: 1.35em;
	}
`)

func main() {
	os.Exit(NewApplication(context.Background()).Run(os.Args))
}

// Application is the Dissent application.
type Application struct {
	adw.Application
	app     *app.Application
	session *session.Session
	win     *window.Window
}

var applicationType = coreglib.RegisterSubclass[*Application](
	coreglib.WithGTypeName("DissentApplication"),
	coreglib.WithOverrides(func(a *Application) gio.ApplicationOverrides {
		return gio.ApplicationOverrides{
			Startup:  a.startup,
			Activate: a.activate,
			Shutdown: a.shutdown,
		}
	}),
)

// NewApplication creates a new Dissent application.
func NewApplication(ctx context.Context) *Application {
	a := applicationType.NewWithProperties(map[string]any{
		"application-id": "so.libdb.dissent",
	})
	a.app = app.Wrap(ctx, &a.Application.Application, "Dissent")
	return a
}

// Run runs the application until it quits or the process is interrupted.
func (a *Application) Run(args []string) int {
	return a.app.Run(args)
}

// startup starts the Discord session. It runs once per process, including when
// Dissent is started in the background as a D-Bus service.
func (a *Application) startup() {
	coreglib.ParentOverrides[gio.ApplicationOverrides](a).Startup()
	adaptive.Init()

	a.app.AddJSONActions(map[string]interface{}{
		"app.preferences": func() { prefui.ShowDialog(a.window().Context()) },
		"app.about":       func() { about.New(a.window().Context()).Present(a.window()) },
		"app.logs":        func() { logui.ShowDefaultViewer(a.window().Context()) },
		"app.quit":        func() { a.app.Quit() },
	})
	a.app.AddActionCallbacks(map[string]gtkutil.ActionCallback{
		"app.open-channel": a.forwardActionToWindow("open-channel", gtkcord.SnowflakeVariant),
		"app.open-guild":   a.forwardActionToWindow("open-guild", gtkcord.SnowflakeVariant),
	})
	a.app.AddActionShortcuts(map[string]string{
		"<Ctrl>Q": "app.quit",
	})

	ctx := a.app.Context()
	a.session = session.New(ctx)

	prefs.AsyncLoadSaved(ctx, func(err error) {
		if err != nil {
			app.Error(ctx, err)
		}
		a.session.StartBackgroundPortal()
	})

	a.session.LoadKeyring()
}

func (a *Application) activate() {
	a.window().Present()
}

func (a *Application) shutdown() {
	a.session.Close()
	coreglib.ParentOverrides[gio.ApplicationOverrides](a).Shutdown()
}

func (a *Application) forwardActionToWindow(name string, t *glib.VariantType) gtkutil.ActionCallback {
	return gtkutil.ActionCallback{
		ArgType: t,
		Func: func(args *glib.Variant) {
			// Present after the action, otherwise a window that was hidden
			// comes back at its minimum size.
			win := a.window()
			win.ActivateAction(name, args)
			win.Present()
		},
	}
}

// window returns the window, creating it if it was closed or never opened.
func (a *Application) window() *window.Window {
	if a.win == nil {
		a.win = window.NewWindow(a.app.Context(), a.session)
		a.win.ConnectDestroy(func() { a.win = nil })
	}
	return a.win
}
