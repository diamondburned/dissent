package main

import (
	"context"
	"embed"
	"io/fs"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/window"
	"github.com/diamondburned/gtkcord4/internal/window/about"

	_ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
	_ "github.com/diamondburned/gtkcord4/internal/icons"
)

//go:embed po/*
var po embed.FS

func init() {
	po, _ := fs.Sub(po, "po")
	locale.LoadLocale(po)
}

var _ = cssutil.WriteCSS(`
	window.background,
	window.background.solid-csd {
		background-color: @theme_bg_color;
	}

	.adaptive-avatar > image {
		background: none;
	}
	.adaptive-avatar > label {
		background: @borders;
	}
`)

func main() {
	m := manager{}
	m.app = app.New(context.Background(), "so.libdb.gtkcord4", "gtkcord4")
	m.app.AddJSONActions(map[string]interface{}{
		"app.open-channel": m.openChannel,
		"app.preferences":  func() { prefui.ShowDialog(m.win.Context()) },
		"app.about":        func() { about.New(m.win.Context()).Present() },
		"app.logs":         func() { logui.ShowDefaultViewer(m.win.Context()) },
		"app.quit":         func() { m.app.Quit() },
	})
	m.app.AddActionShortcuts(map[string]string{
		"<Ctrl>Q": "app.quit",
	})
	m.app.ConnectActivate(func() { m.activate(m.app.Context()) })
	m.app.RunMain()
}

type manager struct {
	app *app.Application
	win *window.Window
}

func (m *manager) openChannel(cmd gtkcord.OpenChannelCommand) {
	// TODO: highlight message.
	m.win.ActivateAction("win.open-channel", gtkcord.NewChannelIDVariant(cmd.ChannelID))
}

func (m *manager) activate(ctx context.Context) {
	adw.Init()
	adaptive.Init()

	if m.win != nil {
		m.win.Present()
		return
	}

	m.win = window.NewWindow(ctx)
	m.win.Show()

	prefs.AsyncLoadSaved(ctx, func(err error) {
		if err != nil {
			app.Error(ctx, err)
		}
	})
}
