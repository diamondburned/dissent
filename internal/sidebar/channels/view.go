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

	HeaderView *gtk.Overlay
	HeaderBar  *adw.HeaderBar
	GuildName  *gtk.Label
	Banner     *Banner

	Scroll      *gtk.ScrolledWindow
	ChannelList *gtk.ListView

	ctx gtkutil.Cancellable

	model     *modelManager
	selection *gtk.SingleSelection

	guildID  discord.GuildID
	selectID discord.ChannelID // delegate to select later
}

var viewCSS = cssutil.Applier("channels-view", `
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
	.channels-has-banner  windowhandle,
	.channels-has-banner .channels-header {
		transition: linear 65ms all;
	}
	.channels-has-banner .channels-header {
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
	.channels-has-banner .channels-header * {
		color: white;
		text-shadow: 0px 0px 6px alpha(black, 0.65);
	}
	.channels-has-banner .channels-header *:backdrop {
		color: alpha(white, 0.75);
		text-shadow: 0px 0px 3px alpha(black, 0.35);
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

	// Bind the context to cancel when we're hidden.
	v.ctx = gtkutil.WithVisibility(ctx, v)

	v.GuildName = gtk.NewLabel("")
	v.GuildName.AddCSSClass("channels-name")
	v.GuildName.SetHAlign(gtk.AlignStart)
	v.GuildName.SetEllipsize(pango.EllipsizeEnd)

	// The header is placed on top of the overlay, kind of like the official
	// client.
	v.HeaderBar = adw.NewHeaderBar()
	v.HeaderBar.AddCSSClass("titlebar")
	v.HeaderBar.AddCSSClass("channels-header")
	v.HeaderBar.SetShowTitle(false)
	v.HeaderBar.PackStart(v.GuildName)
	v.HeaderBar.SetShowStartTitleButtons(false)
	v.HeaderBar.SetShowEndTitleButtons(false)
	v.HeaderBar.SetShowBackButton(false)
	v.HeaderBar.SetVAlign(gtk.AlignStart)
	v.HeaderBar.SetHAlign(gtk.AlignFill)

	v.Banner = NewBanner(ctx, guildID)
	v.Banner.Invalidate()

	v.HeaderView = gtk.NewOverlay()
	v.HeaderView.SetChild(v.Banner)
	v.HeaderView.AddOverlay(v.HeaderBar)
	v.HeaderView.SetMeasureOverlay(v.HeaderBar, true)

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
		if scrolled := v.Banner.SetScrollOpacity(vadj.Value()); scrolled {
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

	v.selection = gtk.NewSingleSelection(v.model)
	v.selection.SetAutoselect(false)
	v.selection.SetCanUnselect(true)

	v.ChannelList = gtk.NewListView(v.selection, newChannelItemFactory(ctx, v.model.TreeListModel))
	v.ChannelList.SetSizeRequest(bannerWidth, -1)
	v.ChannelList.AddCSSClass("channels-viewtree")
	v.ChannelList.SetVExpand(true)
	v.ChannelList.SetHExpand(true)

	viewport.SetChild(v.ChannelList)
	viewport.SetFocusChild(v.ChannelList)

	v.ToolbarView.AddTopBar(v.HeaderView)
	v.ToolbarView.SetContent(v.Scroll)

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

		parent := gtk.BaseWidget(v.ChannelList.Parent())
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
	v.GuildName.SetText(g.Name)
	v.invalidateBanner()
}

func (v *View) invalidateBanner() {
	v.Banner.Invalidate()
	if v.Banner.HasBanner() {
		v.AddCSSClass("channels-has-banner")
	} else {
		v.RemoveCSSClass("channels-has-banner")
	}
}
