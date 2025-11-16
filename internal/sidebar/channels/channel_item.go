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
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/states/read"
	"libdb.so/dissent/internal/components/hoverpopover"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/signaling"
)

var revealStateKey = app.NewStateKey[bool]("collapsed-channels-state")

type channelItemState struct {
	state  *gtkcord.State
	reveal *app.TypedState[bool]
}

func newChannelItemFactory(ctx context.Context, model *gtk.TreeListModel) *gtk.ListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	state := channelItemState{
		state:  gtkcord.FromContext(ctx),
		reveal: revealStateKey.Acquire(ctx),
	}

	unbindFns := make(map[uintptr]func())

	factory.ConnectBind(func(obj *glib.Object) {
		item := obj.Cast().(*gtk.ListItem)
		row := model.Row(item.Position())
		unbind := bindChannelItem(state, item, row)
		unbindFns[item.Native()] = unbind
	})

	factory.ConnectUnbind(func(obj *glib.Object) {
		item := obj.Cast().(*gtk.ListItem)
		unbind := unbindFns[item.Native()]
		unbind()
		delete(unbindFns, item.Native())
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

type channelItem struct {
	state  *gtkcord.State
	item   *gtk.ListItem
	row    *gtk.TreeListRow
	reveal *app.TypedState[bool]

	child struct {
		*gtk.Box
		content   gtk.Widgetter
		indicator *gtk.Label
	}

	chID discord.ChannelID
}

func bindChannelItem(state channelItemState, item *gtk.ListItem, row *gtk.TreeListRow) func() {
	i := &channelItem{
		state:  state.state,
		item:   item,
		row:    row,
		reveal: state.reveal,
		chID:   channelIDFromListItem(item),
	}

	i.child.indicator = gtk.NewLabel("")
	i.child.indicator.AddCSSClass("channel-unread-indicator")
	i.child.indicator.SetHExpand(true)
	i.child.indicator.SetHAlign(gtk.AlignEnd)
	i.child.indicator.SetVAlign(gtk.AlignCenter)

	i.child.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	i.child.Box.Append(i.child.indicator)

	hoverpopover.NewMarkupHoverPopover(i.child.Box, func(w *hoverpopover.MarkupHoverPopoverWidget) bool {
		summary := i.state.SummaryState.LastSummary(i.chID)
		if summary == nil {
			return false
		}

		window := app.GTKWindowFromContext(i.state.Context())
		if window.Width() > 600 {
			w.SetPosition(gtk.PosRight)
		} else {
			w.SetPosition(gtk.PosBottom)
		}

		w.Label.SetEllipsize(pango.EllipsizeEnd)
		w.Label.SetSingleLineMode(true)
		w.Label.SetMaxWidthChars(50)
		w.Label.SetMarkup(fmt.Sprintf(
			"<b>%s</b>%s",
			locale.Get("Chatting about: "),
			summary.Topic,
		))

		return true
	})

	i.item.SetChild(i.child.Box)

	var unbind signaling.DisconnectStack
	unbind.Push(
		i.state.AddHandler(func(ev *read.UpdateEvent) {
			if ev.ChannelID == i.chID {
				i.Invalidate()
			}
		}),
		i.state.AddHandler(func(ev *gateway.ChannelUpdateEvent) {
			if ev.ID == i.chID {
				i.Invalidate()
			}
		}),
	)

	ch, _ := i.state.Offline().Channel(i.chID)
	if ch != nil {
		switch ch.Type {
		case discord.GuildPublicThread, discord.GuildPrivateThread, discord.GuildAnnouncementThread:
			unbind.Push(i.state.AddHandler(func(ev *gateway.ThreadUpdateEvent) {
				if ev.ID == i.chID {
					i.Invalidate()
				}
			}))
		}

		guildID := ch.GuildID
		switch ch.Type {
		case discord.GuildVoice, discord.GuildStageVoice:
			unbind.Push(i.state.AddHandler(func(ev *gateway.VoiceStateUpdateEvent) {
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
	ningen.ChannelMentioned: "channel-item-mentioned",
}

const channelMutedClass = "channel-item-muted"

// Invalidate updates the channel item's contents.
func (i *channelItem) Invalidate() {
	if i.child.content != nil {
		i.child.Box.Remove(i.child.content)
	}

	i.item.SetSelectable(true)

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
			switch ch.Type {
			case discord.GuildCategory:
				i.child.content = newChannelItemCategory(ch, i.row, i.reveal)
				i.item.SetSelectable(false)
			case discord.GuildForum:
				i.child.content = newChannelItemForum(ch, i.row)
			}

		case discord.GuildVoice, discord.GuildStageVoice:
			i.child.content = newChannelItemVoice(i.state, ch)

		default:
			panic("unreachable")
		}
	}

	i.child.Box.SetCSSClasses(nil)
	i.child.Box.Prepend(i.child.content)

	// Steal CSS classes from the child.
	for _, class := range gtk.BaseWidget(i.child.content).CSSClasses() {
		i.child.Box.AddCSSClass(class + "-outer")
	}

	unreadOpts := ningen.UnreadOpts{
		// We can do this within the channel list itself because it's easy to
		// expand categories and see the unread channels within them.
		IncludeMutedCategories: true,
	}

	unread := i.state.ChannelIsUnread(i.chID, unreadOpts)
	if unread != ningen.ChannelRead {
		i.child.Box.AddCSSClass(readCSSClasses[unread])
	}

	i.updateIndicator(unread)

	if i.state.ChannelIsMuted(i.chID, unreadOpts) {
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

func newUnknownChannelItem(name string) gtk.Widgetter {
	icon := NewChannelIcon(nil)

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

func newChannelItemText(ch *discord.Channel) gtk.Widgetter {
	icon := NewChannelIcon(ch)

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

func newChannelItemForum(ch *discord.Channel, row *gtk.TreeListRow) gtk.Widgetter {
	label := gtk.NewLabel(ch.Name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)
	bindLabelTooltip(label, false)

	expander := gtk.NewTreeExpander()
	expander.AddCSSClass("channel-item")
	expander.AddCSSClass("channel-item-forum")
	expander.SetHExpand(true)
	expander.SetListRow(row)
	expander.SetChild(label)

	// GTK 4.10 or later only.
	expander.SetObjectProperty("indent-for-depth", false)

	return expander
}

func newChannelItemCategory(ch *discord.Channel, row *gtk.TreeListRow, reveal *app.TypedState[bool]) gtk.Widgetter {
	label := gtk.NewLabel(ch.Name)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetXAlign(0)
	bindLabelTooltip(label, false)

	expander := gtk.NewTreeExpander()
	expander.AddCSSClass("channel-item")
	expander.AddCSSClass("channel-item-category")
	expander.SetHExpand(true)
	expander.SetListRow(row)
	expander.SetChild(label)

	ref := glib.NewWeakRef[*gtk.TreeListRow](row)
	chID := ch.ID

	// Add this notifier after a small delay so GTK can initialize the row.
	// Otherwise, it will falsely emit the signal.
	glib.TimeoutSecondsAdd(1, func() {
		row := ref.Get()
		if row == nil {
			return
		}

		row.NotifyProperty("expanded", func() {
			row := ref.Get()
			if row == nil {
				return
			}

			// Only retain collapsed states. Expanded states are assumed to be
			// the default.
			if !row.Expanded() {
				reveal.Set(chID.String(), true)
			} else {
				reveal.Delete(chID.String())
			}
		})
	})

	reveal.Get(ch.ID.String(), func(collapsed bool) {
		if collapsed {
			// GTK will actually explode if we set the expanded property without
			// waiting for it to load for some reason?
			glib.IdleAdd(func() { row.SetExpanded(false) })
		}
	})

	return expander
}

func newChannelItemVoice(state *gtkcord.State, ch *discord.Channel) gtk.Widgetter {
	icon := NewChannelIcon(ch)

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
