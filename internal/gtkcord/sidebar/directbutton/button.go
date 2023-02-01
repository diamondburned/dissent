package directbutton

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar/sidebutton"
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
`)

func NewButton(ctx context.Context, open func()) *Button {
	b := Button{ctx: ctx}

	icon := gtk.NewImageFromFile("internal/icons/png/dm.png")
	icon.SetIconSize(gtk.IconSizeLarge)
	icon.SetPixelSize(gtkcord.GuildIconSize)

	b.Button = gtk.NewButton()
	b.Button.AddCSSClass("sidebar-dm-button")
	b.Button.SetTooltipText("Direct Messages")
	b.Button.SetChild(icon)
	b.Button.SetHasFrame(false)
	b.Button.SetHAlign(gtk.AlignCenter)
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
