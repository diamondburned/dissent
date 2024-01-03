package threads

import (
	"context"
	"fmt"
	"html"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/signaling"
)

// type messagePreviewType int
//
// const (
// 	noMessagePreview messagePreviewType = iota
// 	previewFirstMessage
// 	previewLastMessage
// )

// ThreadOpts is the options for a thread item.
type ThreadOpts struct {
	// previewType messagePreviewType
}

func newThreadItemFactory(ctx context.Context, model *ThreadsListModel, opts ThreadOpts) *gtk.ListItemFactory {
	unbindFns := map[uintptr]func(){}

	factory := gtk.NewSignalListItemFactory()

	factory.ConnectBind(func(item *gtk.ListItem) {
		unbindFns[item.Native()] = bindThreadItem(ctx, item, opts)
	})

	factory.ConnectUnbind(func(item *gtk.ListItem) {
		unbind := unbindFns[item.Native()]
		unbind()
		delete(unbindFns, item.Native())
		item.SetChild(nil)
	})

	return &factory.ListItemFactory
}

func bindThreadItem(ctx context.Context, item *gtk.ListItem, opts ThreadOpts) func() {
	var unbind signaling.DisconnectStack
	state := gtkcord.FromContext(ctx)
	chID := gtkutil.ListItemValue[discord.ChannelID](item)

	thread := newThreadItem(opts)
	thread.Update(ctx, chID)
	item.SetChild(thread)

	unbind.Push(
		state.AddHandler(func(ev *gateway.MessageCreateEvent) {
			if ev.ChannelID == chID {
				thread.Update(ctx, chID)
			}
		}),
	)

	return unbind.Disconnect
}

// fetchFirstMessage fetches the first message of the channel.
// An error is returned if the channel is empty.
func fetchFirstMessage(state *gtkcord.State, chID discord.ChannelID) (*discord.Message, error) {
	messages, err := state.MessagesAfter(chID, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("channel %d is empty", chID)
	}
	return &messages[0], nil
}

type threadItem struct {
	*gtk.Box
	heading struct {
		*gtk.Box
		name   *gtk.Label
		badges *threadBadges
	}
	preview *gtk.Label
	footer  *gtk.Label

	opts ThreadOpts
}

func newThreadItem(opts ThreadOpts) *threadItem {
	t := threadItem{opts: opts}

	t.heading.name = gtk.NewLabel("")
	t.heading.name.AddCSSClass("thread-item-name")
	t.heading.name.SetHExpand(true)
	t.heading.name.SetXAlign(0)

	t.heading.badges = newThreadBadges()
	t.heading.badges.AddCSSClass("thread-item-badges")

	t.heading.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	t.heading.Box.SetHExpand(true)
	t.heading.Box.Append(t.heading.name)
	t.heading.Box.Append(t.heading.badges)

	t.preview = gtk.NewLabel("")
	t.preview.AddCSSClass("thread-item-preview")
	t.preview.SetHExpand(true)
	t.preview.SetXAlign(0)
	t.preview.SetEllipsize(pango.EllipsizeEnd)
	t.SetPreviewExpanded(false)

	t.footer = gtk.NewLabel("")
	t.footer.AddCSSClass("thread-item-footer")
	t.footer.SetHExpand(true)
	t.footer.SetXAlign(0)
	t.footer.Hide()

	t.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	t.Box.AddCSSClass("thread-item")
	t.Box.Append(t.heading.Box)
	t.Box.Append(t.preview)
	t.Box.Append(t.footer)

	return &t
}

// Update asynchronously updates the thread item with the given channel ID.
func (t *threadItem) Update(ctx context.Context, id discord.ChannelID) {
	// I'm always doing first message preview for now, and there's nothing you
	// can do about it.
	gtkutil.Async(ctx, func() func() {
		state := gtkcord.FromContext(ctx)

		ch, _ := state.Cabinet.Channel(id)
		if ch == nil {
			return func() {}
		}

		return func() {
			t.SetTitle(ch.Name)
			t.SetBadges(ch)
			t.SetFooter(ch)

			// TODO: figure out how to reliably get the first message in a
			// forum. This might involve an undocumented gateway command.

			// preview := renderPreviewMarkup(state, message)
			// thread.preview.SetMarkup(preview)
		}
	})
}

// SetTitle sets the title of the thread item.
func (t *threadItem) SetTitle(title string) {
	t.heading.name.SetText(title)
}

// SetBadges sets the badges of the thread item from the thread metadata.
func (t *threadItem) SetBadges(ch *discord.Channel) {
	t.heading.badges.SetPinned(ch.Flags&discord.PinnedThread != 0)
}

// SetFooter sets the footer of the thread item.
func (t *threadItem) SetFooter(ch *discord.Channel) {
	if ch.ThreadMetadata == nil {
		t.HideFooter()
		return
	}

	footer := fmt.Sprintf(
		`%d messages • %s`,
		ch.MessageCount,
		locale.TimeAgo(ch.ThreadMetadata.CreateTimestamp.Time()))
	t.footer.SetMarkup(footer)
	t.footer.Show()
}

// HideFooter hides the footer of the thread item.
func (t *threadItem) HideFooter() {
	t.footer.Hide()
}

// SetPreview sets the preview of the thread item from the message.
func (t *threadItem) SetPreview(state *gtkcord.State, msg *discord.Message) {
	t.preview.SetMarkup(fmt.Sprintf(
		`<b>%s</b>: %s`,
		state.UserMarkup(msg.GuildID, &msg.Author, author.WithMinimal()),
		html.EscapeString(state.MessagePreview(msg)),
	))
}

// SetPreviewExpanded sets whether the preview is expanded.
func (t *threadItem) SetPreviewExpanded(expanded bool) {
	if expanded {
		t.preview.SetLines(-1)
	} else {
		t.preview.SetLines(2)
	}
}

type threadBadges struct {
	*gtk.Box
	pinned *gtk.Image // nullable
}

func newThreadBadges() *threadBadges {
	var b threadBadges
	b.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.Box.SetHAlign(gtk.AlignEnd)
	return &b
}

// SetPinned sets the pinned badge. The pinned badge is a pin icon.
func (b *threadBadges) SetPinned(pinned bool) {
	if pinned == (b.pinned != nil) {
		return
	}

	if pinned {
		b.pinned = gtk.NewImageFromIconName("pin-symbolic")
		b.pinned.AddCSSClass("thread-item-badge")
		b.pinned.AddCSSClass("thread-item-badge-pinned")
		b.pinned.SetTooltipText("Pinned")
	} else {
		b.pinned = nil
	}

	b.reAddAll()
}

// // SetNew sets the new badge. The new badge is a label that says "new".
// func (b *threadBadges) SetNew(new bool) {
// 	if new == (b.new != nil) {
// 		return
// 	}
//
// 	if new {
// 		// TODO
// 	} else {
// 		b.Box.Remove(b.new)
// 		b.new = nil
// 	}
// }

func (b *threadBadges) reAddAll() {
	// NOTE: Make sure to update this if new badges are added.
	widgets := []gtk.Widgetter{b.pinned}
	gtkutil.RemoveChildren(b.Box)
	for _, w := range widgets {
		if w != nil {
			b.Box.Append(w)
		}
	}
}
