package guilds

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/sidebar/sidebutton"
	"github.com/diamondburned/ningen/v3"
)

type GuildController interface {
	GuildOpener
}

// Guild is a widget showing a single guild icon.
type Guild struct {
	*sidebutton.Button
	ctx    context.Context
	parent *Folder
	id     discord.GuildID
	name   string
}

var guildCSS = cssutil.Applier("guild-guild", `
`)

func NewGuild(ctx context.Context, ctrl GuildController, id discord.GuildID) *Guild {
	g := Guild{ctx: ctx, id: id}
	g.Button = sidebutton.NewButton(ctx, func() {
		ctrl.OpenGuild(id)
	})
	g.SetUnavailable()
	guildCSS(g)
	return &g
}

// ID returns the guild ID.
func (g *Guild) ID() discord.GuildID { return g.id }

// Name returns the guild's name.
func (g *Guild) Name() string { return g.name }

// Invalidate invalidates and updates the state of the guild.
func (g *Guild) Invalidate() {
	state := gtkcord.FromContext(g.ctx)

	guild, err := state.Cabinet.Guild(g.id)
	if err != nil {
		g.SetUnavailable()
		return
	}

	g.Update(guild)
}

// SetUnavailable sets the guild as unavailable. It stays unavailable until
// either Invalidate sees it or Update is called on it.
func (g *Guild) SetUnavailable() {
	g.name = "(guild unavailable)"

	g.Button.SetTooltipMarkup(`<span color="#FF0033">Guild unavailable</span>`)
	g.SetSensitive(false)

	if g.Icon.Initials() == "" {
		g.Icon.SetInitials("?")
	}
}

// Update updates the guild with the given Discord object.
func (g *Guild) Update(guild *discord.Guild) {
	g.name = guild.Name

	g.SetSensitive(true)
	g.Button.SetTooltipText(guild.Name)
	g.Icon.SetInitials(guild.Name)
	g.Icon.SetFromURL(gtkcord.InjectAvatarSize(guild.IconURL()))

	g.InvalidateUnread()
}

// SetParentFolder sets the parent guild folder.
func (g *Guild) SetParentFolder(parent *Folder) {
	g.parent = parent
}

// ParentFolder returns the guild's parent folder.
func (g *Guild) ParentFolder() *Folder {
	return g.parent
}

func (g *Guild) viewChild() {}

// var channelUnreadTypes = []discord.ChannelType{
// 	discord.GuildText,
// 	discord.GuildPublicThread,
// 	discord.GuildPrivateThread,
// }

// InvalidateUnread invalidates the guild's unread state.
func (g *Guild) InvalidateUnread() {
	state := gtkcord.FromContext(g.ctx)

	var mentions int

	chs, _ := state.Cabinet.Channels(g.id)
	for _, ch := range chs {
		read := state.ReadState.ReadState(ch.ID)
		if read != nil {
			mentions += read.MentionCount
		}
	}

	g.SetIndicator(state.GuildIsUnread(g.id, ningen.GuildUnreadOpts{
		UnreadOpts: ningen.UnreadOpts{},
		Types:      gtkcord.AllowedChannelTypes,
	}))
	g.Mentions.SetCount(mentions)

	if g.parent != nil {
		g.parent.InvalidateUnread()
	}
}
