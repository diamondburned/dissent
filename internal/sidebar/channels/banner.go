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
	b.Shadows.SetOpacity(0)

	b.Overlay = gtk.NewOverlay()
	b.Overlay.SetHAlign(gtk.AlignStart)
	b.Overlay.SetChild(b.Picture)
	b.Overlay.AddOverlay(b.Shadows)
	b.Overlay.SetCanTarget(false)
	b.Overlay.SetCanFocus(false)
	b.Hide()
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

// SetScrollOpacity sets the opacity of the shadow depending on the scroll
// level. If the scroll goes past the banner, then scrolled is true.
func (b *Banner) SetScrollOpacity(scrollY float64) (scrolled bool) {
	// Calculate the height of the banner but account for the height of the
	// header bar.
	height := float64(b.AllocatedHeight()) - (gtkcord.HeaderHeight)
	opacity := clamp(scrollY/height, 0, 1)

	b.Shadows.SetOpacity(opacity)
	return opacity >= 0.995
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
