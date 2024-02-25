// Package sidebar contains the sidebar showing guilds and channels.
package sidebar

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/sidebar/channels"
	"libdb.so/dissent/internal/sidebar/direct"
	"libdb.so/dissent/internal/sidebar/directbutton"
	"libdb.so/dissent/internal/sidebar/guilds"
)

// Sidebar is the bar on the left side of the application once it's logged in.
type Sidebar struct {
	*gtk.Box // horizontal

	Left   *gtk.Box
	DMView *directbutton.View
	Guilds *guilds.View
	Right  *gtk.Stack

	// Keep track of the last child to remove.
	current struct {
		w gtk.Widgetter
		// id discord.GuildID
	}
	placeholder gtk.Widgetter

	ctx context.Context
}

var sidebarCSS = cssutil.Applier("sidebar-sidebar", `
	@define-color sidebar_bg mix(@borders, @theme_bg_color, 0.25);

	windowcontrols.end:not(.empty) {
		margin-right: 4px;
	}
	windowcontrols.start:not(.empty) {
		margin: 4px;
		margin-right: 0;
	}
	.sidebar-guildside {
		background-color: @sidebar_bg;
	}
`)

// NewSidebar creates a new Sidebar.
func NewSidebar(ctx context.Context) *Sidebar {
	s := Sidebar{
		ctx: ctx,
	}

	s.Guilds = guilds.NewView(ctx)
	s.Guilds.Invalidate()

	s.DMView = directbutton.NewView(ctx)
	s.DMView.Invalidate()

	dmSeparator := gtk.NewSeparator(gtk.OrientationHorizontal)
	dmSeparator.AddCSSClass("sidebar-dm-separator")

	// leftBox holds just the DM button and the guild view, as opposed to s.Left
	// which holds the scrolled window and the window controls.
	leftBox := gtk.NewBox(gtk.OrientationVertical, 0)
	leftBox.Append(s.DMView)
	leftBox.Append(dmSeparator)
	leftBox.Append(s.Guilds)

	leftScroll := gtk.NewScrolledWindow()
	leftScroll.SetVExpand(true)
	leftScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyExternal)
	leftScroll.SetChild(leftBox)

	leftCtrl := gtk.NewWindowControls(gtk.PackStart)
	leftCtrl.SetHAlign(gtk.AlignCenter)

	s.Left = gtk.NewBox(gtk.OrientationVertical, 0)
	s.Left.AddCSSClass("sidebar-guildside")
	s.Left.Append(leftCtrl)
	s.Left.Append(leftScroll)

	s.placeholder = gtk.NewWindowHandle()

	s.Right = gtk.NewStack()
	s.Right.SetSizeRequest(channels.ChannelsWidth, -1)
	s.Right.SetVExpand(true)
	s.Right.SetHExpand(true)
	s.Right.AddChild(s.placeholder)
	s.Right.SetVisibleChild(s.placeholder)
	s.Right.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	userBar := newUserBar(ctx, []gtkutil.PopoverMenuItem{
		gtkutil.MenuItem("Quick Switcher", "win.quick-switcher"),
		gtkutil.MenuSeparator("User Settings"),
		gtkutil.Submenu("Set _Status", []gtkutil.PopoverMenuItem{
			gtkutil.MenuItem("_Online", "win.set-online"),
			gtkutil.MenuItem("_Idle", "win.set-idle"),
			gtkutil.MenuItem("_Do Not Disturb", "win.set-dnd"),
			gtkutil.MenuItem("In_visible", "win.set-invisible"),
		}),
		gtkutil.MenuSeparator(""),
		gtkutil.MenuItem("_Preferences", "app.preferences"),
		gtkutil.MenuItem("_About", "app.about"),
		gtkutil.MenuItem("_Logs", "app.logs"),
		gtkutil.MenuItem("_Quit", "app.quit"),
	})

	// TODO: consider if we can merge this ToolbarView with the one in channels
	// and direct.
	rightWrap := adw.NewToolbarView()
	rightWrap.AddBottomBar(userBar)
	rightWrap.SetContent(s.Right)

	s.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	s.Box.SetHExpand(false)
	s.Box.Append(s.Left)
	s.Box.Append(rightWrap)
	sidebarCSS(s)

	return &s
}

