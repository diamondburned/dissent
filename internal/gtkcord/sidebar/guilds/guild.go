package guilds

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3"
)

type GuildController interface {
	GuildOpener
	MotionGrouped
}

// Guild is a widget showing a single guild icon.
type Guild struct {
	*gtk.Overlay
	Button *gtk.Button
	Icon   *onlineimage.Avatar
	Name   *NamePopover
	Pill   *Pill
	parent *Folder

	ctx    context.Context
	id     discord.GuildID
	unread ningen.UnreadIndication
}

var guildCSS = cssutil.Applier("guild-guild", `
	.guild-guild > button {
		padding: 4px 12px;
		border: none;
		border-radius: 0;
		background: none;
	}
	.guild-guild image {
		background-color: @theme_bg_color;
	}
	.guild-guild > button .adaptive-avatar > image,
	.guild-guild > button .adaptive-avatar > label {
		outline: 0px solid transparent;
	}
	.guild-guild > button:hover .adaptive-avatar > image,
	.guild-guild > button:hover .adaptive-avatar > label {
		outline: 2px solid @theme_selected_bg_color;
		background-color: alpha(@theme_selected_bg_color, 0.35);
	}
	.guild-guild > button > .adaptive-avatar > image,
	.guild-guild > button > .adaptive-avatar > label {
		border-radius: calc({$guild_icon_size} / 2);
	}
	.guild-guild > button:hover > .adaptive-avatar > image,
	.guild-guild > button:hover > .adaptive-avatar > label {
		border-radius: calc({$guild_icon_size} / 4);
	}
	.guild-guild > button image,
	.guild-guild > button > .adaptive-avatar > image,
	.guild-guild > button > .adaptive-avatar > label {
		transition: 200ms ease;
		transition-property: all;
	}
`)

func NewGuild(ctx context.Context, ctrl GuildController, id discord.GuildID) *Guild {
	g := Guild{
		ctx: ctx,
		id:  id,
	}

	g.Icon = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.GuildIconSize)
	iconAnimation := g.Icon.EnableAnimation()

	g.Button = gtk.NewButton()
	g.Button.SetHasFrame(false)
	g.Button.SetHAlign(gtk.AlignCenter)
	g.Button.SetChild(g.Icon)
	g.Button.ConnectClicked(func() {
		g.Pill.State = PillActive
		g.Pill.Invalidate()

		ctrl.OpenGuild(id)
	})

	g.Pill = NewPill()

	g.Name = NewNamePopover()
	g.Name.SetParent(g.Button)

	ctrl.MotionGroup().ConnectEventControllerMotion(
		g.Button,
		func() {
			g.Name.Popup()
			iconAnimation.Start()
		},
		func() {
			g.Name.Popdown()
			iconAnimation.Stop()
		},
	)

	g.Overlay = gtk.NewOverlay()
	g.Overlay.SetChild(g.Button)
	g.Overlay.AddOverlay(g.Pill)
	guildCSS(g)

	g.SetUnavailable()
	return &g
}

// ID returns the guild ID.
func (g *Guild) ID() discord.GuildID { return g.id }

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
	g.Name.Label.SetMarkup(`<span color="#FF0033">Guild unavailable</span>`)
	g.SetSensitive(false)

	if g.Icon.Initials() == "" {
		g.Icon.SetInitials("?")
	}
}

// Update updates the guild with the given Discord object.
func (g *Guild) Update(guild *discord.Guild) {
	g.SetSensitive(true)
	g.Name.SetName(guild.Name)
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

// Activate activates the button.
func (g *Guild) Activate() bool {
	return g.Button.Activate()
}

// Unselect unselects the guild visually. This is mostly used by the parent
// widget for list-keeping.
func (g *Guild) Unselect() {
	g.Pill.State = PillDisabled
	g.Pill.Invalidate()
}

// IsSelected returns true if the guild is selected.
func (g *Guild) IsSelected() bool {
	return g.Pill.State == PillActive
}

func (g *Guild) viewChild() {}

var channelUnreadTypes = []discord.ChannelType{
	discord.GuildText,
	discord.GuildPublicThread,
	discord.GuildPrivateThread,
}

// InvalidateUnread invalidates the guild's unread state.
func (g *Guild) InvalidateUnread() {
	state := gtkcord.FromContext(g.ctx)
	g.unread = state.GuildIsUnread(g.id, channelUnreadTypes)

	g.Pill.Attrs = PillAttrsFromUnread(g.unread)
	g.Pill.Invalidate()

	if g.parent != nil {
		g.parent.InvalidateUnread()
	}
}
