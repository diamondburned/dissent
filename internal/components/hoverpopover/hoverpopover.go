package hoverpopover

import (
	"log"

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
	parent      *gtk.Widget
	hover       *gtk.EventControllerMotion
	current     *MarkupHoverPopoverWidget // nil if not shown
	initFn      func(*MarkupHoverPopoverWidget)
	hideTimeout glib.SourceHandle
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
		parent: gtk.BaseWidget(parent),
		initFn: initFn,
	}

	// TODO: this has potential issues when we unfocus a window.
	// Consider additional checks.
	p.hover = gtk.NewEventControllerMotion()
	p.hover.ConnectEnter(func(_, _ float64) { p.showPopover() })
	p.hover.ConnectLeave(func() { p.hidePopover() })

	p.parent.AddController(p.hover)

	var windowSignal glib.SignalHandle
	onMap := func() {
		window := p.parent.Root().CastType(gtk.GTypeWindow).(*gtk.Window)
		windowSignal = window.NotifyProperty("is-active", func() {
			if p.current != nil && !window.IsActive() {
				p.hidePopover()
			}
		})
	}
	p.parent.ConnectMap(onMap)
	p.parent.ConnectUnmap(func() {
		p.parent.HandlerDisconnect(windowSignal)
		windowSignal = 0
	})
	if p.parent.Mapped() {
		onMap()
	}

	return p
}

func (p *MarkupHoverPopover) showPopover() {
	if p.current != nil {
		// If we already have a background hide timeout, remove it.
		if p.hideTimeout != 0 {
			glib.SourceRemove(p.hideTimeout)
			p.hideTimeout = 0
		}

		p.current.Popup()
		return
	}

	p.current = &MarkupHoverPopoverWidget{}

	p.current.Label = gtk.NewLabel("")
	p.current.Label.AddCSSClass("hover-popover-label")

	p.current.Popover = gtk.NewPopover()
	p.current.AddCSSClass("hover-popover")
	p.current.AddCSSClass("hover-popover-markup")
	p.current.SetParent(p.parent)
	p.current.SetAutohide(false)
	p.current.SetMnemonicsVisible(false)
	p.current.SetCanFocus(false)
	p.current.SetCanTarget(false)
	p.current.SetChild(p.current.Label)

	p.initFn(p.current)
	p.current.Popup()
}

func (p *MarkupHoverPopover) hidePopover() {
	if p.current == nil {
		log.Println("hidePopover: current is nil when leaving hover")
		return
	}

	p.current.Popover.Popdown()

	if p.hideTimeout != 0 {
		// We've already set a timeout, so don't set another one.
		return
	}

	p.hideTimeout = glib.TimeoutSecondsAddPriority(3, glib.PriorityLow, func() {
		p.current.Popover.Unparent()
		p.current = nil
		p.hideTimeout = 0
	})
}
