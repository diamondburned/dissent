package threads

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ThreadsViewStyle is the style of a threads view.
type ThreadsViewStyle int

const (
	// ThreadsViewStyleNormal is the default style for a threads view.
	// It looks similarly to the small Threads popup box in Discord.
	ThreadsViewStyleNormal ThreadsViewStyle = iota
	// ThreadsViewStyleForumList is the style for a forum view where the
	// threads are shown as a list.
	ThreadsViewStyleForumList

	// TODO: implement ThreadsViewStyleForumGrid
)

// ThreadsView is a view that shows a list of threads.
type ThreadsView struct {
	*gtk.Box
	activeList *ThreadsList
}

// NewThreadsView creates a new threads view.
func NewThreadsView(ctx context.Context, id discord.ChannelID, style ThreadsViewStyle) *ThreadsView {
	activeLabel := gtk.NewLabel("Active Threads")
	activeLabel.SetXAlign(0)

	// TODO: alright, listen up. What the fuck have I gotten myself into?
	// First, I need a list of active and archived threads separately. But I
	// don't actually want to maintain two separate models, because that means I
	// would need two SelectionModels and synchronize them differently. I can't
	// just join the models together either. There's nothing for that! So what
	// do I do?
	//   1. Manually bind the two models together and make its selection
	//      mutually exclusive.
	//   2. I don't know, fuck it we ball? Try to deal with active and archived
	//      threads in the same model? Sounds fucking painful, honestly.

	activeModel := NewActiveThreadsModel(ctx, id)

	activeList := NewThreadsList(ctx, activeModel, ThreadOpts{})
	activeList.AddCSSClass("threads-list-active")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(activeLabel)
	box.Append(activeList)

	return &ThreadsView{
		box,
		activeList,
	}
}

func (v *ThreadsView) ConnectSelected(f func(discord.ChannelID)) func() {

}
