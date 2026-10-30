package messages

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// newSizer creates an empty widget with the natural size of w by h. It can
// shrink to nothing while keeping its aspect ratio.
func newSizer(w, h int) *gtk.Picture {
	sizer := gtk.NewPictureForPaintable(gdk.NewPaintableEmpty(w, h))
	sizer.SetCanShrink(true)
	sizer.SetCanTarget(false)
	return sizer
}

// aspectFrame shows a child at the aspect ratio of its size. It is as large as
// its size when there is space, but it shrinks with the window instead of
// stopping it from getting narrower.
type aspectFrame struct {
	*gtk.Overlay
	sizer *gtk.Picture
	w, h  int
	// width is the allocated width, or 0 if the frame is not allocated yet.
	width int
}

func newAspectFrame(child gtk.Widgetter, w, h int) *aspectFrame {
	f := &aspectFrame{
		Overlay: gtk.NewOverlay(),
		sizer:   newSizer(w, h),
	}
	// Neither align fills, or GTK finds the natural width for the height.
	f.SetHAlign(gtk.AlignStart)
	f.SetVAlign(gtk.AlignStart)
	f.SetChild(f.sizer)

	// The message list gives its children their minimum height, which is 0
	// for the sizer, so the height is requested once the width is known.
	resizer := gtk.NewDrawingArea()
	resizer.SetCanTarget(false)
	resizer.ConnectResize(func(width, _ int) {
		f.width = width
		glib.IdleAdd(f.updateHeight)
	})

	// The child is laid over the frame without being measured.
	f.AddOverlay(resizer)
	f.AddOverlay(child)
	f.SetSize(w, h)
	return f
}

// SetSize sets the size of the frame when there is enough space for it.
func (f *aspectFrame) SetSize(w, h int) {
	f.w, f.h = w, h
	f.sizer.SetPaintable(gdk.NewPaintableEmpty(w, h))
	f.updateHeight()
}

func (f *aspectFrame) updateHeight() {
	h := f.h
	if f.width > 0 && f.width < f.w {
		h = f.width * f.h / f.w
	}
	if _, current := f.SizeRequest(); current != h {
		f.SetSizeRequest(-1, h)
	}
}
