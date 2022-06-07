package main

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotkit/app"
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

	app := app.New("com.github.diamondburned.gtkcord4", "gtkcord4")
	app.AddJSONActions(map[string]interface{}{
		"app.open-channel": m.openChannel,
		"app.preferences":  func() { prefui.ShowDialog(app.Context()) },
		"app.about":        func() { /* TODO */ },
		"app.quit":         func() { app.Quit() },
	})
	app.ConnectActivate(func() { m.activate(app.Context()) })
	app.RunMain(context.Background())
}

type manager struct {
	*window.Window
	ctx context.Context
}

func (m *manager) openChannel(cmd gtkcord.OpenChannelCommand) {
	if m.Chat == nil {
		return
	}

	// TODO: highlight message.
	m.Chat.OpenChannel(cmd.ChannelID)
}

func (m *manager) activate(ctx context.Context) {
	adaptive.Init()
	m.ctx = ctx

	if m.Window != nil {
		m.Window.Present()
		return
	}

	m.Window = window.NewWindow(ctx)
	m.Window.Show()
}
