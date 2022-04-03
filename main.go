package main

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/window"
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
	app.ConnectActivate(func() { m.activate(app.Context()) })
	app.RunMain(context.Background())
}

type manager struct {
	*window.Window
	ctx context.Context
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
