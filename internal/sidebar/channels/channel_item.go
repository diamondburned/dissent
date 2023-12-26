package channels

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/signaling"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/states/read"
)

func newChannelItemFactory(state *gtkcord.State, model *gtk.TreeListModel) *gtk.ListItemFactory {
	factory := gtk.NewSignalListItemFactory()

	unbindFns := make(map[uintptr]func())

	factory.ConnectBind(func(item *gtk.ListItem) {
		row := model.Row(item.Position())
		unbind := bindChannelItem(state, item, row)
		unbindFns[item.Native()] = unbind
	})

	factory.ConnectUnbind(func(item *gtk.ListItem) {
		unbind := unbindFns[item.Native()]
		unbind()
		item.SetChild(nil)
	})

	return &factory.ListItemFactory
}

func channelIDFromListItem(item *gtk.ListItem) discord.ChannelID {
	return channelIDFromItem(item.Item())
}

func channelIDFromItem(item *glib.Object) discord.ChannelID {
	str := item.Cast().(*gtk.StringObject)

	id, err := discord.ParseSnowflake(str.String())
	if err != nil {
		panic(fmt.Sprintf("channelIDFromListItem: failed to parse ID: %v", err))
	}

	return discord.ChannelID(id)
}

var _ = cssutil.WriteCSS(`
	.channel-item {
		padding: 0.35em 0;
	}
	.channel-item image {
		margin: 0 0.65em;
	}
	.channel-item-muted {
		opacity: 0.5;
	}
	.channel-unread-indicator {
		font-size: 0.75em;
		font-weight: 700;
	}
	.channel-item-unread .channel-unread-indicator,
	.channel-item-mentioned .channel-unread-indicator {
		font-size: 0.7em;
		font-weight: 900;
		font-family: monospace;

		min-width: 1em;
		min-height: 1em;
		line-height: 1em;

		padding: 0;
		margin: 0 1em;

		outline: 1.5px solid @theme_fg_color;
		border-radius: 99px;
	}
	.channel-item-mentioned .channel-unread-indicator {
		font-size: 0.8em;
		outline-color: @mentioned;
		background: @mentioned;
		color: @theme_bg_color;
	}
`)

type channelItem struct {
	state *gtkcord.State
	item  *gtk.ListItem
	row   *gtk.TreeListRow

	child struct {
		*gtk.Box
		content   gtk.Widgetter
		indicator *gtk.Label
	}

	chID discord.ChannelID
}

func bindChannelItem(state *gtkcord.State, item *gtk.ListItem, row *gtk.TreeListRow) func() {
	i := &channelItem{
		state: state,
		item:  item,
		row:   row,
		chID:  channelIDFromListItem(item),
	}

	i.child.indicator = gtk.NewLabel("")
	i.child.indicator.AddCSSClass("channel-unread-indicator")
	i.child.indicator.SetHExpand(true)
	i.child.indicator.SetHAlign(gtk.AlignEnd)
	i.child.indicator.SetVAlign(gtk.AlignCenter)

	i.child.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	i.child.Box.AddCSSClass("channel-item-outer")
	i.child.Box.Append(i.child.indicator)

	i.item.SetChild(i.child.Box)

	var unbind signaling.DisconnectStack
	unbind.Push(
		state.AddHandler(func(ev *read.UpdateEvent) {
			if ev.ChannelID == i.chID {
				i.Invalidate()
			}
		}),
		state.AddHandler(func(ev *gateway.ChannelUpdateEvent) {
			if ev.ID == i.chID {
				i.Invalidate()
			}
		}),
	)

	ch, _ := state.Offline().Channel(i.chID)
	if ch != nil {
		switch ch.Type {
		case discord.GuildPublicThread, discord.GuildPrivateThread, discord.GuildAnnouncementThread:
			unbind.Push(state.AddHandler(func(ev *gateway.ThreadUpdateEvent) {
				if ev.ID == i.chID {
					i.Invalidate()
				}
			}))
		}

		guildID := ch.GuildID
		switch ch.Type {
		case discord.GuildVoice, discord.GuildStageVoice:
			unbind.Push(state.AddHandler(func(ev *gateway.VoiceStateUpdateEvent) {
				// The channel ID becomes null when the user leaves the channel,
				// so we'll just update when any guild state changes.
				if ev.GuildID == guildID {
					i.Invalidate()
				}
			}))
		}
	}

	i.Invalidate()
	return unbind.Disconnect
}

