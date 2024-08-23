package channels

import (
	"context"
	"math"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"libdb.so/dissent/internal/gtkcord"
)

const (
	bannerWidth  = 240
	bannerHeight = 120
)

// Banner is the guild banner display on top of the channel view.
type Banner struct {
	*gtk.Overlay
	Shadows *gtk.Box
	Picture *onlineimage.Picture
	ctx     context.Context
	gID     discord.GuildID
}

var bannerCSS = cssutil.Applier("channels-banner", `
	.channels-banner-shadow {
		transition: all 0.25s;
	}
	.channels-banner-shadow {
		/* ease-in-out-opacity -max 0 -min 0.25 -start 24px -end 75px -steps 10 */
		background-image: linear-gradient(
			to top,
			alpha(black, 0.25) 24px,
			alpha(black, 0.25) 30px,
			alpha(black, 0.24) 35px,
			alpha(black, 0.21) 41px,
			alpha(black, 0.16) 47px,
			alpha(black, 0.09) 52px,
			alpha(black, 0.04) 58px,
			alpha(black, 0.01) 64px,
			alpha(black, 0.00) 69px,
			alpha(black, 0.00) 75px
		);
	}
	.channels-scrolled .channels-banner-shadow {
		/* ease-in-out-opacity -max 0.45 -min 0.65 -steps 10 */
		background-image: linear-gradient(
			to top,
			alpha(black, 0.65) 0%,
			alpha(black, 0.65) 11%,
			alpha(black, 0.64) 22%,
			alpha(black, 0.62) 33%,
			alpha(black, 0.58) 44%,
			alpha(black, 0.52) 56%,
			alpha(black, 0.48) 67%,
			alpha(black, 0.46) 78%,
			alpha(black, 0.45) 89%,
			alpha(black, 0.45) 100%
		);
	}
`)

// NewBanner creates a new Banner.
func NewBanner(ctx context.Context, guildID discord.GuildID) *Banner {
	b := Banner{
		ctx: ctx,
		gID: guildID,
	}

	b.Picture = onlineimage.NewPicture(ctx, imgutil.HTTPProvider)
	b.Picture.SetLayoutManager(gtk.NewBinLayout()) // magically force min size
	b.Picture.SetContentFit(gtk.ContentFitCover)
	b.Picture.SetSizeRequest(bannerWidth, bannerHeight)

	b.Shadows = gtk.NewBox(gtk.OrientationVertical, 0)
	b.Shadows.AddCSSClass("channels-banner-shadow")

	b.Overlay = gtk.NewOverlay()
	b.Overlay.SetHAlign(gtk.AlignStart)
	b.Overlay.SetChild(b.Picture)
	b.Overlay.AddOverlay(b.Shadows)
	b.Overlay.SetCanTarget(false)
	b.Overlay.SetCanFocus(false)
	b.SetVisible(false)
	bannerCSS(b)

	state := gtkcord.FromContext(ctx)
	state.AddHandlerForWidget(b, func(ev *gateway.GuildUpdateEvent) {
		if ev.Guild.ID == guildID {
			b.Invalidate()
		}
	})

	return &b
}

// Invalidate invalidates and updates the Banner image.
func (b *Banner) Invalidate() {
	state := gtkcord.FromContext(b.ctx)

	g, err := state.Cabinet.Guild(b.gID)
	if err != nil {
		b.SetVisible(false)
		return
	}

	url := g.BannerURL()
	if url == "" {
		b.SetVisible(false)
		return
	}

	b.SetVisible(true)
	b.SetURL(gtkcord.InjectSize(url, bannerWidth))
}

// HasBanner returns true if the banner is visible.
func (b *Banner) HasBanner() bool {
	return b.Visible()
}

func (b *Banner) SetURL(url string) { b.Picture.SetURL(url) }

// SetScrollOpacity sets the opacity of the shadow depending on the scroll
// level. If the scroll goes past the banner, then scrolled is true.
func (b *Banner) SetScrollOpacity(scrollY float64) (scrolled bool) {
	// Calculate the height of the banner but account for the height of the
	// header bar.
	// height := float64(b.Height()) - (gtkcord.HeaderHeight)
	// opacity := clamp(scrollY/height, 0, 1)

	// b.Shadows.SetOpacity(opacity)
	// return opacity >= 0.995

	return scrollY > 0 // delegate to styling
}

func clamp(f, min, max float64) float64 {
	return math.Max(math.Min(f, max), min)
}

func strsEq(strs1, strs2 []string) bool {
	if len(strs1) != len(strs2) {
		return false
	}
	for i := range strs1 {
		if strs1[i] != strs2[i] {
			return false
		}
	}
	return true
}
