package hoverpopover

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// PopoverController  provides a way to open and close a popover while also
// reusing the widget if it's been open recently.
type PopoverController struct {
	parent      *gtk.Widget
	popover     *gtk.Popover
	initPopover func(*gtk.Popover) bool
	hideTimeout glib.SourceHandle
}

// NewPopoverController creates a new PopoverController.
func NewPopoverController(parent gtk.Widgetter, initFn func(*gtk.Popover) bool) *PopoverController {
	return &PopoverController{
		parent:      gtk.BaseWidget(parent),
		initPopover: initFn,
	}
}

// Popup pops up the popover.
func (p *PopoverController) Popup() *gtk.Popover {
	if p.popover != nil {
		p.popover.SetCSSClasses(nil)
		if !p.initPopover(p.popover) {
			return nil
		}

		if p.hideTimeout != 0 {
			glib.SourceRemove(p.hideTimeout)
			p.hideTimeout = 0
		}

		p.popover.Popup()
		return p.popover
	}

	p.popover = gtk.NewPopover()
	p.popover.SetCSSClasses(nil)

	if !p.initPopover(p.popover) {
		p.popover = nil
		return nil
	}

	p.popover.SetParent(p.parent)
	p.popover.Popup()
	return p.popover
}

// Popdown pops down the popover.
func (p *PopoverController) Popdown() {
	if p.popover == nil {
		return
	}

	p.popover.Popdown()

	if p.hideTimeout != 0 {
		return
	}

	p.hideTimeout = glib.TimeoutSecondsAdd(3, func() {
		p.popover.Unparent()
		p.popover = nil
		p.hideTimeout = 0
	})
}

// IsPoppedUp returns whether the popover is popped up.
func (p *PopoverController) IsPoppedUp() bool {
	return p.popover != nil && p.popover.IsVisible()
}
