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
	*adw.ApplicationWindow
	QuickSwitcher *QuickSwitcher
}

// ShowDialog shows a new Quick Switcher dialog.
func ShowDialog(ctx context.Context) {
	d := NewDialog(ctx)
	d.Show()
}

var dialogCSS = cssutil.Applier("quickswitcher-dialog", `
	.quickswitcher-dialog .quickswitcher-list {
		margin: 8px;
	}
	.quickswitcher-dialog .quickswitcher-search {
		margin: 8px 0;
	}
`)

// NewDialog creates a new Quick Switcher dialog.
func NewDialog(ctx context.Context) *Dialog {
	qs := NewQuickSwitcher(ctx)
	qs.Box.Remove(qs.search) // jank
	qs.search.SetHExpand(true)

	win := app.GTKWindowFromContext(ctx)
	app := app.FromContext(ctx)

	header := adw.NewHeaderBar()
	header.SetTitleWidget(qs.search)

	toolbarView := adw.NewToolbarView()
	toolbarView.SetTopBarStyle(adw.ToolbarFlat)
	toolbarView.AddTopBar(header)
	toolbarView.SetContent(qs)

	d := Dialog{QuickSwitcher: qs}
	d.ApplicationWindow = adw.NewApplicationWindow(app.Application)
	d.SetTransientFor(win)
	d.SetDefaultSize(400, 275)
	d.SetModal(true)
	d.SetDestroyWithParent(true)
	d.SetTitle(app.SuffixedTitle("Quick Switcher"))
	d.SetContent(toolbarView)
	d.ConnectShow(func() {
		qs.search.GrabFocus()
	})
	dialogCSS(d)

	// SetDestroyWithParent doesn't work for some reason, so we have to manually
	// destroy the QuickSwitcher on transient window destroy.
	win.ConnectCloseRequest(func() bool {
		d.Destroy()
		return false
	})

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
