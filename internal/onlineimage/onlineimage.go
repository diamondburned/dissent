// SPDX-License-Identifier: MPL-2.0
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Ported from gotkit (github.com/diamondburned/gotkit) at commit
// d0c316dcdb4e, components/onlineimage/onlineimage.go.

// Package onlineimage contains image widgets that can fetch from image
// providers, usually online ones. It provides lazy HiDPI scaling by
// automatically reloading images when the scale factor changes.
package onlineimage

import (
	"context"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

// CanAnimate is true by default, which allows EnableAnimation to be called on
// certain images to allow animations to be played back.
var CanAnimate = true

type AnimationController baseImage

// Start starts the animation playback in the background. The animation isn't
// stopped until it is either unmapped or Stop is called.
func (c *AnimationController) Start() {
	(*baseImage)(c).startAnimation()
}

// Stop stops the animation playback.
func (c *AnimationController) Stop() {
	(*baseImage)(c).stopAnimation()
}

// OnHover binds the controller to a motion controller attached to the image
// widget. When the user hovers over the image, the animation plays.
func (c *AnimationController) OnHover() {
	c.ConnectMotion(c.parent)
}

// ConnectMotion connects a motion controller to the given widget that will
// activate the animation when it's hovered over (entered).
func (c *AnimationController) ConnectMotion(w gtk.Widgetter) {
	motion := gtk.NewEventControllerMotion()
	motion.ConnectEnter(func(x, y float64) { c.Start() })
	motion.ConnectLeave(func() { c.Stop() })

	parent := gtk.BaseWidget(c.parent)
	parent.ConnectMap(func() {
		base := gtk.BaseWidget(w)
		base.AddController(motion)
	})
	parent.ConnectUnmap(func() {
		base := gtk.BaseWidget(w)
		base.RemoveController(motion)
	})
}

// Avatar is an online variant of adaptive.Avatar.
type Avatar struct {
	*adw.Avatar
	base baseImage
}

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, p imgutil.Provider, size int) *Avatar {
	a := Avatar{
		Avatar: adw.NewAvatar(size, "", true),
	}
	a.AddCSSClass("onlineimage")
	a.base.init(ctx, imageParent{&a, a.Avatar, a.set()}, p)

	return &a
}

// CreateAvatarFromObj wraps an existing adw.Avatar object, usually one loaded
// from a UI file, into an online Avatar. The given size is applied to the
// avatar, mirroring NewAvatar.
func CreateAvatarFromObj(ctx context.Context, obj *adw.Avatar, p imgutil.Provider, size int) *Avatar {
	obj.SetSize(size)

	a := Avatar{Avatar: obj}
	a.AddCSSClass("onlineimage")
	a.base.init(ctx, imageParent{&a, a.Avatar, a.set()}, p)

	return &a
}

// SetFromURL sets the Avatar's URL.
func (a *Avatar) SetFromURL(url string) {
	a.base.SetFromURL(url)
}

// Disable disables the online capability of the avatar, and sets the avatar to
// the default avatar.
func (a *Avatar) Disable() {
	a.base.Disable()
}

// SetSizeRequest sets the avatar size.
func (a *Avatar) SetSizeRequest(size int) {
	a.Avatar.SetSizeRequest(size, size)
	a.base.scaler.Invalidate()
}

// EnableAnimation enables animation for the avatar. The controller is returned
// for the user to determine when to play the animation.
func (a *Avatar) EnableAnimation() *AnimationController {
	return a.base.enableAnimation()
}

func (a *Avatar) set() imgutil.ImageSetter {
	return imgutil.ImageSetter{
		SetFromPixbuf: func(pb *gdkpixbuf.Pixbuf) {
			if pb != nil {
				texture := gdk.NewTextureForPixbuf(pb)
				a.Avatar.SetCustomImage(texture)
			} else {
				a.Avatar.SetCustomImage(nil)
			}
		},
		SetFromPaintable: a.Avatar.SetCustomImage,
	}
}

// Image is an online variant of gtk.Image.
type Image struct {
	*gtk.Image
	base baseImage
}

// NewImage creates a new online image.
func NewImage(ctx context.Context, p imgutil.Provider) *Image {
	i := Image{Image: gtk.NewImage()}
	i.AddCSSClass("onlineimage")
	i.base.init(ctx, imageParent{&i, &i, i.set()}, p)

	return &i
}

// SetFromURL sets the Image's URL.
func (i *Image) SetFromURL(url string) {
	i.base.SetFromURL(url)
}

// Disable disables the online capability of the image, turning it into a
// normal gtk.Image. To re-enable it, call SetURL again.
func (i *Image) Disable() {
	i.base.Disable()
}

// SetSizeRequest sets the minimum size of a widget.
func (i *Image) SetSizeRequest(w, h int) {
	i.Image.SetSizeRequest(w, h)
	i.base.scaler.Invalidate()
}

// EnableAnimation enables animation for the avatar. The controller is returned
// for the user to determine when to play the animation.
func (i *Image) EnableAnimation() *AnimationController {
	return i.base.enableAnimation()
}

func (i *Image) set() imgutil.ImageSetter {
	return imgutil.ImageSetter{
		SetFromPixbuf:    i.SetFromPixbuf,
		SetFromPaintable: i.SetFromPaintable,
	}
}

// Picture is an online variant of gtk.Picture.
type Picture struct {
	*gtk.Picture
	base baseImage
}

// NewPicture creates a new Picture.
func NewPicture(ctx context.Context, prov imgutil.Provider) *Picture {
	p := Picture{Picture: gtk.NewPicture()}
	p.AddCSSClass("onlineimage")
	p.base.init(ctx, imageParent{&p, &p, p.set()}, prov)

	return &p
}

// SetURL sets the Avatar's URL.
func (p *Picture) SetURL(url string) {
	p.base.SetFromURL(url)
}

// Disable disables the online capability of the picture, turning it into a
// normal gtk.Picture. To re-enable it, call SetURL again.
func (p *Picture) Disable() {
	p.base.Disable()
}

// SetSizeRequest sets the minimum size of a widget.
func (p *Picture) SetSizeRequest(w, h int) {
	p.Picture.SetSizeRequest(w, h)
	p.base.scaler.Invalidate()
}

// EnableAnimation enables animation for the avatar. The controller is returned
// for the user to determine when to play the animation.
func (p *Picture) EnableAnimation() *AnimationController {
	return p.base.enableAnimation()
}

func (p *Picture) set() imgutil.ImageSetter {
	return imgutil.ImageSetter{
		SetFromPixbuf:    p.SetPixbuf,
		SetFromPaintable: p.SetPaintable,
	}
}
