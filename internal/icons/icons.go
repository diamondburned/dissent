// Package icons embeds several PNG icons.
package icons

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

//go:embed png/*.png
var PNGs embed.FS

var pngCache struct {
	mu sync.RWMutex
	m  map[string]*gdkpixbuf.Pixbuf
}

// PixbufScale calls Pixbuf and scales it to size.
func PixbufScale(name string, size int) *gdkpixbuf.Pixbuf {
	p := Pixbuf(name)
	if p != nil {
		p = p.ScaleSimple(size, size, gdkpixbuf.InterpTiles)
	}
	return p
}

// Pixbuf returns a pixbuf of the given PNG. The size of the pixbuf returned is
// the original size.
func Pixbuf(name string) *gdkpixbuf.Pixbuf {
	if !strings.HasSuffix(name, "-dark") && !strings.HasSuffix(name, "-light") {
		themedName := name
		if textutil.IsDarkTheme() {
			themedName += "-dark"
		} else {
			themedName += "-light"
		}

		if pb := pixbuf(themedName); pb != nil {
			return pb
		}
	}

	if pb := pixbuf(name); pb != nil {
		return pb
	}

	log.Printf("icon: unknown icon %q", name)
	return nil
}

func pixbuf(name string) *gdkpixbuf.Pixbuf {
	if filepath.Ext(name) == "" {
		name += ".png"
	}

	pngCache.mu.RLock()
	p, ok := pngCache.m[name]
	pngCache.mu.RUnlock()
	if ok {
		return p
	}

	pngCache.mu.Lock()
	defer pngCache.mu.Unlock()

	if pngCache.m == nil {
		pngCache.m = make(map[string]*gdkpixbuf.Pixbuf, 6)
	}

	p, ok = pngCache.m[name]
	if ok {
		return p
	}

	b, err := fs.ReadFile(PNGs, path.Join("png", name))
	if err != nil {
		return nil
	}

	p, err = gdkpixbuf.NewPixbufFromStream(
		context.Background(),
		gio.NewMemoryInputStreamFromBytes(glib.NewBytesWithGo(b)),
	)
	if err != nil {
		log.Printf("icon: corrupted icon %q: %v", name, err)
	}

	pngCache.m[name] = p
	return p
}

type provider struct{}

// Provider is an imgutil.Provider that handles icon:// URLs. Use it like
// icon://channel-dark.png.
var Provider imgutil.Provider = provider{}

// Schemes implements imgutil.Provider.
func (p provider) Schemes() []string { return []string{"icon"} }

// Do implements imgutil.Provider.
func (p provider) Do(ctx context.Context, url *url.URL, img imgutil.ImageSetter) {
	// Combine Host and Path since url.Parse splits the path into Host.
	pixbuf := Pixbuf(url.Host + url.Path)

	switch {
	case img.SetFromPixbuf != nil:
		img.SetFromPixbuf(pixbuf)
	case img.SetFromPaintable != nil:
		img.SetFromPaintable(gdk.NewTextureForPixbuf(pixbuf))
	}
}
