package sidebar

import (
	"context"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar/guilds"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/states/read"
)

type DMButton struct {
	*gtk.Overlay
	Name   *guilds.NamePopover
	Pill   *guilds.Pill
	Button *gtk.Button

	ctx context.Context
}

var dmButtonCSS = cssutil.Applier("sidebar-dm-button-overlay", `
	.sidebar-dm-button {
		padding: 4px 12px;
		border-radius: 0;
	}
`)

func NewDMButton(ctx context.Context, open func()) *DMButton {
	b := DMButton{ctx: ctx}

	icon := gtk.NewImageFromIconName("user-available")
	icon.SetIconSize(gtk.IconSizeLarge)
	icon.SetPixelSize(gtkcord.GuildIconSize)

	b.Button = gtk.NewButton()
	b.Button.AddCSSClass("sidebar-dm-button")
	b.Button.SetChild(icon)
	b.Button.SetHasFrame(false)
	b.Button.SetHAlign(gtk.AlignCenter)
	b.Button.ConnectClicked(func() {
		b.Pill.State = guilds.PillActive
		b.Pill.Invalidate()

		open()
	})

	b.Pill = guilds.NewPill()

	b.Name = guilds.NewNamePopover()
	b.Name.SetName("Direct Messages")
	b.Name.SetParent(b.Button)

	// TODO: guilds should share an upper-level MotionGroup.
	motion := gtk.NewEventControllerMotion()
	motion.ConnectEnter(func(_, _ float64) { b.Name.Popup() })
	motion.ConnectLeave(func() { b.Name.Popdown() })

	b.Button.AddController(motion)

	b.Overlay = gtk.NewOverlay()
	b.Overlay.SetChild(b.Button)
	b.Overlay.AddOverlay(b.Pill)

	vis := gtkutil.WithVisibility(ctx, b)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(vis, func(ev gateway.Event) {
		switch ev := ev.(type) {
		case *read.UpdateEvent:
			if ev.GuildID.IsValid() {
				return
			}

			b.Invalidate()
		}
	})

	dmButtonCSS(b)
	return &b
}

// Invalidate forces a complete recheck of all direct messaging channels to
// update the unread indicator.
func (b *DMButton) Invalidate() {
	state := gtkcord.FromContext(b.ctx)
	unread := dmUnreadState(state)

	b.Pill.Attrs = guilds.PillAttrsFromUnread(unread)
	b.Pill.Invalidate()
}

func dmUnreadState(state *gtkcord.State) ningen.UnreadIndication {
	var unread ningen.UnreadIndication

	chs, _ := state.Cabinet.PrivateChannels()

	for _, ch := range chs {
		reads := state.ReadState.ReadState(ch.ID)
		if reads == nil || !reads.LastMessageID.IsValid() {
			continue
		}

		if state.MutedState.Channel(ch.ID) {
			continue
		}

		if reads.LastMessageID < ch.LastMessageID {
			if unread < ningen.ChannelUnread {
				unread = ningen.ChannelUnread
			}
		}

		if reads.MentionCount > 0 {
			if unread < ningen.ChannelMentioned {
				unread = ningen.ChannelMentioned
			}
			// Max, so exit early.
			return unread
		}
	}

	return unread
}
