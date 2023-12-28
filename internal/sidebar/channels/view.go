package channels

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// Refactor notice
//
// We should probably settle for an API that's kind of like this:
//
//    ch := NewView(ctx, ctrl, guildID)
//    var signal glib.SignalHandle
//    signal = ch.ConnectOnUpdate(func() bool {
//        if node := ch.Node(wantedChID); node != nil {
//            node.Select()
//            ch.HandlerDisconnect(signal)
//        }
//    })
//    ch.Invalidate()
//

const ChannelsWidth = bannerWidth

// Opener is the parent controller that View controls.
type Opener interface {
	OpenChannel(discord.ChannelID)
}

// View holds the entire channel sidebar containing all the categories, channels
// and threads.
type View struct {
	*adw.ToolbarView

	Header struct {
		*adw.HeaderBar
		Name *gtk.Label
	}

	Scroll *gtk.ScrolledWindow
	Child  struct {
		*gtk.Box
		Banner *Banner
		View   *gtk.ListView
	}

	ctx   gtkutil.Cancellable
	ctrl  Opener
	model *modelManager

	guildID  discord.GuildID
	selectID discord.ChannelID // delegate to select later
}

var viewCSS = cssutil.Applier("channels-view", `
	.channels-viewtree {
		background: none;
	}
	/* GTK is dumb. There's absolutely no way to get a ListItemWidget instance
	 * to style it, so we'll just unstyle everything and use the child instead.
	 */
	.channels-viewtree > row {
		margin: 0;
		padding: 0;
	}
	.channels-viewtree > row:hover:not(:selected) {
		background: @borders;
	}
	.channels-viewtree > row:hover:selected {
		background: mix(@borders, @theme_selected_bg_color, 0.25);
	}
	.channels-header {
		padding: 0 {$header_padding};
		border-radius: 0;
	}
	.channels-view-scroll {
		/* Space out the header, since it's in an overlay. */
		margin-top: {$header_height};
	}
	.channels-has-banner .channels-view-scroll {
		/* No need to space out here, since we have the banner. We do need to
		 * turn the header opaque with the styling below though, so the user can
		 * see it.
		 */
		margin-top: 0;
	}
	.channels-has-banner .top-bar {
		background-color: transparent;
		box-shadow: none;
	}
	.channels-has-banner  windowhandle,
	.channels-has-banner .channels-header {
		transition: linear 65ms all;
	}
	.channels-has-banner.channels-scrolled windowhandle {
		background-color: transparent;
	}
	.channels-has-banner.channels-scrolled headerbar {
		background-color: @theme_bg_color;
	}
	.channels-has-banner .channels-header {
		box-shadow: 0 0 6px 0px @theme_bg_color;
	}
	.channels-has-banner:not(.channels-scrolled) .channels-header {
		/* go run ./cmd/ease-in-out-gradient/ -max 0.25 -min 0 -steps 5 */
		background: linear-gradient(to bottom,
			alpha(black, 0.24),
			alpha(black, 0.19),
			alpha(black, 0.06),
			alpha(black, 0.01),
			alpha(black, 0.00) 100%
		);
		box-shadow: none;
		border: none;
	}
	.channels-has-banner .channels-banner-shadow {
		background: alpha(black, 0.75);
	}
	.channels-has-banner:not(.channels-scrolled) .channels-header * {
		color: white;
		text-shadow: 0px 0px 5px alpha(black, 0.75);
	}
	.channels-has-banner:not(.channels-scrolled) .channels-header *:backdrop {
		color: alpha(white, 0.75);
		text-shadow: 0px 0px 2px alpha(black, 0.35);
	}
	.channels-name {
		font-weight: 600;
		font-size: 1.1em;
	}
`)

