package sidebutton

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3"
)

// Button is a widget showing a single guild icon.
type Button struct {
	*gtk.Overlay
	Button *gtk.Button

	IconOverlay *gtk.Overlay
	Icon        *onlineimage.Avatar
	Mentions    *MentionsIndicator

	Pill *Pill

	ctx       context.Context
	mentions  int
	indicator ningen.UnreadIndication
}

var buttonCSS = cssutil.Applier("sidebar-button", `
	.sidebar-button > button {
		padding: 4px 12px;
		border: none;
		border-radius: 0;
		background: none;
	}
	.sidebar-button image {
		background-color: @theme_bg_color;
	}
	.sidebar-button > button .adaptive-avatar {
		border-radius: 0; /* reset */
	}
	.sidebar-button > button .adaptive-avatar > image,
	.sidebar-button > button .adaptive-avatar > label {
		outline: 0px solid transparent;
		outline-offset: 0;
	}
	.sidebar-button > button:hover .adaptive-avatar > image,
	.sidebar-button > button:hover .adaptive-avatar > label {
		outline: 2px solid @theme_selected_bg_color;
		background-color: alpha(@theme_selected_bg_color, 0.35);
	}
	.sidebar-button > button .adaptive-avatar > image,
	.sidebar-button > button .adaptive-avatar > label {
		border-radius: calc({$guild_icon_size} / 2);
	}
	.sidebar-button > button:hover .adaptive-avatar > image,
	.sidebar-button > button:hover .adaptive-avatar > label {
		border-radius: calc({$guild_icon_size} / 4);
	}
	.sidebar-button > button image,
	.sidebar-button > button .adaptive-avatar > image,
	.sidebar-button > button .adaptive-avatar > label {
		transition: 200ms ease;
		transition-property: all;
	}
`)

// NewButton creates a new button.
func NewButton(ctx context.Context, open func()) *Button {
	g := Button{
		ctx: ctx,
	}

	g.Icon = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.GuildIconSize)
	g.Mentions = NewMentionsIndicator()

	g.IconOverlay = gtk.NewOverlay()
	g.IconOverlay.AddCSSClass("sidebar-button-overlay")
	g.IconOverlay.SetChild(g.Icon.Avatar)
	g.IconOverlay.AddOverlay(g.Mentions)

	g.Button = gtk.NewButton()
	g.Button.SetHasFrame(false)
	g.Button.SetHAlign(gtk.AlignCenter)
	g.Button.SetChild(g.IconOverlay)
	g.Button.ConnectClicked(func() {
		g.Pill.State = PillActive
		g.Pill.Invalidate()

		open()
	})

	iconAnimation := g.Icon.EnableAnimation()
	iconAnimation.ConnectMotion(g.Button)

	g.Pill = NewPill()

	g.Overlay = gtk.NewOverlay()
	g.Overlay.SetChild(g.Button)
	g.Overlay.AddOverlay(g.Pill)
	buttonCSS(g)

	return &g
}

// Context returns the context of the button that was passed in during
// construction.
func (g *Button) Context() context.Context {
	return g.ctx
}

// Activate activates the button.
func (g *Button) Activate() bool {
	return g.Button.Activate()
}

// Unselect unselects the guild visually. This is mostly used by the parent
// widget for list-keeping.
func (g *Button) Unselect() {
	g.Pill.State = PillDisabled
	g.Pill.Invalidate()
}

// IsSelected returns true if the guild is selected.
func (g *Button) IsSelected() bool {
	return g.Pill.State == PillActive
}

// SetIndicator sets the button's unread indicator.
func (g *Button) SetIndicator(indicator ningen.UnreadIndication) {
	if g.indicator == indicator {
		return
	}

	g.indicator = indicator
	g.Pill.Attrs = PillAttrsFromUnread(g.indicator)
	g.Pill.Invalidate()
}

// SetMentions sets the button's mention indicator.
func (g *Button) SetMentions(mentions int) {
	if g.mentions == mentions {
		return
	}

	g.mentions = mentions
}
