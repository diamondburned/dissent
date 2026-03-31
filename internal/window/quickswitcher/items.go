package quickswitcher

import (
	"context"
	"html"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/sidebar/channels"
)

// TODO : ABSOLUTE MESS TO REFACTOR
type channelIndexItem interface {
	String() string
	ChannelID() discord.ChannelID
	Row(context.Context) *gtk.ListBoxRow
}
type guildIndexItem interface {
	String() string
	GuildID() discord.GuildID
	QSItem(context.Context) *gtk.Button
}

type guildIndexItems []guildIndexItem
type channelIndexItems []channelIndexItem

func (its guildIndexItems) String(i int) string { return its[i].String() }
func (its guildIndexItems) Len() int            { return len(its) }

func (its channelIndexItems) String(i int) string { return its[i].String() }
func (its channelIndexItems) Len() int            { return len(its) }

// ========================

type channelItem struct {
	*discord.Channel
	guild  *discord.Guild
	name   string
	search string
}

// TODO: move this to gtkcord
var threadTypes = map[discord.ChannelType]bool{
	discord.GuildAnnouncementThread: true,
	discord.GuildPublicThread:       true,
	discord.GuildPrivateThread:      true,
}

var voiceTypes = map[discord.ChannelType]bool{
	discord.GuildVoice:      true,
	discord.GuildStageVoice: true,
}

func newChannelItem(state *gtkcord.State, guild *discord.Guild, ch *discord.Channel) channelItem {
	item := channelItem{
		Channel: ch,
		guild:   guild,
	}

	if ch.Name != "" {
		item.name = ch.Name
	} else {
		item.name = gtkcord.RecipientNames(ch)
	}

	if threadTypes[ch.Type] {
		parent, _ := state.Cabinet.Channel(ch.ParentID)
		if parent != nil {
			item.name = parent.Name + " › #" + item.name
		}
	}

	if item.guild != nil {
		item.search = item.guild.Name + " " + item.name
	} else {
		item.search = item.name
	}

	return item
}

func (it channelItem) String() string               { return it.search }
func (it channelItem) ChannelID() discord.ChannelID { return it.ID }

func (it channelItem) Row(ctx context.Context) *gtk.ListBoxRow {
	tooltip := it.name
	if it.guild != nil {
		tooltip += " (" + it.guild.Name + ")"
	}

	box := adw.NewActionRow()
	right_arrow := gtk.NewImage()
	right_arrow.SetFromIconName("go-next")

	box.SetTitle(html.EscapeString(it.name))
	if it.guild != nil {
		box.SetSubtitle(html.EscapeString(it.guild.Name))

		guildIcon := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.InlineEmojiSize)
		guildIcon.SetText(html.EscapeString(it.guild.Name))
		guildIcon.SetFromURL(it.guild.IconURL())
		box.AddSuffix(guildIcon)
	} else {
		box.SetSubtitle("<i>Direct Message</i>")
	}

	box.AddSuffix(right_arrow)

	row := gtk.NewListBoxRow()
	row.SetTooltipText(tooltip)
	row.SetChild(box)

	switch it.Type {
	case discord.DirectMessage, discord.GroupDM:
		icon := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.ChannelIconSize)
		icon.AddCSSClass("quickswitcher-channel-icon")
		icon.AddCSSClass("quickswitcher-channel-image")
		icon.SetHAlign(gtk.AlignCenter)
		icon.SetText(html.EscapeString(it.name))
		if len(it.DMRecipients) == 1 {
			icon.SetFromURL(gtkcord.InjectAvatarSize(it.DMRecipients[0].AvatarURL()))
		}

		anim := icon.EnableAnimation()
		anim.ConnectMotion(row) // TODO: I wonder if this causes memory leaks.

		box.AddPrefix(icon)
	default:
		icon := channels.NewChannelIcon(it.Channel, func(t discord.ChannelType) (string, bool) {
			_, isThread := threadTypes[t]
			return "thread-branch-symbolic", isThread
		})

		icon.AddCSSClass("quickswitcher-channel-icon")
		icon.AddCSSClass("quickswitcher-channel-hash")
		icon.SetHAlign(gtk.AlignCenter)

		box.AddPrefix(icon)
	}
	return row
}

type guildItem struct {
	*discord.Guild
}

func newGuildItem(guild *discord.Guild) guildItem {
	return guildItem{
		Guild: guild,
	}
}

func (it guildItem) String() string           { return it.Name }
func (it guildItem) GuildID() discord.GuildID { return it.ID }

func (it guildItem) QSItem(ctx context.Context) *gtk.Button {
	row := gtk.NewButton()

	icon := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.GuildIconSize)
	icon.SetText(html.EscapeString(it.Name))
	icon.SetFromURL(it.IconURL())

	anim := icon.EnableAnimation()
	anim.ConnectMotion(row)

	row.SetSizeRequest(48, 48)
	row.SetVAlign(gtk.AlignCenter)
	row.SetHAlign(gtk.AlignCenter)
	row.AddCSSClass("circular")
	row.SetChild(icon)
	return row
}
