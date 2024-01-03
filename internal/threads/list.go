package threads

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var threadsListCSS = cssutil.Applier("threads-list", ``)

// ThreadsList is a list of threads.
type ThreadsList struct {
	*gtk.ListView
	selection *gtk.SingleSelection
}

// NewThreadsList creates a new list of threads from the given threads model.
func NewThreadsList(ctx context.Context, model *ThreadsListModel, opts ThreadOpts) *ThreadsList {
	selection := gtk.NewSingleSelection(model)
	selection.SetCanUnselect(false)
	selection.SetAutoselect(false)

	view := gtk.NewListView(selection, newThreadItemFactory(ctx, model, opts))
	threadsListCSS(view)

	return &ThreadsList{
		view,
		selection,
	}
}

func (l *ThreadsList) ConnectSelected(f func(discord.ChannelID)) glib.SignalHandle {
	return l.selection.ConnectSelectionChanged(func(_, _ uint) {
		item := l.selection.SelectedItem()
		if item == nil {
			f(0)
			return
		}

		chID := gtkutil.StringObjectValue[discord.ChannelID](item)
		f(chID)
	})
}
