package window

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/messages"
	"github.com/diamondburned/gtkcord4/internal/sidebar"
	"github.com/diamondburned/gtkcord4/internal/window/backbutton"
	"github.com/diamondburned/gtkcord4/internal/window/quickswitcher"
	"github.com/pkg/errors"
)

const lastOpenKey = "last-open-channel-state"

var lastOpenStateKey = app.NewStateKey[discord.ChannelID](lastOpenKey)

type ChatPage struct {
	*adw.OverlaySplitView
	Left       *sidebar.Sidebar
	RightLabel *gtk.Label
	RightChild *gtk.Stack

	prevView gtk.Widgetter
	lastOpen *app.TypedState[discord.ChannelID]

	ctx         context.Context
	placeholder gtk.Widgetter
}

var chatPageCSS = cssutil.Applier("window-chatpage", `
	.right-header {
		border-radius: 0;
		box-shadow: none;
	}
	.right-header .adaptive-sidebar-reveal-button {
		margin: 0;
	}
	.right-header .adaptive-sidebar-reveal-button button {
		margin-left: 8px;
		margin-right: 4px;
	}
	.right-header-label {
		font-weight: bold;
	}
`)

func NewChatPage(ctx context.Context, w *Window) *ChatPage {
	p := ChatPage{
		ctx:      ctx,
		lastOpen: lastOpenStateKey.Acquire(ctx),
	}
	p.Left = sidebar.NewSidebar(ctx, (*sidebarChatPage)(&p), &p)
	p.Left.SetHAlign(gtk.AlignStart)

	p.RightLabel = gtk.NewLabel("")
	p.RightLabel.AddCSSClass("right-header-label")
	p.RightLabel.SetXAlign(0)
	p.RightLabel.SetHExpand(true)
	p.RightLabel.SetEllipsize(pango.EllipsizeEnd)

	back := backbutton.New()
	back.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)

	rightHeader := adw.NewHeaderBar()
	rightHeader.AddCSSClass("right-header")
	rightHeader.SetShowEndTitleButtons(true)
	rightHeader.SetShowBackButton(false) // this is useless with OverlaySplitView
	rightHeader.SetShowTitle(false)
	rightHeader.PackStart(back)
	rightHeader.PackStart(p.RightLabel)

	p.placeholder = newEmptyMessagePlaceholer()

	p.RightChild = gtk.NewStack()
	p.RightChild.AddCSSClass("window-message-page")
	p.RightChild.SetVExpand(true)
	p.RightChild.AddChild(p.placeholder)
	p.RightChild.SetVisibleChild(p.placeholder)
	p.RightChild.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	p.SwitchToPlaceholder()

	rightBox := adw.NewToolbarView()
	rightBox.SetHExpand(true)
	rightBox.AddTopBar(rightHeader)
	rightBox.SetContent(p.RightChild)
	rightBox.SetTopBarStyle(adw.ToolbarFlat)

	p.OverlaySplitView = adw.NewOverlaySplitView()
	p.OverlaySplitView.SetSidebar(p.Left)
	p.OverlaySplitView.SetSidebarPosition(gtk.PackStart)
	p.OverlaySplitView.SetContent(rightBox)
	p.OverlaySplitView.SetEnableHideGesture(true)
	p.OverlaySplitView.SetEnableShowGesture(true)
	p.OverlaySplitView.SetMinSidebarWidth(200)
	p.OverlaySplitView.SetMaxSidebarWidth(300)
	p.OverlaySplitView.SetSidebarWidthFraction(0.5)

	back.ConnectSplitView(p.OverlaySplitView)

	breakpoint := adw.NewBreakpoint(adw.BreakpointConditionParse("max-width: 500sp"))
	breakpoint.AddSetter(p.OverlaySplitView, "collapsed", true)
	w.AddBreakpoint(breakpoint)

	setStatus := func(status discord.Status) {
		state := gtkcord.FromContext(ctx)
		if err := state.SetStatus(status, nil); err != nil {
			app.Error(ctx, errors.Wrap(err, "invalid status"))
		}
	}

	gtkutil.BindActionMap(p, map[string]func(){
		"discord.show-qs":       p.ShowQuickSwitcher,
		"discord.set-online":    func() { setStatus(discord.OnlineStatus) },
		"discord.set-idle":      func() { setStatus(discord.IdleStatus) },
		"discord.set-dnd":       func() { setStatus(discord.DoNotDisturbStatus) },
		"discord.set-invisible": func() { setStatus(discord.InvisibleStatus) },
	})

	chatPageCSS(p)
	return &p
}

