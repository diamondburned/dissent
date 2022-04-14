package guilds

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/states/read"
	"github.com/pkg/errors"
)

// ViewChild is a child inside the guilds view. It is either a *Guild or a
// *Folder containing more *Guilds.
type ViewChild interface {
	gtk.Widgetter
	viewChild()
}

// GuildOpener is an interface having an OpenGuild method.
type GuildOpener interface {
	// OpenGuild opens the given guild.
	OpenGuild(discord.GuildID)
}

// Controller is the praent controller that View controls.
type Controller interface {
	GuildOpener
	// CloseGuild is called by View if the guild no longer becomes available. If
	// permanent is true, then the UI must be redirected to the homepage,
	// otherwise, a loading screen is fine.
	CloseGuild(permanent bool)
}

// View contains a list of guilds and folders.
type View struct {
	*gtk.Box
	Children []ViewChild

	current currentGuild

	ctx  context.Context
	ctrl Controller
}

type currentGuild struct {
	guild  *Guild
	folder *Folder
}

var viewCSS = cssutil.Applier("guild-view", `
	.guild-view button:active:not(:hover) {
		background: initial;
	}
`)

// NewView creates a new View.
func NewView(ctx context.Context, ctrl Controller) *View {
	v := View{
		ctx:  ctx,
		ctrl: ctrl,
	}

	v.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	viewCSS(v)

	cancellable := gtkutil.WithVisibility(ctx, v)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(cancellable, func(ev gateway.Event) {
		switch ev := ev.(type) {
		case *read.UpdateEvent:
			if guild := v.Guild(ev.GuildID); guild != nil {
				guild.InvalidateUnread()
			}
		case *gateway.GuildCreateEvent:
			if guild := v.Guild(ev.ID); guild != nil {
				guild.Update(&ev.Guild)
			} else {
				v.AddGuild(&ev.Guild)
			}
		case *gateway.GuildUpdateEvent:
			if guild := v.Guild(ev.ID); guild != nil {
				guild.Invalidate()
			}
		case *gateway.GuildDeleteEvent:
			if ev.Unavailable {
				if guild := v.Guild(ev.ID); guild != nil {
					guild.SetUnavailable()
					ctrl.CloseGuild(false)
					return
				}
			}
			v.RemoveGuild(ev.ID)
		}
	})

	return &v
}

// Invalidate invalidates the view and recreates everything. Use with care.
func (v *View) Invalidate() {
	// TODO: reselect.

	state := gtkcord.FromContext(v.ctx)
	ready := state.Ready()

	switch {
	case ready.UserSettings != nil && ready.UserSettings.GuildFolders != nil:
		v.SetFolders(ready.UserSettings.GuildFolders)

	case ready.UserSettings != nil && ready.UserSettings.GuildPositions != nil:
		v.SetGuildIDs(ready.UserSettings.GuildPositions)

	default:
		gtkutil.Async(v.ctx, func() func() {
			guilds, err := state.Guilds()
			if err != nil {
				app.Error(v.ctx, errors.Wrap(err, "cannot get guilds"))
				return nil
			}

			// We don't actually store GuildCreateEvents, which turned out to be
			// what we need for the Joined timestamp. We can't sort this list
			// correctly.

			guildIDs := make([]discord.GuildID, len(guilds))
			for i, g := range guilds {
				guildIDs[i] = g.ID
			}

			return func() {
				v.SetGuildIDs(guildIDs)
			}
		})
	}
}

// SetFolders sets the guild folders to use.
func (v *View) SetFolders(folders []gateway.GuildFolder) {
	v.clear()

	for i, folder := range folders {
		if len(folder.GuildIDs) == 1 {
			// Contains a single guild, so we just unbox it.
			g := NewGuild(v.ctx, (*guildOpenerView)(v), folder.GuildIDs[0])
			g.Invalidate()

			v.append(g)
			continue
		}

		f := NewFolder(v.ctx, (*guildOpenerView)(v))
		f.Set(&folders[i])

		v.append(f)
	}
}

// AddGuild prepends a single guild into the view.
func (v *View) AddGuild(guild *discord.Guild) {
	g := NewGuild(v.ctx, (*guildOpenerView)(v), guild.ID)
	g.Update(guild)

	v.Box.Prepend(g)
	v.Children = append([]ViewChild{g}, v.Children...)
}

// RemoveGuild removes the given guild.
func (v *View) RemoveGuild(id discord.GuildID) {
	guild := v.Guild(id)
	if guild == nil {
		return
	}

	if guild.IsSelected() {
		v.ctrl.CloseGuild(true)
	}

	if folder := guild.ParentFolder(); folder != nil {
		folder.Remove(guild.ID())
		if len(folder.Guilds) == 0 {
			v.remove(folder)
		}
	} else {
		v.remove(guild)
	}
}

// SetGuildIDs sets the guilds by a list of IDs. If sort is true, then the
// guilds will be sorted according to the order that the user joins them.
func (v *View) SetGuildIDs(guildIDs []discord.GuildID) {
	v.clear()

	for _, id := range guildIDs {
		g := NewGuild(v.ctx, (*guildOpenerView)(v), id)
		g.Invalidate()

		v.append(g)
	}
}

func (v *View) append(this ViewChild) {
	v.Children = append(v.Children, this)
	v.Box.Append(this)
}

func (v *View) remove(this ViewChild) {
	for i, child := range v.Children {
		if child == this {
			v.Children = append(v.Children[:i], v.Children[i+1:]...)
			v.Box.Remove(child)
			break
		}
	}
}

func (v *View) clear() {
	for _, child := range v.Children {
		v.Box.Remove(child)
	}
	v.Children = nil
}

// Guild finds a guild inside View by its ID.
func (v *View) Guild(id discord.GuildID) *Guild {
	for _, child := range v.Children {
		switch child := child.(type) {
		case *Guild:
			if child.ID() == id {
				return child
			}
		case *Folder:
			for _, guild := range child.Guilds {
				if guild.ID() == id {
					return guild
				}
			}
		}
	}

	return nil
}

// SelectGuild selects the guild with the given ID. If the guild is not known,
// then the sidebar's guild view is closed.
func (v *View) SelectGuild(id discord.GuildID) {
	guild := (*View)(v).Guild(id)
	if guild == nil {
		v.ctrl.CloseGuild(true)
		return
	}

	current := currentGuild{
		guild:  guild,
		folder: guild.ParentFolder(),
	}

	if current != v.current {
		(*View)(v).Unselect()
		v.current = current
	}

	v.ctrl.OpenGuild(id)
}

// Unselect unselects any guilds inside this guild view. Use this when the
// window is showing a channel that's not from any guild.
func (v *View) Unselect() {
	if v.current.folder != nil {
		v.current.folder.Unselect()
		v.current.folder = nil
	}

	if v.current.guild != nil {
		v.current.guild.Unselect()
		v.current.guild = nil
	}
}

type guildOpenerView View

func (v *guildOpenerView) OpenGuild(id discord.GuildID) {
	(*View)(v).SelectGuild(id)
}
