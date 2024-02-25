package window

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/ningen/v3/states/read"
	"libdb.so/ctxt"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/messages"
	"libdb.so/dissent/internal/sidebar"
	"libdb.so/dissent/internal/window/backbutton"
	"libdb.so/dissent/internal/window/quickswitcher"
)

var lastGuildKey = app.NewSingleStateKey[discord.GuildID]("last-guild-state")
var lastChannelKey = app.NewStateKey[discord.ChannelID]("guild-last-open")

// TODO: refactor this to support TabOverview. We do this by refactoring Sidebar
// out completely and merging it into ChatPage. We can then get rid of the logic
// to keep the Sidebar in sync with the ChatPage, since each tab will have its
// own Sidebar.

type ChatPage struct {
	*adw.OverlaySplitView
	Sidebar     *sidebar.Sidebar
	RightHeader *adw.HeaderBar
	RightTitle  *gtk.Label

	tabView       *adw.TabView
	quickswitcher *quickswitcher.Dialog

	lastGuildState   *app.TypedSingleState[discord.GuildID]
	lastChannelState *app.TypedState[discord.ChannelID]

	lastGuild discord.GuildID

	// lastButtons keeps tracks of the header buttons of the previous view.
	// On view change, these buttons will be removed.
	lastButtons []gtk.Widgetter

	tabs map[uintptr]*chatTab // K: *adw.TabPage
	ctx  context.Context
}

type chatPageView struct {
	body          gtk.Widgetter
	headerButtons []gtk.Widgetter
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
		ctx:              ctx,
		tabs:             make(map[uintptr]*chatTab),
		lastGuildState:   lastGuildKey.Acquire(ctx),
		lastChannelState: lastChannelKey.Acquire(ctx),
	}

	p.quickswitcher = quickswitcher.NewDialog(ctx)
	p.quickswitcher.SetHideOnClose(true) // so we can reopen it later

	p.tabView = adw.NewTabView()
	p.tabView.AddCSSClass("window-chatpage-tabview")
	p.tabView.SetDefaultIcon(gio.NewThemedIcon("channel-symbolic"))
	p.tabView.NotifyProperty("selected-page", func() {
		p.onActiveTabChange(p.tabView.SelectedPage())
	})
	p.tabView.ConnectClosePage(func(page *adw.TabPage) bool {
		_, ok := p.tabs[page.Native()]
		if ok {
			delete(p.tabs, page.Native())
			p.tabView.ClosePageFinish(page, true)
		}
		return gdk.EVENT_STOP
	})

	p.Sidebar = sidebar.NewSidebar(ctx)
	p.Sidebar.SetHAlign(gtk.AlignStart)

	p.RightTitle = gtk.NewLabel("")
	p.RightTitle.AddCSSClass("right-header-label")
	p.RightTitle.SetXAlign(0)
	p.RightTitle.SetHExpand(true)
	p.RightTitle.SetEllipsize(pango.EllipsizeEnd)

	back := backbutton.New()
	back.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)

	newTabButton := gtk.NewButtonFromIconName("list-add-symbolic")
	newTabButton.SetTooltipText("Open a New Tab")
	newTabButton.ConnectClicked(func() { p.newTab() })

	p.RightHeader = adw.NewHeaderBar()
	p.RightHeader.AddCSSClass("right-header")
	p.RightHeader.SetShowEndTitleButtons(true)
	p.RightHeader.SetShowBackButton(false) // this is useless with OverlaySplitView
	p.RightHeader.SetShowTitle(false)
	p.RightHeader.PackStart(back)
	p.RightHeader.PackStart(p.RightTitle)
	p.RightHeader.PackEnd(newTabButton)

	tabBar := adw.NewTabBar()
	tabBar.AddCSSClass("window-chatpage-tabbar")
	tabBar.SetView(p.tabView)
	tabBar.SetAutohide(true)

	rightBox := adw.NewToolbarView()
	rightBox.SetTopBarStyle(adw.ToolbarFlat)
	rightBox.SetHExpand(true)
	rightBox.AddTopBar(p.RightHeader)
	rightBox.AddTopBar(tabBar)
	rightBox.SetContent(p.tabView)

	p.OverlaySplitView = adw.NewOverlaySplitView()
	p.OverlaySplitView.SetSidebar(p.Sidebar)
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

	state := gtkcord.FromContext(ctx)
	w.ConnectDestroy(state.AddHandler(
		func(*gateway.MessageCreateEvent) { p.updateWindowTitle() },
		func(*gateway.MessageUpdateEvent) { p.updateWindowTitle() },
		func(*gateway.MessageDeleteEvent) { p.updateWindowTitle() },
		func(*read.UpdateEvent) { p.updateWindowTitle() },
	))

	chatPageCSS(p)
	return &p
}

