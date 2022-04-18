package channels

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

const (
	bannerWidth  = 240
	bannerHeight = 135
)

// Banner is the guild banner display on top of the channel view.
type Banner struct {
	*gtk.Overlay
	Shadows *gtk.Box
	Picture *onlineimage.Picture
	ctx     context.Context
	gID     discord.GuildID
}

var bannerCSS = cssutil.Applier("channels-banner", ``)

// NewBanner creates a new Banner.
func NewBanner(ctx context.Context, guildID discord.GuildID) *Banner {
	b := Banner{
		ctx: ctx,
		gID: guildID,
	}

	b.Picture = onlineimage.NewPicture(ctx, imgutil.HTTPProvider)
	b.Picture.SetLayoutManager(gtk.NewBinLayout()) // magically force min size
	b.Picture.SetSizeRequest(bannerWidth, bannerHeight)

	b.Shadows = gtk.NewBox(gtk.OrientationVertical, 0)
	b.Shadows.AddCSSClass("channels-banner-shadow")

	b.Overlay = gtk.NewOverlay()
	b.Overlay.SetHAlign(gtk.AlignStart)
	b.Overlay.SetChild(b.Picture)
	b.Overlay.AddOverlay(b.Shadows)
	b.Hide()
	bannerCSS(b)

	return &b
}

// Invalidate invalidates and updates the Banner image.
func (b *Banner) Invalidate() {
	state := gtkcord.FromContext(b.ctx)

	g, err := state.Cabinet.Guild(b.gID)
	if err != nil {
		b.Hide()
		return
	}

	url := g.BannerURL()
	if url == "" {
		b.Hide()
		return
	}

	b.Show()
	b.SetURL(gtkcord.InjectSize(url, bannerWidth))
}

// HasBanner returns true if the banner is visible.
func (b *Banner) HasBanner() bool {
	return b.Visible()
}

func (b *Banner) SetURL(url string) { b.Picture.SetURL(url) }
