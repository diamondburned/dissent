package direct

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// Channel is an individual direct messaging channel.
type Channel struct {
	*gtk.ListBoxRow
	box    *gtk.Box
	avatar *onlineimage.Avatar
	name   *gtk.Label

	ctx context.Context
	id  discord.ChannelID
}

var channelCSS = cssutil.Applier("direct-channel", `
	.direct-channel {
		padding: 4px 6px;
	}
	.direct-channel-avatar {
		margin-right: 6px;
	}
`)

// NewChannel creates a new Channel.
func NewChannel(ctx context.Context, id discord.ChannelID) *Channel {
	ch := Channel{
		ctx: ctx,
		id:  id,
	}

	ch.name = gtk.NewLabel("")
	ch.name.AddCSSClass("direct-channel-name")
	ch.name.SetXAlign(0)
	ch.name.SetHExpand(true)
	ch.name.SetEllipsize(pango.EllipsizeEnd)
	ch.name.SetSingleLineMode(true)

	ch.avatar = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.ChannelIconSize)
	ch.avatar.AddCSSClass("direct-channel-avatar")

	ch.box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	ch.box.Append(ch.avatar)
	ch.box.Append(ch.name)

	ch.ListBoxRow = gtk.NewListBoxRow()
	ch.SetChild(ch.box)
	ch.SetName(id.String())
	channelCSS(ch)

	return &ch
}

// Invalidate fetches the same channel from the state and updates itself.
func (ch *Channel) Invalidate() {
	state := gtkcord.FromContext(ch.ctx)

	if channel, _ := state.Cabinet.Channel(ch.id); channel != nil {
		ch.Update(channel)
	}
}

// Update updates the channel to show information from the instance given. ID is
// not checked.
func (ch *Channel) Update(channel *discord.Channel) {
	name := gtkcord.ChannelName(ch.ctx, channel)
	ch.name.SetText(name)
	ch.avatar.SetInitials(name)

	if channel.Type == discord.DirectMessage && len(channel.DMRecipients) > 0 {
		u := channel.DMRecipients[0]
		ch.avatar.SetFromURL(gtkcord.InjectAvatarSize(u.AvatarURL()))
	} else {
		ch.avatar.SetFromURL(gtkcord.InjectAvatarSize(channel.IconURL()))
	}
}

// LastMessageID queries the local state for the channel's last message ID.
func (ch *Channel) LastMessageID() discord.MessageID {
	state := gtkcord.FromContext(ch.ctx)

	channel, _ := state.Cabinet.Channel(ch.id)
	if channel == nil {
		return 0
	}

	return channel.LastMessageID
}

// Name returns the current displaying name of the channel.
func (ch *Channel) Name() string {
	return ch.name.Text()
}

// InvalidateSort invalidates the sorting position of this channel within the
// major channel list.
func (ch *Channel) InvalidateSort() {
	ch.Changed()
}
