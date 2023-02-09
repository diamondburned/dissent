package sidebutton

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/ningen/v3"
)

type PillState uint8

// Pill state enums.
const (
	PillInactive PillState = iota
	PillDisabled
	PillActive
	PillOpened
)

// CSSClass returns the CSS class for the current state.
func (s PillState) CSSClass() string {
	switch s {
	case PillInactive, PillDisabled:
		return ""
	case PillActive:
		return "guilds-pill-active"
	case PillOpened:
		return "guilds-pill-opened"
	default:
		return ""
	}
}

type PillAttributes uint8

// Additional pill attributes.
const (
	PillUnread PillAttributes = 1 << iota
	PillMentioned
)

// PillAttrsFromUnread creates a new PillAttributes reflecting the correct
// unread indication state.
func PillAttrsFromUnread(state ningen.UnreadIndication) PillAttributes {
	var attrs PillAttributes

	switch state {
	case ningen.ChannelUnread:
		attrs |= PillUnread
	case ningen.ChannelMentioned:
		attrs |= PillUnread | PillMentioned
	}

	return attrs
}

// CSSClasses returns the CSS classes that corresponds to the current state of
// the pill.
func (a PillAttributes) CSSClasses() []string {
	var classes []string
	if a.Has(PillUnread) {
		classes = append(classes, "guilds-pill-unread")
	}
	if a.Has(PillMentioned) {
		classes = append(classes, "guilds-pill-mentioned")
	}
	return classes
}

// Has returns true if a has these in its bits.
func (a PillAttributes) Has(these PillAttributes) bool {
	return a&these == these
}

type Pill struct {
	*gtk.Box // width IconPadding

	State PillState
	Attrs PillAttributes

	oldState PillState
	oldAttrs PillAttributes
}

var stripCSS = cssutil.Applier("guilds-pill", `
	@define-color mentioned rgb(240, 71, 71);

	.guilds-pill {
		padding: 0;
		transition: 100ms linear;
		border-radius: 0 99px 99px 0;
		background-color: @theme_fg_color;
	}
	.guilds-pill.guilds-pill-active {
		padding: 20px 3px;
	}
	.guilds-pill.guilds-pill-unread:not(.guilds-pill-opened):not(.guilds-pill-active) {
		padding: 6px 3px;
	}
	.guilds-pill.guilds-pill-mentioned {
		background-color: @mentioned;
	}
`)

func NewPill() *Pill {
	p := Pill{}
	p.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	p.Box.SetVExpand(true)
	p.Box.SetVAlign(gtk.AlignCenter)
	p.Box.SetHAlign(gtk.AlignStart)

	stripCSS(p)
	return &p
}

// Invalidate reads the 2 fields inside Pill and updates the widget accordingly.
func (r *Pill) Invalidate() {
	if r.State != r.oldState {
		if class := r.oldState.CSSClass(); class != "" {
			r.RemoveCSSClass(r.oldState.CSSClass())
		}

		r.oldState = r.State

		if class := r.oldState.CSSClass(); class != "" {
			r.AddCSSClass(class)
		}
	}

	if r.Attrs != r.oldAttrs {
		for _, class := range r.oldAttrs.CSSClasses() {
			r.RemoveCSSClass(class)
		}

		r.oldAttrs = r.Attrs

		for _, class := range r.oldAttrs.CSSClasses() {
			r.AddCSSClass(class)
		}
	}
}
