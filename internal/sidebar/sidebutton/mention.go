package sidebutton

import (
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// MentionsIndicator is a small indicator that shows the mention count.
type MentionsIndicator struct {
	*gtk.Revealer
	Label *gtk.Label

	count  int
	reveal bool
}

var mentionCSS = cssutil.Applier("sidebar-mention", `
	.sidebar-mention {
		background: none;
	}
	.sidebar-mention.sidebar-mention-active,
	.sidebar-mention.sidebar-mention-active label {
		border-radius: 100px;
		background-color: @theme_bg_color;
	}
	.sidebar-mention.sidebar-mention-active label {
		color: white;
		background-color: @mentioned;
		min-width:  12pt;
		min-height: 12pt;
		padding: 0;
		margin: 2px;
		font-size: 8pt;
		font-weight: bold;
	}
`)

// NewMentionsIndicator creates a new mention indicator.
func NewMentionsIndicator() *MentionsIndicator {
	m := &MentionsIndicator{
		Revealer: gtk.NewRevealer(),
		Label:    gtk.NewLabel(""),
		reveal:   true,
	}

	m.SetChild(m.Label)
	m.SetHAlign(gtk.AlignEnd)
	m.SetVAlign(gtk.AlignEnd)
	m.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	m.SetTransitionDuration(100)

	m.update()
	mentionCSS(m)
	return m
}

// SetCount sets the mention count.
func (m *MentionsIndicator) SetCount(count int) {
	if count == m.count {
		return
	}

	m.count = count
	m.update()
}

// Count returns the mention count.
func (m *MentionsIndicator) Count() int {
	return m.count
}

// SetRevealChild sets whether the indicator should be revealed.
// This lets the user hide the indicator even if there are mentions.
func (m *MentionsIndicator) SetRevealChild(reveal bool) {
	m.reveal = reveal
	m.update()
}

func (m *MentionsIndicator) update() {
	if m.count == 0 {
		m.RemoveCSSClass("sidebar-mention-active")
		m.Revealer.SetRevealChild(false)
		return
	}

	m.AddCSSClass("sidebar-mention-active")
	m.Label.SetText(strconv.Itoa(m.count))
	m.Revealer.SetRevealChild(m.reveal && m.count > 0)
}
