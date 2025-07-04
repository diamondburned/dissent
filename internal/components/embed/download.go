package embed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/httputil"
	"github.com/diamondburned/gotkit/utils/cachegc"
	"github.com/dustin/go-humanize"
)

func fetchURL(ctx context.Context, url, cacheDst string, bar *gtk.ProgressBar) bool {
	if err := cachegc.WithTmpFile(cacheDst, "*", func(f *os.File) error {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		client := httputil.FromContext(ctx, http.DefaultClient)

		r, err := client.Do(req)
		if err != nil {
			return err
		}
		defer r.Body.Close()

		if r.StatusCode < 200 || r.StatusCode > 299 {
			return fmt.Errorf("unexpected status code %d getting %q", r.StatusCode, url)
		}

		rWithProgress := &progressReader{
			Reader: r.Body,
			Update: func(n int64) {
				if r.ContentLength == -1 {
					bar.Pulse()
					bar.SetText("Downloading")
					return
				}

				frac := float64(n) / float64(r.ContentLength)
				bar.SetFraction(frac)
				bar.SetText(fmt.Sprintf(
					"%s (%s/%s)",
					locale.Get("Downloading"),
					humanize.IBytes(uint64(n)),
					humanize.IBytes(uint64(r.ContentLength)),
				))
			},
		}

		if _, err := io.Copy(f, rWithProgress); err != nil {
			return err
		}

		return nil
	}); err != nil {
		glib.IdleAdd(func() {
			bar.SetText(progressLabelError(err))
		})
		return false
	}

	return true
}

func progressLabelPulsated() string {
	return locale.Get("Downloading")
}

func progressLabelWithBytes(n, max int64) string {
	if max == 0 {
		return progressLabelPulsated()
	}
	return fmt.Sprintf(
		"%s (%s/%s)",
		locale.Get("Downloading"),
		humanize.IBytes(uint64(n)),
		humanize.IBytes(uint64(max)),
	)
}

func progressLabelError(err error) string {
	return locale.Sprintf("Download error: %s", err.Error())
}

type progressReader struct {
	Reader io.Reader
	Update func(int64)

	n      atomic.Int64
	handle glib.SourceHandle
}

func (r *progressReader) Read(b []byte) (int, error) {
	const progressReaderUpdateFreq = 1000 / 20 // 20Hz

	if r.handle == 0 {
		r.handle = glib.TimeoutAddPriority(progressReaderUpdateFreq, glib.PriorityDefaultIdle, func() bool {
			r.Update(r.n.Load())
			return true
		})
	}

	n, err := r.Reader.Read(b)
	r.n.Add(int64(n))

	if err != nil {
		glib.SourceRemove(r.handle)
		r.handle = 0

		// Ensure that the state gets updated one last time.
		glib.IdleAdd(func() {
			r.Update(r.n.Load())
		})
	}

	return n, err
}
