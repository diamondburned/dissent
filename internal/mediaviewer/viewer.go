// Package mediaviewer shows an image or a video in its own window, where
// images can be zoomed into.
package mediaviewer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

var _ = cssutil.WriteCSS(`
	window.mediaviewer {
		background-color: black;
		color: white;
	}
	.mediaviewer-title > button {
		min-height: 34px;
		padding: 0 12px 0 16px;
		border-radius: 9999px;
	}
`)

// controlsTimeout is how long the controls stay shown without any input.
const controlsTimeout = 3 // seconds

// Viewer is a window that shows an image or a video.
type Viewer struct {
	*adw.Window
	toasts     *adw.ToastOverlay
	zoom       *zoomView // nil for videos
	fullscreen *gtk.Button
	// menu is the title, which opens a menu of actions on the media.
	menu *gtk.MenuButton

	// controls are laid over the media and hide when there is no input.
	controls      []*gtk.Revealer
	controlsShown bool
	hideControls  glib.SourceHandle
	// hovering is true while the pointer is over the controls, which keeps
	// them shown.
	hovering bool
	// pointer is the last position of the pointer.
	pointer [2]float64

	ctx      context.Context
	url      string
	filename string
}

// Open opens a Viewer for the media at uri. opts.Type decides how it is shown.
func Open(ctx context.Context, uri string, opts embed.Opts) {
	v := New(ctx, uri, opts)
	v.Present()
}