var readCSSClasses = map[ningen.UnreadIndication]string{
	ningen.ChannelUnread:    "channel-item-unread",
	ningen.ChannelMentioned: "channel-item-mention",
}

const channelMutedClass = "channel-item-muted"

// Invalidate updates the channel item's contents.
func (i *channelItem) Invalidate() {
	if i.child.content != nil {
		i.child.Box.Remove(i.child.content)
	}

	i.item.SetSelectable(true)
	i.item.SetActivatable(false)

	ch, _ := i.state.Offline().Channel(i.chID)
	if ch == nil {
		i.child.content = newUnknownChannelItem(i.chID.String())
		i.item.SetSelectable(false)
	} else {
		switch ch.Type {
		case
			discord.GuildText, discord.GuildAnnouncement,
			discord.GuildPublicThread, discord.GuildPrivateThread, discord.GuildAnnouncementThread:

			i.child.content = newChannelItemText(ch)

		case discord.GuildCategory, discord.GuildForum:
			// allow double-clicking to expand/collapse categories
			i.item.SetSelectable(false)
			i.item.SetActivatable(true)

			switch ch.Type {
			case discord.GuildCategory:
				i.child.content = newChannelItemCategory(ch, i.row)
			case discord.GuildForum:
				i.child.content = newChannelItemForum(ch, i.row)
			}

		case discord.GuildVoice, discord.GuildStageVoice:
			i.child.content = newChannelItemVoice(i.state, ch)

		default:
			panic("unreachable")
		}
	}

	i.child.Box.Prepend(i.child.content)

	for _, cssClass := range readCSSClasses {
		i.child.Box.RemoveCSSClass(cssClass)
	}

	unread := i.state.ChannelIsUnread(i.chID)
	if unread != ningen.ChannelRead {
		i.child.Box.AddCSSClass(readCSSClasses[unread])
	}
	i.updateIndicator(unread)

	if i.state.ChannelIsMuted(i.chID, false) {
		i.child.Box.AddCSSClass(channelMutedClass)
	} else {
		i.child.Box.RemoveCSSClass(channelMutedClass)
	}
}

func (i *channelItem) updateIndicator(unread ningen.UnreadIndication) {
	if unread == ningen.ChannelMentioned {
		i.child.indicator.SetText("!")
	} else {
		i.child.indicator.SetText("")
	}
}

var _ = cssutil.WriteCSS(`
	.channel-item-unknown {
		opacity: 0.5;
		font-style: italic;
	}
`)

