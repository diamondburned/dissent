package main

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/window"

	_ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
)

var _ = cssutil.WriteCSS(`
	.adaptive-avatar > image {
		background: none;
	}
	.adaptive-avatar > label {
		background: @borders;
	}
`)

func main() {
	m := manager{}
	m.app = app.New("com.github.diamondburned.gtkcord4", "gtkcord4")
	m.app.AddJSONActions(map[string]interface{}{
		"app.open-channel": m.openChannel,
		"app.preferences":  func() { prefui.ShowDialog(m.win.Context()) },
		"app.about":        func() { /* TODO */ },
		"app.logs":         func() { logui.ShowDefaultViewer(m.win.Context()) },
		"app.quit":         func() { m.app.Quit() },
	})
	m.app.ConnectActivate(func() { m.activate(m.app.Context()) })
	m.app.RunMain(context.Background())
}

type manager struct {
	app *app.Application
	win *window.Window
}

func (m *manager) openChannel(cmd gtkcord.OpenChannelCommand) {
	if m.win == nil || m.win.Chat == nil {
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
}
