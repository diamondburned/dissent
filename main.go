package main

import (
	"context"
	"embed"
	"io/fs"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/window"
	"github.com/diamondburned/gtkcord4/internal/window/about"
	_ "github.com/diamondburned/gtkcord4/internal/icons"

	_ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
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
		"app.show-qs":      m.openQuickSwitcher,
		"app.about":        func() { about.New(m.win.Context()).Present() },
		"app.logs":         func() { logui.ShowDefaultViewer(m.win.Context()) },
		"app.quit":         func() { m.app.Quit() },
	})
	m.app.AddActionShortcuts(map[string]string{
		"<Ctrl>K": "app.show-qs",
		"<Ctrl>Q": "app.quit",
	})
	m.app.ConnectActivate(func() { m.activate(m.app.Context()) })
	m.app.RunMain()
}

type manager struct {
	app *app.Application
	win *window.Window
}

func (m *manager) isLoggedIn() bool {
	return m.win != nil && m.win.Chat != nil
}

func (m *manager) openQuickSwitcher() {
	if !m.isLoggedIn() {
		return
	}
	m.win.Chat.ShowQuickSwitcher()
}

func (m *manager) openChannel(cmd gtkcord.OpenChannelCommand) {
	if !m.isLoggedIn() {
		return
	}
	// TODO: highlight message.
	m.win.Chat.OpenChannel(cmd.ChannelID)
}

func (m *manager) activate(ctx context.Context) {
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
