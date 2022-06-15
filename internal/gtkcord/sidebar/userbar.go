package sidebar

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

type userBar struct {
	// *gtk.ActionBar
	*gtk.Box
	avatar *onlineimage.Avatar
	name   *gtk.Label
	status *gtk.Image
	menu   *gtk.ToggleButton

	ctx context.Context
}

var userBarCSS = cssutil.Applier("user-bar", `
	.user-bar-avatar {
		padding: 6px;
	}
	.user-bar-menu {
		margin: 0 6px;
	}
	.user-bar {
		border-top: 1px solid @borders;
	}
`)

func newUserBar(ctx context.Context, menuActions []gtkutil.PopoverMenuItem) *userBar {
	b := userBar{ctx: ctx}
	b.avatar = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.UserBarAvatarSize)
	b.avatar.AddCSSClass("user-bar-avatar")

	b.name = gtk.NewLabel("")
	b.name.AddCSSClass("user-bar-name")
	b.name.SetSelectable(true)
	b.name.SetXAlign(0)
	b.name.SetHExpand(true)
	b.name.SetWrap(false)
	b.name.SetEllipsize(pango.EllipsizeEnd)

	b.status = gtk.NewImage()
	b.status.AddCSSClass("user-bar-status")
	b.updatePresence(nil)

	b.menu = gtk.NewToggleButton()
	b.menu.AddCSSClass("user-bar-menu")
	b.menu.SetIconName("open-menu-symbolic")
	b.menu.SetHasFrame(false)
	b.menu.SetVAlign(gtk.AlignCenter)
	b.menu.ConnectClicked(func() {
		p := gtkutil.NewPopoverMenuCustom(b.menu, gtk.PosTop, menuActions)
		p.ConnectHide(func() { b.menu.SetActive(false) })
		gtkutil.PopupFinally(p)
	})

	b.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.Box.Append(b.avatar)
	b.Box.Append(b.name)
	b.Box.Append(b.status)
	b.Box.Append(b.menu)
	userBarCSS(b)

	anim := b.avatar.EnableAnimation()
	anim.ConnectMotion(b)

	vis := gtkutil.WithVisibility(ctx, b)

	client := gtkcord.FromContext(ctx)
	client.BindHandler(vis,
		func(ev gateway.Event) {
			switch ev := ev.(type) {
			case *gateway.UserUpdateEvent:
				b.updateUser(&ev.User)
			case
				*gateway.PresenceUpdateEvent,
				*gateway.PresencesReplaceEvent,
				*gateway.SessionsReplaceEvent,
				*gateway.UserSettingsUpdateEvent,
				*gateway.ReadyEvent:
				b.invalidatePresence()
			}
		},
		(*gateway.UserUpdateEvent)(nil),
		(*gateway.PresenceUpdateEvent)(nil),
		(*gateway.PresencesReplaceEvent)(nil),
		(*gateway.SessionsReplaceEvent)(nil),
		(*gateway.UserSettingsUpdateEvent)(nil),
		(*gateway.ReadyEvent)(nil),
	)

	me, _ := client.Me()
	if me != nil {
		b.updateUser(me)

	}

	return &b
}

func (b *userBar) updateUser(me *discord.User) {
	b.avatar.SetInitials(me.Username)
	b.avatar.SetFromURL(me.AvatarURL())
	b.name.SetMarkup(fmt.Sprintf(
		`%s`+"\n"+`<span size="smaller">#%s</span>`,
		me.Username, me.Discriminator,
	))
}

func (b *userBar) updatePresence(presence *discord.Presence) {
	if presence == nil {
		b.status.SetTooltipText(statusText(discord.UnknownStatus))
		b.status.SetFromIconName(statusIcon(discord.UnknownStatus))
		return
	}

	if presence.User.Username != "" {
		b.updateUser(&presence.User)
	}

	b.status.SetTooltipText(statusText(presence.Status))
	b.status.SetFromIconName(statusIcon(presence.Status))
}

func (b *userBar) invalidatePresence() {
	state := gtkcord.FromContext(b.ctx)
	me, _ := state.Me()

	presence, _ := state.PresenceStore.Presence(0, me.ID)
	if presence != nil {
		b.updatePresence(presence)
	}
}

func statusIcon(status discord.Status) string {
	switch status {
	case discord.OnlineStatus:
		return "user-available"
	case discord.DoNotDisturbStatus:
		return "user-busy"
	case discord.IdleStatus:
		return "user-idle"
	case discord.InvisibleStatus:
		return "user-invisible"
	case discord.OfflineStatus:
		return "user-offline"
	case discord.UnknownStatus:
		fallthrough
	default:
		return "user-status-pending"
	}
}

func statusText(status discord.Status) string {
	switch status {
	case discord.OnlineStatus:
		return "Online"
	case discord.DoNotDisturbStatus:
		return "Busy"
	case discord.IdleStatus:
		return "Idle"
	case discord.InvisibleStatus:
		return "Invisible"
	case discord.OfflineStatus:
		return "Offline"
	case discord.UnknownStatus:
		fallthrough
	default:
		return "Unknown"
	}
}
