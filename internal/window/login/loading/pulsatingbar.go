package loading

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type PulsatingFlags uint8

const (
	// PulseFast pulses at double speed.
	PulseFast PulsatingFlags = 1 << iota
	// PulseBarOSD adds the .osd class.
	PulseBarOSD
)

func (f PulsatingFlags) has(these PulsatingFlags) bool {
	return f&these == these
}

// PulsatingBar is a bar that moves back and forth asynchronously for as long as
// it is visible (mapped).
type PulsatingBar struct {
	*gtk.Box
	Bar *gtk.ProgressBar
}

const pulsateRate = 1000 / 30 // 30Hz update

var pulsatingBarCSS = cssutil.Applier("loading-pulsatingbar", `
	.loading-pulsatingbar {
		opacity: 0;
		transition: all 0.15s ease-in-out;
	}
	.loading-pulsatingbar.loading {
		opacity: 1;
	}
	.loading-pulsatingbar,
	.loading-pulsatingbar progerssbar trough {
		min-height: 4px;
	}
	.loading-pulsatingbar progressbar.osd {
		margin:  0;
		padding: 0;
	}
`)

// NewPulsatingBar creates a new PulsatingBar.
func NewPulsatingBar(flags PulsatingFlags) *PulsatingBar {
	p := PulsatingBar{}
	p.Bar = gtk.NewProgressBar()
	p.Bar.Hide()

	p.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	p.Box.Append(p.Bar)
	pulsatingBarCSS(p)

	if flags.has(PulseFast) {
		p.Bar.SetPulseStep(0.1)
	} else {
		p.Bar.SetPulseStep(0.05)
	}

	if flags.has(PulseBarOSD) {
		p.Box.AddCSSClass("osd")
		p.Bar.AddCSSClass("osd")
	}

	var source glib.SourceHandle
	p.Bar.ConnectShow(func() {
		p.Bar.SetFraction(0)
		source = glib.TimeoutAdd(pulsateRate, func() bool {
			p.Bar.Pulse()
			return true
		})
	})
	p.Bar.ConnectHide(func() {
		if source > 0 {
			glib.SourceRemove(source)
			source = 0
		}
	})

	return &p
}

func (p *PulsatingBar) Show() {
	p.Bar.Show()
	p.AddCSSClass("loading")
}

func (p *PulsatingBar) Hide() {
	p.Bar.Hide()
	p.RemoveCSSClass("loading")
}
