package guilds

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

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

func (p *NamePopover) Bind(parent gtk.Widgetter) {
	if p.Parent() != nil {
		p.Unparent()
	}

	p.SetParent(parent)

	hoverer := gtk.NewEventControllerMotion()
	hoverer.ConnectEnter(func(x, y float64) { p.Popup() })
	hoverer.ConnectLeave(func() { p.Popdown() })

	w := gtk.BaseWidget(parent)
	w.AddController(hoverer)
}

// SetName sets the name to display in the popover.
func (p *NamePopover) SetName(name string) {
	p.Label.SetText(name)
}