// OpenQuickSwitcher opens the Quick Switcher dialog.
func (p *ChatPage) OpenQuickSwitcher() { p.quickswitcher.Show() }

// ResetView switches out of any channel view and into the placeholder view.
// This method is used when the guild becomes unavailable.
func (p *ChatPage) ResetView() { p.SwitchToPlaceholder() }

// SwitchToPlaceholder switches to the empty placeholder view.
func (p *ChatPage) SwitchToPlaceholder() {
	tab := p.currentTab()
	tab.switchToPlaceholder()

	p.onActiveTabChange(p.tabView.Page(tab))
}

// SwitchToMessages reopens a new message page of the same channel ID if the
// user is opening one. Otherwise, the placeholder is seen.
func (p *ChatPage) SwitchToMessages() {
	tab := p.currentTab()
	tab.switchToPlaceholder()

	p.lastGuildState.Exists(func(exists bool) {
		if !exists {
			// Open DMs if there is no last opened channel.
			p.OpenDMs()
			return
		}
		// Restore the last opened channel if there is one.
		p.lastGuildState.Get(func(id discord.GuildID) {
			if id.IsValid() {
				p.OpenGuild(id)
			} else {
				p.OpenDMs()
			}
		})
	})
}

// OpenDMs opens the DMs page.
func (p *ChatPage) OpenDMs() {
	p.lastGuild = 0
	p.lastGuildState.Set(0)
	p.Sidebar.OpenDMs()
	p.restoreLastChannel(0)
}

// OpenGuild opens the guild with the given ID.
func (p *ChatPage) OpenGuild(guildID discord.GuildID) {
	p.lastGuild = guildID
	p.lastGuildState.Set(guildID)
	p.Sidebar.SetSelectedGuild(guildID)
	p.restoreLastChannel(guildID)
}

func (p *ChatPage) restoreLastChannel(guildID discord.GuildID) {
	var k string
	if guildID.IsValid() {
		k = guildID.String()
	}

	// Allow a bit of delay for the page to finish loading.
	glib.IdleAdd(func() {
		p.lastChannelState.Exists(k, func(exists bool) {
			if exists {
				p.lastChannelState.Get(k, p.OpenChannel)
			} else {
				p.SwitchToPlaceholder()
			}
		})
	})
}

// OpenChannel opens the channel with the given ID. Use this method to direct
// the user to a new channel when they request to, e.g. through a notification.
func (p *ChatPage) OpenChannel(chID discord.ChannelID) {
	var tab *chatTab
	var reselect bool
	for _, t := range p.tabs {
		if t.alreadyOpens(chID) {
			tab = t
			reselect = true
			break
		}
	}
	if tab == nil {
		tab = p.currentTab()
	}

	tab.switchToChannel(chID)

	page := p.tabView.Page(tab)
	updateTabInfo(p.ctx, page, chID)
	if reselect {
		p.tabView.SetSelectedPage(page)
	}

	p.onActiveTabChange(page)

	state := gtkcord.FromContext(p.ctx).Offline()
	ch, _ := state.Channel(chID)
	if ch != nil {
		var k string
		if ch.GuildID.IsValid() {
			k = ch.GuildID.String()
		}
		// Save the last opened channel for the guild.
		p.lastChannelState.Set(k, chID)
	}
}

func updateTabInfo(ctx context.Context, page *adw.TabPage, chID discord.ChannelID) {
	if chID.IsValid() {
		page.SetIcon(gio.NewThemedIcon("channel-symbolic"))

		title := gtkcord.WindowTitleFromID(ctx, chID)
		// We don't actually want the prefixing # because we already have the
		// tab icon.
		title = strings.TrimPrefix(title, "#")
		page.SetTitle(title)
	} else {
		page.SetIcon(nil)
		page.SetTitle("New Tab")
	}
}

// currentTab returns the current tab. If there is no tab, then it creates one.
func (p *ChatPage) currentTab() *chatTab {
	var tab *chatTab

	page := p.tabView.SelectedPage()
	if page != nil {
		// We already have a tab.
		// Ensure our window gets updated by the end.
		tab = p.tabs[page.Native()]
	} else {
		// We don't have an active tab right now. Create one.
		tab = p.newTab()
	}

	return tab
}

