package hoverpopover

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

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
	initFn     func(*MarkupHoverPopoverWidget)
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
func NewMarkupHoverPopover(parent gtk.Widgetter, initFn func(*MarkupHoverPopoverWidget)) *MarkupHoverPopover {
	p := &MarkupHoverPopover{
		initFn: initFn,
	}
	p.controller = NewPopoverController(parent, p.initPopover)

	p.hover = gtk.NewEventControllerMotion()
	p.hover.ConnectEnter(func(_, _ float64) { p.controller.Popup() })
	p.hover.ConnectLeave(func() { p.controller.Popdown() })

	parentWidget := gtk.BaseWidget(parent)
	parentWidget.AddController(p.hover)

	var windowSignal glib.SignalHandle
	onMap := func() {
		window := parentWidget.Root().CastType(gtk.GTypeWindow).(*gtk.Window)
		windowSignal = window.NotifyProperty("is-active", func() {
			if p.controller.IsPoppedUp() && !window.IsActive() {
				p.controller.Popdown()
			}
		})
	}
	parentWidget.ConnectMap(onMap)
	parentWidget.ConnectUnmap(func() {
		parentWidget.HandlerDisconnect(windowSignal)
		windowSignal = 0
	})
	if parentWidget.Mapped() {
		onMap()
	}

	return p
}

func (p *MarkupHoverPopover) initPopover(popover *gtk.Popover) {
	current := &MarkupHoverPopoverWidget{Popover: popover}

	current.Label = gtk.NewLabel("")
	current.Label.AddCSSClass("hover-popover-label")

	current.AddCSSClass("hover-popover")
	current.AddCSSClass("hover-popover-markup")
	current.SetAutohide(false)
	current.SetMnemonicsVisible(false)
	current.SetCanFocus(false)
	current.SetCanTarget(false)
	current.SetChild(current.Label)

	p.initFn(current)
}
