package message

import (
	"context"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type spoiler struct {
	*gtk.Overlay
	ctx context.Context
}

var spoilerCSS = cssutil.Applier("message-spoiler", `
	.message-spoiler {
		background-color: black;
	}
`)

func newSpoiler(ctx context.Context) *spoiler {
	s := spoiler{
		ctx: ctx,
	}

	s.Overlay = gtk.NewOverlay()

	spoilerBin := adw.NewBin()
	spoilerCSS(spoilerBin)
	spoilerBin.SetCSSClasses([]string{"message-spoiler", "card"})

	reveal_gesture := gtk.NewGestureClick()
	reveal_gesture.ConnectReleased(func(nPress int, x, y float64) {
		spoilerBin.SetVisible(false)
	})

	spoilerBin.AddController(reveal_gesture)
	s.Overlay.AddOverlay(spoilerBin)

	return &s
}
