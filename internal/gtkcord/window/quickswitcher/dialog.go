package quickswitcher

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
)

// Dialog is a Quick Switcher dialog.
type Dialog struct {
	*gtk.Dialog
	ctrl Controller
}

const dialogFlags = 0 |
	gtk.DialogDestroyWithParent |
	gtk.DialogModal |
	gtk.DialogUseHeaderBar

// ShowDialog shows a new Quick Switcher dialog.
func ShowDialog(ctx context.Context, ctrl Controller) {
	d := NewDialog(ctx, ctrl)
	d.Show()
}

const (
	refreshResponse = iota + 1
)

// NewDialog creates a new Quick Switcher dialog.
func NewDialog(ctx context.Context, ctrl Controller) *Dialog {
	d := Dialog{ctrl: ctrl}

	qs := NewQuickSwitcher(ctx, (*dialogControlling)(&d))

	d.Dialog = gtk.NewDialogWithFlags(
		app.FromContext(ctx).SuffixedTitle("Quick Switcher"),
		app.GTKWindowFromContext(ctx),
		dialogFlags,
	)
	d.Dialog.SetHideOnClose(false)
	d.Dialog.SetDefaultSize(400, 275)
	d.Dialog.SetChild(qs)
	d.Dialog.ConnectShow(func() {
		qs.search.GrabFocus()
	})

	// Jank.
	qs.Box.Remove(qs.search)
	header := d.Dialog.HeaderBar()
	header.SetTitleWidget(qs.search)

	esc := gtk.NewEventControllerKey()
	esc.SetName("dialog-escape")
	esc.ConnectKeyPressed(func(val, code uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Escape:
			d.Dialog.Close()
			return true
		}
		return false
	})
	d.Dialog.AddController(esc)

	qs.search.SetKeyCaptureWidget(d.Dialog)

	return &d
}

type dialogControlling Dialog

func (d *dialogControlling) OpenGuild(id discord.GuildID) {
	(*Dialog)(d).ctrl.OpenGuild(id)
	(*Dialog)(d).Close()
}

func (d *dialogControlling) OpenChannel(id discord.ChannelID) {
	(*Dialog)(d).ctrl.OpenChannel(id)
	(*Dialog)(d).Close()
}
