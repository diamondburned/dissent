package sidebar

import (
	"context"
	"regexp"
	"strconv"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"libdb.so/dissent/internal/gtkcord"
)

type userBar struct {
	// *gtk.ActionBar
	*gtk.Box
	avatar *onlineimage.Avatar
	name   *gtk.Label
	status *gtk.MenuButton
	menu   *gtk.MenuButton

	ctx context.Context
}

func setupUserBar(ctx context.Context, box *gtk.Box, avatar *adw.Avatar, username *gtk.Label, status *gtk.MenuButton, menu *gtk.MenuButton) {
	b := userBar{
		Box:    box,
		ctx:    ctx,
		name:   username,
		status: status,
		menu:   menu,
	}
	b.avatar = onlineimage.CreateAvatarFromObj(
		ctx,
		avatar,
		imgutil.HTTPProvider,
		gtkcord.UserBarAvatarSize,
	)

	b.updatePresence(nil)

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
}

var discriminatorRe = regexp.MustCompile(`#\d{1,4}$`)

func (b *userBar) updateUser(me *discord.User) {
	tag := me.Username
	if v, _ := strconv.Atoi(me.Discriminator); v != 0 {
		tag += `<span size="smaller">` + "#" + me.Discriminator + "</span>"
	}

	var name string
	if me.DisplayName != "" {
		name = me.DisplayName + "\n" + `<span size="smaller">` + tag + "</span>"
	} else {
		name = tag
	}

	displayName := me.DisplayName
	if displayName == "" {
		displayName = me.Username
	}

	b.avatar.SetText(displayName)
	b.avatar.SetFromURL(me.AvatarURL())
	b.name.SetMarkup(name)
	b.name.SetTooltipMarkup(name)
}

func (b *userBar) updatePresence(presence *discord.Presence) {
	if presence == nil {
		b.status.SetTooltipText(statusText(discord.UnknownStatus))
		b.status.SetIconName(statusIcon(discord.UnknownStatus))
		return
	}

	if presence.User.Username != "" {
		b.updateUser(&presence.User)
	}

	b.status.SetTooltipText(statusText(presence.Status))
	b.status.SetIconName(statusIcon(presence.Status))
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
