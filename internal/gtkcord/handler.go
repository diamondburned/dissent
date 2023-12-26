package gtkcord

import (
	"github.com/diamondburned/arikawa/v3/utils/handler"
	"github.com/diamondburned/gotk4/pkg/core/glib"
)

// MainThreadHandler wraps a [handler.Handler] to run all events on the main
// thread.
type MainThreadHandler struct {
	h *handler.Handler
}

// NewMainThreadHandler creates a new MainThreadHandler.
func NewMainThreadHandler(h *handler.Handler) *MainThreadHandler {
	hh := &MainThreadHandler{h: handler.New()}
	h.AddSyncHandler(func(ev any) {
		glib.IdleAddPriority(glib.PriorityDefault, func() {
			hh.h.Call(ev)
		})
	})
	return hh
}

// AddHandler adds a handler to the handler.Handler.
// The given handler will be called on the main thread.
// The returned function will remove the handler.
func (h *MainThreadHandler) AddHandler(handler any) func() {
	return h.h.AddSyncHandler(handler)
}

// AddSyncHandler is the same as [AddHandler].
// It exists for compatibility.
func (h *MainThreadHandler) AddSyncHandler(handler any) func() {
	return h.h.AddSyncHandler(handler)
}
