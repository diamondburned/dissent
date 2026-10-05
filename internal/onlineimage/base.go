// SPDX-License-Identifier: MPL-2.0
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Ported from gotkit (github.com/diamondburned/gotkit) at commit
// d0c316dcdb4e, components/onlineimage/base.go.

package onlineimage

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// MaxFPS is the maximum FPS to play an animation (often a GIF) at. In reality,
// the actual frame rate heavily depends on the draw clock of GTK, but this
// duration determines the background ticker.
//
// For more information, see
// https://wunkolo.github.io/post/2020/02/buttery-smooth-10fps/.
const MaxFPS = 50

const maxFPSDelay = 1000 / MaxFPS

type imageParent struct {
	parent gtk.Widgetter
	image  gtk.Widgetter
	setter imgutil.ImageSetter
}

func (p imageParent) sizeRequest() (w, h int) {
	base := gtk.BaseWidget(p.image)
	return base.SizeRequest()
}

func (p imageParent) sizeAllocated() (w, h int) {
	base := gtk.BaseWidget(p.image)
	alloc := base.Allocation()
	w = alloc.Width()
	h = alloc.Height()
	return
}

func (p imageParent) scale() int {
	base := gtk.BaseWidget(p.image)
	return base.ScaleFactor()
}

type baseImage struct {
	imageParent
	prov imgutil.Provider

	scaler    pixbufScaler
	animation *animation

	ctx gtkutil.Cancellable
	url string
	off bool
	ok  bool
}

type animation struct {
	pixbuf    *gdkpixbuf.PixbufAnimation
	animating glib.SourceHandle
	paused    bool
}

// init initializes the base image with the given parent widget and provider.
func (b *baseImage) init(ctx context.Context, parent imageParent, p imgutil.Provider) {
	b.imageParent = parent
	b.prov = p
	b.scaler.init(b)

	b.ctx = gtkutil.WithVisibility(ctx, parent.parent)
	b.ctx.OnRenew(func(ctx context.Context) func() {
		b.scaler.Invalidate()
		b.fetch(ctx)
		return nil
	})
}

func (b *baseImage) SetFromURL(url string) {
	if b.url == url {
		return
	}

	b.url = url
	b.off = false
	b.refetch()
}

func (b *baseImage) Disable() {
	b.url = ""
	b.off = true
	b.scaler.SetFromPixbuf(nil)
}

func (b *baseImage) refetch() {
	b.ok = false
	b.fetch(b.ctx.Take())
}

func (b *baseImage) fetch(ctx context.Context) {
	if b.off || b.ok || ctx.Err() != nil {
		return
	}

	url := b.url
	if url == "" {
		b.setter.SetFromPixbuf(nil)
		b.scaler.SetFromPixbuf(nil)
		return
	}

	imgutil.DoProviderURL(ctx, b.prov, url, imgutil.ImageSetter{
		SetFromPixbuf: func(p *gdkpixbuf.Pixbuf) {
			if b.url != url {
				return
			}

			b.ok = true
			b.scaler.SetFromPixbuf(p)

			if b.animation != nil {
				b.animation.pixbuf = nil
			}
		},
		SetFromAnimation: func(anim *gdkpixbuf.PixbufAnimation) {
			if b.url != url {
				return
			}

			b.ok = true
			b.scaler.SetFromPixbuf(anim.StaticImage())

			if b.animation != nil {
				b.animation.pixbuf = anim
			}
		},
	})
}

func (b *baseImage) enableAnimation() *AnimationController {
	if !CanAnimate {
		return (*AnimationController)(b)
	}

	b.animation = &animation{}

	setPause := func(pause bool) {
		if pause {
			b.stopAnimation()
		}

		b.animation.paused = pause
	}

	base := gtk.BaseWidget(b.parent)
	base.ConnectMap(func() { setPause(false) })
	base.ConnectUnmap(func() { setPause(true) })

	var bindRoot func()
	var unbindRoot func()

	bindRoot = func() {
		if unbindRoot != nil {
			unbindRoot()
			unbindRoot = nil
		}

		w, ok := rootWindow(gtk.BaseWidget(b.parent).Root())
		if ok {
			s := w.NotifyProperty("is-active", func() {
				// Pause animation on window unfocus.
				setPause(!w.IsActive())
			})
			unbindRoot = func() { w.HandlerDisconnect(s) }
		}
	}

	b.parent.NotifyProperty("root", bindRoot)
	bindRoot()

	return (*AnimationController)(b)
}

func rootWindow(w *gtk.Root) (*gtk.Window, bool) {
	if w == nil {
		return nil, false
	}

	obj := coreglib.InternObject(w)
	win := obj.WalkCast(func(obj glib.Objector) bool {
		_, isWindow := obj.(*gtk.Window)
		return isWindow
	})
	if win == nil {
		return nil, false
	}

	return win.(*gtk.Window), true
}

func (b *baseImage) startAnimation() {
	if b.animation == nil ||
		b.animation.paused ||
		b.animation.pixbuf == nil ||
		b.animation.animating != 0 {
		return
	}

	iter := b.animation.pixbuf.Iter(nil)
	setter := b.imageParent.setter

	scale := b.scale()
	w, h := b.sizeAllocated()
	w *= scale
	h *= scale

	useIter := func(iter *gdkpixbuf.PixbufAnimationIter) {
		setter.SetFromPixbuf(iter.Pixbuf())
	}
	// Kickstart the animation.
	useIter(iter)

	var scheduleNext func()
	scheduleNext = func() {
		if delay := animDelay(iter); delay != -1 {
			// Schedule next frame.
			b.animation.animating = glib.TimeoutAddPriority(uint(delay), glib.PriorityLow, func() {
				if iter.Advance(nil) {
					useIter(iter)
				}
				scheduleNext()
			})
		} else {
			// End of animation.
			b.stopAnimation()
		}
	}
	// Schedule the next frame.
	scheduleNext()
}

func (b *baseImage) stopAnimation() {
	if b.animation == nil || b.animation.animating == 0 {
		return
	}

	if b.animation.animating != 0 {
		glib.SourceRemove(b.animation.animating)
		b.animation.animating = 0
	}

	b.finishStopAnimation()
}

func (b *baseImage) finishStopAnimation() {
	if b.animation.pixbuf != nil {
		iter := b.animation.pixbuf.Iter(nil)
		b.scaler.SetFromPixbuf(iter.Pixbuf())
	} else {
		b.scaler.Invalidate()
	}
}

func animDelay(iter *gdkpixbuf.PixbufAnimationIter) int {
	delayMs := iter.DelayTime()
	if delayMs == -1 {
		return -1
	}

	if delayMs < maxFPSDelay {
		delayMs = maxFPSDelay
	}

	return delayMs
}
