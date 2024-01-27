package hoverpopover

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var _ = cssutil.WriteCSS(`
	.popover-label {
		padding: 0 0.25em;
	}
`)

// MarkupHoverPopover is a struct that represents a hover popover
// that is displayed when the user hovers over a widget. The popover
// only displays markup text. Markup texts are persisted across
// hovers.
//
// Popovers are lazily allocated, so they are only created when
// the user hovers over a widget. They are automatically destroyed
// a moment after the user stops hovering over the widget.
type MarkupHoverPopover struct {
	hover      *gtk.EventControllerMotion
	controller *PopoverController
	initFn     func(*MarkupHoverPopoverWidget) bool
}

// MarkupHoverPopoverWidget is a struct that represents a popover
// widget that displays markup text.
type MarkupHoverPopoverWidget struct {
	*gtk.Popover
	Label *gtk.Label
}

var markupHoverPopoverClasses = []string{
	"hover-popover",
	"hover-popover-markup",
}

// NewMarkupHoverPopover creates a new MarkupHoverPopover.
func NewMarkupHoverPopover(parent gtk.Widgetter, initFn func(*MarkupHoverPopoverWidget) bool) *MarkupHoverPopover {
	p := &MarkupHoverPopover{
		initFn: initFn,
	}
	p.controller = NewPopoverController(parent, p.initPopover)

	p.hover = gtk.NewEventControllerMotion()

	// Implement a very primitive delay. This is to prevent the popover
	// from popping up when the user is scrolling on a touchpad.
	var delayID glib.SourceHandle
	var hovered bool
	const hoverDelay = 100 // ms

	p.hover.ConnectEnter(func(_, _ float64) {
		hovered = true

		if delayID != 0 {
			return
		}

		delayID = glib.TimeoutAdd(hoverDelay, func() {
			delayID = 0

			if hovered {
				p.controller.Popup()
			}
		})
	})

	p.hover.ConnectLeave(func() {
		hovered = false

		if delayID != 0 {
			glib.SourceRemove(delayID)
			delayID = 0
			return
		}

		p.controller.Popdown()
	})

	parentWidget := gtk.BaseWidget(parent)
	parentWidget.AddController(p.hover)

	var windowSignal glib.SignalHandle
	var window *gtk.Window

	attachWindow := func() {
		if window != nil {
			window.HandlerDisconnect(windowSignal)
			window = nil
		}

		root := parentWidget.Root()
		if root != nil {
			window = root.CastType(gtk.GTypeWindow).(*gtk.Window)
			windowSignal = window.NotifyProperty("is-active", func() {
				if p.controller.IsPoppedUp() && !window.IsActive() {
					p.controller.Popdown()
				}
			})
		}
	}

	parentWidget.NotifyProperty("root", attachWindow)
	attachWindow()

	return p
}

func (p *MarkupHoverPopover) initPopover(popover *gtk.Popover) bool {
	current := &MarkupHoverPopoverWidget{Popover: popover}

	current.Label = gtk.NewLabel("")
	current.Label.AddCSSClass("popover-label")
	current.Label.AddCSSClass("hover-popover-label")

	current.AddCSSClass("hover-popover")
	current.AddCSSClass("hover-popover-markup")
	current.SetAutohide(false)
	current.SetMnemonicsVisible(false)
	current.SetCanFocus(false)
	current.SetCanTarget(false)
	current.SetChild(current.Label)

	return p.initFn(current)
}
