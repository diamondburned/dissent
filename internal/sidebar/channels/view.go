package channels

import (
	"context"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/dissent/internal/gtkcord"
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

	ctx gtkutil.Cancellable

	model     *modelManager
	selection *gtk.SingleSelection

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
func NewView(ctx context.Context, guildID discord.GuildID) *View {
	state := gtkcord.FromContext(ctx)
	state.MemberState.Subscribe(guildID)

	v := View{
		model:   newModelManager(state, guildID),
		guildID: guildID,
	}

	v.ToolbarView = adw.NewToolbarView()
	v.ToolbarView.SetTopBarStyle(adw.ToolbarFlat)
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
	v.Header.HeaderBar.AddCSSClass("titlebar")
	v.Header.HeaderBar.AddCSSClass("channels-header")
	v.Header.HeaderBar.SetShowTitle(false)
	v.Header.HeaderBar.PackStart(v.Header.Name)
	v.Header.HeaderBar.SetShowStartTitleButtons(false)
	v.Header.HeaderBar.SetShowEndTitleButtons(false)
	v.Header.HeaderBar.SetShowBackButton(false)

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

	v.selection = gtk.NewSingleSelection(v.model)
	v.selection.SetAutoselect(false)
	v.selection.SetCanUnselect(true)

	v.Child.View = gtk.NewListView(v.selection, newChannelItemFactory(ctx, v.model.TreeListModel))
	v.Child.View.SetSizeRequest(bannerWidth, -1)
	v.Child.View.AddCSSClass("channels-viewtree")
	v.Child.View.SetVExpand(true)
	v.Child.View.SetHExpand(true)

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

	var lastOpen discord.ChannelID

	v.selection.ConnectSelectionChanged(func(position, nItems uint) {
		item := v.selection.SelectedItem()
		if item == nil {
			// ctrl.OpenChannel(0)
			return
		}

		chID := channelIDFromItem(item)

		if lastOpen == chID {
			return
		}
		lastOpen = chID

		ch, _ := state.Cabinet.Channel(chID)
		if ch == nil {
			slog.Error(
				"tried opening non-existent channel",
				"channel_id", chID)
			return
		}

		switch ch.Type {
		case discord.GuildCategory, discord.GuildForum:
			// We cannot display these channel types.
			// TODO: implement forum browsing
			slog.Warn(
				"category or forum channel selected, ignoring",
				"channel_type", ch.Type,
				"channel_id", chID)
			return
		}

		slog.Debug(
			"selection change signal emitted, selecting channel and clearing selectID",
			"channel_type", ch.Type,
			"channel_id", chID)
		v.selectID = 0

		row := v.model.Row(v.selection.Selected())
		row.SetExpanded(true)

		parent := gtk.BaseWidget(v.Child.View.Parent())
		parent.ActivateAction("win.open-channel", gtkcord.NewChannelIDVariant(chID))
	})

	// Bind to a signal that selects any channel that we need to be selected.
	// This lets the channel be lazy-loaded.
	v.selection.ConnectAfter("items-changed", func() {
		if v.selectID == 0 {
			return
		}

		i, ok := v.findChannelItem(v.selectID)
		if ok {
			slog.Debug(
				"items-changed signal emitted, re-selecting stored channel",
				"channel_id", v.selectID,
				"channel_index", i)
			v.selection.SelectItem(i, true)
			v.selectID = 0
		} else {
			slog.Debug(
				"items-changed signal emitted but stored channel not found",
				"channel_id", v.selectID)
		}
	})

	viewCSS(v)
	return &v
}

// SelectChannel selects a known channel. If none is known, then it is selected
// later when the list is changed or never selected if the user selects
// something else.
func (v *View) SelectChannel(selectID discord.ChannelID) bool {
	i, ok := v.findChannelItem(selectID)
	if ok && v.selection.SelectItem(i, true) {
		slog.Debug(
			"channel found and selected immediately",
			"channel_id", selectID,
			"channel_index", i)
		v.selectID = 0
		return true
	}

	slog.Debug(
		"channel not found, selecting later",
		"channel_id", selectID)
	v.selectID = selectID
	return false
}

// findChannelItem finds the channel item by ID.
// BUG: this function is not able to find channels within collapsed categories.
func (v *View) findChannelItem(id discord.ChannelID) (uint, bool) {
	n := v.selection.NItems()
	for i := uint(0); i < n; i++ {
		item := v.selection.Item(i)
		chID := channelIDFromItem(item)
		if chID == id {
			return i, true
		}
	}
	// TODO: recursively search v.model so we can find collapsed channels.
	return n, false
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
		slog.Warn(
			"cannot fetch guild to check banner",
			"guild_id", v.guildID,
			"err", err)
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
