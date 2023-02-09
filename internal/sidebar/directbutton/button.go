package directbutton

import (
	"context"
	"math"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/sidebar/sidebutton"
	"github.com/diamondburned/gtkcord4/internal/icons"
)

type Button struct {
	*gtk.Overlay
	Pill   *sidebutton.Pill
	Button *gtk.Button

	ctx context.Context
}

var dmButtonCSS = cssutil.Applier("sidebar-dm-button-overlay", `
	.sidebar-dm-button {
		padding: 4px 12px;
		border-radius: 0;
	}
	.sidebar-dm-button image {
		padding-top: 4px;
		padding-bottom: 2px;
	}
`)

func NewButton(ctx context.Context, open func()) *Button {
	b := Button{ctx: ctx}

	icon := gtk.NewImageFromPixbuf(icons.Pixbuf("dm"))
	icon.SetIconSize(gtk.IconSizeLarge)
	icon.SetPixelSize(int(math.Round(gtkcord.GuildIconSize * 0.85)))

	b.Button = gtk.NewButton()
	b.Button.AddCSSClass("sidebar-dm-button")
	b.Button.SetTooltipText("Direct Messages")
	b.Button.SetChild(icon)
	b.Button.SetHasFrame(false)
	b.Button.ConnectClicked(func() {
		b.Pill.State = sidebutton.PillActive
		b.Pill.Invalidate()

		open()
	})

	b.Pill = sidebutton.NewPill()

	b.Overlay = gtk.NewOverlay()
	b.Overlay.SetChild(b.Button)
	b.Overlay.AddOverlay(b.Pill)

	dmButtonCSS(b)
	return &b
}