// NewView creates a new View.
func NewView(ctx context.Context, ctrl Opener, guildID discord.GuildID) *View {
	state := gtkcord.FromContext(ctx)
	state.MemberState.Subscribe(guildID)

	v := View{
		ctrl:    ctrl,
		model:   newModelManager(state, guildID),
		guildID: guildID,
	}

	v.ToolbarView = adw.NewToolbarView()
	v.ToolbarView.SetExtendContentToTopEdge(true) // basically act like an overlay

	// Bind the context to cancel when we're hidden.
	v.ctx = gtkutil.WithVisibility(ctx, v)

	v.Header.Name = gtk.NewLabel("")
	v.Header.Name.AddCSSClass("channels-name")
	v.Header.Name.SetHAlign(gtk.AlignStart)
	v.Header.Name.SetEllipsize(pango.EllipsizeEnd)

	// The header is placed on top of the overlay, kind of like the official
	// client.
	v.Header.HeaderBar = adw.NewHeaderBar()
	v.Header.HeaderBar.AddCSSClass("channels-header")
	v.Header.HeaderBar.SetShowTitle(false)
	v.Header.HeaderBar.PackStart(v.Header.Name)

	viewport := gtk.NewViewport(nil, nil)

	v.Scroll = gtk.NewScrolledWindow()
	v.Scroll.AddCSSClass("channels-view-scroll")
	v.Scroll.SetVExpand(true)
	v.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	v.Scroll.SetChild(viewport)
	// v.Scroll.SetPropagateNaturalWidth(true)
	// v.Scroll.SetPropagateNaturalHeight(true)

	var headerScrolled bool

	vadj := v.Scroll.VAdjustment()
	vadj.ConnectValueChanged(func() {
		if scrolled := v.Child.Banner.SetScrollOpacity(vadj.Value()); scrolled {
			if !headerScrolled {
				headerScrolled = true
				v.AddCSSClass("channels-scrolled")
			}
		} else {
			if headerScrolled {
				headerScrolled = false
				v.RemoveCSSClass("channels-scrolled")
			}
		}
	})

	v.Child.Banner = NewBanner(ctx, guildID)
	v.Child.Banner.Invalidate()

	selection := gtk.NewSingleSelection(v.model)
	selection.SetCanUnselect(false)

	var selecting bool
	selection.ConnectSelectionChanged(func(position, nItems uint) {
		log.Printf("channels.View: selection changed: %d %d", position, nItems)

		if selecting {
			log.Println("BUG: infinite recursion in selection.ConnectChanged detected")
			log.Println("BUG: ignoring selection change")
			return
		}

		selecting = true
		glib.IdleAdd(func() { selecting = false })

		chID := channelIDFromItem(selection.SelectedItem())

		ch, _ := state.Cabinet.Channel(chID)
		if ch == nil {
			log.Printf("channels.View: tried opening non-existent channel %d", chID)
			return
		}

		switch ch.Type {
		case discord.GuildCategory, discord.GuildForum:
			// We cannot display these channel types.
			// TODO: implement forum browsing
			log.Printf("channels.View: ignoring channel %d of type %d", chID, ch.Type)
			return
		}

		v.selectID = chID
		ctrl.OpenChannel(chID)

		row := v.model.Row(selection.Selected())
		row.SetExpanded(true)
	})

	v.Child.View = gtk.NewListView(selection, newChannelItemFactory(ctx, v.model.TreeListModel))
	v.Child.View.SetSizeRequest(bannerWidth, -1)
	v.Child.View.AddCSSClass("channels-viewtree")
	v.Child.View.SetVExpand(true)
	v.Child.View.SetHExpand(true)
	v.Child.View.ConnectActivate(func(position uint) {
		row := v.model.Row(position)
		row.SetExpanded(!row.Expanded())
	})

	v.Child.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Child.Box.SetVExpand(true)
	v.Child.Box.Append(v.Child.Banner)
	v.Child.Box.Append(v.Child.View)
	v.Child.Box.SetFocusChild(v.Child.View)

	viewport.SetChild(v.Child)
	viewport.SetFocusChild(v.Child)

	v.ToolbarView.AddTopBar(v.Header)
	v.ToolbarView.SetContent(v.Scroll)
	v.ToolbarView.SetFocusChild(v.Scroll)

	viewCSS(v)
	return &v
}

// SelectChannel selects a known channel. If none is known, then it is selected
// later when the list is changed or never selected if the user selects
// something else.
func (v *View) SelectChannel(selectID discord.ChannelID) bool {
	v.selectID = selectID
	log.Println("selecting channel", selectID)

	n := v.model.NItems()
	for i := uint(0); i < n; i++ {
		item := v.model.Item(i)
		chID := channelIDFromItem(item)
		if chID != selectID {
			continue
		}
		selectionModel := v.Child.View.Model()
		return selectionModel.SelectItem(i, true)
	}

	return false
}

// GuildID returns the view's guild ID.
func (v *View) GuildID() discord.GuildID {
	return v.guildID
}

// InvalidateHeader invalidates the guild name and banner.
func (v *View) InvalidateHeader() {
	state := gtkcord.FromContext(v.ctx.Take())

	g, err := state.Cabinet.Guild(v.guildID)
	if err != nil {
		log.Printf("channels.View: cannot fetch guild %d: %v", v.guildID, err)
		return
	}

	// TODO: Nitro boost level
	v.Header.Name.SetText(g.Name)
	v.invalidateBanner()
}

func (v *View) invalidateBanner() {
	v.Child.Banner.Invalidate()

	if v.Child.Banner.HasBanner() {
		v.AddCSSClass("channels-has-banner")
	} else {
		v.RemoveCSSClass("channels-has-banner")
	}
}
