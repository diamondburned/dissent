package window

import (
	"context"
	"log"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/message"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar"
	"github.com/diamondburned/gtkcord4/internal/icons"
	"github.com/pkg/errors"
)

type ChatPage struct {
	*adaptive.Fold
	Left       *sidebar.Sidebar
	RightLabel *gtk.Label
	RightChild *gtk.Stack

	prevView gtk.Widgetter

	ctx         context.Context
	placeholder gtk.Widgetter
}

var chatPageCSS = cssutil.Applier("window-chatpage", `
	.right-header {
		border-radius: 0;
		box-shadow: none;
	}
	.right-header .adaptive-sidebar-reveal-button {
		margin: 0 8px;
	}
	.right-header .adaptive-sidebar-reveal-button button {
		margin: 0;
		min-width: calc({$message_avatar_size} - 12px);
	}
	.right-header-label {
		font-weight: bold;
	}
`)

func NewChatPage(ctx context.Context) *ChatPage {
	p := ChatPage{ctx: ctx}
	p.Left = sidebar.NewSidebar(ctx, (*sidebarChatPage)(&p))

	back := adaptive.NewFoldRevealButton()
	back.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)
	back.Button.SetIconName("open-menu")
	back.Button.SetHAlign(gtk.AlignCenter)
	back.Button.SetVAlign(gtk.AlignCenter)

	p.RightLabel = gtk.NewLabel("")
	p.RightLabel.AddCSSClass("right-header-label")
	p.RightLabel.SetXAlign(0)
	p.RightLabel.SetHExpand(true)

	rightHeader := gtk.NewBox(gtk.OrientationHorizontal, 0)
	rightHeader.AddCSSClass("titlebar")
	rightHeader.AddCSSClass("right-header")
	rightHeader.Append(back)
	rightHeader.Append(p.RightLabel)
	rightHeader.Append(gtk.NewWindowControls(gtk.PackEnd))

	rightHandle := gtk.NewWindowHandle()
	rightHandle.SetChild(rightHeader)

	p.placeholder = newEmptyMessagePlaceholer()

	p.RightChild = gtk.NewStack()
	p.RightChild.AddCSSClass("window-message-page")
	p.RightChild.SetVExpand(true)
	p.RightChild.AddChild(p.placeholder)
	p.RightChild.SetVisibleChild(p.placeholder)
	p.RightChild.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	p.SwitchToPlaceholder()

	rightBox := gtk.NewBox(gtk.OrientationVertical, 0)
	rightBox.Append(rightHandle)
	rightBox.Append(p.RightChild)

	p.Fold = adaptive.NewFold(gtk.PosLeft)
	p.Fold.SetFoldThreshold(700)
	p.Fold.SetFoldWidth(225)
	p.Fold.SetChild(rightBox)
	p.Fold.SetSideChild(p.Left)
	p.Fold.SetRevealSide(true)

	back.ConnectFold(p.Fold)

	setStatus := func(status discord.Status) {
		state := gtkcord.FromContext(ctx)
		if err := state.SetStatus(status, nil); err != nil {
			app.Error(ctx, errors.Wrap(err, "invalid status"))
		}
	}

	gtkutil.BindActionMap(p, map[string]func(){
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
	status.Icon.SetOpacity(0.45)
	status.Icon.SetSizeRequest(128, 128)
	status.Icon.SetFromPixbuf(icons.Pixbuf("forum"))
	status.Icon.Show()

	return status
}

// SwitchToPlaceholder switches to the empty placeholder view.
func (p *ChatPage) SwitchToPlaceholder() {
	win := app.WindowFromContext(p.ctx)
	win.SetTitle("")

	p.switchTo(nil)
	p.RightChild.SetVisibleChild(p.placeholder)
}

// SwitchToMessages reopens a new message page of the same channel ID if the
// user is opening one. Otherwise, the placeholder is seen.
func (p *ChatPage) SwitchToMessages() {
	view, ok := p.prevView.(*message.View)
	if ok {
		p.OpenChannel(view.ChannelID())
	} else {
		p.SwitchToPlaceholder()
	}
}

// OpenChannel opens the channel with the given ID. Use this method to direct
// the user to a new channel when they request to, e.g. through a notification.
func (p *ChatPage) OpenChannel(chID discord.ChannelID) {
	p.Left.SelectChannel(chID)
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
			return true
		}
		return false
	})
}

// sidebarChatPage implements SidebarController.
type sidebarChatPage ChatPage

func (p *sidebarChatPage) OpenChannel(chID discord.ChannelID) {
	log.Println("load channel", chID)

	view := message.NewView(p.ctx, chID)
	view.Load()

	p.RightLabel.SetText(gtkcord.ChannelNameFromID(p.ctx, chID))

	win := app.WindowFromContext(p.ctx)
	win.SetTitle(gtkcord.ChannelNameFromID(p.ctx, chID))

	(*ChatPage)(p).switchTo(view)
}

func (p *sidebarChatPage) CloseGuild(permanent bool) {
	(*ChatPage)(p).SwitchToPlaceholder()
}
