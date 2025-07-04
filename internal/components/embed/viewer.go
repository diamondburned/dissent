package embed

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// TODO: In libadwaita 1.4 replace BackButton with `set_show_back_button“
type Viewer struct {
	*adw.Window

	Header       *adw.HeaderBar
	ToastOverlay *adw.ToastOverlay
	Overlay      *gtk.Overlay
	Scroll       *gtk.ScrolledWindow
	Embed        *Embed

	BackButton    *gtk.Button
	ControlsStart ControlsBoxStart
	ControlsEnd   ControlsBoxEnd

	zoom     float64
	filename string

	ctx context.Context
}

type ControlsBoxStart struct {
	*gtk.Box

	Download     *gtk.Button
	CopyURL      *gtk.Button
	OpenOriginal *gtk.Button
}

type ControlsBoxEnd struct {
	*gtk.Box
}

var controlsStyles = []string{"osd", "circular"}

var _ = cssutil.WriteCSS(`
	.thumbnail-embed-viewer .thumbnail-embed {
		border: none;
	}
	.thumbnail-embed-viewer .thumbnail-embed,
	.thumbnail-embed-viewer .thumbnail-embed > button,
	.thumbnail-embed-viewer .thumbnail-embed > button > * {
		border-radius: 0;
	}
`)

// NewViewer creates a new instance of Viewer window, representing an image viewer.
func NewViewer(ctx context.Context, uri string, opts Opts) (*Viewer, error) {
	opts.Autoplay = true

	parentWindow := app.GTKWindowFromContext(ctx)
	w := parentWindow.Width()
	h := parentWindow.Height()

	v := Viewer{ctx: ctx}
	v.Embed = New(ctx, w, h, opts)
	v.Embed.SetFromURL(uri)

	v.ToastOverlay = adw.NewToastOverlay()

	v.Overlay = gtk.NewOverlay()

	v.ToastOverlay.SetChild(v.Overlay)

	v.Scroll = gtk.NewScrolledWindow()
	v.Scroll.SetVExpand(true)
	v.Scroll.SetHExpand(true)

	v.Overlay.SetChild(v.Scroll)

	v.zoom = 1.0

	v.Window = adw.NewWindow()
	v.AddCSSClass("thumbnail-embed-viewer")
	v.SetTransientFor(parentWindow)
	v.SetDefaultSize(w, h)
	v.SetModal(true)

	url, err := url.Parse(v.Embed.URL())
	if err != nil {
		fmt.Printf("Invalid raw URI structure: %s\n", url)
		return nil, err
	}
	v.filename = path.Base(url.Path)

	v.BackButton = newActionButton(v, "Back", "go-previous-symbolic", "embedviewer.close", nil)

	v.Header = adw.NewHeaderBar()
	v.Header.SetShowEndTitleButtons(false)
	v.Header.SetShowStartTitleButtons(false)
	v.Header.SetCenteringPolicy(adw.CenteringPolicyStrict)
	v.Header.SetTitleWidget(adw.NewWindowTitle(v.filename, ""))

	v.SetShowBackButton(true)

	v.ControlsStart = ControlsBoxStart{
		Box:          gtk.NewBox(gtk.OrientationHorizontal, 6),
		Download:     newActionButton(v, "Download", "folder-download-symbolic", "embedviewer.download", controlsStyles),
		CopyURL:      newActionButton(v, "Copy URL", "edit-copy-symbolic", "embedviewer.copy-url", controlsStyles),
		OpenOriginal: newActionButton(v, "Open Original", "earth-symbolic", "embedviewer.open-original", controlsStyles),
	}

	v.ControlsStart.SetMarginBottom(18)
	v.ControlsStart.SetMarginStart(18)
	v.ControlsStart.SetHAlign(gtk.AlignStart)
	v.ControlsStart.SetVAlign(gtk.AlignEnd)

	v.ControlsStart.Append(v.ControlsStart.OpenOriginal)
	v.ControlsStart.Append(v.ControlsStart.Download)
	v.ControlsStart.Append(v.ControlsStart.CopyURL)

	v.ControlsEnd = ControlsBoxEnd{
		Box: gtk.NewBox(gtk.OrientationHorizontal, 6),
	}

	v.ControlsEnd.SetMarginBottom(18)
	v.ControlsEnd.SetMarginStart(18)
	v.ControlsEnd.SetHAlign(gtk.AlignEnd)
	v.ControlsEnd.SetVAlign(gtk.AlignEnd)

	v.Overlay.AddOverlay(v.ControlsStart)
	v.Overlay.AddOverlay(v.ControlsEnd)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(v.Header)
	box.Append(v.ToastOverlay)

	v.SetContent(box)

	gtkutil.BindActionMap(v, map[string]func(){
		"embedviewer.close":         v.close,
		"embedviewer.download":      v.download,
		"embedviewer.copy-url":      v.copyURL,
		"embedviewer.open-original": v.openOriginal,
	})

	switch opts.Type {
	case EmbedTypeImage, EmbedTypeGIF:
		v.Embed.SetHExpand(true)
		v.Embed.SetVExpand(true)

		// Keep original size of the image when resizing window
		v.Embed.SetVAlign(gtk.AlignCenter)
		v.Embed.SetHAlign(gtk.AlignCenter)

		v.Scroll.SetChild(v.Embed)
		v.Scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)

	case EmbedTypeGIFV, EmbedTypeVideo:
		v.Embed.SetVExpand(true)
		v.Embed.SetHExpand(true)
		v.Embed.SetVAlign(gtk.AlignFill)
		v.Embed.SetHAlign(gtk.AlignFill)

		v.Scroll.SetChild(v.Embed)
		v.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyNever)
	default:
		err := fmt.Errorf("unsupported embed type: %#v", opts.Type)
		return nil, err
	}

	return &v, nil
}

