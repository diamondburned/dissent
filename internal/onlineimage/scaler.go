// SPDX-License-Identifier: MPL-2.0
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Ported from gotkit (github.com/diamondburned/gotkit) at commit
// d0c316dcdb4e, components/onlineimage/scaler.go.

package onlineimage

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
)

type pixbufScaler struct {
	parent *baseImage
	// parentSz keeps track of the parent widget's sizes in case it has been
	// changed, which would force us to invalidate all scaled pixbufs.
	parentSz [2]int
	// src is the source pixbuf.
	src *gdkpixbuf.Pixbuf
	// src1x is the source pixbuf at 1x scale.
	src1x *gdkpixbuf.Pixbuf
}

// SetFromPixbuf invalidates and sets the internal scaler's pixbuf. The
// SetFromPixbuf call might be bubbled up to the parent widget.
func (p *pixbufScaler) SetFromPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	p.src = pixbuf
	p.src1x = nil
	p.invalidate()
}

// Invalidate prompts the scaler to rescale.
func (p *pixbufScaler) Invalidate() {
	p.invalidate()
}

// ParentSize gets the cached parent widget's size request.
func (p *pixbufScaler) ParentSize() (w, h int) {
	return p.parentSz[0], p.parentSz[1]
}

func (p *pixbufScaler) init(parent *baseImage) {
	if parent.setter.SetFromPixbuf == nil {
		log.Panicf("pixbufScaler: baseImage %T missing SetFromPixbuf", parent.imageParent)
	}

	p.parent = parent

	base := gtk.BaseWidget(parent.parent)
	base.ConnectMap(func() {
		p.Invalidate()
	})
	base.NotifyProperty("scale-factor", func() {
		gtkutil.SetScaleFactor(parent.scale())
		p.Invalidate()
	})

	// Call Invalidate for 5 ticks, which seems to be enough to trick GTK into
	// giving us the correct scale factor. The actual fix would probably involve
	// connecting to various different signals, but this is good enough for now.
	var ticks int
	base.AddTickCallback(func(gtk.Widgetter, gdk.FrameClocker) bool {
		p.Invalidate()
		ticks++
		return ticks < 5 && p.parent.scale() != gtkutil.ScaleFactor()
	})
}

func (p *pixbufScaler) setParentPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	setter := p.parent.setter
	setter.SetFromPixbuf(pixbuf)
}

// invalidate invalidates the scaled pixbuf and optionally refetches one if
// needed. The user should use this method instead of calling on the parent
// widget's Refetch method.
func (p *pixbufScaler) invalidate() {
	if p.src == nil {
		return
	}

	scale := p.parent.scale()
	if scale == 0 {
		// No allocations yet.
		return
	}

	dstW, dstH := p.parent.sizeRequest()
	if dstW < 1 || dstH < 1 {
		// No exact size requested, so we can't really scale relatively to that
		// size. Use the original pixbuf.
		p.setParentPixbuf(p.src)
		return
	}

	if p.parentSz != [2]int{dstW, dstH} {
		// Size changed, so invalidate all known pixbufs.
		p.src1x = nil
		p.parentSz = [2]int{dstW, dstH}
	}

	// Scale the width and height up.
	dstW *= scale
	dstH *= scale

	srcW := p.src.Width()
	srcH := p.src.Height()

	if dstW >= srcW || dstH >= srcH {
		p.parent.setter.SetFromPixbuf(p.src)
		return
	}

	pixbuf := p.src
	if scale == 1 && dstW != srcW && dstH != srcH {
		if p.src1x == nil {
			// InterpTiles is apparently as good as bilinear when downscaling,
			// which is what we want.
			p.src1x = p.src.ScaleSimple(dstW, dstH, gdkpixbuf.InterpBilinear)
		}
		pixbuf = p.src1x
	}

	p.setParentPixbuf(pixbuf)
}
