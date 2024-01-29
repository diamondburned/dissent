package quickswitcher

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
)

// Dialog is a Quick Switcher dialog.
type Dialog struct {
	*gtk.Dialog
	QuickSwitcher *QuickSwitcher
}

const dialogFlags = 0 |
	gtk.DialogDestroyWithParent |
	gtk.DialogModal |
	gtk.DialogUseHeaderBar

// ShowDialog shows a new Quick Switcher dialog.
func ShowDialog(ctx context.Context) {
	d := NewDialog(ctx)
	d.Show()
}

// NewDialog creates a new Quick Switcher dialog.
func NewDialog(ctx context.Context) *Dialog {
	qs := NewQuickSwitcher(ctx)

	d := Dialog{QuickSwitcher: qs}
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
	esc.ConnectKeyPressed(func(val, _ uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Escape:
			d.Dialog.Close()
			return true
		}
		return false
	})

	qs.search.SetKeyCaptureWidget(d.Dialog)
	qs.search.AddController(esc)

	if app.IsDevel() {
		d.Dialog.AddCSSClass("devel")
	}

	return &d
}
