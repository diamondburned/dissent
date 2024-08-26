package quickswitcher

import (
	"context"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// Dialog is a Quick Switcher dialog.
type Dialog struct {
	*adw.Dialog
	QuickSwitcher *QuickSwitcher
}

// ShowDialog shows a new Quick Switcher dialog.
func ShowDialog(ctx context.Context) {
	d := NewDialog(ctx)
	d.Present(app.GTKWindowFromContext(ctx))
}

var dialogCSS = cssutil.Applier("quickswitcher-dialog", "")

// NewDialog creates a new Quick Switcher dialog.
func NewDialog(ctx context.Context) *Dialog {
	qs := NewQuickSwitcher(ctx)
	qs.Box.Remove(qs.search) // jank
	qs.search.SetHExpand(true)

	app := app.FromContext(ctx)

	header := adw.NewHeaderBar()
	header.SetTitleWidget(qs.search)

	toolbarView := adw.NewToolbarView()
	toolbarView.SetTopBarStyle(adw.ToolbarFlat)
	toolbarView.AddTopBar(header)
	toolbarView.SetContent(qs)

	d := Dialog{QuickSwitcher: qs}
	d.Dialog = adw.NewDialog()
	d.SetContentWidth(375)
	d.SetContentHeight(275)
	d.SetTitle(app.SuffixedTitle("Quick Switcher"))
	d.SetChild(toolbarView)
	d.ConnectShow(func() {
		qs.Clear()
		qs.search.GrabFocus()
	})
	dialogCSS(d)

	qs.ConnectChosen(func() {
		d.Close()
	})

	esc := gtk.NewEventControllerKey()
	esc.SetName("dialog-escape")
	esc.ConnectKeyPressed(func(val, _ uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Escape:
			d.Close()
			return true
		}
		return false
	})

	qs.search.SetKeyCaptureWidget(d)
	qs.search.AddController(esc)

	return &d
}