// New creates a Viewer for the media at uri. opts.Type decides how it is
// shown.
func New(ctx context.Context, uri string, opts embed.Opts) *Viewer {
	parent := app.GTKWindowFromContext(ctx)

	v := &Viewer{
		ctx:      ctx,
		url:      uri,
		filename: path.Base(uri),
	}
	if u, err := url.Parse(uri); err == nil {
		v.filename = path.Base(u.Path)
	}

	v.toasts = adw.NewToastOverlay()
	v.fullscreen = newActionButton(locale.Get("Fullscreen"), "view-fullscreen-symbolic", "viewer.fullscreen")

	var isVideo bool
	switch opts.Type {
	case embed.EmbedTypeGIFV, embed.EmbedTypeVideo:
		isVideo = true
		opts.Autoplay = true
		video := embed.New(ctx, parent.Width(), parent.Height(), opts)
		video.SetHExpand(true)
		video.SetVExpand(true)
		video.SetFromURL(uri)
		v.toasts.SetChild(video)
	default:
		v.toasts.SetChild(v.newImage(opts))
	}

	// There is no header bar. The controls are laid over the media instead.
	overlay := gtk.NewOverlay()
	overlay.SetChild(v.toasts)

	menu := gio.NewMenu()
	menu.Append(locale.Get("Open Original"), "viewer.open-original")
	menu.Append(locale.Get("Copy URL"), "viewer.copy-url")
	menu.Append(locale.Get("Download"), "viewer.download")

	v.menu = gtk.NewMenuButton()
	v.menu.AddCSSClass("osd")
	v.menu.AddCSSClass("mediaviewer-title")
	v.menu.SetChild(adw.NewWindowTitle(v.filename, ""))
	v.menu.SetAlwaysShowArrow(true)
	v.menu.SetMenuModel(menu)
	v.menu.NotifyProperty("active", func() {
		// The controls stay shown while the menu is open.
		if !v.menu.Active() {
			v.showControls()
		}
	})

	// The window is moved by dragging the title.
	title := gtk.NewWindowHandle()
	title.SetChild(v.menu)

	top := gtk.NewBox(gtk.OrientationHorizontal, 6)
	if isVideo {
		// The video's own controls take the bottom of the window.
		top.Append(v.fullscreen)
	} else {
		bottom := gtk.NewBox(gtk.OrientationHorizontal, 6)
		bottom.Append(v.fullscreen)
		bottom.Append(newActionButton(locale.Get("Zoom Out"), "zoom-out-symbolic", "viewer.zoom-out"))
		bottom.Append(newActionButton(locale.Get("Original Size"), "zoom-original-symbolic", "viewer.zoom-original"))
		bottom.Append(newActionButton(locale.Get("Zoom In"), "zoom-in-symbolic", "viewer.zoom-in"))
		v.addControl(overlay, bottom, gtk.AlignEnd, gtk.AlignEnd)
	}
	top.Append(newActionButton(locale.Get("Close"), "window-close-symbolic", "viewer.close"))
	v.addControl(overlay, top, gtk.AlignEnd, gtk.AlignStart)

	// Keep the title clear of the buttons, which are 34px wide with 6px
	// between them.
	var buttons int
	for child := top.FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		buttons++
	}
	v.addControl(overlay, title, gtk.AlignStart, gtk.AlignStart).SetMarginEnd(18 + 12 + 40*buttons)

	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) {
		// Ignore motion without movement, e.g. from the controls hiding.
		if p := [2]float64{x, y}; p != v.pointer {
			v.pointer = p
			v.showControls()
		}
	})
	overlay.AddController(motion)

	// Touchscreens have no pointer to move, so a tap toggles the controls.
	tap := gtk.NewGestureClick()
	tap.SetTouchOnly(true)
	tap.ConnectReleased(func(n int, _, _ float64) {
		if n != 1 {
			return
		}
		if v.controlsShown {
			v.revealControls(false)
		} else {
			v.showControls()
		}
	})
	overlay.AddController(tap)

	v.Window = adw.NewWindow()
	v.AddCSSClass("mediaviewer")
	v.SetTitle(v.filename)
	v.SetTransientFor(parent)
	v.SetModal(true)
	v.SetDefaultSize(parent.Width(), parent.Height())
	v.SetContent(overlay)

	keys := gtk.NewEventControllerKey()
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(uint, uint, gdk.ModifierType) bool {
		v.showControls()
		return false
	})
	v.AddController(keys)
	v.showControls()

	v.NotifyProperty("fullscreened", func() {
		if v.IsFullscreen() {
			v.fullscreen.SetIconName("view-restore-symbolic")
			v.fullscreen.SetTooltipText(locale.Get("Leave Fullscreen"))
		} else {
			v.fullscreen.SetIconName("view-fullscreen-symbolic")
			v.fullscreen.SetTooltipText(locale.Get("Fullscreen"))
		}
	})

	actions := map[string]func(){
		"viewer.close":         v.Close,
		"viewer.escape":        v.escape,
		"viewer.fullscreen":    v.toggleFullscreen,
		"viewer.download":      v.download,
		"viewer.copy-url":      v.copyURL,
		"viewer.open-original": v.openOriginal,
	}
	shortcuts := map[string]string{
		"Escape":  "viewer.escape",
		"F11":     "viewer.fullscreen",
		"<Ctrl>S": "viewer.download",
		"<Ctrl>W": "viewer.close",
	}
	if v.zoom != nil {
		actions["viewer.zoom-in"] = func() { v.zoom.zoomBy(zoomStep) }
		actions["viewer.zoom-out"] = func() { v.zoom.zoomBy(1 / zoomStep) }
		actions["viewer.zoom-fit"] = v.zoom.fit
		actions["viewer.zoom-original"] = func() { v.zoom.zoomTo(1) }
		shortcuts["<Ctrl>plus"] = "viewer.zoom-in"
		shortcuts["<Ctrl>equal"] = "viewer.zoom-in"
		shortcuts["<Ctrl>minus"] = "viewer.zoom-out"
		shortcuts["<Ctrl>0"] = "viewer.zoom-fit"
		shortcuts["<Ctrl>1"] = "viewer.zoom-original"
	}
	gtkutil.BindActionMap(v, actions)
	gtkutil.AddActionShortcuts(v, shortcuts)

	return v
}

