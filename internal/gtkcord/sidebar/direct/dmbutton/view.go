package dmbutton

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type View struct {
	*gtk.Box
	DM *Button
}

func NewView(ctx context.Context) *View {
	v := View{
		Box: gtk.NewBox(gtk.OrientationVertical, 0),
		DM:  NewButton(ctx, func() {}),
	}

	v.Append(v.DM)

	return &v
}
