package guilds

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// SharedNamePopover is a single NamePopover shared over a group of widgets. At
// most one NamePopover is visible at a time.
type SharedNamePopover struct {
	namePopover *NamePopover
	lastWidget  gtk.Widgetter
}

// MotionGrouped describes a widget that shares a motion group.
type MotionGrouped interface {
	gtk.Widgetter
	MotionGroup() *MotionGroup
}

// MotionGroup is a group of widgets that have mutually exclusive Motion event
// controllers.
type MotionGroup struct {
	current *eventControllerMotionGroup
}

type eventControllerMotionGroup struct {
	motion *gtk.EventControllerMotion
	left   bool
}

// ConnectEventControllerMotion is a convenient function.
func (m *MotionGroup) ConnectEventControllerMotion(w gtk.Widgetter, enter, leave func()) {
	eventer := m.NewEventControllerMotion()
	eventer.ConnectEnter(func(_, _ float64) { enter() })
	eventer.ConnectLeave(leave)

	base := gtk.BaseWidget(w)
	base.AddController(eventer)
}

// NewEventControllerMotion creates a new EventControllerMotion that belongs to
// the mutually-exclusive MotionGroup.
func (m *MotionGroup) NewEventControllerMotion() *gtk.EventControllerMotion {
	eventer := &eventControllerMotionGroup{
		motion: gtk.NewEventControllerMotion(),
	}

	eventer.motion.ConnectEnter(func(x, y float64) {
		if m.current != nil && !m.current.left {
			log.Println("emitting leave...")
			m.current.motion.Emit("leave")
		}
		m.current = eventer
	})

	eventer.motion.ConnectLeave(func() {
		eventer.left = true
	})

	return eventer.motion
}

// NamePopover is a popover that shows the guild name on hover.
type NamePopover struct {
	*gtk.Popover
	Label *gtk.Label
}

var namePopoverCSS = cssutil.Applier("guilds-namepopover", `
	.guilds-namepopover label {
		font-weight: bold;
	}
`)

// NewNamePopover creates a new NamePopover.
func NewNamePopover() *NamePopover {
	p := NamePopover{}
	p.Label = gtk.NewLabel("")
	p.Label.SetJustify(gtk.JustifyLeft)
	p.Label.SetWrap(true)
	p.Label.SetWrapMode(pango.WrapWordChar)
	p.Label.SetMaxWidthChars(100)

	p.Popover = gtk.NewPopover()
	p.Popover.SetCanTarget(false)
	p.Popover.SetAutohide(false)
	p.Popover.SetChild(p.Label)
	p.Popover.SetPosition(gtk.PosRight)
	namePopoverCSS(p)

	return &p
}

func (p *NamePopover) SetParent(parent gtk.Widgetter) {
	if p.Parent() != nil {
		p.Unparent()
	}
	p.Popover.SetParent(parent)
}

// SetName sets the name to display in the popover.
func (p *NamePopover) SetName(name string) {
	p.Label.SetText(name)
}
