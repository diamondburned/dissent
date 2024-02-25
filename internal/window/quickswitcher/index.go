package quickswitcher

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/sahilm/fuzzy"
	"libdb.so/dissent/internal/gtkcord"
)

type indexItem interface {
	Row(context.Context) *gtk.ListBoxRow
	String() string
}

type indexItems []indexItem

func (its indexItems) String(i int) string { return its[i].String() }
func (its indexItems) Len() int            { return len(its) }

type channelItem struct {
	*discord.Channel
	guild  *discord.Guild
	name   string
	search string
}

// TODO: move this to gtkcord
var threadTypes = map[discord.ChannelType]bool{
	discord.GuildNewsThread:    true,
	discord.GuildPublicThread:  true,
	discord.GuildPrivateThread: true,
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
	} else if len(ch.DMRecipients) == 1 {
		item.name = ch.DMRecipients[0].Tag()
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

func (it channelItem) String() string { return it.search }

const (
	chHash       = `<span face="monospace"><b><span size="x-large" rise="-800">#</span><span size="x-small" rise="-2000">  </span></b></span>`
	chNSFWHash   = `<span face="monospace"><b><span size="x-large" rise="-800">#</span><span size="x-small" rise="-2000">! </span></b></span>`
	chVoiceHash  = `<span face="monospace"><b><span size="x-large" rise="-800">#</span><span size="xx-small" rise="-2000">🔊</span></b></span>`
	chThreadHash = `<span face="monospace"><b><span size="x-large" rise="-800">#</span><span size="x-small" rise="-2000"># </span></b></span>`
)

var channelCSS = cssutil.Applier("quickswitcher-channel", `
	.quickswitcher-channel-icon {
		margin: 2px 12px;
		margin-right: 1px;
		min-width:  {$inline_emoji_size};
		min-height: {$inline_emoji_size};
	}
	.quickswitcher-channel-hash {
		padding-left: 1px; /* account for the NSFW mark */
		margin-right: 7px;
	}
	.quickswitcher-channel-image {
		margin-left: 8px;
		margin-right: 12px;
	}
	.quickswitcher-channel-guildname {
		font-size: 0.9em;
		color: alpha(@theme_fg_color, 0.75);
		margin: 4px;
		margin-left: 18px;
		margin-bottom: calc(4px - 0.1em);
	}
`)

func (it channelItem) Row(ctx context.Context) *gtk.ListBoxRow {
	tooltip := it.name
	if it.guild != nil {
		tooltip += " (" + it.guild.Name + ")"
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)

	row := gtk.NewListBoxRow()
	row.SetTooltipText(tooltip)
	row.SetChild(box)
	channelCSS(row)

	switch it.Type {
	case discord.DirectMessage, discord.GroupDM:
		icon := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.InlineEmojiSize)
		icon.AddCSSClass("quickswitcher-channel-icon")
		icon.AddCSSClass("quickswitcher-channel-image")
		icon.SetHAlign(gtk.AlignCenter)
		icon.SetInitials(it.name)
		if len(it.DMRecipients) == 1 {
			icon.SetFromURL(gtkcord.InjectAvatarSize(it.DMRecipients[0].AvatarURL()))
		}

		anim := icon.EnableAnimation()
		anim.ConnectMotion(row) // TODO: I wonder if this causes memory leaks.

		box.Append(icon)
	default:
		icon := gtk.NewLabel("")
		icon.AddCSSClass("quickswitcher-channel-icon")
		icon.AddCSSClass("quickswitcher-channel-hash")
		icon.SetHAlign(gtk.AlignCenter)
		switch {
		case it.NSFW:
			icon.SetMarkup(chNSFWHash)
		case voiceTypes[it.Type]:
			icon.SetMarkup(chVoiceHash)
		case threadTypes[it.Type]:
			icon.SetMarkup(chThreadHash)
		default:
			icon.SetMarkup(chHash)
		}

		box.Append(icon)
	}

	name := gtk.NewLabel(it.name)
	name.AddCSSClass("quickswitcher-channel-name")
	name.SetHExpand(true)
	name.SetXAlign(0)
	name.SetEllipsize(pango.EllipsizeEnd)

	box.Append(name)

	if it.guild != nil {
		guildName := gtk.NewLabel(it.guild.Name)
		guildName.AddCSSClass("quickswitcher-channel-guildname")
		guildName.SetEllipsize(pango.EllipsizeEnd)

		box.Append(guildName)
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

func (it guildItem) String() string { return it.Name }

var guildCSS = cssutil.Applier("quickswitcher-guild", `
	.quickswitcher-guild-icon {
		margin: 2px 8px;
		min-width:  {$inline_emoji_size};
		min-height: {$inline_emoji_size};
	}
`)

func (it guildItem) Row(ctx context.Context) *gtk.ListBoxRow {
	row := gtk.NewListBoxRow()
	guildCSS(row)

	icon := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.InlineEmojiSize)
	icon.AddCSSClass("quickswitcher-guild-icon")
	icon.SetInitials(it.Name)
	icon.SetFromURL(it.IconURL())
	icon.SetHAlign(gtk.AlignCenter)

	anim := icon.EnableAnimation()
	anim.ConnectMotion(row)

	name := gtk.NewLabel(it.Name)
	name.AddCSSClass("quickswitcher-guild-name")
	name.SetHExpand(true)
	name.SetXAlign(0)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(icon)
	box.Append(name)

	row.SetChild(box)
	return row
}

type index struct {
	items  indexItems
	buffer indexItems
}

const searchLimit = 25

func (idx *index) update(ctx context.Context) {
	state := gtkcord.FromContext(ctx).Offline()
	items := make([]indexItem, 0, 250)

	dms, err := state.PrivateChannels()
	if err != nil {
		app.Error(ctx, err)
		return
	}

	for i := range dms {
		items = append(items, newChannelItem(state, nil, &dms[i]))
	}

	guilds, err := state.Guilds()
	if err != nil {
		app.Error(ctx, err)
		return
	}

	for i, guild := range guilds {
		chs, err := state.Channels(guild.ID, gtkcord.AllowedChannelTypes)
		if err != nil {
			log.Print("quickswitcher: cannot populate channels for guild ", guild.Name, ": ", err)
			continue
		}
		items = append(items, newGuildItem(&guilds[i]))
		for j := range chs {
			items = append(items, newChannelItem(state, &guilds[i], &chs[j]))
		}
	}

	idx.items = items
}

func (idx *index) search(str string) []indexItem {
	if idx.items == nil {
		return nil
	}

	idx.buffer = idx.buffer[:0]
	if idx.buffer == nil {
		idx.buffer = make([]indexItem, 0, searchLimit)
	}

	matches := fuzzy.FindFrom(str, idx.items)
	for i := 0; i < len(matches) && i < searchLimit; i++ {
		idx.buffer = append(idx.buffer, idx.items[matches[i].Index])
	}

	return idx.buffer
}