// newImage creates the zoomable image with its zoom controls.
func (v *Viewer) newImage(opts embed.Opts) gtk.Widgetter {
	overlay := gtk.NewOverlay()

	spinner := adw.NewSpinner()
	spinner.SetSizeRequest(32, 32)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)
	spinner.SetCanTarget(false)
	overlay.AddOverlay(spinner)

	ctx := imgutil.WithOpts(v.ctx, imgutil.WithErrorFn(func(err error) {
		status := adw.NewStatusPage()
		status.SetIconName("image-missing-symbolic")
		status.SetTitle(locale.Get("Couldn't Load Image"))
		status.SetDescription(err.Error())
		overlay.SetChild(status)
		spinner.SetVisible(false)
	}))

	provider := opts.Provider
	if provider == nil {
		provider = imgutil.HTTPProvider
	}

	picture := onlineimage.NewPicture(ctx, provider)
	picture.NotifyProperty("paintable", func() {
		spinner.SetVisible(picture.Paintable() == nil)
	})

	var anim *onlineimage.AnimationController
	if opts.Type == embed.EmbedTypeGIF {
		anim = picture.EnableAnimation()
	}
	picture.SetURL(v.url)
	if anim != nil {
		anim.Start()
	}

	v.zoom = newZoomView(picture.Picture)
	overlay.SetChild(v.zoom)

	return overlay
}

// addControl lays w over the media in the corner given by halign and valign.
func (v *Viewer) addControl(overlay *gtk.Overlay, w gtk.Widgetter, halign, valign gtk.Align) *gtk.Revealer {
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	revealer.SetChild(w)
	revealer.SetHAlign(halign)
	revealer.SetVAlign(valign)
	revealer.SetMarginTop(18)
	revealer.SetMarginBottom(18)
	revealer.SetMarginStart(18)
	revealer.SetMarginEnd(18)

	hover := gtk.NewEventControllerMotion()
	hover.ConnectEnter(func(_, _ float64) { v.hovering = true })
	hover.ConnectLeave(func() { v.hovering = false })
	revealer.AddController(hover)

	overlay.AddOverlay(revealer)
	v.controls = append(v.controls, revealer)
	return revealer
}

// showControls shows the controls until there is no input for a while.
func (v *Viewer) showControls() {
	v.revealControls(true)
	if v.hideControls != 0 {
		glib.SourceRemove(v.hideControls)
	}
	v.hideControls = glib.TimeoutSecondsAdd(controlsTimeout, func() {
		v.hideControls = 0
		if !v.hovering && !v.menu.Active() {
			v.revealControls(false)
		}
	})
}

func (v *Viewer) revealControls(reveal bool) {
	v.controlsShown = reveal
	for _, revealer := range v.controls {
		revealer.SetRevealChild(reveal)
	}
	// The pointer hides with the controls.
	if reveal {
		v.SetCursor(nil)
	} else {
		v.SetCursorFromName("none")
	}
}

func newActionButton(tooltip, icon, action string) *gtk.Button {
	button := gtk.NewButtonFromIconName(icon)
	button.SetTooltipText(tooltip)
	button.SetActionName(action)
	button.AddCSSClass("osd")
	button.AddCSSClass("circular")
	return button
}

func (v *Viewer) toggleFullscreen() {
	if v.IsFullscreen() {
		v.Unfullscreen()
	} else {
		v.Fullscreen()
	}
}

// escape leaves fullscreen, or closes the viewer if it isn't fullscreen.
func (v *Viewer) escape() {
	if v.IsFullscreen() {
		v.Unfullscreen()
		return
	}
	v.Close()
}

func (v *Viewer) toast(text string) {
	v.toasts.AddToast(adw.NewToast(text))
}

func (v *Viewer) download() {
	d := gtk.NewFileDialog()
	d.SetTitle(locale.Get("Download"))
	d.SetInitialName(v.filename)
	d.Save(v.ctx, &v.Window.Window, func(async gio.AsyncResulter) {
		file, err := d.SaveFinish(async)
		if err != nil {
			return
		}

		dst := file.Path()
		go func() {
			err := saveURL(v.ctx, v.url, dst)
			glib.IdleAdd(func() {
				if err != nil {
					slog.Error("cannot download media", "url", v.url, "err", err)
					v.toast(locale.Get("Couldn't download the file"))
					return
				}
				v.toast(locale.Get("File downloaded"))
			})
		}()
	})
}

func saveURL(ctx context.Context, src, dst string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (v *Viewer) copyURL() {
	gdk.DisplayGetDefault().Clipboard().SetText(v.url)
	v.toast(locale.Get("Copied URL"))
}

func (v *Viewer) openOriginal() {
	app.OpenURI(v.ctx, v.url)
}
