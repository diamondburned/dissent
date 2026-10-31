package messages

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
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
	gtk.Widget
	child gtk.Widgetter
	w, h  int
}

var aspectFrameType = coreglib.RegisterSubclass[*aspectFrame](
	coreglib.WithGTypeName("DissentAspectFrame"),
	coreglib.WithOverrides(func(f *aspectFrame) coreglib.ObjectOverrides {
		return coreglib.ObjectOverrides{Dispose: f.dispose}
	}),
	coreglib.WithOverrides(func(f *aspectFrame) gtk.WidgetOverrides {
		return gtk.WidgetOverrides{
			RequestMode:  f.requestMode,
			Measure:      f.measure,
			SizeAllocate: f.sizeAllocate,
		}
	}),
)

func newAspectFrame(child gtk.Widgetter, w, h int) *aspectFrame {
	f := aspectFrameType.New()
	// Neither align fills, or GTK finds the natural width for the height.
	f.SetHAlign(gtk.AlignStart)
	f.SetVAlign(gtk.AlignStart)
	f.child = child
	gtk.BaseWidget(child).SetParent(f)
	f.SetSize(w, h)
	return f
}

// SetSize sets the size of the frame when there is enough space for it.
func (f *aspectFrame) SetSize(w, h int) {
	f.w, f.h = w, h
	f.QueueResize()
}

func (f *aspectFrame) requestMode() gtk.SizeRequestMode {
	return gtk.SizeRequestHeightForWidth
}

// measure asks for the size of the frame, and for a height that keeps its
// aspect ratio at narrower widths. The child is not measured, so it shrinks
// with the frame.
func (f *aspectFrame) measure(orientation gtk.Orientation, forSize int) (minimum, natural, minimumBaseline, naturalBaseline int) {
	if orientation == gtk.OrientationHorizontal {
		return 0, f.w, -1, -1
	}
	if forSize < 0 {
		// The frame can be as narrow as nothing, so it can be as short too.
		// Containers like GtkBox take this as the least height they can have.
		return 0, f.h, -1, -1
	}
	h := f.h
	if forSize >= 0 && forSize < f.w {
		h = forSize * f.h / f.w
	}
	return h, h, -1, -1
}

func (f *aspectFrame) sizeAllocate(width, height, baseline int) {
	// GTK wants children to be measured before they are allocated.
	child := gtk.BaseWidget(f.child)
	child.Measure(gtk.OrientationHorizontal, -1)
	child.Allocate(width, height, baseline, nil)
}

func (f *aspectFrame) dispose() {
	if f.child != nil {
		gtk.BaseWidget(f.child).Unparent()
		f.child = nil
	}
}
