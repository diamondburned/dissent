package message

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

type uploadingLabel struct {
	*gtk.Label
	ctx context.Context
	err []error
	cur int
	max int
}

var uploadingLabelCSS = cssutil.Applier("message-uploading-label", `
	.message-uploading-label {
		opacity: 0.75;
		font-size: 0.8em;
	}
`)

func newUploadingLabel(ctx context.Context, count int) *uploadingLabel {
	l := uploadingLabel{max: count}
	l.Label = gtk.NewLabel("")
	l.Label.SetXAlign(0)
	l.Label.SetWrap(true)
	l.Label.SetWrapMode(pango.WrapWordChar)
	uploadingLabelCSS(l.Label)

	l.invalidate()
	return &l
}

// Done increments the done counter.
func (l *uploadingLabel) Done() {
	l.cur++
	l.invalidate()
}

// HasErrored returns true if the label contains any error.
func (l *uploadingLabel) HasErrored() bool { return len(l.err) > 0 }

// AppendError adds an error into the label.
func (l *uploadingLabel) AppendError(err error) {
	l.err = append(l.err, err)
	l.invalidate()
}

func (l *uploadingLabel) invalidate() {
	var m string
	if l.max > 0 {
		m += fmt.Sprintf("<i>Uploaded %d/%d...</i>", l.cur, l.max)
	}
	for _, err := range l.err {
		m += "\n" + textutil.ErrorMarkup(err.Error())
	}
	l.Label.SetMarkup(strings.TrimPrefix(m, "\n"))
}

type wrappedReader struct {
	r io.Reader
	l *uploadingLabel
}

func (r wrappedReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	if err != nil {
		glib.IdleAdd(func() {
			if errors.Is(err, io.EOF) {
				r.l.Done()
			} else {
				r.l.AppendError(err)
			}
		})
	}
	return n, err
}