func newEmptyMessagePlaceholer() gtk.Widgetter {
	status := adaptive.NewStatusPage()
	status.SetIconName("chat-bubbles-empty-symbolic")
	status.Icon.SetOpacity(0.45)

	return status
}

// ShowQuickSwitcher shows the Quick Switcher dialog.
func (p *ChatPage) ShowQuickSwitcher() {
	quickswitcher.ShowDialog(p.ctx, (*quickSwitcherChatPage)(p))
}

// SwitchToPlaceholder switches to the empty placeholder view.
func (p *ChatPage) SwitchToPlaceholder() {
	win := app.WindowFromContext(p.ctx)
	win.SetTitle("")

	p.RightLabel.SetText("")
	p.switchTo(nil)
	p.RightChild.SetVisibleChild(p.placeholder)
}

// SwitchToMessages reopens a new message page of the same channel ID if the
// user is opening one. Otherwise, the placeholder is seen.
func (p *ChatPage) SwitchToMessages() {
	view, ok := p.prevView.(*messages.View)
	if ok {
		p.OpenChannel(view.ChannelID())
		return
	}

	p.SwitchToPlaceholder()

	p.lastOpen.Exists(lastOpenKey, func(exists bool) {
		if !exists {
			// Open DMs if there is no last opened channel.
			p.OpenDMs()
			return
		}
		// Restore the last opened channel if there is one.
		p.lastOpen.Get(lastOpenKey, p.OpenChannel)
	})
}

// OpenDMs opens the DMs page.
func (p *ChatPage) OpenDMs() {
	p.SwitchToPlaceholder()
	p.Left.OpenDMs()
}

// OpenChannel opens the channel with the given ID. Use this method to direct
// the user to a new channel when they request to, e.g. through a notification.
func (p *ChatPage) OpenChannel(chID discord.ChannelID) {
	p.SwitchToPlaceholder()

	if !chID.IsValid() {
		return
	}

	p.Left.SelectChannel(chID)

	p.RightLabel.SetText(gtkcord.ChannelNameFromID(p.ctx, chID))

	win := app.WindowFromContext(p.ctx)
	win.SetTitle(gtkcord.ChannelNameFromID(p.ctx, chID))

	view := messages.NewView(p.ctx, chID)
	p.switchTo(view)

	p.lastOpen.Set(lastOpenKey, chID)
}

// OpenGuild opens the guild with the given ID.
func (p *ChatPage) OpenGuild(guildID discord.GuildID) {
	p.SwitchToPlaceholder()
	p.Left.SelectGuild(guildID)
}

func (p *ChatPage) switchTo(w gtk.Widgetter) {
	old := p.prevView
	p.prevView = w

	if w != nil {
		p.RightChild.AddChild(w)
		p.RightChild.SetVisibleChild(w)

		base := gtk.BaseWidget(w)
		base.GrabFocus()
	}

	if old == nil {
		return
	}

	gtkutil.NotifyProperty(p.RightChild, "transition-running", func() bool {
		// Remove the widget when the transition is done.
		if !p.RightChild.TransitionRunning() {
			p.RightChild.Remove(old)

			// Hack: destroy everything!
			// log.Println("destroying previous message view")
			// gtkutil.RecursiveUnfuck(old)

			return true
		}
		return false
	})
}

// sidebarChatPage implements SidebarController.
type sidebarChatPage ChatPage

func (p *sidebarChatPage) CloseGuild(permanent bool) {
	(*ChatPage)(p).SwitchToPlaceholder()
}

type quickSwitcherChatPage ChatPage

func (p *quickSwitcherChatPage) OpenChannel(chID discord.ChannelID) {
	(*ChatPage)(p).OpenChannel(chID)
}

func (p *quickSwitcherChatPage) OpenGuild(guildID discord.GuildID) {
	(*ChatPage)(p).OpenGuild(guildID)
}
