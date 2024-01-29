package guilds

import (
	"context"
	"log"
	"sort"

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

// View contains a list of guilds and folders.
type View struct {
	*gtk.Box
	Children []ViewChild

	current currentGuild

	ctx context.Context
}

var viewCSS = cssutil.Applier("guild-view", `
	.guild-view {
		margin: 4px 0;
	}
	.guild-view button:active:not(:hover) {
		background: initial;
	}
`)

// NewView creates a new View.
func NewView(ctx context.Context) *View {
	v := View{
		ctx: ctx,
	}

	v.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	viewCSS(v)

	cancellable := gtkutil.WithVisibility(ctx, v)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(cancellable, func(ev gateway.Event) {
		switch ev := ev.(type) {
		case *gateway.ReadyEvent, *gateway.ResumedEvent:
			// Recreate the whole list in case we have some new info.
			v.Invalidate()

		case *read.UpdateEvent:
			if guild := v.Guild(ev.GuildID); guild != nil {
				guild.InvalidateUnread()
			}
		case *gateway.ChannelCreateEvent:
			if ev.GuildID.IsValid() {
				if guild := v.Guild(ev.GuildID); guild != nil {
					guild.InvalidateUnread()
				}
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

					parent := gtk.BaseWidget(guild.Parent())
					parent.ActivateAction("win.reset-view", nil)
					return
				}
			}

			guild := v.RemoveGuild(ev.ID)
			if guild != nil && guild.IsSelected() {
				parent := gtk.BaseWidget(guild.Parent())
				parent.ActivateAction("win.reset-view", nil)
			}
		}
	})

	return &v
}

// InvalidateUnreads invalidates the unread states of all guilds.
func (v *View) InvalidateUnreads() {
	for _, child := range v.Children {
		if child, ok := child.(*Guild); ok {
			child.InvalidateUnread()
		}
	}
}

// Invalidate invalidates the view and recreates everything. Use with care.
func (v *View) Invalidate() {
	// TODO: reselect.

	state := gtkcord.FromContext(v.ctx)
	ready := state.Ready()

	if ready.UserSettings != nil {
		switch {
		case ready.UserSettings.GuildFolders != nil:
			v.SetFolders(ready.UserSettings.GuildFolders)
		case ready.UserSettings.GuildPositions != nil:
			v.SetGuildsFromIDs(ready.UserSettings.GuildPositions)
		}
	}

	guilds, err := state.Cabinet.Guilds()
	if err != nil {
		app.Error(v.ctx, errors.Wrap(err, "cannot get guilds"))
		return
	}

	// Sort so that the guilds that we've joined last are at the bottom.
	// This means we can prepend guilds as we go, and the latest one will be
	// prepended to the top.
	sort.Slice(guilds, func(i, j int) bool {
		ti, ok := state.GuildState.JoinedAt(guilds[i].ID)
		if !ok {
			return false // put last
		}
		tj, ok := state.GuildState.JoinedAt(guilds[j].ID)
		if !ok {
			return true
		}
		return ti.Before(tj)
	})

	// Construct a map of shownGuilds guilds, so we know to not create a
	// guild if it's already shown.
	shownGuilds := make(map[discord.GuildID]struct{}, 200)
	v.eachGuild(func(g *Guild) bool {
		shownGuilds[g.ID()] = struct{}{}
		return false
	})

	for i, guild := range guilds {
		_, shown := shownGuilds[guild.ID]
		if shown {
			continue
		}

		g := NewGuild(v.ctx, guild.ID)
		g.Update(&guilds[i])

		// Prepend the guild.
		v.prepend(g)
	}
}

// SetFolders sets the guild folders to use.
func (v *View) SetFolders(folders []gateway.GuildFolder) {
	restore := v.saveSelection()
	defer restore()

	v.clear()

	for i, folder := range folders {
		if folder.ID == 0 {
			// Contains a single guild, so we just unbox it.
			g := NewGuild(v.ctx, folder.GuildIDs[0])
			g.Invalidate()

			v.append(g)
			continue
		}

		f := NewFolder(v.ctx)
		f.Set(&folders[i])

		v.append(f)
	}
}