func newActionButton(target gtk.Widgetter, text, icon, action string, styles []string) *gtk.Button {
	button := gtk.NewButtonFromIconName(icon)
	button.SetTooltipText(text)

	if styles != nil {
		button.SetCSSClasses(styles)
	}

	targetRef := coreglib.NewWeakRef(target)

	button.ConnectClicked(func() {
		base := gtk.BaseWidget(targetRef.Get())
		base.ActivateAction(action, nil)
	})

	return button
}

// SetShowBackButton sets whether to show back button at the start of headerbar.
func (v *Viewer) SetShowBackButton(show bool) {
	if !show {
		v.Header.Remove(v.BackButton)
	}

	v.Header.PackStart(v.BackButton)
}

// AddStartButton adds a button into the ControlsBoxStart.
func (cs *ControlsBoxStart) AddStartButton(pack gtk.PositionType, button *gtk.Button) {
	switch pack {
	case gtk.PosTop, gtk.PosLeft:
		cs.Prepend(button)
	case gtk.PosBottom, gtk.PosRight:
		cs.Append(button)
	}
}

// AddEndButton adds a button into the ControlsBoxEnd.
func (ce *ControlsBoxEnd) AddEndButton(pack gtk.PositionType, button *gtk.Button) {
	switch pack {
	case gtk.PosTop, gtk.PosLeft:
		ce.Prepend(button)
	case gtk.PosBottom, gtk.PosRight:
		ce.Append(button)
	}
}

func (v *Viewer) close() {
	v.Close()
}

func (v *Viewer) download() {
	ctx := v.ctx
	window := app.WindowFromContext(ctx)

	dialog := gtk.NewFileDialog()
	dialog.SetInitialName(v.filename)
	dialog.Save(ctx, &window.Window, func(res gio.AsyncResulter) {
		file, err := dialog.SaveFinish(res)
		if err != nil {
			slog.InfoContext(ctx,
				"Embed viewer download not saving file",
				"err", err)
			return
		}

		saveToFile(file, v.Embed.URL(), v.ToastOverlay)
	})
}

func saveToFile(file *gio.File, pictureURL string, toast *adw.ToastOverlay) {
	outPath := file.Path()

	response, err := http.Get(pictureURL)
	if err != nil {
		toast.AddToast(adw.NewToast("An error occured while downloading picture data"))
		fmt.Println("An error occured while downloading picture data:", err)
		return
	}
	defer response.Body.Close()

	out, err := os.Create(outPath)
	if err != nil {
		toast.AddToast(adw.NewToast("An I/O error occurred while creating the output file"))
		fmt.Println("An I/O error occurred while creating the output file:", err)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, response.Body)
	if err != nil {
		toast.AddToast(adw.NewToast("An I/O error occurred while saving the file"))
		fmt.Println("An I/O error occurred while saving the file:", err)
		return
	}

	toast.AddToast(adw.NewToast("Picture saved successfully"))
}

func (v *Viewer) copyURL() {
	url := v.Embed.URL()

	display := gdk.DisplayGetDefault()

	clipboard := display.Clipboard()
	clipboard.SetText(url)

	v.ToastOverlay.AddToast(adw.NewToast("Copied URL!"))
}

func (v *Viewer) openOriginal() {
	app.OpenURI(v.ctx, v.Embed.URL())
}
