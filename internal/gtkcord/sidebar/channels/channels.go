package channels

import (
	"context"
	"log"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/states/read"
	"github.com/pkg/errors"
)

const ChannelsWidth = bannerWidth

// Controller is the parent controller that View controls.
type Controller interface {
	OpenChannel(discord.ChannelID)
}

// View holds the entire channel sidebar containing all the categories, channels
// and threads.
type View struct {
	*adaptive.LoadablePage
	Overlay *gtk.Overlay // covers whole

	Header struct {
		*gtk.WindowHandle
		Box  *gtk.Box
		Name *gtk.Label
	}

	Scroll *gtk.ScrolledWindow
	Child  struct {
		*gtk.Box
		Banner *Banner
		Tree   *gtk.TreeView
	}

	ctx  gtkutil.Cancellable
	ctrl Controller
	tree *GuildTree
	cols []*gtk.TreeViewColumn

	guildID discord.GuildID
}

var viewCSS = cssutil.Applier("channels-view", `
	.channels-header {
		padding: 0 {$header_padding};
		box-shadow: none;
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
	.channels-has-banner .channels-header {
		transition: none;
		box-shadow: 0 0 6px 0px @theme_bg_color;
	}
	.channels-has-banner:not(.channels-scrolled) .channels-header {
		background: none;
		box-shadow: none;
	}
	.channels-has-banner .channels-banner-shadow {
		transition: none;
		background: none;
	}
	.channels-has-banner:not(.channels-scrolled) .channels-banner-shadow {
		background: linear-gradient(to bottom,
			alpha(@theme_bg_color, 0.49),
			alpha(@theme_bg_color, 0.45),
			alpha(@theme_bg_color, 0.34),
			alpha(@theme_bg_color, 0.16),
			alpha(@theme_bg_color, 0.05),
			alpha(@theme_bg_color, 0.01),
			alpha(@theme_bg_color, 0.00) 50%
		);
	}
	.channels-name {
		font-weight: 600;
		font-size: 1.1em;
	}
	.channels-viewtree {
		color: alpha(@theme_fg_color, 0.9);
	}
`)

var channelsViewEvents = []gateway.Event{
	// (*gateway.GuildUpdateEvent)(nil),
	// (*gateway.ChannelCreateEvent)(nil),
	// (*gateway.ChannelUpdateEvent)(nil),
	// (*gateway.ChannelDeleteEvent)(nil),
	// (*gateway.ThreadCreateEvent)(nil),
	// (*gateway.ThreadUpdateEvent)(nil),
	// (*gateway.ThreadDeleteEvent)(nil),
	// (*gateway.ThreadListSyncEvent)(nil),
}

