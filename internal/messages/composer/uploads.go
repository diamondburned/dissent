package composer

import (
	"context"
	"fmt"
	"html"
	"path/filepath"
	"slices"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/dustin/go-humanize"
)

const (
	spoiledDisabledIcon = "visibility-symbolic"
	spoiledEnabledIcon  = "visibility-off-symbolic"
)

func fileSpoilerIcon(file *File) string {
	if file.IsSpoiler() {
		return spoiledEnabledIcon
	}
	return spoiledDisabledIcon
}

func mimeIsText(mime string) bool {
	// How is utf8_string a valid MIME type? GTK, what the fuck?
	return strings.HasPrefix(mime, "text") || mime == "utf8_string"
}

// UploadTray is the tray holding files to be uploaded.
type UploadTray struct {
	*gtk.Box
	files         []uploadFile
	maxUploadSize int64
}

type uploadFile struct {
	gtk.Widgetter
	box *gtk.CenterBox
	bin *adw.BreakpointBin

	icon    *gtk.Image
	name    *gtk.Label
	size    *gtk.Label
	spoiler *gtk.ToggleButton
	delete  *gtk.Button

	file *File
}

var uploadTrayCSS = cssutil.Applier("composer-upload-tray", `
	.composer-upload-item {
		margin: 0.25em 0.65em;
		margin-top: 0;
	}
	.composer-upload-file-name {
		font-size: 0.9em;
	}
	.composer-upload-file-icon {
		margin: 0 0.5em;
		margin-bottom: 1px;
	}
	.composer-upload-file-size {
		font-size: 0.75em;
		opacity: 0.85;
		margin: 0 0.25em;
	}
`)

// NewUploadTray creates a new UploadTray.
func NewUploadTray() *UploadTray {
	t := UploadTray{}
	t.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	uploadTrayCSS(t.Box)
	return &t
}

// SetMaxUploadSize sets the maximum upload size for the tray.
// If the size is set to 0, it will not limit the upload size.
// If a file exceeds this size, it will not be added to the tray, and an alert
// will be shown to the user.
func (t *UploadTray) SetMaxUploadSize(size int64) {
	t.maxUploadSize = size
}

// AddFile adds a file into the tray.
func (t *UploadTray) AddFile(ctx context.Context, f *File) {
	if t.maxUploadSize > 0 && f.Size > t.maxUploadSize {
		// Ellipsize the file name if it is too long. Preserve the file
		// extension for convenience.
		shortName := f.Name
		if len(shortName) > 50 {
			shortName = f.Name[:45] + "…" + strings.TrimPrefix(filepath.Ext(f.Name), ".")
		}

		alert := adw.NewAlertDialog(
			locale.Get("File Too Large"),
			fmt.Sprintf(
				"%s\n%s",
				locale.Sprintf(
					"<i>%s</i> (%s) is larger than %s.",
					html.EscapeString(shortName),
					humanize.IBytes(uint64(f.Size)),
					humanize.IBytes(uint64(t.maxUploadSize)),
				),
				locale.Get("Discord will not allow you to upload this file."),
			))
		alert.SetBodyUseMarkup(true)
		alert.SetPreferWideLayout(true)

		alert.AddResponse("continue", locale.Get("Continue (dangerous)"))
		alert.AddResponse("skip", locale.Get("Skip"))
		alert.SetResponseAppearance("continue", adw.ResponseDestructive)
		alert.SetResponseAppearance("skip", adw.ResponseDefault)
		alert.SetDefaultResponse("skip")
		alert.SetCloseResponse("skip")
		// Disable the continue response by default unless users start
		// complaining in our issues otherwise.
		alert.SetResponseEnabled("continue", false)

		alert.Choose(ctx, app.WindowFromContext(ctx), func(res gio.AsyncResulter) {
			response := alert.ChooseFinish(res)
			switch response {
			case "skip":
				// do nothing
			case "continue":
				t.addFile(f)
			}
		})

		return
	}

	t.addFile(f)
}

