package hoverpopover

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// PopoverButton extends a ToggleButton to show a popover when toggled.
type PopoverButton struct {
	*gtk.ToggleButton
	controller *PopoverController
}

// NewPopoverButton creates a new PopoverButton.
func NewPopoverButton(initFn func(*gtk.Popover) bool) *PopoverButton {
	b := &PopoverButton{ToggleButton: gtk.NewToggleButton()}
	controller := NewPopoverController(b.ToggleButton, initFn)
	b.ConnectClicked(func() {
		if !b.Active() {
			controller.Popdown()
			return
		}

		popover := controller.Popup()

		var closedSignal glib.SignalHandle
		closedSignal = popover.ConnectClosed(func() {
			b.SetActive(false)
			if closedSignal != 0 {
				popover.HandlerDisconnect(closedSignal)
				closedSignal = 0
			}
		})
	})
	return b
}