// GuildID returns the guild ID that the channel list is showing for, if any.
// If not, 0 is returned.
func (s *Sidebar) GuildID() discord.GuildID {
	ch, ok := s.current.w.(*channels.View)
	if !ok {
		return 0
	}
	return ch.GuildID()
}

func (s *Sidebar) removeCurrent() {
	if s.current.w == nil {
		return
	}

	w := s.current.w
	s.current.w = nil

	if w == nil {
		return
	}

	gtkutil.NotifyProperty(s.Right, "transition-running", func() bool {
		// Remove the widget when the transition is done.
		if !s.Right.TransitionRunning() {
			s.Right.Remove(w)
			return true
		}
		return false
	})
}

func (s *Sidebar) OpenDMs() *direct.ChannelView {
	if direct, ok := s.current.w.(*direct.ChannelView); ok {
		// we're already there
		return direct
	}

	s.unselect()
	s.DMView.SetSelected(true)

	direct := direct.NewChannelView(s.ctx)
	direct.SetVExpand(true)
	s.current.w = direct

	s.Right.AddChild(direct)
	s.Right.SetVisibleChild(direct)

	direct.Invalidate()
	return direct
}

func (s *Sidebar) openGuild(guildID discord.GuildID) *channels.View {
	chs, ok := s.current.w.(*channels.View)
	if ok && chs.GuildID() == guildID {
		// We're already there.
		return chs
	}

	s.unselect()
	s.Guilds.SetSelectedGuild(guildID)

	chs = channels.NewView(s.ctx, guildID)
	chs.SetVExpand(true)
	s.current.w = chs

	s.Right.AddChild(chs)
	s.Right.SetVisibleChild(chs)

	chs.Child.View.GrabFocus()
	chs.InvalidateHeader()
	return chs
}

func (s *Sidebar) unselect() {
	s.Guilds.Unselect()
	s.DMView.Unselect()
	s.removeCurrent()
}

// Unselect unselects the current guild or channel.
func (s *Sidebar) Unselect() {
	s.unselect()
	s.Right.SetVisibleChild(s.placeholder)
}

// SetSelectedGuild marks the guild with the given ID as selected.
func (s *Sidebar) SetSelectedGuild(guildID discord.GuildID) {
	s.Guilds.SetSelectedGuild(guildID)
	s.openGuild(guildID)
}

// // SelectGuild selects and activates the guild with the given ID.
// func (s *Sidebar) SelectGuild(guildID discord.GuildID) {
// 	if s.Guilds.SelectedGuildID() != guildID {
// 		s.Guilds.SetSelectedGuild(guildID)
//
// 		parent := gtk.BaseWidget(s.Parent())
// 		parent.ActivateAction("win.open-guild", gtkcord.NewGuildIDVariant(guildID))
// 	}
// }

// SelectChannel selects and activates the channel with the given ID. It ensures
// that the sidebar is at the right place then activates the controller.
// This function acts the same as if the user clicked on the channel, meaning it
// funnels down to a single widget that then floats up to the controller.
func (s *Sidebar) SelectChannel(chID discord.ChannelID) {
	state := gtkcord.FromContext(s.ctx)
	ch, _ := state.Cabinet.Channel(chID)
	if ch == nil {
		log.Println("sidebar: channel with ID", chID, "not found")
		return
	}

	s.Guilds.SetSelectedGuild(ch.GuildID)

	if ch.GuildID.IsValid() {
		guild := s.openGuild(ch.GuildID)
		guild.SelectChannel(chID)
	} else {
		direct := s.OpenDMs()
		direct.SelectChannel(chID)
	}
}
