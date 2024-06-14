package guilds

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/dissent/internal/sidebar/sidebutton"
)

const (
	FolderSize     = 32
	FolderMiniSize = 16
)

// Folder is the widget containing the folder icon on top and a child list of
// guilds beneath it.
type Folder struct {
	*gtk.Box

	Button struct {
		*gtk.Overlay
		Pill   *sidebutton.Pill
		Folder *FolderButton
	}

	Revealer *gtk.Revealer
	GuildBox *gtk.Box
	Guilds   []*Guild

	ctx  context.Context
	open bool

	// TODO: if we ever track the unread indicator, then our unread container
	// will need to count the guilds for us for SetGuildOpen to work
	// correctly.
}

var folderCSS = cssutil.Applier("guild-folder", `
	.guild-folder .guild-guild > button {
		padding: 0px 12px;
	}
	.guild-folder .guild-guild > button > * {
		padding: 4px 0;
		transition: 200ms ease;
		background-color: @theme_bg_color;
	}
	.guild-folder .guild-guild > button avatar {
		padding: 0;
	}
	.guild-folder .guild-guild > button       avatar,
	.guild-folder .guild-guild > button:hover avatar  {
		background-color: @theme_bg_color;
	}
	.guild-folder .guild-guild:last-child > button {
		padding-bottom: 4px;
	}
	.guild-folder .guild-guild:last-child > button > * {
		padding: 0;
		padding-top: 4px;
		border-radius: 0 0 99px 99px;
	}
`)

// NewFolder creates a new Folder.
func NewFolder(ctx context.Context) *Folder {
	f := Folder{
		ctx: ctx,
	}

	f.Button.Folder = NewFolderButton(ctx)
	f.Button.Folder.SetRevealed(false)
	f.Button.Folder.ConnectClicked(f.toggle)

	f.Button.Pill = sidebutton.NewPill()

	f.Button.Overlay = gtk.NewOverlay()
	f.Button.Overlay.SetChild(f.Button.Folder)
	f.Button.Overlay.AddOverlay(f.Button.Pill)

	f.GuildBox = gtk.NewBox(gtk.OrientationVertical, 0)

	f.Revealer = gtk.NewRevealer()
	f.Revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	f.Revealer.SetRevealChild(false)
	f.Revealer.SetChild(f.GuildBox)

	f.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	f.Box.Append(f.Button.Overlay)
	f.Box.Append(f.Revealer)
	f.AddCSSClass("guild-folder-collapsed")
	folderCSS(f.Box)

	return &f
}

// Unselect unselects the folder visually.
func (f *Folder) Unselect() {
	f.setGuildOpen(false)
}

// SetSelected sets the folder's selected state.
func (f *Folder) SetSelected(selected bool) {
	f.setGuildOpen(selected)
}

func (f *Folder) setGuildOpen(open bool) {
	f.open = open

	if f.Revealer.RevealChild() {
		f.Button.Pill.State = sidebutton.PillOpened
	} else {
		if open {
			f.Button.Pill.State = sidebutton.PillActive
		} else {
			f.Button.Pill.State = sidebutton.PillInactive
		}
	}

	f.Button.Pill.Invalidate()
}

func (f *Folder) toggle() {
	reveal := !f.Revealer.RevealChild()
	f.Revealer.SetRevealChild(reveal)
	f.Button.Folder.SetRevealed(reveal)
	f.Button.Folder.Mentions.SetRevealChild(!reveal)
	f.setGuildOpen(f.open)

	if reveal {
		f.RemoveCSSClass("guild-folder-collapsed")
	} else {
		f.AddCSSClass("guild-folder-collapsed")
	}
}

// Set sets a fresh list of guilds.
func (f *Folder) Set(folder *gateway.GuildFolder) {
	f.Button.Folder.SetIcons(folder.GuildIDs)
	if folder.Color != discord.NullColor {
		f.Button.Folder.SetColor(folder.Color)
	}

	for _, guild := range f.Guilds {
		f.GuildBox.Remove(guild)
	}

	f.Guilds = make([]*Guild, len(folder.GuildIDs))

	for i, id := range folder.GuildIDs {
		g := NewGuild(f.ctx, id)
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
			folderName += g.Name()
			if (i + 1) < len(f.Guilds) {
				folderName += ", "
			}
			if len(folderName) > 40 {
				folderName += "..."
				break
			}
		}
	}

	f.Button.SetTooltipText(folderName)
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
	f.Button.Pill.Attrs = 0
	for _, guild := range f.Guilds {
		f.Button.Pill.Attrs |= guild.Pill.Attrs
	}

	f.Button.Pill.Invalidate()

	var mentions int
	for _, guild := range f.Guilds {
		mentions += guild.Mentions.Count()
	}

	f.Button.Folder.Mentions.SetCount(mentions)
}
