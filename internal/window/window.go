package window

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/pkg/errors"
	"libdb.so/ctxt"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/window/login"
	"libdb.so/dissent/internal/window/quickswitcher"
)

var useDiscordColorScheme = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Use Discord's color preference",
	Section:     "Discord",
	Description: "Whether or not to use Discord's dark/light mode preference.",
})

// SetPreferDarkTheme sets whether or not GTK should use a dark theme.
func SetPreferDarkTheme(prefer bool) {
	if !useDiscordColorScheme.Value() {
		return
	}

	scheme := adw.ColorSchemePreferLight
	if prefer {
		scheme = adw.ColorSchemePreferDark
	}

	adwStyles := adw.StyleManagerGetDefault()
	adwStyles.SetColorScheme(scheme)
}

var _ = cssutil.WriteCSS(`
	.titlebar {
		background-color: @headerbar_bg_color;
	}

	window.devel .titlebar {
		background-image: cross-fade(
			5% -gtk-recolor(url("resource:/org/gnome/Adwaita/styles/assets/devel-symbolic.svg")),
			image(transparent));
		background-repeat: repeat-x;
	}
`)

// Window is the main gtkcord window.
type Window struct {
	*adw.ApplicationWindow
	win *app.Window
	ctx context.Context

	Stack   *gtk.Stack
	Login   *login.Page
	Loading *login.LoadingPage
	Chat    *ChatPage
}

// NewWindow creates a new Window.
func NewWindow(ctx context.Context) *Window {
	appInstance := app.FromContext(ctx)

	win := adw.NewApplicationWindow(appInstance.Application)
	win.SetSizeRequest(320, 320)
	win.SetDefaultSize(800, 600)

	appWindow := app.WrapWindow(appInstance, &win.ApplicationWindow)
	ctx = app.WithWindow(ctx, appWindow)

	w := Window{
		ApplicationWindow: win,
		win:               appWindow,
		ctx:               ctx,
	}
	w.ctx = ctxt.With(w.ctx, &w)

	w.Login = login.NewPage(ctx, &loginWindow{Window: &w})
	w.Login.LoadKeyring()

	w.Loading = login.NewLoadingPage(ctx)

	w.Stack = gtk.NewStack()
	w.Stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	w.Stack.AddChild(w.Login)
	w.Stack.AddChild(w.Loading)
	w.Stack.SetVisibleChild(w.Login)
	win.SetContent(w.Stack)

	w.SwitchToLoginPage()
	return &w
}

func (w *Window) Context() context.Context {
	return w.ctx
}

func (w *Window) initChatPage() {
	w.Chat = NewChatPage(w.ctx, w)
	w.Stack.AddChild(w.Chat)
}

// It's not happy with how this requires a check for ChatPage, but it makes
// sense why these actions are bounded to Window and not ChatPage. Maybe?
// This requires long and hard thinking, which is simply too much for its
// brain.
func (w *Window) initActions() {
	gtkutil.AddActions(w, map[string]func(){
		"set-online":     func() { w.setStatus(discord.OnlineStatus) },
		"set-idle":       func() { w.setStatus(discord.IdleStatus) },
		"set-dnd":        func() { w.setStatus(discord.DoNotDisturbStatus) },
		"set-invisible":  func() { w.setStatus(discord.InvisibleStatus) },
		"open-dms":       func() { w.useChatPage((*ChatPage).OpenDMs) },
		"reset-view":     func() { w.useChatPage((*ChatPage).ResetView) },
		"quick-switcher": func() { w.useChatPage((*ChatPage).OpenQuickSwitcher) },
	})

	gtkutil.AddActionCallbacks(w, map[string]gtkutil.ActionCallback{
		"open-channel": {
			ArgType: gtkcord.SnowflakeVariant,
			Func: func(variant *glib.Variant) {
				id := discord.ChannelID(variant.Int64())
				w.useChatPage(func(p *ChatPage) { p.OpenChannel(id) })
			},
		},
		"open-guild": {
			ArgType: gtkcord.SnowflakeVariant,
			Func: func(variant *glib.Variant) {
				log.Println("opening guild")
				id := discord.GuildID(variant.Int64())
				w.useChatPage(func(p *ChatPage) { p.OpenGuild(id) })
			},
		},
	})

	gtkutil.AddActionShortcuts(w, map[string]string{
		"<Ctrl>K": "win.quick-switcher",
	})
}

func (w *Window) SwitchToChatPage() {
	w.Stack.SetVisibleChild(w.Chat)
	w.Chat.SwitchToMessages()
	w.SetTitle("")
}

func (w *Window) SwitchToLoginPage() {
	w.Stack.SetVisibleChild(w.Login)
	w.SetTitle("Login")
}

func (w *Window) SetLoading() {
	panic("not implemented")
}

// SetTitle sets the window title.
func (w *Window) SetTitle(title string) {
	w.ApplicationWindow.SetTitle(app.FromContext(w.ctx).SuffixedTitle(title))
}

func (w *Window) showQuickSwitcher() {
	w.useChatPage(func(*ChatPage) {
		quickswitcher.ShowDialog(w.ctx)
	})
}

func (w *Window) useChatPage(f func(*ChatPage)) {
	if w.Chat != nil {
		f(w.Chat)
	}
}

func (w *Window) setStatus(status discord.Status) {
	w.useChatPage(func(*ChatPage) {
		state := gtkcord.FromContext(w.ctx).Online()
		go func() {
			if err := state.SetStatus(status, nil); err != nil {
				app.Error(w.ctx, errors.Wrap(err, "invalid status"))
			}
		}()
	})
}

var emptyHeaderCSS = cssutil.Applier("empty-header", `
	.empty-header {
		min-height: 0;
		min-width: 0;
		padding: 0;
		margin: 0;
		border: 0;
	}
`)

func newEmptyHeader() *gtk.Box {
	b := gtk.NewBox(gtk.OrientationVertical, 0)
	emptyHeaderCSS(b)
	return b
}
