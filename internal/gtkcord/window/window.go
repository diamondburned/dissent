package window

import (
	"context"
	"fmt"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/notify"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/window/login"
	"github.com/diamondburned/ningen/v3"
)

// Window is the main gtkcord window.
type Window struct {
	*app.Window
	ctx context.Context

	Stack   *gtk.Stack
	Login   *login.Page
	Loading *login.LoadingPage
	Chat    *ChatPage
}

// NewWindow creates a new Window.
func NewWindow(ctx context.Context) *Window {
	win := app.FromContext(ctx).NewWindow()
	win.SetTitle("")
	win.SetTitlebar(newEmptyHeader())
	win.SetDefaultSize(800, 600)

	ctx = app.WithWindow(ctx, win)

	w := Window{
		Window: win,
		ctx:    ctx,
	}

	w.Login = login.NewPage(ctx, (*loginWindow)(&w))
	w.Login.LoadKeyring()

	w.Loading = login.NewLoadingPage(ctx)

	w.Stack = gtk.NewStack()
	w.Stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	w.Stack.AddChild(w.Login)
	w.Stack.AddChild(w.Loading)
	w.Stack.SetVisibleChild(w.Login)

	win.SetChild(w.Stack)
	return &w
}

func (w *Window) Context() context.Context {
	return w.ctx
}

func (w *Window) SwitchToChatPage() {
	if w.Chat == nil {
		w.Chat = NewChatPage(w.ctx)
		w.Stack.AddChild(w.Chat)
	}
	w.Stack.SetVisibleChild(w.Chat)
	w.Chat.SwitchToMessages()
	w.SetTitle("")
}

func (w *Window) SwitchToLoginPage() {
	w.Stack.SetVisibleChild(w.Login)
	w.SetTitle("Login")
}

type loginWindow Window

var monitorEvents = []gateway.Event{
	(*ningen.ConnectedEvent)(nil),
	(*ningen.DisconnectedEvent)(nil),
	(*ws.CloseEvent)(nil),
	(*ws.BackgroundErrorEvent)(nil),
	(*gateway.MessageCreateEvent)(nil), // notifications
}

func (w *loginWindow) Hook(state *gtkcord.State) {
	w.ctx = gtkcord.InjectState(w.ctx, state)
	w.Reconnecting()

	ctx := gtkutil.WithCanceller(w.ctx)
	ctx.Renew()
	w.ConnectDestroy(ctx.Cancel)

	var reconnecting glib.SourceHandle

	// When the websocket closes, the screen must be changed to a busy one. The
	// websocket may close if it's disconnected unexpectedly.
	state.BindHandler(ctx, func(ev gateway.Event) {
		switch ev := ev.(type) {
		case *ningen.ConnectedEvent:
			log.Println("connected:", ev.EventType())

			// Cancel the 3s delay if we're already connected during that.
			if reconnecting != 0 {
				glib.SourceRemove(reconnecting)
				reconnecting = 0
			}

			w.Connected()

		case *ws.BackgroundErrorEvent:
			log.Println("warning: gateway:", ev)

		case *ws.CloseEvent:
			log.Println("disconnected (*ws.CloseEvent), err:", ev.Err, ", code:", ev.Code)

		case *ningen.DisconnectedEvent:
			log.Println("disconnected, err:", ev.Err, ", code:", ev.Code)

			if ev.IsLoggedOut() {
				w.PromptLogin()
				return
			}

			// Add a 3s delay in case we have a sudden disruption that
			// immediately recovers.
			reconnecting = glib.TimeoutSecondsAdd(3, func() {
				w.Reconnecting()
				reconnecting = 0
			})

		case *gateway.MessageCreateEvent:
			mentions := state.MessageMentions(&ev.Message)
			if mentions == 0 {
				return
			}

			if state.Status() == discord.DoNotDisturbStatus {
				return
			}

			avatarURL := gtkcord.InjectAvatarSize(ev.Author.AvatarURL())

			notify.Send(w.ctx, notify.Notification{
				ID: notify.HashID(ev.ChannelID),
				Title: fmt.Sprintf(
					"%s (%s)",
					state.AuthorDisplayName(ev),
					gtkcord.ChannelNameFromID(w.ctx, ev.ChannelID),
				),
				Body:  state.MessagePreview(&ev.Message),
				Icon:  notify.IconURL(w.ctx, avatarURL, notify.IconName("avatar-default-symbolic")),
				Sound: notify.MessageSound,
				Action: notify.ActionJSONData("app.open-channel", gtkcord.OpenChannelCommand{
					ChannelID: ev.ChannelID,
					MessageID: ev.ID,
				}),
			})
		}
	}, monitorEvents...)
}

func (w *loginWindow) Ready(state *gtkcord.State) {
	app := w.Application()
	app.ConnectShutdown(func() {
		log.Println("Closing Discord session...")

		if err := state.Close(); err != nil {
			log.Println("error closing session:", err)
		}
	})
}

func (w *loginWindow) Reconnecting() {
	w.Stack.SetVisibleChild(w.Loading)
	w.SetTitle("Connecting")
}

func (w *loginWindow) Connected() {
	(*Window)(w).SwitchToChatPage()
}

func (w *loginWindow) PromptLogin() {
	(*Window)(w).SwitchToLoginPage()
}

var emptyHeaderCSS = cssutil.Applier("empty-header", `
	.empty-header { min-height: 0; }
`)

func newEmptyHeader() *gtk.Box {
	b := gtk.NewBox(gtk.OrientationVertical, 0)
	emptyHeaderCSS(b)
	return b
}
