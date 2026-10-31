package mediaviewer

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	maxZoom  = 16.0
	zoomStep = 1.25
)

// zoomView shows a picture that can be zoomed into and panned around. The
// picture fits into the view until it is zoomed in.
type zoomView struct {
	*gtk.ScrolledWindow
	picture *gtk.Picture
	// sizer gives the frame its size, since the picture's natural size is
	// always its intrinsic size.
	sizer *gtk.Picture
	frame *gtk.Overlay

	// zoom is the size of an image pixel in logical pixels, or 0 if the image
	// fits into the view.
	zoom float64
	// pointer is the last position of the pointer in the view.
	pointer [2]float64
}

func newZoomView(picture *gtk.Picture) *zoomView {
	z := &zoomView{
		picture: picture,
		sizer:   gtk.NewPicture(),
		frame:   gtk.NewOverlay(),
	}

	z.picture.SetCanShrink(true)
	z.picture.SetContentFit(gtk.ContentFitContain)
	z.picture.Paintable().ConnectInvalidateSize(z.fit)
	z.fit()

	z.sizer.SetCanTarget(false)

	// The frame is centered, so it is never larger than the sizer when there
	// is space. It doesn't measure the picture laid over it.
	z.frame.SetHAlign(gtk.AlignCenter)
	z.frame.SetVAlign(gtk.AlignCenter)
	z.frame.SetChild(z.sizer)
	z.frame.AddOverlay(z.picture)

	z.ScrolledWindow = gtk.NewScrolledWindow()
	z.SetHExpand(true)
	z.SetVExpand(true)
	z.SetChild(z.frame)

	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) { z.pointer = [2]float64{x, y} })
	z.AddController(motion)

	// Zoom with Ctrl+scroll before the scrolled window scrolls instead.
	scroll := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical)
	scroll.SetPropagationPhase(gtk.PhaseCapture)
	scroll.ConnectScroll(func(_, dy float64) bool {
		if scroll.CurrentEventState()&gdk.ControlMask == 0 {
			return false
		}
		z.setZoom(z.currentZoom()*math.Pow(zoomStep, -dy), z.pointer[0], z.pointer[1])
		return true
	})
	z.AddController(scroll)

	var pinchZoom float64
	pinch := gtk.NewGestureZoom()
	pinch.ConnectBegin(func(*gdk.EventSequence) { pinchZoom = z.currentZoom() })
	pinch.ConnectScaleChanged(func(scale float64) {
		if x, y, ok := pinch.BoundingBoxCenter(); ok {
			z.setZoom(pinchZoom*scale, x, y)
		}
	})
	z.AddController(pinch)

	// The scrolled window already pans on touchscreens, so only pan with the
	// mouse here.
	var dragStart [2]float64
	drag := gtk.NewGestureDrag()
	drag.ConnectDragBegin(func(_, _ float64) {
		dev := drag.CurrentEventDevice()
		if dev != nil && gdk.BaseDevice(dev).Source() == gdk.SourceTouchscreen {
			drag.SetState(gtk.EventSequenceDenied)
			return
		}
		dragStart = [2]float64{z.HAdjustment().Value(), z.VAdjustment().Value()}
	})
	drag.ConnectDragUpdate(func(dx, dy float64) {
		z.HAdjustment().SetValue(dragStart[0] - dx)
		z.VAdjustment().SetValue(dragStart[1] - dy)
	})
	z.AddController(drag)

	click := gtk.NewGestureClick()
	click.ConnectPressed(func(n int, x, y float64) {
		if n != 2 {
			return
		}
		if z.zoom != 0 {
			z.fit()
			return
		}
		// Zoom to the original size, or further if it already fits.
		z.setZoom(max(1, 2*z.fitZoom()), x, y)
	})
	z.AddController(click)

	return z
}

// imageSize returns the size of the image in pixels.
func (z *zoomView) imageSize() (w, h float64, ok bool) {
	p := z.picture.Paintable()
	if p == nil {
		return 0, 0, false
	}
	w = float64(p.IntrinsicWidth())
	h = float64(p.IntrinsicHeight())
	return w, h, w > 0 && h > 0
}

// fitZoom returns the zoom at which the image fits into the view. Images are
// never enlarged to fit.
func (z *zoomView) fitZoom() float64 {
	w, h, ok := z.imageSize()
	if !ok {
		return 1
	}
	return min(1, float64(z.Width())/w, float64(z.Height())/h)
}

func (z *zoomView) currentZoom() float64 {
	if z.zoom == 0 {
		return z.fitZoom()
	}
	return z.zoom
}

// fit makes the image fit into the view.
func (z *zoomView) fit() {
	z.zoom = 0
	z.sizer.SetCanShrink(true)
	if w, h, ok := z.imageSize(); ok {
		z.sizer.SetPaintable(gdk.NewPaintableEmpty(int(w), int(h)))
	}
}

// zoomBy multiplies the zoom by factor around the center of the view.
func (z *zoomView) zoomBy(factor float64) {
	z.zoomTo(z.currentZoom() * factor)
}

// zoomTo zooms around the center of the view.
func (z *zoomView) zoomTo(zoom float64) {
	z.setZoom(zoom, float64(z.Width())/2, float64(z.Height())/2)
}

// setZoom zooms the image while keeping the point at x, y in the view still.
// The image fits into the view if zoom is too small for it to fill the view.
func (z *zoomView) setZoom(zoom, x, y float64) {
	w, h, ok := z.imageSize()
	if !ok {
		return
	}

	fit := z.fitZoom()
	if zoom <= fit {
		z.fit()
		return
	}
	zoom = min(zoom, max(maxZoom, fit))

	hadj := z.HAdjustment()
	vadj := z.VAdjustment()

	// Find the image pixel under the point before resizing.
	old := z.currentZoom()
	ix := viewToContent(hadj, x, w*old) / old
	iy := viewToContent(vadj, y, h*old) / old

	z.zoom = zoom
	z.sizer.SetCanShrink(false)
	z.sizer.SetPaintable(gdk.NewPaintableEmpty(int(w*zoom), int(h*zoom)))

	scrollContentTo(hadj, ix*zoom-x, w*zoom)
	scrollContentTo(vadj, iy*zoom-y, h*zoom)
}

// viewToContent converts a position in the view to one in the content, which
// is centered if it is smaller than the view.
func viewToContent(adj *gtk.Adjustment, pos, size float64) float64 {
	if page := adj.PageSize(); size < page {
		return pos - (page-size)/2
	}
	return adj.Value() + pos
}

// scrollContentTo scrolls to value as if the content were already size large.
// The scrolled window only resizes the content on its next allocation.
func scrollContentTo(adj *gtk.Adjustment, value, size float64) {
	adj.SetUpper(max(size, adj.PageSize()))
	adj.SetValue(value)
}
