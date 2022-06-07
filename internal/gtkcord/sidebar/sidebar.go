// Package sidebar contains the sidebar showing guilds and channels.
package sidebar

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar/channels"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar/direct"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar/guilds"
)

// Controller is the parent controller that Sidebar controls.
type Controller interface {
	channels.Controller
	CloseGuild(permanent bool)
}

// Sidebar is the bar on the left side of the application once it's logged in.
type Sidebar struct {
	*gtk.Box // horizontal

	Left     *gtk.Box
	DMButton *DMButton
	Guilds   *guilds.View
	Right    *gtk.Stack

	// Keep track of the last child to remove.
	current struct {
		w gtk.Widgetter
		// id discord.GuildID
	}

	ctx  context.Context
	ctrl Controller
}

var sidebarCSS = cssutil.Applier("sidebar-sidebar", `
	windowcontrols.end:not(.empty) {
		margin-right: 4px;
	}
	windowcontrols.start:not(.empty) {
		margin: 4px;
		margin-right: 0;
	}
	.sidebar-guildside {
		background-color: mix(@borders, @theme_bg_color, 0.25);
	}
`)

// NewSidebar creates a new Sidebar.
func NewSidebar(ctx context.Context, ctrl Controller) *Sidebar {
	s := Sidebar{
		ctx:  ctx,
		ctrl: ctrl,
	}

	s.Guilds = guilds.NewView(ctx, (*guildsSidebar)(&s))
	s.Guilds.Invalidate()

	s.DMButton = NewDMButton(ctx, func() {
		direct := s.openDMs()
		direct.Invalidate()
	})
	s.DMButton.Invalidate()

	dmSeparator := gtk.NewSeparator(gtk.OrientationHorizontal)
	dmSeparator.AddCSSClass("sidebar-dm-separator")

	// leftBox holds just the DM button and the guild view, as opposed to s.Left
	// which holds the scrolled window and the window controls.
	leftBox := gtk.NewBox(gtk.OrientationVertical, 0)
	leftBox.Append(s.DMButton)
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

	s.current.w = gtk.NewWindowHandle()

	s.Right = gtk.NewStack()
	s.Right.SetSizeRequest(channels.ChannelsWidth, -1)
	s.Right.SetVExpand(true)
	s.Right.SetHExpand(true)
	s.Right.AddChild(s.current.w)
	s.Right.SetVisibleChild(s.current.w)
	s.Right.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	userBar := newUserBar(ctx, [][2]string{
		{"_Preferences", "app.preferences"},
		{"_About", "app.about"},
		{"_Quit", "app.quit"},
	})

	rightWrap := gtk.NewBox(gtk.OrientationVertical, 0)
	rightWrap.Append(s.Right)
	rightWrap.Append(userBar)

	s.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	s.Box.Append(s.Left)
	s.Box.Append(rightWrap)
	s.Box.Append(gtk.NewSeparator(gtk.OrientationVertical))
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

func (s *Sidebar) openDMs() *direct.ChannelView {
	if direct, ok := s.current.w.(*direct.ChannelView); ok {
		// we're already there
		return direct
	}

	s.ctrl.CloseGuild(true)
	s.Guilds.Unselect()

	direct := direct.NewChannelView(s.ctx, s.ctrl)
	direct.SetVExpand(true)

	s.Right.AddChild(direct)
	s.Right.SetVisibleChild(direct)

	s.removeCurrent()
	s.current.w = direct

	return direct
}

func (s *Sidebar) openGuild(guildID discord.GuildID) *channels.View {
	s.DMButton.Pill.State = 0
	s.DMButton.Pill.Invalidate()

	if ch, ok := s.current.w.(*channels.View); ok && ch.GuildID() == guildID {
		// We're already there.
		return ch
	}

	s.ctrl.CloseGuild(true)

	ch := channels.NewView(s.ctx, s.ctrl, guildID)
	ch.SetVExpand(true)
	ch.InvalidateHeader()

	s.Right.AddChild(ch)
	s.Right.SetVisibleChild(ch)

	s.removeCurrent()
	s.current.w = ch

	ch.Child.Tree.GrabFocus()
	return ch
}

// SelectChannel selects and activates the channel with the given ID. It ensures
// that the sidebar is at the right place then activates the controller.
func (s *Sidebar) SelectChannel(chID discord.ChannelID) {
	state := gtkcord.FromContext(s.ctx)
	ch, _ := state.Cabinet.Channel(chID)
	if ch == nil {
		return
	}

	if ch.GuildID.IsValid() {
		guild := s.openGuild(ch.GuildID)
		guild.SelectChannel(chID)
		guild.InvalidateChannels()
	} else {
		direct := s.openDMs()
		direct.SelectChannel(chID)
		direct.Invalidate()
	}
}

// guildsSidebar implements guilds.Controller.
type guildsSidebar Sidebar

func (s *guildsSidebar) OpenGuild(guildID discord.GuildID) {
	ch := (*Sidebar)(s).openGuild(guildID)
	ch.InvalidateChannels()
}

// CloseGuild implements guilds.Controller.
func (s *guildsSidebar) CloseGuild(permanent bool) {
	s.ctrl.CloseGuild(permanent)
	s.removeCurrent()
}

func (s *guildsSidebar) removeCurrent() {
	(*Sidebar)(s).removeCurrent()
}
