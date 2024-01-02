package directbutton

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/sidebar/sidebutton"
	"github.com/diamondburned/ningen/v3"
)

type ChannelButton struct {
	*sidebutton.Button
	id discord.ChannelID
}

var channelCSS = cssutil.Applier("dmbutton-channel", `
`)

func NewChannelButton(ctx context.Context, id discord.ChannelID, opener Opener) *ChannelButton {
	ch := ChannelButton{id: id}
	ch.Button = sidebutton.NewButton(ctx, func() { opener.OpenChannel(id) })
	channelCSS(ch)
	return &ch
}

// ID returns the channel ID.
func (c *ChannelButton) ID() discord.ChannelID { return c.id }

// Invalidate invalidates and updates the state of the channel.
func (c *ChannelButton) Invalidate() {
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
func (c *ChannelButton) Update(ch *discord.Channel) {
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
func (c *ChannelButton) InvalidateUnread() {
	state := gtkcord.FromContext(c.Context())
	unreads := state.ChannelCountUnreads(c.id, ningen.UnreadOpts{})

	indicator := state.ChannelIsUnread(c.id, ningen.UnreadOpts{})
	if indicator != ningen.ChannelRead && unreads == 0 {
		unreads = 1
	}

	c.SetIndicator(indicator)
	c.Mentions.SetCount(unreads)
}
