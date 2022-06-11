package guilds

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

const (
	FolderSize     = 32
	FolderMiniSize = 16
)

// Folder is the widget containing the folder icon on top and a child list of
// guilds beneath it.
type Folder struct {
	*gtk.Box

	ButtonOverlay *gtk.Overlay
	ButtonPill    *Pill
	Button        *FolderButton

	Name *NamePopover

	Revealer *gtk.Revealer
	GuildBox *gtk.Box
	Guilds   []*Guild

	ctx  context.Context
	ctrl GuildController
	open bool

	// TODO: if we ever track the unread indicator, then our unread container
	// will need to count the guilds for us for SetGuildOpen to work
	// correctly.
}

var folderCSS = cssutil.Applier("guild-folder", `
	.guild-folder .guild-guild > button {
		padding: 0 12px;
	}
	.guild-folder .guild-guild > button .adaptive-avatar {
		padding: 4px 0;
		transition: 200ms ease;
		background-color: @theme_bg_color;
	}
	.guild-folder .guild-guild > button       .adaptive-avatar,
	.guild-folder .guild-guild > button:hover .adaptive-avatar  {
		background-color: @theme_bg_color;
	}
	.guild-folder .guild-guild:last-child > button {
		padding-bottom: 4px;
	}
	.guild-folder .guild-guild:last-child > button .adaptive-avatar {
		padding: 0;
		padding-top: 4px;
		border-radius: 0 0 99px 99px;
	}
`)

// NewFolder creates a new Folder.
func NewFolder(ctx context.Context, ctrl GuildController) *Folder {
	f := Folder{
		ctx:  ctx,
		ctrl: ctrl,
	}

	f.Button = NewFolderButton(ctx)
	f.Button.SetRevealed(false)
	f.Button.ConnectClicked(f.toggle)

	f.ButtonPill = NewPill()

	f.ButtonOverlay = gtk.NewOverlay()
	f.ButtonOverlay.SetChild(f.Button)
	f.ButtonOverlay.AddOverlay(f.ButtonPill)

	f.Name = NewNamePopover()
	f.Name.SetParent(f.Button)

	ctrl.MotionGroup().ConnectEventControllerMotion(
		f.Button,
		f.Name.Popup,
		f.Name.Popdown,
	)

	f.GuildBox = gtk.NewBox(gtk.OrientationVertical, 0)

	f.Revealer = gtk.NewRevealer()
	f.Revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	f.Revealer.SetRevealChild(false)
	f.Revealer.SetChild(f.GuildBox)

	f.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	f.Box.Append(f.ButtonOverlay)
	f.Box.Append(f.Revealer)
	f.AddCSSClass("guild-folder-collapsed")
	folderCSS(f.Box)

	return &f
}

// Unselect unselects the folder visually.
func (f *Folder) Unselect() {
	f.setGuildOpen(false)
}

func (f *Folder) setGuildOpen(open bool) {
	f.open = open

	if f.Revealer.RevealChild() {
		f.ButtonPill.State = PillOpened
	} else {
		if open {
			f.ButtonPill.State = PillActive
		} else {
			f.ButtonPill.State = PillInactive
		}
	}

	f.ButtonPill.Invalidate()
}

func (f *Folder) toggle() {
	reveal := !f.Revealer.RevealChild()
	f.Revealer.SetRevealChild(reveal)
	f.Button.SetRevealed(reveal)
	f.setGuildOpen(f.open)

	if reveal {
		f.RemoveCSSClass("guild-folder-collapsed")
	} else {
		f.AddCSSClass("guild-folder-collapsed")
	}
}

// Set sets a fresh list of guilds.
func (f *Folder) Set(folder *gateway.GuildFolder) {
	f.Button.SetIcons(folder.GuildIDs)

	if folder.Color != discord.NullColor {
		f.Button.SetColor(folder.Color)
	}

	for _, guild := range f.Guilds {
		f.GuildBox.Remove(guild)
	}

	f.Guilds = make([]*Guild, len(folder.GuildIDs))

	for i, id := range folder.GuildIDs {
		g := NewGuild(f.ctx, (*guildControllerFolder)(f), id)
		g.SetParentFolder(f)

		f.Guilds[i] = g
		f.GuildBox.Append(g)
	}

	// Invalidate after, since guilds can call back onto our Folder list.
	for _, g := range f.Guilds {
		g.Invalidate()
	}

	// After guilds are loaded, read their labels and set the folder name if unset.
	folderName := folder.Name
	if folderName == "" {
		for i, g := range f.Guilds {
			folderName += g.Name.Label.Text()
			if (i + 1) < len(f.Guilds) {
				folderName += ", "
			}
			if len(folderName) > 40 {
				folderName += "..."
				break
			}
		}
	}

	f.Name.SetName(folderName)
}

// Remove removes the given guild by its ID.
func (f *Folder) Remove(id discord.GuildID) {
	for i, guild := range f.Guilds {
		if guild.ID() == id {
			f.GuildBox.Remove(guild)
			f.Guilds = append(f.Guilds[:i], f.Guilds[i+1:]...)
			return
		}
	}
}

func (f *Folder) viewChild() {}

// InvalidateUnread invalidates the folder's unread state.
func (f *Folder) InvalidateUnread() {
	f.ButtonPill.Attrs = 0
	for _, guild := range f.Guilds {
		f.ButtonPill.Attrs |= guild.Pill.Attrs
	}

	f.ButtonPill.Invalidate()
}

type guildControllerFolder Folder

func (f *guildControllerFolder) MotionGroup() *MotionGroup {
	return f.ctrl.MotionGroup()
}

func (f *guildControllerFolder) OpenGuild(id discord.GuildID) {
	(*Folder)(f).setGuildOpen(true)
	f.ctrl.OpenGuild(id)
}