// AddGuild prepends a single guild into the view.
func (v *View) AddGuild(guild *discord.Guild) {
	g := NewGuild(v.ctx, guild.ID)
	g.Update(guild)

	v.Box.Prepend(g)
	v.Children = append([]ViewChild{g}, v.Children...)
}

// RemoveGuild removes the given guild.
func (v *View) RemoveGuild(id discord.GuildID) *Guild {
	guild := v.Guild(id)
	if guild == nil {
		return nil
	}

	if folder := guild.ParentFolder(); folder != nil {
		folder.Remove(guild.ID())
		if len(folder.Guilds) == 0 {
			v.remove(folder)
		}
	} else {
		v.remove(guild)
	}

	return guild
}

// SetGuildsFromIDs calls SetGuilds with guilds fetched from the state by the
// given ID list.
func (v *View) SetGuildsFromIDs(guildIDs []discord.GuildID) {
	restore := v.saveSelection()
	defer restore()

	v.clear()

	for _, id := range guildIDs {
		g := NewGuild(v.ctx, id)
		g.Invalidate()

		v.append(g)
	}
}

// SetGuilds sets the guilds shown.
func (v *View) SetGuilds(guilds []discord.Guild) {
	restore := v.saveSelection()
	defer restore()

	v.clear()

	for i, guild := range guilds {
		g := NewGuild(v.ctx, guild.ID)
		g.Update(&guilds[i])

		v.append(g)
	}
}

func (v *View) append(this ViewChild) {
	v.Children = append(v.Children, this)
	v.Box.Append(this)
}

func (v *View) prepend(this ViewChild) {
	v.Children = append(v.Children, nil)
	copy(v.Children[1:], v.Children)
	v.Children[0] = this

	v.Box.Prepend(this)
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

// SelectedGuildID returns the selected guild ID, if any.
func (v *View) SelectedGuildID() discord.GuildID {
	if v.current.guild == nil {
		return 0
	}
	return v.current.guild.id
}

// Guild finds a guild inside View by its ID.
func (v *View) Guild(id discord.GuildID) *Guild {
	var guild *Guild
	v.eachGuild(func(g *Guild) bool {
		if g.ID() == id {
			guild = g
			return true
		}
		return false
	})
	return guild
}

func (v *View) eachGuild(f func(*Guild) (stop bool)) {
	for _, child := range v.Children {
		switch child := child.(type) {
		case *Guild:
			if f(child) {
				return
			}
		case *Folder:
			for _, guild := range child.Guilds {
				if f(guild) {
					return
				}
			}
		}
	}
}

// SetSelectedGuild sets the selected guild. It does not propagate the selection
// to the sidebar.
func (v *View) SetSelectedGuild(id discord.GuildID) {
	guild := v.Guild(id)
	if guild == nil {
		log.Printf("guilds.View: cannot select guild %d: not found", id)
		v.Unselect()
		return
	}

	current := currentGuild{
		guild:  guild,
		folder: guild.ParentFolder(),
	}

	if current != v.current {
		v.Unselect()
		v.current = current
		v.current.SetSelected(true)
	}
}

// Unselect unselects any guilds inside this guild view. Use this when the
// window is showing a channel that's not from any guild.
func (v *View) Unselect() {
	v.current.Unselect()
	v.current = currentGuild{}
}

// saveSelection saves the current guild selection to be restored later using
// the returned callback.
func (v *View) saveSelection() (restore func()) {
	if v.current.guild == nil {
		// Nothing to restore.
		return func() {}
	}

	guildID := v.current.guild.id
	return func() {
		parent := gtk.BaseWidget(v.Parent())
		parent.ActivateAction("win.open-guild", gtkcord.NewGuildIDVariant(guildID))
	}
}

type currentGuild struct {
	guild  *Guild
	folder *Folder
}

func (c currentGuild) Unselect() {
	c.SetSelected(false)
}

func (c currentGuild) SetSelected(selected bool) {
	if c.folder != nil {
		c.folder.SetSelected(selected)
	}
	if c.guild != nil {
		c.guild.SetSelected(selected)
	}
}
