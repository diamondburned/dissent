package guilds

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// FolderButton is the folder icon containing the four guild icons.
type FolderButton struct {
	*gtk.Button
	// Main stack, switches between "guilds" and "folder"
	MainStack  *gtk.Stack
	GuildGrid  *gtk.Grid // contains 4 images always.
	FolderIcon *gtk.Image
	Images     [4]*onlineimage.Avatar // first 4 of folder.Guilds

	prov *gtk.CSSProvider
	ctx  context.Context
}

// [0] [1]
// [2] [3]
var folderIconMatrix = [4][2]int{
	{0, 0},
	{1, 0},
	{0, 1},
	{1, 1},
}

var folderButtonCSS = cssutil.Applier("guild-folderbutton", `
	.guild-folderbutton {
		padding:  0 12px; /* reset styling */
		padding-top: 8px;
		border: none;
		border-radius: 0;
		min-width:  0;
		min-height: 0;
		background: none;
	}
	.guild-foldericons {
		transition: 200ms ease;
		padding: 4px 0;
		/* Super quirky hack to give bottom margin control to the Stack
		 * instead of the button. We make up for the negative margin in
		 * the button.
		 */
		margin-top:   -4px;
		margin-bottom: 4px;
		border-radius: calc({$guild_icon_size} / 4);
	}
	.guild-foldericons:not(.guild-foldericons-collapsed) {
		background-color: @theme_bg_color;
		padding-bottom: 8px;
		margin-bottom:  0px;
		border-radius: 
			calc({$guild_icon_size} / 4)
			calc({$guild_icon_size} / 4)
			0 0;
	}
	.guild-foldericons > image {
		color: rgb(88, 101, 242);
	}
	.guild-foldericons-collapsed {
		background-color: rgba(88, 101, 242, 0.35);
	}
	.guild-foldericons image {
		background: none;
	}
	.guild-folderbutton .guild-foldericons {
		transition-property: all;
		outline: 0px solid transparent;
	}
	.guild-folderbutton:hover .guild-foldericons.guild-foldericons-collapsed {
		outline: 2px solid @theme_selected_bg_color;
	}
`)

// NewFolderButton creates a new FolderButton.
func NewFolderButton(ctx context.Context) *FolderButton {
	b := FolderButton{ctx: ctx}
	b.FolderIcon = gtk.NewImageFromIconName("folder-symbolic")
	b.FolderIcon.SetPixelSize(FolderSize)

	b.GuildGrid = gtk.NewGrid()
	b.GuildGrid.SetHAlign(gtk.AlignCenter)
	b.GuildGrid.SetVAlign(gtk.AlignCenter)
	b.GuildGrid.SetRowSpacing(4) // calculated from Discord
	b.GuildGrid.SetRowHomogeneous(true)
	b.GuildGrid.SetColumnSpacing(4)
	b.GuildGrid.SetColumnHomogeneous(true)

	for ix := range b.Images {
		b.Images[ix] = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, FolderMiniSize)
		b.Images[ix].SetInitials("#")

		pos := folderIconMatrix[ix]
		b.GuildGrid.Attach(b.Images[ix], pos[0], pos[1], 1, 1)
	}

	b.MainStack = gtk.NewStack()
	b.MainStack.SetTransitionType(gtk.StackTransitionTypeSlideUp) // unsure
	b.MainStack.SetSizeRequest(gtkcord.GuildIconSize, gtkcord.GuildIconSize)
	b.MainStack.SetHAlign(gtk.AlignCenter)
	b.MainStack.AddCSSClass("guild-foldericons")
	b.MainStack.AddCSSClass("guild-foldericons-collapsed")
	b.MainStack.AddChild(b.GuildGrid)
	b.MainStack.AddChild(b.FolderIcon)

	b.Button = gtk.NewButton()
	b.Button.SetHasFrame(false)
	b.Button.SetChild(b.MainStack)

	folderButtonCSS(b)
	return &b
}

// SetIcons sets the guild icons to be shown.
func (b *FolderButton) SetIcons(guildIDs []discord.GuildID) {
	state := gtkcord.FromContext(b.ctx)

	for ix, image := range b.Images {
		if ix >= len(guildIDs) {
			image.Hide()
			continue
		}

		image.Show()

		g, err := state.Cabinet.Guild(guildIDs[ix])
		if err != nil {
			image.SetInitials("?")
			continue
		}

		image.SetFromURL(gtkcord.InjectSize(g.IconURL(), 64))
	}
}

// SetRevealed sets what the FolderButton should show depending on if the folder
// is revealed/opened or not.
func (b *FolderButton) SetRevealed(revealed bool) {
	if revealed {
		b.MainStack.SetTransitionType(gtk.StackTransitionTypeSlideDown)
		b.MainStack.SetVisibleChild(b.FolderIcon)
		b.MainStack.RemoveCSSClass("guild-foldericons-collapsed")
	} else {
		b.MainStack.SetTransitionType(gtk.StackTransitionTypeSlideUp)
		b.MainStack.SetVisibleChild(b.GuildGrid)
		b.MainStack.AddCSSClass("guild-foldericons-collapsed")
	}
}

const colorCSSf = `
	image {
		color: rgb(%[1]d, %[2]d, %[3]d);
	}
	stack.guild-foldericons-collapsed {
		background-color: rgba(%[1]d, %[2]d, %[3]d, 0.4);
	}
`

func (b *FolderButton) colorWidgets() []gtk.Widgetter {
	return []gtk.Widgetter{
		b.Button,
		b.FolderIcon,
		b.MainStack,
	}
}

// SetColor sets the color of the folder.
func (b *FolderButton) SetColor(color discord.Color) {
	rr, gg, bb := color.RGB()

	p := gtk.NewCSSProvider()
	p.LoadFromData(fmt.Sprintf(colorCSSf, rr, gg, bb))

	for _, w := range b.colorWidgets() {
		s := gtk.BaseWidget(w).StyleContext()
		if b.prov != nil {
			s.RemoveProvider(b.prov)
		}
		s.AddProvider(p, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+10)
	}

	b.prov = p
}
