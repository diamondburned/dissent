package window

import (
	"context"
	"log"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/message"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/sidebar"
	"github.com/diamondburned/gtkcord4/internal/icons"
)

type ChatPage struct {
	*adaptive.Fold
	Left       *sidebar.Sidebar
	RightLabel *gtk.Label
	RightChild *gtk.Stack

	prevView *message.View

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
	p.Fold.SetFoldThreshold(680)
	p.Fold.SetFoldWidth(250)
	p.Fold.SetChild(rightBox)
	p.Fold.SetSideChild(p.Left)
	p.Fold.SetRevealSide(true)

	back.ConnectFold(p.Fold)

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
	p.RightChild.SetVisibleChild(p.placeholder)

	if p.prevView != nil {
		p.RightChild.Remove(p.prevView)
		p.prevView = nil
	}
}

// sidebarChatPage implements SidebarController.
type sidebarChatPage ChatPage

func (p *sidebarChatPage) OpenChannel(chID discord.ChannelID) {
	log.Println("load channel", chID)

	view := message.NewView(p.ctx, chID)
	view.Load()

	p.RightChild.AddChild(view)
	p.RightChild.SetVisibleChild(view)

	p.RightLabel.SetText("#" + view.ChannelName())

	win := app.WindowFromContext(p.ctx)
	win.SetTitle("#" + view.ChannelName())

	// Keep track of this.
	if p.prevView != nil {
		p.RightChild.Remove(p.prevView)
	}
	p.prevView = view
}

func (p *sidebarChatPage) CloseGuild(permanent bool) {
	win := app.WindowFromContext(p.ctx)
	win.SetTitle("")

	(*ChatPage)(p).SwitchToPlaceholder()
}
