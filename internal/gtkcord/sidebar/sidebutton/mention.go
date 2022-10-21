package sidebutton

import (
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// MentionsIndicator is a small indicator that shows the mention count.
type MentionsIndicator struct {
	*gtk.Box
	Reveal *gtk.Revealer
	Label  *gtk.Label
}

var mentionCSS = cssutil.Applier("sidebar-mention", `
	.sidebar-mention,
	.sidebar-mention > label {
		border-radius: 50%;
	}

	.sidebar-mention {
		background-color: @sidebar_bg;
	}

	.sidebar-mention > label {
		background-color: @mention;
		color: white;
		min-width:  12px;
		min-height: 12px;
		font-size:  10px;
	}
`)

// NewMentionsIndicator creates a new mention indicator.
func NewMentionsIndicator() *MentionsIndicator {
	m := &MentionsIndicator{
		Box:    gtk.NewBox(gtk.OrientationHorizontal, 0),
		Reveal: gtk.NewRevealer(),
		Label:  gtk.NewLabel(""),
	}

	m.Box.Append(m.Reveal)
	m.Reveal.SetChild(m.Label)

	m.SetHAlign(gtk.AlignEnd)
	m.SetVAlign(gtk.AlignEnd)

	m.Reveal.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	m.Reveal.SetTransitionDuration(100)
	m.Reveal.SetRevealChild(false)

	mentionCSS(m.Box)
	return m
}

// SetCount sets the mention count.
func (m *MentionsIndicator) SetCount(count int) {
	if count == 0 {
		m.Reveal.SetRevealChild(false)
		return
	}

	m.Reveal.SetRevealChild(true)
	m.Label.SetText(strconv.Itoa(count))
}