// NewView creates a new View.
func NewView(ctx context.Context, ctrl Controller, guildID discord.GuildID) *View {
	v := View{
		ctrl:    ctrl,
		cols:    newTreeColumns(),
		guildID: guildID,
	}

	v.LoadablePage = adaptive.NewLoadablePage()
	v.LoadablePage.SetLoading()

	// Bind the context to cancel when we're hidden.
	v.ctx = gtkutil.WithVisibility(ctx, v)

	v.Header.Name = gtk.NewLabel("")
	v.Header.Name.AddCSSClass("channels-name")
	v.Header.Name.SetHAlign(gtk.AlignStart)

	// The header is placed on top of the overlay, kind of like the official
	// client.
	v.Header.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	v.Header.Box.AddCSSClass("channels-header")
	v.Header.Box.AddCSSClass("titlebar")
	v.Header.Box.SetHExpand(true)
	v.Header.Box.Append(v.Header.Name)

	v.Header.WindowHandle = gtk.NewWindowHandle()
	v.Header.WindowHandle.SetVAlign(gtk.AlignStart)
	v.Header.WindowHandle.SetChild(v.Header.Box)

	v.Scroll = gtk.NewScrolledWindow()
	v.Scroll.AddCSSClass("channels-view-scroll")
	v.Scroll.SetVExpand(true)
	v.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	v.Scroll.SetPropagateNaturalWidth(true)
	v.Scroll.SetPropagateNaturalHeight(true)

	var headerScrolled bool

	vadj := v.Scroll.VAdjustment()
	vadj.ConnectValueChanged(func() {
		if vadj.Value() > 0 {
			// If the user has scrolled, then revert to a solid background.
			if !headerScrolled {
				headerScrolled = true
				v.Overlay.AddCSSClass("channels-scrolled")
			}
		} else {
			if headerScrolled {
				headerScrolled = false
				v.Overlay.RemoveCSSClass("channels-scrolled")
			}
		}
	})

	v.Child.Banner = NewBanner(ctx, guildID)
	v.Child.Banner.Invalidate()

	v.Child.Tree = gtk.NewTreeView()
	v.Child.Tree.AddCSSClass("channels-viewtree")
	v.Child.Tree.SetSizeRequest(bannerWidth, -1)
	v.Child.Tree.SetVExpand(true)
	v.Child.Tree.SetHExpand(true)
	// v.Child.Tree.SetVAlign(gtk.AlignStart)
	// v.Child.Tree.SetEnableTreeLines(true)
	v.Child.Tree.SetHeadersVisible(false)
	v.Child.Tree.SetLevelIndentation(4)
	v.Child.Tree.SetActivateOnSingleClick(true)
	// v.Child.Tree.SetVAdjustment(v.Scroll.VAdjustment())
	// v.Child.Tree.SetHAdjustment(v.Scroll.HAdjustment())

	for i, col := range v.cols {
		v.Child.Tree.InsertColumn(col, i)
	}

	v.Child.Tree.ConnectRowActivated(func(path *gtk.TreePath, column *gtk.TreeViewColumn) {
		node := v.tree.paths[path.String()]
		if node == nil {
			log.Println("weird, activated unknown path", path)
			return
		}

		switch node.(type) {
		case *CategoryNode:
			if v.Child.Tree.RowExpanded(path) {
				v.Child.Tree.CollapseRow(path)
			} else {
				v.Child.Tree.ExpandRow(path, false)
			}
		}
	})

	v.Child.Tree.ConnectRowExpanded(func(iter *gtk.TreeIter, path *gtk.TreePath) {
		// TODO: handle
	})

	selection := v.Child.Tree.Selection()
	selection.SetMode(gtk.SelectionBrowse)
	selection.ConnectChanged(func() {
		_, iter, ok := selection.Selected()
		if !ok || v.tree == nil {
			return
		}

		path := v.tree.Path(iter)
		node := v.tree.paths[path.String()]
		if node == nil {
			log.Println("weird, selected unknown path", path)
			return
		}

		switch node.(type) {
		case *ChannelNode, *ThreadNode:
			// We can open these channels.
			ctrl.OpenChannel(node.ID())
		}
	})

	v.Child.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Child.Box.SetVExpand(true)
	v.Child.Box.SetVAlign(gtk.AlignStart)
	v.Child.Box.Append(v.Child.Banner)
	v.Child.Box.Append(v.Child.Tree)

	v.Scroll.SetChild(v.Child)

	v.Overlay = gtk.NewOverlay()
	v.Overlay.SetChild(v.Scroll)
	v.Overlay.AddOverlay(v.Header)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(v.ctx, func(ev gateway.Event) {
		if v.tree == nil {
			return
		}

		switch ev := ev.(type) {
		case *read.UpdateEvent:
			v.tree.UpdateUnread(ev.ChannelID)
		case *gateway.GuildUpdateEvent:
			if ev.ID == v.guildID {
				v.InvalidateHeader()
			}
		case *gateway.ThreadListSyncEvent:
			if ev.GuildID == v.guildID {
				v.InvalidateChannels()
			}
		case *gateway.ChannelCreateEvent:
			if ev.GuildID == v.guildID {
				v.tree.Add([]discord.Channel{ev.Channel})
			}
		case *gateway.ChannelUpdateEvent:
			if ev.GuildID == v.guildID {
				v.tree.UpdateChannel(ev.ID)
			}
		case *gateway.ChannelDeleteEvent:
			if ev.GuildID == v.guildID {
				v.InvalidateChannels()
			}
		case *gateway.ThreadCreateEvent:
			if ev.GuildID == v.guildID {
				v.tree.Add([]discord.Channel{ev.Channel})
			}
		case *gateway.ThreadUpdateEvent:
			if ev.GuildID == v.guildID {
				v.tree.UpdateChannel(ev.ID)
			}
		case *gateway.ThreadDeleteEvent:
			if ev.GuildID == v.guildID {
				v.InvalidateChannels()
			}
		}
	}, channelsViewEvents...)

	viewCSS(v)
	return &v
}

func (v *View) setDone() {
	v.LoadablePage.SetChild(v.Overlay)
}

// InvalidateHeader invalidates the guild name and banner.
func (v *View) InvalidateHeader() {
	state := gtkcord.FromContext(v.ctx.Take())

	g, err := state.Cabinet.Guild(v.guildID)
	if err != nil {
		v.SetError(errors.Wrap(err, "cannot fetch guilds"))
		return
	}

	// TODO: Nitro boost level
	v.Header.Name.SetText(g.Name)
	v.invalidateBanner()
}

// InvalidateChannels invalidates the channels list.
func (v *View) InvalidateChannels() {
	state := gtkcord.FromContext(v.ctx.Take())
	state.MemberState.Subscribe(v.guildID)

	chs, err := state.Offline().Channels(v.guildID)
	if err != nil {
		v.SetError(errors.Wrap(err, "cannot fetch channels"))
		return
	}

	v.tree = NewGuildTree(v.ctx.Take())
	v.tree.Add(chs)
	v.Child.Tree.SetModel(v.tree)
	v.setDone()

	// Legacy and unmaintained code, they said. This is a workaround for the
	// tree not taking any space if we don't HExpand it. If we do, then it shows
	// some background artifact.
	glib.IdleAdd(func() {
		v.Child.Tree.QueueResize()
		v.Child.Box.QueueResize()
	})
}

func (v *View) invalidateBanner() {
	v.Child.Banner.Invalidate()

	if v.Child.Banner.HasBanner() {
		v.Overlay.AddCSSClass("channels-has-banner")
	} else {
		v.Overlay.RemoveCSSClass("channels-has-banner")
	}
}

func newTreeColumns() []*gtk.TreeViewColumn {
	return []*gtk.TreeViewColumn{
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererText()
			ren.SetPadding(0, 6)
			ren.SetObjectProperty("sensitive", true)
			ren.SetObjectProperty("ellipsize", pango.EllipsizeEnd)
			ren.SetObjectProperty("ellipsize-set", true)

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, true)
			col.AddAttribute(ren, "markup", columnName)
			col.AddAttribute(ren, "sensitive", columnSensitive)
			col.SetSizing(gtk.TreeViewColumnAutosize)
			col.SetExpand(true)

			return col
		}(),
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererText()
			ren.SetPadding(4, 0)

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, false)
			col.AddAttribute(ren, "text", columnUnread)
			col.SetSizing(gtk.TreeViewColumnAutosize)

			return col
		}(),
	}
}
