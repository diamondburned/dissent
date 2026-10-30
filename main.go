package main

import (
	"context"
	"embed"
	"io/fs"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
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

func init() {
	app.Hook(func(*app.Application) {
		adw.Init()
		adaptive.Init()
	})
}

func main() {
	m := manager{}
	m.app = app.New(context.Background(), "so.libdb.dissent", "Dissent")
	m.app.AddJSONActions(map[string]interface{}{
		"app.preferences": func() { prefui.ShowDialog(m.window().Context()) },
		"app.about":       func() { about.New(m.window().Context()).Present(m.window()) },
		"app.logs":        func() { logui.ShowDefaultViewer(m.window().Context()) },
		"app.quit":        func() { m.app.Quit() },
	})
	m.app.AddActionCallbacks(map[string]gtkutil.ActionCallback{
		"app.open-channel": m.forwardSignalToWindow("open-channel", gtkcord.SnowflakeVariant),
		"app.open-guild":   m.forwardSignalToWindow("open-guild", gtkcord.SnowflakeVariant),
	})
	m.app.AddActionShortcuts(map[string]string{
		"<Ctrl>Q": "app.quit",
	})
	m.app.ConnectStartup(func() { m.startup(m.app.Context()) })
	m.app.ConnectActivate(func() { m.window().Present() })
	m.app.RunMain()
}

type manager struct {
	app     *app.Application
	session *session.Session
	win     *window.Window
}

func (m *manager) forwardSignalToWindow(name string, t *glib.VariantType) gtkutil.ActionCallback {
	return gtkutil.ActionCallback{
		ArgType: t,
		Func: func(args *glib.Variant) {
			// Present after the action, otherwise a window that was hidden
			// comes back at its minimum size.
			win := m.window()
			win.ActivateAction(name, args)
			win.Present()
		},
	}
}

// startup starts the Discord session. It runs once per process, including when
// Dissent is started in the background as a D-Bus service.
func (m *manager) startup(ctx context.Context) {
	m.session = session.New(ctx)

	prefs.AsyncLoadSaved(ctx, func(err error) {
		if err != nil {
			app.Error(ctx, err)
		}
		m.session.StartBackgroundPortal()
	})

	m.session.LoadKeyring()
	m.app.ConnectShutdown(m.session.Close)
}

// window returns the window, creating it if it was closed or never opened.
func (m *manager) window() *window.Window {
	if m.win == nil {
		m.win = window.NewWindow(m.app.Context(), m.session)
		m.win.ConnectDestroy(func() { m.win = nil })
	}
	return m.win
}
