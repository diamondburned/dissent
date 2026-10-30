package window

import (
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/session"
)

// updateSession switches the window's page to match the session's status.
func (w *Window) updateSession() {
	switch w.session.Status() {
	case session.LoggedOut:
		w.SwitchToLoginPage()
		if err := w.session.Err(); err != nil {
			w.Login.Login.ShowError(err)
		}

	case session.Connecting:
		w.Stack.SetVisibleChild(w.Loading)
		w.SetTitle("Connecting")

	case session.Connected:
		if w.Chat == nil || w.chatState != w.session.State() {
			w.initChatPage(w.session.State())
		}
		w.SwitchToChatPage()
	}
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