func newUnknownChannelItem(name string) gtk.Widgetter {
	icon := gtk.NewImageFromIconName("channel-symbolic")

	label := gtk.NewLabel(name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("channel-item")
	box.AddCSSClass("channel-item-unknown")
	box.Append(icon)
	box.Append(label)

	return box
}

var _ = cssutil.WriteCSS(`
	.channel-item-thread {
		padding: 0.25em 0;
		opacity: 0.5;
	}
	.channel-item-unread .channel-item-thread,
	.channel-item-mention .channel-item-thread {
		opacity: 1;
	}
`)

func newChannelItemText(ch *discord.Channel) gtk.Widgetter {
	icon := gtk.NewImageFromIconName("")
	switch ch.Type {
	case discord.GuildText:
		icon.SetFromIconName("channel-symbolic")
	case discord.GuildAnnouncement:
		icon.SetFromIconName("channel-broadcast-symbolic")
	case discord.GuildPublicThread, discord.GuildPrivateThread, discord.GuildAnnouncementThread:
		icon.SetFromIconName("thread-branch-symbolic")
	}

	label := gtk.NewLabel(ch.Name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)
	bindLabelTooltip(label, false)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("channel-item")
	box.Append(icon)
	box.Append(label)

	switch ch.Type {
	case discord.GuildText:
		box.AddCSSClass("channel-item-text")
	case discord.GuildAnnouncement:
		box.AddCSSClass("channel-item-announcement")
	case discord.GuildPublicThread, discord.GuildPrivateThread, discord.GuildAnnouncementThread:
		box.AddCSSClass("channel-item-thread")
	}

	return box
}

var _ = cssutil.WriteCSS(`
	.channel-item-forum {
		padding: 0.35em 0;
	}
	.channel-item-forum expander {
		margin-left: 0.65em;
		margin-right: 0.35em;
	}
	.channel-item-forum label {
		padding: 0;
	}
`)

func newChannelItemForum(ch *discord.Channel, row *gtk.TreeListRow) gtk.Widgetter {
	label := gtk.NewLabel(ch.Name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)
	bindLabelTooltip(label, false)

	expander := gtk.NewTreeExpander()
	expander.AddCSSClass("channel-item")
	expander.AddCSSClass("channel-item-forum")
	expander.SetListRow(row)
	expander.SetChild(label)

	// GTK 4.10 or later only.
	expander.SetObjectProperty("indent-for-depth", false)

	return expander
}

var _ = cssutil.WriteCSS(`
	.channel-item-category {
		margin-top: 0.5em;
		padding: 0.4em;
	}
	.channel-item-category expander {
		margin: 0 0.3em;
	}
	.channel-item-category label {
		margin-bottom: -0.2em;
		padding: 0;
		font-size: 0.85em;
		font-weight: 700;
		text-transform: uppercase;
	}
`)

func newChannelItemCategory(ch *discord.Channel, row *gtk.TreeListRow) gtk.Widgetter {
	label := gtk.NewLabel(ch.Name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)
	bindLabelTooltip(label, false)

	expander := gtk.NewTreeExpander()
	expander.AddCSSClass("channel-item")
	expander.AddCSSClass("channel-item-category")
	expander.SetListRow(row)
	expander.SetChild(label)

	return expander
}

var _ = cssutil.WriteCSS(`
	.channel-item-voice .mauthor-chip {
		margin: 0.15em 0;
		margin-left: 2.5em;
		margin-right: 1em;
	}
	.channel-item-voice .mauthor-chip:nth-child(2) {
		margin-top: 0;
	}
	.channel-item-voice .mauthor-chip:last-child {
		margin-bottom: 0.3em;
	}
	.channel-item-voice-counter {
		margin-left: 0.5em;
		margin-right: 0.5em;
		font-size: 0.8em;
		opacity: 0.5;
	}
`)

func newChannelItemVoice(state *gtkcord.State, ch *discord.Channel) gtk.Widgetter {
	icon := gtk.NewImageFromIconName("channel-voice-symbolic")

	label := gtk.NewLabel(ch.Name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)
	label.SetTooltipText(ch.Name)

	top := gtk.NewBox(gtk.OrientationHorizontal, 0)
	top.AddCSSClass("channel-item")
	top.Append(icon)
	top.Append(label)

	var voiceParticipants int
	voiceStates, _ := state.VoiceStates(ch.GuildID)
	for _, voiceState := range voiceStates {
		if voiceState.ChannelID == ch.ID {
			voiceParticipants++
		}
	}

	if voiceParticipants > 0 {
		counter := gtk.NewLabel(fmt.Sprintf("%d", voiceParticipants))
		counter.AddCSSClass("channel-item-voice-counter")
		counter.SetVExpand(true)
		counter.SetXAlign(0)
		counter.SetYAlign(1)
		top.Append(counter)
	}

	return top

	// TODO: fix read indicator alignment. This probably should be in a separate
	// ListModel instead.

	// box := gtk.NewBox(gtk.OrientationVertical, 0)
	// box.AddCSSClass("channel-item-voice")
	// box.Append(top)

	// voiceStates, _ := state.VoiceStates(ch.GuildID)
	// for _, voiceState := range voiceStates {
	// 	if voiceState.ChannelID == ch.ID {
	// 		box.Append(newVoiceParticipant(state, voiceState))
	// 	}
	// }

	// return box
}

func newVoiceParticipant(state *gtkcord.State, voiceState discord.VoiceState) gtk.Widgetter {
	chip := author.NewChip(context.Background(), imgutil.HTTPProvider)
	chip.Unpad()

	member := voiceState.Member
	if member == nil {
		member, _ = state.Member(voiceState.GuildID, voiceState.UserID)
	}

	if member != nil {
		chip.SetName(member.User.DisplayOrUsername())
		chip.SetAvatar(gtkcord.InjectAvatarSize(member.AvatarURL(voiceState.GuildID)))
		if color, ok := state.MemberColor(voiceState.GuildID, voiceState.UserID); ok {
			chip.SetColor(color.String())
		}
	} else {
		chip.SetName(voiceState.UserID.String())
	}

	return chip
}

func bindLabelTooltip(label *gtk.Label, markup bool) {
	ref := glib.NewWeakRef(label)
	label.NotifyProperty("label", func() {
		label := ref.Get()
		inner := label.Label()
		if markup {
			label.SetTooltipMarkup(inner)
		} else {
			label.SetTooltipText(inner)
		}
	})
}
