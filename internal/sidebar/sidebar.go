// Package sidebar contains the sidebar showing guilds and channels.
package sidebar

import (
	"context"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"libdb.so/dissent/internal/gresources"
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

	ctx context.Context
}

// NewSidebar creates a new Sidebar.
func NewSidebar(ctx context.Context) *Sidebar {
	s := Sidebar{
		ctx: ctx,
	}

	// Open the UI file and attach it to the struct
	uiFile := gresources.New("sidebar.ui")
	s.Box = uiFile.GetRoot().(*gtk.Box)

	// Guild list in the left sidebar
	s.Guilds = guilds.NewView(ctx)
	s.Guilds.Invalidate()

	// DM button on the to left corner + unread DMs
	s.DMView = directbutton.NewView(ctx)
	s.DMView.Invalidate()

	dmSeparator := gtk.NewSeparator(gtk.OrientationHorizontal)
	dmSeparator.AddCSSClass("sidebar-dm-separator")

	s.Left = uiFile.GetComponent("Left").(*gtk.Box)

	// leftBox holds just the DM button and the guild view, as opposed to s.Left
	// which holds the scrolled window and the window controls.
	leftBox := uiFile.GetComponent("leftBox").(*gtk.Box)
	leftBox.Append(s.DMView)
	leftBox.Append(dmSeparator)
	leftBox.Append(s.Guilds)

	s.Right = uiFile.GetComponent("Right").(*gtk.Stack)

	setupUserBar(
		ctx, // Login context
		uiFile.GetComponent("UserRoot").(*gtk.Box),          // UserbarRootContent
		uiFile.GetComponent("UserAvatar").(*adw.Avatar),     // Avatar
		uiFile.GetComponent("UserName").(*gtk.Label),        // Nametag
		uiFile.GetComponent("UserStatus").(*gtk.MenuButton), // Status button
		uiFile.GetComponent("UserMenu").(*gtk.MenuButton),   // Menu button
	)

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

func (s *Sidebar) stackSelect(w gtk.Widgetter) {
	if w == s.current.w {
		return
	}

	old := s.current.w
	s.current.w = w

	if w == nil {
		s.Right.SetVisibleChildName("placeholder")
	} else {
		// This should do nothing if the widget is already in the stack.
		// Maybe???
		s.Right.AddChild(w)
		s.Right.SetVisibleChild(w)

		w := gtk.BaseWidget(w)
		w.GrabFocus()
	}

	if old != nil {
		gtkutil.NotifyProperty(s.Right, "transition-running", func() bool {
			// Remove the widget when the transition is done.
			if !s.Right.TransitionRunning() {
				s.Right.Remove(old)

				w := gtk.BaseWidget(old)
				slog.Debug(
					"sidebar: right stack transition done, removed widget",
					"widget", w.Type().String()+"."+strings.Join(w.CSSClasses(), "."))

				return true
			} else {
				slog.Debug("sidebar: right stack transition started")
				return false
			}
		})
	}
}

// OpenDMs opens the DMs view. It automatically loads the DMs on first open, so
// the returned ChannelView is guaranteed to be ready.
func (s *Sidebar) OpenDMs() *direct.ChannelView {
	if direct, ok := s.current.w.(*direct.ChannelView); ok {
		// we're already there
		return direct
	}

	s.unselect()
	s.DMView.SetSelected(true)

	direct := direct.NewChannelView(s.ctx)
	direct.SetVExpand(true)
	direct.Invalidate()

	s.stackSelect(direct)

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
	chs.InvalidateHeader()

	s.stackSelect(chs)
	return chs
}

func (s *Sidebar) unselect() {
	s.Guilds.Unselect()
	s.DMView.Unselect()
	s.stackSelect(nil)
}

// Unselect unselects the current guild or channel.
func (s *Sidebar) Unselect() {
	s.unselect()
	s.Right.SetVisibleChildName("placeholder")
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
		slog.Error(
			"cannot select channel in sidebar since it's not found in state",
			"channel_id", chID)
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