func (t *UploadTray) addFile(f *File) {
	u := uploadFile{file: f}

	u.icon = gtk.NewImageFromIconName(mimeIcon(f.Type))
	u.icon.AddCSSClass("composer-upload-file-icon")

	u.name = gtk.NewLabel(f.Name)
	u.name.AddCSSClass("composer-upload-file-name")
	u.name.SetEllipsize(pango.EllipsizeEnd)
	u.name.SetVExpand(true)
	u.name.SetVAlign(gtk.AlignBaseline)

	u.size = gtk.NewLabel(humanize.IBytes(uint64(f.Size)))
	u.size.AddCSSClass("composer-upload-file-size")
	u.size.SetVisible(f.Size > 0)
	u.size.SetVExpand(true)
	u.size.SetVAlign(gtk.AlignBaseline)

	u.spoiler = gtk.NewToggleButton()
	u.spoiler.AddCSSClass("composer-upload-toggle-spoiler")
	u.spoiler.SetIconName(fileSpoilerIcon(f))
	u.spoiler.SetHasFrame(false)
	u.spoiler.SetTooltipText(locale.Get("Spoiler"))

	u.delete = gtk.NewButtonFromIconName("edit-clear-all-symbolic")
	u.delete.AddCSSClass("composer-upload-delete")
	u.delete.SetHasFrame(false)
	u.delete.SetTooltipText(locale.Get("Remove File"))

	start := gtk.NewBox(gtk.OrientationHorizontal, 0)
	start.Append(u.icon)
	start.Append(u.name)
	start.Append(u.size)

	end := gtk.NewBox(gtk.OrientationHorizontal, 0)
	end.Append(u.spoiler)
	end.Append(u.delete)

	u.box = gtk.NewCenterBox()
	u.box.AddCSSClass("composer-upload-item")
	u.box.SetHExpand(true)
	u.box.SetStartWidget(start)
	u.box.SetEndWidget(end)

	smallBreakpoint := adw.NewBreakpoint(adw.NewBreakpointConditionLength(
		adw.BreakpointConditionMaxWidth,
		275, adw.LengthUnitSp,
	))

	// Hide the size and icon on small screens.
	smallBreakpoint.AddSetter(u.size, "visible", false)
	smallBreakpoint.AddSetter(u.icon, "visible", false)

	u.bin = adw.NewBreakpointBin()
	u.bin.SetSizeRequest(100, 1)
	u.bin.AddBreakpoint(smallBreakpoint)
	u.bin.SetChild(u.box)

	u.Widgetter = u.bin

	t.Box.Append(u)
	t.files = append(t.files, u)

	u.delete.ConnectClicked(func() {
		t.files = slices.DeleteFunc(t.files, func(searching uploadFile) bool {
			if glib.ObjectEq(searching, u) {
				t.Box.Remove(u)
				return true
			}
			return false
		})
	})

	u.spoiler.ConnectClicked(func() {
		spoiler := !f.IsSpoiler()
		f.SetSpoiler(spoiler)

		u.name.SetText(f.Name)
		u.spoiler.SetActive(spoiler)
		u.spoiler.SetIconName(fileSpoilerIcon(u.file))
	})
}

func mimeIcon(mime string) string {
	if mime == "" {
		return "text-x-generic-symbolic"
	}

	switch strings.SplitN(mime, "/", 2)[0] {
	case "image":
		return "image-x-generic-symbolic"
	case "video":
		return "video-x-generic-symbolic"
	case "audio":
		return "audio-x-generic-symbolic"
	default:
		return "text-x-generic-symbolic"
	}
}

// Files returns the list of files in the tray.
func (t *UploadTray) Files() []*File {
	files := make([]*File, len(t.files))
	for i, file := range t.files {
		files[i] = file.file
	}
	return files
}

// Clear clears the tray and returns the list of paths that it held.
func (t *UploadTray) Clear() []*File {
	files := make([]*File, len(t.files))
	for i, file := range t.files {
		files[i] = file.file
		t.Remove(file)
	}
	t.files = nil
	return files
}
