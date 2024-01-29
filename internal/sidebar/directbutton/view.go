package directbutton

import (
	"context"
	"log"
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/sidebar/sidebutton"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/states/read"
)

type View struct {
	*gtk.Box
	DM *Button

	mentioned struct {
		IDs     []discord.ChannelID
		Buttons map[discord.ChannelID]*ChannelButton
	}

	ctx context.Context
}

var viewCSS = cssutil.Applier("dmbutton-view", `
`)

func NewView(ctx context.Context) *View {
	v := View{
		Box: gtk.NewBox(gtk.OrientationVertical, 0),
		DM:  NewButton(ctx),
		ctx: ctx,
	}

	v.mentioned.IDs = make([]discord.ChannelID, 0, 4)
	v.mentioned.Buttons = make(map[discord.ChannelID]*ChannelButton, 4)

	v.Append(v.DM)
	viewCSS(v)

	vis := gtkutil.WithVisibility(ctx, v)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(vis, func(ev gateway.Event) {
		switch ev := ev.(type) {
		case *read.UpdateEvent:
			if !ev.GuildID.IsValid() {
				v.Invalidate()
			}
		case *gateway.MessageCreateEvent:
			if !ev.GuildID.IsValid() {
				v.Invalidate()
			}
		}
	},
		(*read.UpdateEvent)(nil),
		(*gateway.MessageCreateEvent)(nil),
	)

	return &v
}

type channelUnreadStatus struct {
	*discord.Channel
	UnreadCount int
}

func (v *View) Invalidate() {
	state := gtkcord.FromContext(v.ctx)

	// This is slow, but whatever.
	dms, err := state.PrivateChannels()
	if err != nil {
		log.Println("dmbutton.View: failed to get private channels:", err)

		// Clear all DMs.
		v.update(nil)
		return
	}

	var unreads map[discord.ChannelID]channelUnreadStatus
	for i, dm := range dms {
		count := state.ChannelCountUnreads(dm.ID, ningen.UnreadOpts{})
		if count == 0 {
			continue
		}

		if unreads == nil {
			unreads = make(map[discord.ChannelID]channelUnreadStatus, 4)
		}

		unreads[dm.ID] = channelUnreadStatus{
			Channel:     &dms[i],
			UnreadCount: count,
		}
	}

	v.update(unreads)
}

func (v *View) update(unreads map[discord.ChannelID]channelUnreadStatus) {
	for _, unread := range unreads {
		button, ok := v.mentioned.Buttons[unread.Channel.ID]
		if !ok {
			button = NewChannelButton(v.ctx, unread.Channel.ID)
			v.mentioned.Buttons[unread.Channel.ID] = button
		}

		button.Update(unread.Channel)
		button.InvalidateUnread()
	}

	// Purge all buttons off the widget.
	gtkutil.RemoveChildren(v)

	// Delete unused buttons.
	for id := range v.mentioned.Buttons {
		if _, ok := unreads[id]; !ok {
			delete(v.mentioned.Buttons, id)
		}
	}

	// Recreate the IDs slice.
	v.mentioned.IDs = v.mentioned.IDs[:0]
	for id := range unreads {
		v.mentioned.IDs = append(v.mentioned.IDs, id)
	}

	// Sort the IDs slice. We'll sort it according to the time that the last
	// message was sent: the most recent message will be at the top.
	sort.Slice(v.mentioned.IDs, func(i, j int) bool {
		mi := unreads[v.mentioned.IDs[i]]
		mj := unreads[v.mentioned.IDs[j]]
		return mi.LastMessageID > mj.LastMessageID
	})

	// Append the buttons back to the widget.
	v.Append(v.DM)
	for _, id := range v.mentioned.IDs {
		v.Append(v.mentioned.Buttons[id])
	}
}

func (v *View) Unselect() {
	v.SetSelected(false)
}

func (v *View) SetSelected(selected bool) {
	if selected {
		v.DM.Pill.State = sidebutton.PillActive
	} else {
		v.DM.Pill.State = sidebutton.PillDisabled
	}
	v.DM.Pill.Invalidate()
}
