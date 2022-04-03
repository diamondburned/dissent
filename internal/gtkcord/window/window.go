package window

import (
	"context"
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/window/login"
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

func (w *Window) SwitchToChatPage() {
	if w.Chat == nil {
		w.Chat = NewChatPage(w.ctx)
		w.Stack.AddChild(w.Chat)
	}
	w.Stack.SetVisibleChild(w.Chat)
	w.SetTitle("")
}

func (w *Window) SwitchToLoginPage() {
	w.Stack.SetVisibleChild(w.Login)
	w.SetTitle("Login")
}

type loginWindow Window

func (w *loginWindow) Ready(state *gtkcord.State) {
	app := w.Application()
	app.ConnectShutdown(func() {
		log.Println("Closing Discord session...")

		if err := state.Close(); err != nil {
			log.Println("error closing session:", err)
		}
	})

	w.ctx = gtkcord.InjectState(w.ctx, state)
	(*Window)(w).SwitchToChatPage()
}

func (w *loginWindow) Reconnecting() {
	w.Stack.SetVisibleChild(w.Loading)
	w.SetTitle("Connecting")
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
