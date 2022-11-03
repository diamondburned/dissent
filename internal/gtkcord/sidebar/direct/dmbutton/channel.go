package dmbutton

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar/sidebutton"
	"github.com/diamondburned/ningen/v3"
)

type ChannelController interface {
	OpenChannel(discord.ChannelID)
}

type Channel struct {
	*sidebutton.Button
	id discord.ChannelID
}

var channelCSS = cssutil.Applier("dmbutton-channel", `
`)

func NewChannel(ctx context.Context, ctrl ChannelController, id discord.ChannelID) *Channel {
	ch := Channel{id: id}
	ch.Button = sidebutton.NewButton(ctx, func() {
		ctrl.OpenChannel(id)
	})
	channelCSS(ch)
	return &ch
}

// ID returns the channel ID.
func (c *Channel) ID() discord.ChannelID { return c.id }

// Invalidate invalidates and updates the state of the channel.
func (c *Channel) Invalidate() {
	state := gtkcord.FromContext(c.Context())

	ch, err := state.Cabinet.Channel(c.id)
	if err != nil {
		log.Println("dmbutton.Channel.Invalidate: cannot fetch channel:", err)
		return
	}

	c.Update(ch)
	c.InvalidateUnread()
}

// Update updates the channel with the given Discord object.
func (c *Channel) Update(ch *discord.Channel) {
	name := gtkcord.ChannelName(ch)

	var iconURL string
	if ch.Icon != "" {
		iconURL = ch.IconURL()
	} else if len(ch.DMRecipients) == 1 {
		iconURL = ch.DMRecipients[0].AvatarURL()
	}

	c.Button.SetTooltipText(name)
	c.Icon.SetInitials(name)
	c.Icon.SetFromURL(iconURL)
}

// InvalidateUnread invalidates the guild's unread state.
func (c *Channel) InvalidateUnread() {
	state := gtkcord.FromContext(c.Context())
	unreads := state.CountUnreads(c.id)

	indicator := state.ChannelIsUnread(c.id)
	if indicator != ningen.ChannelRead && unreads == 0 {
		unreads = 1
	}

	c.SetIndicator(indicator)
	c.Mentions.SetCount(unreads)
}