func (p *ChatPage) newTab() *chatTab {
	tab := newChatTab(p.ctx)

	page := p.tabView.Append(tab)
	updateTabInfo(p.ctx, page, 0)

	p.tabs[page.Native()] = tab
	p.tabView.SetSelectedPage(page)

	return tab
}

func (p *ChatPage) onActiveTabChange(page *adw.TabPage) {
	// Remove the previous header buttons.
	for _, button := range p.lastButtons {
		p.RightHeader.Remove(button)
	}
	p.lastButtons = nil

	p.updateWindowTitle()

	var tab *chatTab
	var chID discord.ChannelID

	if page != nil {
		tab = p.tabs[page.Native()]
		if tab == nil {
			// Ignore this. It's possible that we're still initializing.
			return
		}

		chID = tab.channelID()

		// Add the new header buttons.
		if tab.messageView != nil {
			p.lastButtons = tab.messageView.HeaderButtons()
			for i := len(p.lastButtons) - 1; i >= 0; i-- {
				button := p.lastButtons[i]
				p.RightHeader.PackEnd(button)
			}
		}
	}

	// Update the left guild list and channel list.
	if chID.IsValid() {
		// TODO: it really has to get rid of this SelectChannel call...
		// It's really hard for it to try and have a SetSelectedChannel function
		// because of how the SelectionChanged signal works.
		p.Sidebar.SelectChannel(chID)
	} else {
		// Hack to ensure that the guild item is selected when we have no
		// channel on display.
		if p.lastGuild.IsValid() {
			p.Sidebar.SetSelectedGuild(p.lastGuild)
		} else {
			p.Sidebar.Unselect()
		}
	}

	// Update the displaying window title.
	var chName string
	if chID.IsValid() {
		chName = gtkcord.ChannelNameFromID(p.ctx, chID)
	}

	// Update the window titles.
	p.RightTitle.SetText(chName)
}

func (p *ChatPage) updateWindowTitle() {
	var title string
	if page := p.tabView.SelectedPage(); page != nil {
		title = page.Title()
	}

	state := gtkcord.FromContext(p.ctx)

	// Add a ping indicator if the user has pings.
	mentions := state.ReadState.TotalMentionCount()
	if mentions > 0 {
		title = fmt.Sprintf("(%d) %s", mentions, title)
	}

	win, _ := ctxt.From[*Window](p.ctx)
	win.SetTitle(title)
}

type chatTab struct {
	*gtk.Stack
	placeholder gtk.Widgetter
	messageView *messages.View // nilable
	ctx         context.Context
}

func newChatTab(ctx context.Context) *chatTab {
	var t chatTab
	t.ctx = ctx
	t.placeholder = newEmptyMessagePlaceholder()

	t.Stack = gtk.NewStack()
	t.Stack.AddCSSClass("window-message-page")
	t.Stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	t.Stack.AddChild(t.placeholder)
	t.Stack.SetVisibleChild(t.placeholder)

	return &t
}

func (t *chatTab) alreadyOpens(id discord.ChannelID) bool {
	return t.channelID() == id
}

func (t *chatTab) channelID() discord.ChannelID {
	if t.messageView == nil {
		return 0
	}
	return t.messageView.ChannelID()
}

func (t *chatTab) switchToPlaceholder() bool {
	return t.switchToChannel(0)
}

func (t *chatTab) switchToChannel(id discord.ChannelID) bool {
	if t.alreadyOpens(id) {
		return false
	}

	old := t.messageView

	if id.IsValid() {
		t.messageView = messages.NewView(t.ctx, id)

		t.Stack.AddChild(t.messageView)
		t.Stack.SetVisibleChild(t.messageView)

		viewWidget := gtk.BaseWidget(t.messageView)
		viewWidget.GrabFocus()
	} else {
		t.messageView = nil
		t.Stack.SetVisibleChild(t.placeholder)
	}

	if old != nil {
		gtkutil.NotifyProperty(t.Stack, "transition-running", func() bool {
			if !t.Stack.TransitionRunning() {
				t.Stack.Remove(old)
				return true
			}
			return false
		})
	}

	return true
}

func newEmptyMessagePlaceholder() gtk.Widgetter {
	status := adaptive.NewStatusPage()
	status.SetIconName("chat-bubbles-empty-symbolic")
	status.Icon.SetOpacity(0.45)
	status.Icon.SetIconSize(gtk.IconSizeLarge)

	return status
}
