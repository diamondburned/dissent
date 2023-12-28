package gtkcord

import (
	"reflect"
	"sync"

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
	m := &MainThreadHandler{
		h: handler.New(),
	}
	p := &sync.Pool{
		New: func() any { return make([]handler.Caller, 0, 10) },
	}
	h.AddSyncHandler(func(ev any) {
		callers := p.Get().([]handler.Caller)

		all := m.h.AllCallersForType(reflect.TypeOf(ev))
		all(func(c handler.Caller) bool {
			callers = append(callers, c)
			return true
		})

		if len(callers) == 0 {
			p.Put(callers)
			return
		}

		v := reflect.ValueOf(ev)
		glib.IdleAddPriority(glib.PriorityHighIdle, func() {
			for _, c := range callers {
				c.Call(v)
			}

			// Return the callers to the pool.
			for i := range callers {
				callers[i] = nil // avoid memory leaks
			}
			p.Put(callers[:0])
		})
	})
	return m
}

// AddHandler adds a handler to the handler.Handler.
// The given handler will be called on the main thread.
// The returned function will remove the handler.
func (m *MainThreadHandler) AddHandler(handler any) func() {
	detach := m.h.AddSyncHandler(handler)
	return func() {
		detach()
	}
}

// AddSyncHandler is the same as [AddHandler].
// It exists for compatibility.
func (m *MainThreadHandler) AddSyncHandler(handler any) func() {
	return m.AddHandler(handler)
}
