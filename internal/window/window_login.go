package window

import (
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"

	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/session"
)

// updateSession switches the window's page to match the session's status.
func (w *Window) updateSession() {
	status := w.session.Status()
	w.setLoading(status == session.Connecting)

	switch status {
	case session.LoggedOut:
		w.SwitchToLoginPage()
		if err := w.session.Err(); err != nil {
			w.Login.Login.ShowError(err)
		}

	case session.Connecting:
		w.SetTitle("Connecting")

	case session.Connected:
		if w.Chat == nil || w.chatState != w.session.State() {
			w.initChatPage(w.session.State())
		}
		// Only switch if needed, so reconnecting keeps the opened channel and
		// its scroll position as they were.
		if !coreglib.ObjectEq(w.Stack.VisibleChild(), w.Chat) {
			w.SwitchToChatPage()
		} else {
			w.SetTitle("")
		}
	}
}

// setLoading shows or hides the loading page on top of the current page.
func (w *Window) setLoading(loading bool) {
	w.loading.SetRevealChild(loading)
	// A hidden revealer still covers the page, so let clicks through.
	w.loading.SetCanTarget(loading)
}

// initChatPage creates a new ChatPage for the given state, replacing the
// existing one if the user logged in again.
func (w *Window) initChatPage(state *gtkcord.State) {
	if w.Chat != nil {
		w.Stack.Remove(w.Chat)
	} else {
		w.initActions()
	}

	w.chatState = state
	w.ctx = gtkcord.InjectState(w.baseCtx, state)

	w.Chat = NewChatPage(w.ctx, w)
	w.Stack.AddChild(w.Chat)
}
